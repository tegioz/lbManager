package main

import (
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/elb"
	"log"
)

type Elb struct {
	LB
	awsClient *elb.ELB
	syncCh    chan int
}

// Setup ELB based load balancer
func (lb *Elb) Setup(meta map[string]string) {
	log.Printf("-> ELB:%s:settingUpLoadBalancerState\n", meta["name"])
	lb.awsClient = elb.New(lb.AwsAuth, aws.Regions[meta["region"]])
	lb.class = meta["class"]
	lb.configKey = lb.ConfigPath + "/elb/" + meta["region"] + "/" + meta["name"] + "/"
	lb.name = meta["name"]
	lb.region = meta["region"]
	lb.syncCh = make(chan int)
	go func() {
		lb.sync()
	}()
}

// Sync state of the load balancer instance with the real service
func (lb *Elb) Sync() {
	lb.syncCh <- 1
}

// Add an instance to the AWS ELB
func (lb *Elb) addInstanceToAwsElb(instance string) {
	log.Printf("-> ELB:%s:addInstanceToAwsElb:%s\n", lb.name, instance)
	options := elb.RegisterInstancesWithLoadBalancer{
		LoadBalancerName: lb.name,
		Instances:        []string{instance},
	}
	_, err := lb.awsClient.RegisterInstancesWithLoadBalancer(&options)
	if err != nil {
		log.Println(err)
	}
}

// Get instances in AWS ELB
func (lb *Elb) getInstancesInAwsElb() (instances []string, err error) {
	instances = []string{}
	options := elb.DescribeLoadBalancer{
		Names: []string{lb.name},
	}
	resp, err := lb.awsClient.DescribeLoadBalancers(&options)
	if err == nil {
		for _, instance := range resp.LoadBalancers[0].Instances {
			instances = append(instances, instance.InstanceId)
		}
		log.Printf("-- ELB:%s:instancesInAws:%s\n", lb.name, instances)
	}
	return
}

// Remove an instance from the AWS ELB
func (lb *Elb) removeInstanceFromAwsElb(instance string) {
	log.Printf("<- ELB:%s:removeInstanceFromAwsElb:%s\n", lb.name, instance)
	options := elb.DeregisterInstancesFromLoadBalancer{
		LoadBalancerName: lb.name,
		Instances:        []string{instance},
	}
	_, err := lb.awsClient.DeregisterInstancesFromLoadBalancer(&options)
	if err != nil {
		log.Println(err)
	}
}

// Sync state of the load balancer instance with the real service
func (lb *Elb) sync() {
	for _ = range lb.syncCh {
		log.Printf("-- ELB:%s:syncing:%s\n", lb.name, lb.members)
		instancesInAwsElb, err := lb.getInstancesInAwsElb()
		if err != nil {
			log.Println(err)
			continue
		}
		for _, instance := range instancesInAwsElb {
			if !lb.memberExists(instance, lb.members) {
				lb.removeInstanceFromAwsElb(instance)
			}
		}
		for _, instance := range lb.members {
			if !lb.memberExists(instance, instancesInAwsElb) {
				lb.addInstanceToAwsElb(instance)
			}
		}
	}
}
