package handlenodeadd

import (
	"context"
	"github.com/whildbear/kubernetes-cluster-autoscaler/pkg/cloud/openstack"
	"github.com/whildbear/kubernetes-cluster-autoscaler/pkg/common/datastructures"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/utils/openstack/imageservice/v2/images"
	"github.com/gophercloud/utils/openstack/compute/v2/flavors"
	"github.com/gophercloud/utils/openstack/networking/v2/networks"
	"github.com/gophercloud/utils/openstack/networking/v2/extensions/security/groups"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"
)

var (
	triggerLock    bool
	wg             sync.WaitGroup
	pendingPodList []string
)

// IsNeededPendingStatus Check whether the pod in in pending state
func IsNeededPendingStatus(status v1.PodCondition) bool {
	return strings.Contains(status.Message, "Insufficient") && (strings.Contains(status.Message, "cpu") || strings.Contains(status.Message, "memory")) && !strings.Contains(status.Message, "had taint {node.kubernetes.io/not-ready: }, that the pod didn't tolerate")
}

// ModifyEventAnalyzer Analyze the Kubernetes events to capture pending nodes
func ModifyEventAnalyzer(EventList datastructures.Event, config *rest.Config) {
	status := EventList.Object.Status.Conditions[0]
	if EventList.Object.Status.Phase == "Pending" && status.Type == "PodScheduled" && status.Status == "False" {
		if IsNeededPendingStatus(status) {
			log.Printf("[ERROR] %s - %s", status.Reason, status.Message)
			wg.Add(1)
			go TriggerStatusCheck(EventList.Object, config)
		}
	}

	if EventList.Object.Status.Phase == "Pending" && len(pendingPodList) >= 1 || pendingPodList != nil {
		PodStatus(EventList.Object)
	}
}

// TriggerStatusCheck Trigger adding a new Kubernetes worker node
func TriggerStatusCheck(pod v1.Pod, config *rest.Config) {
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Println(err)
	}

	node, _ := clientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	nodeCount := len(node.Items)

	if !triggerLock && nodeCount < openstackinit.MaxNodeCount {
		log.Println("[INFO] Node add trigger.")
		triggerLock = true
		if PendingPodListCheck(pod.Name) {
			pendingPodList = append(pendingPodList, pod.Name)
		}
		TriggerAddNode(GetOpenstackFlavor(pod))
	} else {
		if nodeCount == openstackinit.MaxNodeCount {
			log.Println("[INFO] Max node count reached")
		} else if PendingPodListCheck(pod.Name) {
			log.Println("[INFO] Node add triggerd. Waiting for new node")
			pendingPodList = append(pendingPodList, pod.Name)
		}
		wg.Done()
	}
}

// PodStatus Check the status of the pending pod which trigger new node adding process
func PodStatus(pod v1.Pod) {
	conditions := pod.Status.Conditions[0]

	if triggerLock {
		for i, pendingPodName := range pendingPodList {
			if pod.Name == pendingPodName && conditions.Type == "PodScheduled" && conditions.Status == "True" {
				log.Printf("[INFO] %s pod scheduled.", pendingPodName)
				if len(pendingPodList) == 1 {
					pendingPodList = nil
				} else {
					pendingPodList = append(pendingPodList[:i], pendingPodList[i+1:]...)
				}
				triggerLock = false
			}
		}
	}
}

// PendingPodListCheck Check for multiple node add triggers from the same pending pod
func PendingPodListCheck(podName string) bool {
	for _, pendingPodName := range pendingPodList {
		if pendingPodName == podName {
			return false
		}
	}
	return true
}

