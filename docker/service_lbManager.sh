#!/bin/sh

exec /go/bin/lbManager -etcd-host=$ETCD_HOST -aws-access-key=$AWS_ACCESS_KEY -aws-secret-key=$AWS_SECRET_KEY