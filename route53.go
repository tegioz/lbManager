package main

import (
	"github.com/mitchellh/goamz/route53"
	"log"
)

type Route53 struct {
	LB
	ZoneUpdaterCh chan *route53.Change
	hostedZone    string
}

// Setup Route53 dns based load balancer
func (lb *Route53) Setup(meta map[string]string) {
	log.Printf("-> ROUTE53:%s:%s:settingUpLoadBalancerState\n", meta["hostedZone"], meta["name"])
	lb.class = meta["class"]
	lb.configKey = lb.ConfigPath + "/route53/" + meta["region"] + "/" + meta["hostedZone"] + "/" + meta["name"] + "/"
	lb.hostedZone = meta["hostedZone"]
	lb.name = meta["name"]
	lb.region = meta["region"]
}

// Sync state of the load balancer instance with the real service
func (lb *Route53) Sync() {
	log.Printf("-- ROUTE53:%s:syncing:%s\n", lb.name, lb.members)
	lb.ZoneUpdaterCh <- lb.getRecordSet()
}

// Generate a record set change that represents current load balancer's state
func (lb *Route53) getRecordSet() *route53.Change {
	return &route53.Change{
		Action: "UPSERT",
		Record: route53.ResourceRecordSet{
			Name:    lb.name,
			Type:    "A",
			TTL:     60,
			Records: lb.members,
		},
	}
}
