package main

import (
	"flag"
	"github.com/coreos/go-etcd/etcd"
	"github.com/mitchellh/goamz/aws"
	"log"
)

var config struct {
	etcdHost     string
	etcdPath     string
	awsAccessKey string
	awsSecretKey string
}

func init() {
	flag.StringVar(&config.etcdHost, "etcd-host", "http://localhost:4001", "Etcd service address")
	flag.StringVar(&config.etcdPath, "config-path", "/lbManager", "Configuration path")
	flag.StringVar(&config.awsAccessKey, "aws-access-key", "", "AWS access key")
	flag.StringVar(&config.awsSecretKey, "aws-secret-key", "", "AWS secret key")
}

func main() {
	flag.Parse()

	awsAuth, err := aws.GetAuth(config.awsAccessKey, config.awsSecretKey)
	if err != nil {
		log.Println(err)
	}

	manager := &Manager{
		configPath: config.etcdPath,
		etcdClient: etcd.NewClient([]string{config.etcdHost}),
		awsAuth:    awsAuth,
	}
	log.Println("Running load balancers manager...")
	manager.Start()
}
