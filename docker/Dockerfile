FROM phusion/baseimage:0.9.13
MAINTAINER Sergio Castano Arteaga <sergio.castano.arteaga@gmail.com>

# Install dependencies
RUN apt-get update && \
    apt-get dist-upgrade -y && \
    apt-get install -y \
    	git \
    	golang

# Disable SSH access
RUN rm -rf /etc/service/sshd /etc/my_init.d/00_regen_ssh_host_keys.sh

# Build lbManager
ENV GOPATH /go
RUN go get github.com/tegioz/lbManager

# Setup lbManager runit service
RUN mkdir -p /etc/service/lbManager
ADD service_lbManager.sh /etc/service/lbManager/run
RUN chmod +x /etc/service/lbManager/run

# Use baseimage-docker init system
CMD ["/sbin/my_init", "--quiet"]