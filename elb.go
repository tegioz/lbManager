package main

import (
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/elb"
	"regexp"
	"strings"
)

type Elb struct {
	Id         string
	AwsAuth    aws.Auth
	EtcdClient *etcd.Client
	ConfigPath string
	awsClient  *elb.ELB
	name       string
	region     string
	class      string
	instances  []string
	syncCh     chan int
	configKey  string
}

// Add an instance to the load balancer state
func (lb *Elb) AddMember(instance string) {
	if lb.class == "single" {
		fmt.Printf("-> ELB:%s(%s):setLoadBalancerSingleInstance:%s\n", lb.name, lb.region, instance)
		if lb.isLatestAdded(instance) {
			lb.instances = []string{instance}
			lb.removeInvalidInstancesFromConfig(instance)
		}
	} else {
		fmt.Printf("-> ELB:%s(%s):addInstance:%s\n", lb.name, lb.region, instance)
		if p := pos(instance, lb.instances); p == -1 {
			lb.instances = append(lb.instances, instance)
		}
	}
}

// Remove an instance from the load balancer state
func (lb *Elb) RemoveMember(instance string) {
	fmt.Printf("<- ELB:%s(%s):removeInstance:%s\n", lb.name, lb.region, instance)
	if p := pos(instance, lb.instances); p > -1 {
		lb.instances = append(lb.instances[:p], lb.instances[p+1:]...)
	}
}

// Set load balancer's class (single/multiple) -based on the last class seen in a config entry-
func (lb *Elb) SetClass(newClass string) {
	if lb.class != newClass {
		lb.class = newClass
	}
}

// Setup ELB based load balancer
func (lb *Elb) Setup(metadata map[string]string) {
	fmt.Printf("-> ELB:%s(%s):settingUpElb\n", metadata["name"], metadata["region"])
	lb.awsClient = elb.New(lb.AwsAuth, aws.Regions[metadata["region"]])
	lb.name = metadata["name"]
	lb.region = metadata["region"]
	lb.class = metadata["class"]
	lb.syncCh = make(chan int)
	lb.configKey = lb.ConfigPath + "/elb/" + lb.region + "/" + lb.name + "/" + lb.class
	go func() {
		lb.sync()
	}()
}

// Synchronize load balancer's state with the real service
func (lb *Elb) Sync() {
	lb.syncCh <- 1
}

// Add an instance to the AWS ELB
func (lb *Elb) addInstanceToAwsElb(instance string) {
	fmt.Printf("-> ELB:%s(%s):addInstanceToAwsElb:%s\n", lb.name, lb.region, instance)
	options := elb.RegisterInstancesWithLoadBalancer{
		LoadBalancerName: lb.name,
		Instances:        []string{instance},
	}
	_, err := lb.awsClient.RegisterInstancesWithLoadBalancer(&options)
	if err != nil {
		fmt.Println(err)
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
		fmt.Printf("-- ELB:%s(%s):instancesInAws:%s\n", lb.name, lb.region, instances)
	}
	return
}

// Checks if the provided instance is the latest addition to the load balancer
func (lb *Elb) isLatestAdded(instance string) bool {
	if lastAddition := lb.findLastAddition(); lastAddition == instance {
		return true
	}
	return false
}

// Find latest added instance to the load balancer
func (lb *Elb) findLastAddition() (lastAddition string) {
	lastAddition = "not_found"
	var lastSeenIndex uint64 = 0
	instanceRe, _ := regexp.Compile(lb.configKey + "/(.*)")
	response, _ := lb.EtcdClient.Get(lb.configKey, false, false)
	for _, child := range response.Node.Nodes {
		if child.ModifiedIndex > lastSeenIndex {
			result := instanceRe.FindStringSubmatch(child.Key)
			if len(result) > 0 {
				lastSeenIndex = child.ModifiedIndex
				lastAddition = result[1]
			}
		}
	}
	return
}

// Remove an instance from the AWS ELB
func (lb *Elb) removeInstanceFromAwsElb(instance string) {
	fmt.Printf("<- ELB:%s(%s):removeInstanceFromAwsElb:%s\n", lb.name, lb.region, instance)
	options := elb.DeregisterInstancesFromLoadBalancer{
		LoadBalancerName: lb.name,
		Instances:        []string{instance},
	}
	_, err := lb.awsClient.DeregisterInstancesFromLoadBalancer(&options)
	if err != nil {
		fmt.Println(err)
	}
}

// Remove invalid entries from the load balancer configuration in etcd
func (lb *Elb) removeInvalidInstancesFromConfig(validInstance string) {
	response, _ := lb.EtcdClient.Get(lb.configKey, false, false)
	for _, child := range response.Node.Nodes {
		if !strings.HasSuffix(child.Key, validInstance) {
			_, err := lb.EtcdClient.Delete(child.Key, false)
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}

// Sync state of the load balancer instance with its status in AWS
func (lb *Elb) sync() {
	for _ = range lb.syncCh {
		fmt.Printf("-- ELB:%s(%s):syncing:%s\n", lb.name, lb.region, lb.instances)
		instancesInAwsElb, err := lb.getInstancesInAwsElb()
		if err != nil {
			fmt.Println(err)
			continue
		}
		for _, instance := range instancesInAwsElb {
			if !exists(instance, lb.instances) {
				lb.removeInstanceFromAwsElb(instance)
			}
		}
		for _, instance := range lb.instances {
			if !exists(instance, instancesInAwsElb) {
				lb.addInstanceToAwsElb(instance)
			}
		}
	}
}
