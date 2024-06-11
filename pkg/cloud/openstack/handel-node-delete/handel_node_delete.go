package handelnodedelete

import (
	"context"
	"github.com/whildbear/kubernetes-cluster-autoscaler/pkg/cloud/openstack"
	"github.com/whildbear/kubernetes-cluster-autoscaler/pkg/common/datastructures"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"strconv"
	"time"
	"strings"
)

// DeleteEventAnalyzer Analyze Kubernetes events and capture delete event
func DeleteEventAnalyzer(EventList datastructures.Event, config *rest.Config) {
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Println(err)
	}

	if !openstackinit.IgnoreNamespaceList[EventList.Object.Namespace] {
		node, _ := clientSet.CoreV1().Nodes().Get(context.TODO(), EventList.Object.Spec.NodeName, metav1.GetOptions{})
		cpuCap, _ := strconv.ParseFloat(node.Status.Capacity.Cpu().String(), 64)
		//node.Status.Capacity.Memory().Value()

		options := metav1.ListOptions{FieldSelector: "spec.nodeName=" + EventList.Object.Spec.NodeName}
		podList, _ := clientSet.CoreV1().Pods("").List(context.TODO(), options)

		var cpu float64
		var mem int64
		count := 0
		for _, pod := range podList.Items {
			if pod.Status.Phase == "Running" {
				count++
				var requestsCPU float64
				var requestsMemory int64

				for _, container := range pod.Spec.Containers {
					cpuVal, _ := strconv.ParseFloat(container.Resources.Requests.Cpu().AsDec().String(), 64)
					requestsCPU += cpuVal
					requestsMemory += container.Resources.Requests.Memory().Value()
				}
				cpu += requestsCPU
				mem += requestsMemory
			}
		}

		nodeList, _ := clientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		nodeCount := len(nodeList.Items)

		if (cpu/cpuCap)*100 <= 5 && count < 5 && openstackinit.MinNodeCount < nodeCount && strings.HasPrefix(node.Name, openstackinit.PlatformPrefix+"kube-worker-") {
			log.Printf("[INFO] Node Name - %s ID - %s marked to delete. Will delete in %d minutes", node.Name, node.Status.NodeInfo.SystemUUID, openstackinit.CoolDownTime/60)
			go RemoveWorkerNode(clientSet, node.Name, node.Status.NodeInfo.SystemUUID)
		} else {
			log.Printf("[INFO] Node Name - %s ID - %s NOT marked to delete. nodeCount = %d count = %d cpu = %f cpuCap = %f ", node.Name, node.Status.NodeInfo.SystemUUID, nodeCount, count, cpu, cpuCap )

		}
	}
}

// RemoveWorkerNode Check and remove the worker ndoe from the Kubernetes cluster
func RemoveWorkerNode(clientSet *kubernetes.Clientset, nodeName, nodeID string) {
	time.Sleep(openstackinit.CoolDownTime * time.Second)

	node, _ := clientSet.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	cpuCap, _ := strconv.ParseFloat(node.Status.Capacity.Cpu().String(), 64)

	options := metav1.ListOptions{FieldSelector: "spec.nodeName=" + nodeName}
	podList, _ := clientSet.CoreV1().Pods("").List(context.TODO(), options)

	var cpu float64
	count := 0
	for _, pod := range podList.Items {
		if pod.Status.Phase == "Running" {
			count++
			var requestsCPU float64

			for _, container := range pod.Spec.Containers {
				cpuVal, _ := strconv.ParseFloat(container.Resources.Requests.Cpu().AsDec().String(), 64)
				requestsCPU += cpuVal
			}
			cpu += requestsCPU
		}
	}

	if (cpu/cpuCap)*100 > 5 || count > 5 {
		log.Printf("[INFO] Some pod are assing to the %s node. Stop removing the node", nodeName)
	} else {
		clientSet.CoreV1().Nodes().Delete(context.TODO(), nodeName, metav1.DeleteOptions{})
		DeleteVM(nodeID)
		log.Printf("[INFO] %s (%s) Node safly remove from the cluster and delete the virtual machine", nodeName, nodeID)
	}
}

// DeleteVM delete the virtual machine from the OpenStack
func DeleteVM(id string) {
	client := openstackinit.GetOpenstackToken()
	servers.Delete(client, id)
}
