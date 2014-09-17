## lbManager 

[![Docker Repository on Quay.io](https://quay.io/repository/tegioz/lbmanager/status "Docker Repository on Quay.io")](https://quay.io/repository/tegioz/lbmanager)

**A simple load balancers presence manager for docker containers on top of etcd** that supports **AWS ELB** and **AWS Route53** dns based load balancing. The concept is pretty similar to the one described in [CoreOS fleet example deployment](https://coreos.com/docs/launching-containers/launching/fleet-example-deployment/), but using a centralized approach that aims to be more flexible and solid without requiring an extra container *per container* to manage their presence in the load balancer. Using it is really simple, specially if you are deploying your Docker containers in a [CoreOS](https://coreos.com/using-coreos/) cluster.

## Quick start

### Deploy the lbManager container

You just need to edit `systemd/lbmanager.service` setting your AWS credentials and you are ready to deploy the lbManager service container. The service will pull the lbmanager image automatically on start, so you don't have to build it first to use it. If you were using CoreOS, it would be as simple as this:

	fleetctl start lbmanager.service
	
Managing your load balancers is quite simple, you just need to set/remove keys in etcd with a specific format. Some examples using `etcdctl`:

### Adding members to your load balancers

	AWS ELB load balancers
	etcdctl set /lbManager/elb/ap-southeast-2/loadBalancer1/multiple/i-00000001 ""
	
	AWS Route53 dns based load balancing
	etcdctl set /lbManager/route53/ap-southeast-2/Z12345678/www.mydomain.com/multiple/1.1.1.1 ""
	etcdctl set /lbManager/route53/ap-southeast-2/Z12345678/www.mydomain.com/multiple/2.2.2.2 ""

### Removing members from your load balancers

	AWS ELB load balancers
	etcdctl rm /lbManager/elb/ap-southeast-2/loadBalancer1/multiple/i-00000001
	
	AWS Route53 dns based load balancing
	etcdctl rm /lbManager/route53/ap-southeast-2/Z12345678/www.mydomain.com/multiple/1.1.1.1

You can mix both types in the same lbmanager instance and manage multiple load balancers of each type simultaneously. You will probably want to automate these operations, setting `ExecStartPre` and `ExecStop` entries in your services' units files (see full example below).

## Usage

###  Deploy the lbManager container

In the systemd folder of this repository you'll find a systemd unit file called `lbmanager.service`. You have to edit it adding a valid `AWS_ACCESS_KEY` and `AWS_SECRET_KEY` and you are ready to go (more about the AWS credentials below).

When you are done you'll have to deploy the lbmanager service. If you are using CoreOS, you just need to tell `fleet` to deploy the lbmanager service in the cluster using fleetctl:

	fleetctl start lbmanager.service
	
The service will pull the latest lbManager image from `Quay.io` on start, so unless you want to modify the code and build your own image, it's ready to work out of the box.

Then you can verify that everything is working as expected having a look at the service's logs using any of these commands:

	docker logs lbmanager
	fleetctl journal lbmanager
	
lbManager runs as a **supervised runit service** inside the container, so if for any reason it crashes it will be restarted automatically for you without causing any disruption. If you need to upgrade lbManager it's completely safe to stop it and deploy a new container using a new image. Once launched again it will read the full configuration from etcd and it will keep working as if nothing would have happened.

### AWS Credentials

The IAM user/role credentials used in the service must be able to perform the following actions on the resources you plan to manage using `lbManager`:

ELB

	elasticloadbalancing:DescribeLoadBalancers
	elasticloadbalancing:RegisterInstancesWithLoadBalancer
	elasticloadbalancing:DeregisterInstancesFromLoadBalancer

Route53

	route53:ChangeResourceRecordSets

###  lbManager configuration

lbManager uses the configuration in `etcd` as the **source of truth** to manage and sync load balancers. 

When it starts, it will read its configuration from `/lbManager` recursively, creating some internal data structures that will be used to keep load balancers' states in sync with the AWS services in a concurrent way. You can manage as many load balancers simultaneously as you want.

To add/remove members from a load balancer, you have to set/remove keys from etcd that follow a specific format using etcdctl, the etcd rest api or any other language specific wrapper around it. You can use any value, including an empty one, as the value for the key.

The format is as follows:

###### ELB

	/lbManager/elb/REGION/LB_NAME/LB_CLASS/INSTANCE_ID
	
	REGION = ap-southeast-2|us-east-1|... (valid AWS region)
	LB_NAME = AWS ELB name (must exist in AWS, lbManager won't create it)
	LB_CLASS = [single|multiple] (more about this below)
	INSTANCE_ID = AWS InstanceID where the container is running on
	
###### Route53

	/lbManager/route53/REGION/HOSTED_ZONE/FQDN/LB_CLASS/IP
	
	REGION = ap-southeast-2|us-east-1|... (valid AWS region, used for the API endpoint)
	HOSTED_ZONE = Route53 hosted zone id where your records will be set
	FQDN = Full qualified domain name to use in the record set
	LB_CLASS = [single|multiple] (more about this below)
	IP = Public IP address of the instance where the container is running
	
Check out the `Quick start` section above to see some keys in action as well as some examples of adding/removing members to/from a load balancer.

### Automating the addition/removal of members to/from the load balancer

Most of the time you'll want to automate the process of managing the load balancer's members. To do that, you can easily add `ExecStartPre` and `ExecStop` entries to your services units using something like this:

	[Unit]
	Description=WebServer
	After=docker.service
	Requires=docker.service

	[Service]
	User=core
	Environment="INSTANCE_ID=i-00000000"
	ExecStartPre=-/usr/bin/docker kill webserver
	ExecStartPre=-/usr/bin/docker rm webserver
	ExecStart=/usr/bin/docker run --name webserver -p 80:80 webserverdockerimage
	ExecStartPost=/usr/bin/etcdctl set /lbManager/elb/us-east-1/webLB/multiple/$INSTANCE_ID
	ExecStop=/usr/bin/etcdctl rm /lbManager/elb/us-east-1/webLB/multiple/$INSTANCE_ID
	ExecStop=/usr/bin/docker stop webserver

This way when your service starts it will announce its presence, being added to the load balancer automatically. In the same way, when it's stopped or destroyed, it will remove itself from the load balancer without requiring any kind of manual intervention. Getting the instance id dynamically from the instance metadata available locally through http://169.254.169.254/... may help to automate the whole process.

### Load balancer class (single/multiple)

Sometimes you may want to run a single instance behind a load balancer, maybe to offload SSL to it, or just to switch the backend server quickly without having to modify the dns records. In such cases, the `single` load balancer class may come handy.

When you set a load balancer member using the single class, lbManager will remove any other existing members of the load balancer, leaving only active the new one you just added. This can also be useful for replacing a running container with a new one (to upgrade it, for example) and switching the traffic to it without stopping/destroying the old one just in case we have to rollback (which could be quickly done just setting again the key of the old one manually).

The load balancer class is supported by ELB and Route53 based load balancers.

### Configuring lbManager before starting it

You don't have to populate the config tree before starting lbManager, as you can set/remove keys from etcd at any time and lbManager will react accordingly. However, if you want to use lbManager to manage load balancers that have already registered instances or dns entries used in production, it's better to do so.

lbManager delays sync operations till the whole config has been fully read initially, and after that it syncs after any update detected in the config. That means that you might see some instances or dns entries flapping in the load balancer for a few seconds if you add all entries one by one after lbManager has already started. If you add the necessary entries in the config representing what's setup in the real load balancers, lbManager will process the config before interacting with the load balancers, and during the sync process it will detect that everything is fine and no changes will be made. Sync operations in a given load balancer are serialized to avoid unexpected conflicts, although different sync operations in different load balancers will happen concurrently. In Route53, update operations are serialized per hosted zone, as the Route53 API doesn't allow more than one operation at a time in the same hosted zone to ensure consistency.

### Purging lbManager configuration from etcd

If for any reason you need to purge lbManager configuration from the etcd tree, you can use this simple curl command:

	curl -L http://127.0.0.1:4001/v2/keys/lbManager?recursive=true -XDELETE

### Building your own lbManager docker image

If you plan to use lbManager in production, it may be a good idea to build your own image, even if you don't plan to modify the source code. That way, if for any reason in a future version some non-backwards compatible changes are introduced or the image in the docker repository is just broken or not available, you won't be affected.

In the docker directory you'll find the Dockerfile used to build it:

	cd docker
	docker build -t lbmanager .
	
You'll need to update the lbmanager.service file if you use a tag different that quay.io/tegioz/lbmanager when building your own image.