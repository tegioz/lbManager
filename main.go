package main

import (
	"flag"
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"github.com/mitchellh/goamz/aws"
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
	flag.StringVar(&config.awsAccessKey, "aws-access-key", "YOUR_AWS_ACCESS_KEY", "AWS access key")
	flag.StringVar(&config.awsSecretKey, "aws-secret-key", "YOUR_AWS_SECRET_KEY", "AWS secret key")
}

func main() {
	flag.Parse()

	awsAuth, err := aws.GetAuth(config.awsAccessKey, config.awsSecretKey)
	if err != nil {
		fmt.Println(err)
	}

	manager := &Manager{
		configPath: config.etcdPath,
		etcdClient: etcd.NewClient([]string{config.etcdHost}),
		awsAuth:    awsAuth,
	}
	fmt.Println("Running load balancers manager...")
	manager.Start()
}
