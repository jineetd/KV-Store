#!/bin/bash

# Install all the required dependencies on mac.
brew install kind
brew install helm
brew install protobuf

# Install the helm charts.
helm repo add bitnami https://charts.bitnami.com/bitnami

# Install go protobuf dependencies.
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go get k8s.io/apimachinery/pkg/apis/meta/v1
go get k8s.io/client-go/kubernetes
go get k8s.io/client-go/rest