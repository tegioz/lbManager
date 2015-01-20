package main

import (
	"github.com/coreos/go-etcd/etcd"
	"github.com/mitchellh/goamz/aws"
	"log"
	"regexp"
	"strings"
)

type LB struct {
	AwsAuth    aws.Auth
	ConfigPath string
	EtcdClient *etcd.Client
	Id         string
	Type       string
	class      string
	configKey  string
	members    []string
	name       string
	region     string
}

// Add a member to the load balancer state
func (lb *LB) AddMember(member string) {
	if lb.class == "single" {
		log.Printf("-> %s:%s:setSingleMember:%s\n", strings.ToUpper(lb.Type), lb.name, member)
		if lb.isLatestAdded(member) {
			lb.members = []string{member}
			lb.removeInvalidMembersFromConfig(member)
		}
	} else {
		log.Printf("-> %s:%s:addMember:%s\n", strings.ToUpper(lb.Type), lb.name, member)
		if p := lb.memberPosition(member); p == -1 {
			lb.members = append(lb.members, member)
		}
	}
}

// Remove a member from the load balancer state
func (lb *LB) RemoveMember(member string) {
	log.Printf("<- %s:%s:removeMember:%s\n", strings.ToUpper(lb.Type), lb.name, member)
	if p := lb.memberPosition(member); p > -1 {
		lb.members = append(lb.members[:p], lb.members[p+1:]...)
	}
}

// Set load balancer's class (single/multiple) -based on the last class seen in a config entry-
func (lb *LB) SetClass(newClass string) {
	if lb.class != newClass {
		lb.class = newClass
		switch lb.class {
		case "single":
			lb.EtcdClient.Delete(lb.configKey+"multiple", true)
		case "multiple":
			lb.EtcdClient.Delete(lb.configKey+"single", true)
		}
		lb.members = []string{}
		log.Printf("-> %s:%s:lbClassUpdatedTo:%s:resettingMembers:%s\n", strings.ToUpper(lb.Type), lb.name, lb.class, lb.members)
	}
}

// Checks if the provided member is the latest addition to the load balancer
func (lb *LB) isLatestAdded(member string) bool {
	if lastAddition := lb.findLastAddition(); lastAddition == member {
		return true
	}
	return false
}

// Find latest added member to the load balancer
func (lb *LB) findLastAddition() (lastAddition string) {
	lastAddition = "not_found"
	var lastSeenIndex uint64 = 0
	memberRe, _ := regexp.Compile(lb.configKey + lb.class + "/(.*)")
	response, _ := lb.EtcdClient.Get(lb.configKey+lb.class, false, false)
	if response != nil {
		for _, child := range response.Node.Nodes {
			if child.ModifiedIndex > lastSeenIndex {
				result := memberRe.FindStringSubmatch(child.Key)
				if len(result) > 0 {
					lastSeenIndex = child.ModifiedIndex
					lastAddition = result[1]
				}
			}
		}
	}
	return
}

// Remove invalid members from the load balancer configuration in etcd
func (lb *LB) removeInvalidMembersFromConfig(validMember string) {
	response, _ := lb.EtcdClient.Get(lb.configKey+lb.class, false, false)
	for _, child := range response.Node.Nodes {
		if !strings.HasSuffix(child.Key, validMember) {
			_, err := lb.EtcdClient.Delete(child.Key, false)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

// Find a member's position in the list of members
func (lb *LB) memberPosition(member string) int {
	for p, v := range lb.members {
		if v == member {
			return p
		}
	}
	return -1
}

// Check if member exists in the load balancer state
func (lb *LB) memberExists(member string, members []string) bool {
	for _, v := range members {
		if v == member {
			return true
		}
	}
	return false
}
