## lbManager 

[![Docker Repository on Quay.io](https://quay.io/repository/tegioz/lbmanager/status "Docker Repository on Quay.io")](https://quay.io/repository/tegioz/lbmanager)

**A simple load balancers presence manager for docker containers on top of etcd**. Currently supports AWS ELB load balancers (AWS Route53 dns based load balancing support coming soon!). The concept is pretty similar to the one described in [CoreOS fleet example deployment](https://coreos.com/docs/launching-containers/launching/fleet-example-deployment/), but using a centralized approach that aims to be more flexible and solid without requiring an extra container *per container* to manage their presence in the load balancer. Using it is really simple, specially if you are deploying your Docker containers in a `CoreOS` cluster.

### Quick start

###### Deploy the lbManager container

Edit `systemd/lbmanager.service` setting your AWS credentials and you are ready to deploy the lbManager service container. If you were using CoreOS, it would be as simple as this:

	fleetctl start lbmanager.service

###### Start adding/removing members to/from your load balancers

	etcdctl set /lbManager/elb/ap-southeast-2/loadBalancer1/multiple/i-00000001 ""
	etcdctl rm /lbManager/elb/ap-southeast-2/loadBalancer1/multiple/i-00000001

You will probably want to automate these operations, setting `ExecStartPre` and `ExecStopPost` entries in your services' units files (see full example below).

### Usage

####  Deploy the lbManager container

In the systemd folder of this repository you'll find a systemd unit file called `lbmanager.service`. You have to edit it adding a valid `AWS_ACCESS_KEY` and `AWS_SECRET_KEY` and you are ready to go.

When you are done you'll have to deploy the lbmanager service. If you are using CoreOS, you just need to tell `fleet` to deploy the lbmanager service in the cluster using fleetctl:

	fleetctl start lbmanager.service
	
The service will pull the latest lbManager image from `Quay.io` on start, so unless you want to modify the code and build your own image, it's ready to work out of the box.

Then you can verify that everything is working as expected having a look at the service's logs:

	fleetctl journal -f lbmanager
	
lbManager runs as a supervised runit service inside the container, so if for any reason it crashes it will be restarted automatically for you without causing any disruption.

*NOTE: AWS credentials - the IAM user/role credentials used in the service must be able to describe load balancers and register/deregister instances to/from them.*

####  lbManager configuration

lbManager uses the configuration in `etcd` as the **source of truth** to manage and sync load balancers. 

When it starts, it will read its configuration from `/lbManager` recursively, creating some internal data structures that will be used to keep load balancers' states in sync with the AWS services in a concurrent way. You can manage as many load balancers simultaneously as you want.

To add/remove members from a load balancer, we have to set/remove keys from etcd that follow a specific format using etcdctl, the etcd rest api or any other language specific wrapper around it.

The format is as follows:

`/lbManager/LB_TYPE/LB_REGION/LB_NAME/LB_CLASS/MEMBER_ID`
	
	LB_TYPE = [elb] (route53 coming soon!)
	LB_REGION = [ap-southeast-2|us-east-1|...] (valid AWS region)
	LB_NAME = AWS ELB name (must exist in AWS, lbManager won't create it)
	LB_CLASS = [single|multiple] (more about this below)
	MEMBER_ID = AWS InstanceID where the container is running
	
Let's see now how easy is to add/remove a new instance from an AWS ELB load balancer.

###### Add members to a load balancer

	etcdctl set /lbManager/elb/ap-southeast-2/loadBalancer1/multiple/i-00000001 ""
	etcdctl set /lbManager/elb/ap-southeast-2/loadBalancer1/multiple/i-00000002 ""
	
###### Remove a member to a load balancer

	etcdctl rm /lbManager/elb/ap-southeast-2/loadBalancer1/multiple/i-00000001

###### Automating the addition/removal of members to/from the load balancer

Most of the time you'd want to automate the process of managing the load balancer's members. To do that, you can easily add `ExecStartPre` and `ExecStopPost` entries to your services units using something like this:

	[Unit]
	Description=WebServer
	After=docker.service
	Requires=docker.service

	[Service]
	User=core
	ExecStartPre=-/usr/bin/docker kill webserver
	ExecStartPre=-/usr/bin/docker rm webserver
	ExecStart=/usr/bin/docker run --name webserver -p 80:80 webserverdockerimage
	ExecStartPost=/bin/sh -c 'INSTANCE_ID=$(wget -q -O - http://169.254.169.254/latest/meta-data/instance-id); /usr/bin/etcdctl set /lbManager/elb/us-east-1/webserversLB/multiple/$INSTANCE_ID'
	ExecStop=/usr/bin/docker stop webserver
	ExecStopPost=/bin/sh -c 'INSTANCE_ID=$(wget -q -O - http://169.254.169.254/latest/meta-data/instance-id); /usr/bin/etcdctl rm /lbManager/elb/us-east-1/webserversLB/multiple/$INSTANCE_ID'
	
This way when your service starts it will announce its presence, being added to the load balancer automatically. In the same way, when it's stopped or destroyed, it will remove itself from the load balancer without requiring any kind of manual intervention.

###### Load balancer class (single/multiple)

Sometimes you may want to run a single instance behind a load balancer, maybe to offload SSL to it, or just to switch the backend server quickly without having to modify the dns records. In such cases, the `single` load balancer class may come handy.

When you set a load balancer member using the single class, lbManager will remove any other existing members of the load balancer, leaving only active the new one you just added. This can also be useful for replacing a running container with a new one (to upgrade it, for example) and switching the traffic to it without stopping/destroying the old one just in case we have to rollback (which could be quickly done just setting again the key of the old one manually).

###### Configuring lbManager before starting it

You don't have to populate the config tree before starting lbManager, as you can set/remove keys from etcd at any time and lbManager will react accordingly. However, if you want to use lbManager to manage load balancers that have already registered instances in production, it's better to do so. 

lbManager delays sync operations till the whole config has been fully read initially, and after that it syncs after any update detected in the config. That means that you might see some instances flapping in the load balancer for a few seconds if you add entries one by one after lbManager has started. If you add the necessary entries in the config representing what's setup in the real load balancers, lbManager will process the config before interacting with the load balancers, and during the sync process it will detect that everything is fine and no changes will be made. Sync operations in a given load balancer are serialized to avoid unexpected conflicts, although different sync operations in different load balancers will happen concurrently.

###### Purging lbManager configuration from etcd

	curl -L http://127.0.0.1:4001/v2/keys/lbManager?recursive=true -XDELETE

### TODO

- Support AWS Route53 dns based load balancers

