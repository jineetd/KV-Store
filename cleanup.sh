#!/bin/bash
 
# Delete the cluster
kind delete cluster
 
# Delete all containers
docker rm -f $(docker ps -a -q)
 
# Delete all images
docker rmi -f $(docker images -q)

# Remove all protobuf files.
rm -rf protos/*go*
rm -rf protos/*py*
rm $(pwd)/kv_client/kv_store_interface_pb2.py
rm $(pwd)/kv_client/kv_store_interface_pb2_grpc.py

# Remove all binaries.
rm control-manager worker