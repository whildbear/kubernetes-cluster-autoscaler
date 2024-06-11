package main

import (
	"fmt"
	"github.com/whildbear/kubernetes-cluster-autoscaler/pkg/common/datastructures"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"io/ioutil"
	"log"
)

// ModifyEventAnalyzer Analyze the Kubernetes events to capture pending nodes
func ModifyEventAnalyzer(EventList datastructures.Event, ProjectName, ClientSecret, ClientID, AWSRegion, AuthFile string) {
	// Code to Analyzed and trigger NodeAdd function
	log.Println("This is a sample...")
}

// NodeAdd worker node
func NodeAdd(AWSRegion string) {
	awsSession, err := session.NewSession(&aws.Config{
		Region:      aws.String(AWSRegion),
	})

	if err != nil {
		log.Fatal(err)
	}

		service := ec2.New(awsSession)

		keyName := "goLangAPI"
		keyPairInput := ec2.CreateKeyPairInput{
			KeyName: aws.String(keyName),
		}
		keyPair, err := service.CreateKeyPair(&keyPairInput)
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "InvalidKeyPair.Duplicate" {
				log.Printf("[INFO] Keypair %s already exists.", keyName)
			}
		} else {
			fmt.Println(*keyPair.KeyFingerprint, "\n", *keyPair.KeyMaterial)

			privateKey := []byte(*keyPair.KeyMaterial)
			err = ioutil.WriteFile("/<path_to_location>/id_rsa", privateKey, 0400)
			if err != nil {
				log.Println(err)
			}
		}

		runInput := ec2.RunInstancesInput{
			ImageId: aws.String("ami-0b59bfac6be064b78"),
			InstanceType: aws.String("t2.micro"),
			MaxCount: aws.Int64(1),
			MinCount: aws.Int64(1),
			KeyName: aws.String(keyName),
		}
		runResult, err := service.RunInstances(&runInput)
		if err != nil {
			log.Printf("[ERROR] Error creating the Instance %v", err)
		}
		log.Println(*runResult.Instances[0].InstanceId, *runResult.Instances[0].PublicIpAddress)
}

// DeleteEventAnalyzer Analyze Kubernetes events and capture delete event
func DeleteEventAnalyzer(EventList datastructures.Event, ProjectName, ClientSecret, ClientID, AWSRegion, AuthFile string) {
	// Codes to Analyzed and trigger NodeDelete function
	log.Println("This is a sample...")
}

func NodeDelete() {
	// Code to delete the worker node and remove node from Kubernetes cluster
	log.Println("This is a sample...")
}
