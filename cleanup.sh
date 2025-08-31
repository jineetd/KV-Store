#!/bin/bash
 
# Delete the cluster
kind delete cluster
 
# Delete all containers
docker rm -f $(docker ps -a -q)
 
# Delete all images
docker rmi -f $(docker images -q)