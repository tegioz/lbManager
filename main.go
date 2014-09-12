package main

import (
	"flag"
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"github.com/mitchellh/goamz/aws"
)

var etcdHost string
var configPath string
var awsAccessKey string
var awsSecretKey string

func init() {
	flag.StringVar(&etcdHost, "etcd-host", "http://localhost:4001", "Etcd service address")
	flag.StringVar(&configPath, "config-path", "/lbManager", "Configuration path")
	flag.StringVar(&awsAccessKey, "aws-access-key", "YOUR_AWS_ACCESS_KEY", "AWS access key")
	flag.StringVar(&awsSecretKey, "aws-secret-key", "YOUR_AWS_SECRET_KEY", "AWS secret key")
}

func main() {
	flag.Parse()

	awsAuth, err := aws.GetAuth(awsAccessKey, awsSecretKey)
	if err != nil {
		fmt.Println(err)
	}

	manager := &Manager{
		configPath: configPath,
		etcdClient: etcd.NewClient([]string{etcdHost}),
		awsAuth:    awsAuth,
	}
	fmt.Println("Running load balancers manager...")
	manager.Start()
}
