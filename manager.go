package main

import (
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/route53"
	"regexp"
)

type LoadBalancer interface {
	AddMember(member string)
	RemoveMember(member string)
	SetClass(class string)
	Setup(metadata map[string]string)
	Sync()
}

type configEntry struct {
	action     string
	memberId   string
	lbType     string
	lbId       string
	lbMetadata map[string]string
}

type Manager struct {
	configPath       string
	etcdClient       *etcd.Client
	awsAuth          aws.Auth
	loadBalancers    map[string]LoadBalancer
	zonesUpdatersChs map[string]chan *route53.Change
}

func (m *Manager) Start() {
	m.loadBalancers = make(map[string]LoadBalancer)
	m.zonesUpdatersChs = make(map[string]chan *route53.Change)
	readConfigCh, readConfigDoneCh := m.readConfig()
	watchConfigCh := m.watchConfig()

	for {
		select {
		case configEntry := <-readConfigCh:
			m.processConfigEntry(configEntry)
		case response := <-watchConfigCh:
			if response == nil {
				watchConfigCh = m.watchConfig()
			} else {
				configEntry := m.processNodeKey(response.Node.Key, response.Action)
				if configEntry != nil {
					m.processConfigEntry(configEntry)
				}
			}
		case <-readConfigDoneCh:
			for _, lb := range m.loadBalancers {
				lb.Sync()
			}
		}
	}
}

// Read configuration from etcd
func (m *Manager) readConfig() (readConfigCh chan *configEntry, doneCh chan bool) {
	readConfigCh, doneCh = make(chan *configEntry), make(chan bool)
	go func() {
		response, err := m.etcdClient.Get(m.configPath, true, true)
		if err != nil {
			fmt.Println("Initial config not present. Monitoring changes on it from now on..")
		} else {
			action := "readingConfig"
			m.processNode(response.Node, action, readConfigCh)
			doneCh <- true
		}
	}()
	return
}

// Process config nodes recursively
func (m *Manager) processNode(node *etcd.Node, action string, readConfigCh chan *configEntry) {
	if configEntry := m.processNodeKey(node.Key, action); configEntry != nil {
		readConfigCh <- configEntry
	}
	for _, child := range node.Nodes {
		m.processNode(child, action, readConfigCh)
	}
}

// Check if this node's key is a config entry we might be interested in
func (m *Manager) processNodeKey(key string, action string) (entry *configEntry) {
	entry = nil
	elbRe, _ := regexp.Compile(m.configPath + "/elb/(.*)/(.*)/(.*)/(.*)")
	route53Re, _ := regexp.Compile(m.configPath + "/route53/(.*)/(.*)/(.*)/(.*)/(.*)")
	regexps := map[string]*regexp.Regexp{
		"elb":     elbRe,
		"route53": route53Re,
	}
	for lbType, re := range regexps {
		if r := re.FindStringSubmatch(key); len(r) > 0 {
			entry = &configEntry{
				action:     action,
				lbType:     lbType,
				lbMetadata: map[string]string{"region": r[1]},
			}
			switch lbType {
			case "elb":
				name, class, instance := r[2], r[3], r[4]
				entry.lbId = lbType + "_" + entry.lbMetadata["region"] + "_" + name
				entry.memberId = instance
				entry.lbMetadata["class"] = class
				entry.lbMetadata["name"] = name
			case "route53":
				hostedZone, fqdn, class, ip := r[2], r[3], r[4], r[5]
				entry.lbId = lbType + "_" + hostedZone + "_" + fqdn
				entry.memberId = ip
				entry.lbMetadata["class"] = class
				entry.lbMetadata["name"] = fqdn
				entry.lbMetadata["hostedZone"] = hostedZone
			}
		}
	}
	return
}

// Watch etcd for changes in configuration tree
func (m *Manager) watchConfig() (watchConfigCh chan *etcd.Response) {
	watchConfigCh = make(chan *etcd.Response)
	go func() {
		m.etcdClient.Watch(m.configPath, 0, true, watchConfigCh, nil)
	}()
	return
}

// Get a lb from the load balancers registry, creating a new one if it doesn't exist
func (m *Manager) getLoadBalancer(configEntry *configEntry) (lb LoadBalancer) {
	var exists bool
	if lb, exists = m.loadBalancers[configEntry.lbId]; !exists {
		lbConfig := LB{
			AwsAuth:    m.awsAuth,
			ConfigPath: m.configPath,
			EtcdClient: m.etcdClient,
			Id:         configEntry.lbId,
			Type:       configEntry.lbType,
		}
		switch configEntry.lbType {
		case "elb":
			lb = &Elb{LB: lbConfig}
		case "route53":
			zoneUpdaterCh := m.getZoneUpdaterCh(configEntry.lbMetadata["hostedZone"], configEntry.lbMetadata["region"])
			lb = &Route53{
				LB:            lbConfig,
				ZoneUpdaterCh: zoneUpdaterCh,
			}
		}
		lb.Setup(configEntry.lbMetadata)
		m.loadBalancers[configEntry.lbId] = lb
	}
	return
}

// Process configuration entry received, triggering necessary actions in the load balancer affected
func (m *Manager) processConfigEntry(configEntry *configEntry) {
	lb := m.getLoadBalancer(configEntry)
	lb.SetClass(configEntry.lbMetadata["class"])
	switch configEntry.action {
	case "readingConfig":
		lb.AddMember(configEntry.memberId)
	case "set":
		lb.AddMember(configEntry.memberId)
		lb.Sync()
	case "delete":
		lb.RemoveMember(configEntry.memberId)
		lb.Sync()
	}
}

// Get the zone updater channel given a hostedZoneId, creating a new zone updater and a new channel if needed
func (m *Manager) getZoneUpdaterCh(hostedZoneId string, region string) (zoneUpdaterCh chan *route53.Change) {
	var exists bool
	if zoneUpdaterCh, exists = m.zonesUpdatersChs[hostedZoneId]; !exists {
		fmt.Printf("-> ZONEUPDATER:%s:settingUpZoneUpdater\n", hostedZoneId)
		zoneUpdaterCh = make(chan *route53.Change)
		zoneUpdater := &ZoneUpdater{
			AwsClient:  route53.New(m.awsAuth, aws.Regions[region]),
			HostedZone: hostedZoneId,
			UpdatesCh:  zoneUpdaterCh,
		}
		go func() {
			zoneUpdater.listen()
		}()
	}
	m.zonesUpdatersChs[hostedZoneId] = zoneUpdaterCh
	return
}
