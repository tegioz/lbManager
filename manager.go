package main

import (
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"github.com/mitchellh/goamz/aws"
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
	configPath    string
	etcdClient    *etcd.Client
	awsAuth       aws.Auth
	loadBalancers map[string]LoadBalancer
}

func (m *Manager) Start() {
	m.loadBalancers = make(map[string]LoadBalancer)
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
func (m *Manager) processNodeKey(key string, action string) *configEntry {
	configEntryRe, _ := regexp.Compile(m.configPath + "/(elb|route53)/(.*)/(.*)/(.*)/(.*)")
	result := configEntryRe.FindStringSubmatch(key)
	if len(result) > 0 {
		return &configEntry{
			action:   action,
			memberId: result[5],
			lbType:   result[1],
			lbId:     result[1] + "_" + result[2] + "_" + result[3],
			lbMetadata: map[string]string{
				"name":   result[3],
				"region": result[2],
				"class":  result[4],
			},
		}
	}
	return nil
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
		switch configEntry.lbType {
		case "elb":
			lb = &Elb{
				Id:         configEntry.lbId,
				AwsAuth:    m.awsAuth,
				EtcdClient: m.etcdClient,
				ConfigPath: m.configPath,
			}
		case "route53":
			// TODO!
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