// GetOpenstackFlavor Select a flavor from the list of user definded flavors
func GetOpenstackFlavor(pod v1.Pod) string {
	defer PanicRecovery()
	var requestsCPU, requestsMemory int64
	var flavorFound bool
	index := -1

	for _, container := range pod.Spec.Containers {
		requestsCPU += container.Resources.Requests.Cpu().Value()
		requestsMemory += container.Resources.Requests.Memory().Value()
	}
	requestsMemory = requestsMemory / 1024 / 1000

	if requestsCPU != 0 || requestsMemory != 0 {
		for i, flavor := range openstackinit.FlavorsList.Flavor {
			if flavor.RequestsCPU >= requestsCPU && flavor.RequestsMemory >= requestsMemory {
				flavorFound = true
				index = i
				break
			}
		}
	}

	if index != -1 && flavorFound {
		log.Printf("[INFO] %s flavor profile selected", openstackinit.FlavorsList.Flavor[index].Name)
		return openstackinit.FlavorsList.Flavor[index].Name
	} else if requestsCPU != 0 && requestsMemory != 0 {
		panic("[ERROR] No flavor profile found")
	}

	log.Printf("[INFO] Default flavor profile %s selected", openstackinit.FlavorsList.FlavorDefault)
	return openstackinit.FlavorsList.FlavorDefault
}

// GetNodeName Generate a random name for the Kubernetes worker node
func GetNodeName() string {
	rand.Seed(time.Now().UnixNano())
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" + "abcdefghijklmnopqrstuvwxyz" + "0123456789")
	length := 4
	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	str := b.String()
	return openstackinit.PlatformPrefix+"kube-worker-" + str
}

// TriggerAddNode Create new OpenStack virtual machine
func TriggerAddNode(flavorName string) {
	defer PanicRecovery()
	client := openstackinit.GetOpenstackToken()
	client_neutron := openstackinit.GetOpenstackNeutronToken()

        imageId, err := images.IDFromName(client, openstackinit.ImageName)
	flavorID, err := flavors.IDFromName(client, flavorName)	 
	SecurityGroupId, err := groups.IDFromName(client_neutron, openstackinit.SecurityGroupName)
	NetworkUUID_a, err := networks.IDFromName(client_neutron, openstackinit.NetworkUUID_a)
	NetworkUUID_d, err := networks.IDFromName(client_neutron, openstackinit.NetworkUUID_d)
	NetworkUUID_p, err := networks.IDFromName(client_neutron, openstackinit.NetworkUUID_p)
	Node_Name := GetNodeName()
	
	userData := `#!/usr/bin/env bash
curl -L -s `+openstackinit.RepoBaseUrl+`/install.sh | sudo bash -s -- \
    -i init
`

	log.Printf("[INFO] Creating new node with config : Node_Name = %s, imageId = %s, flavorID = %s, SecurityGroupId = %s, NetworkUUID_a = %s, NetworkUUID_d = %s, NetworkUUID_p = %s", Node_Name, imageId, flavorID, SecurityGroupId, NetworkUUID_a, NetworkUUID_d, NetworkUUID_p )
	serverCreatOpts := servers.CreateOpts{
		Name:          Node_Name,
		FlavorRef:     flavorID,
		ImageRef:      imageId,
		SecurityGroups: []string{SecurityGroupId},
		Networks:       []servers.Network{{UUID: NetworkUUID_a}, {UUID: NetworkUUID_d}, {UUID: NetworkUUID_p}},
		UserData:       []byte(userData),
	}

	server, err := servers.Create(client, serverCreatOpts).Extract()
	if err != nil {
		panic(err)
	}
	log.Printf("[INFO] New node added. Node ID - %s", server.ID)
	NewNodeStatus(server.ID)
}

// NewNodeStatus Check the status of the new node
func NewNodeStatus(id string) {
	log.Println("[INFO] Checking node status")
	ready, err := NewNodeReady(id)
	if err != nil {
		log.Printf("Error creating the server %s", err)
	}
	if ready {
		log.Println("[INFO] Node is running.")
	}
	defer wg.Done()
}

// NewNodeReady Check the status of the new node loop
func NewNodeReady(id string) (bool, error) {
	client := openstackinit.GetOpenstackToken()

	for {
		server, err := servers.Get(client, id).Extract()
		if err != nil {
			return false, err
		}

		if server.Status == "ACTIVE" {
			return true, nil
		}
	}
}

// PanicRecovery handle panic
func PanicRecovery() {
	if r := recover(); r != nil {
		log.Println(r)
		triggerLock = false
	}
}
