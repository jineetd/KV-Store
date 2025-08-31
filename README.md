## Installation
We assume you have installed Golang already on your system. For Golang installation instructions, visit their website here: https://golang.org/doc/install
Run install.sh on your system after Golang has been installed. The install script should work for debian distributions of linux.

The install script will do the following:
1. Install kubectl
2. Install protobuf
3. Install Kind
4. Install Helm
5. Install gRPC
6. Set up Go module with dependencies included
```
sh install.sh
```

After set up, remember to add the path of Go and its libraries to the PATH environmental variables.
Do this by pasting the following into ~/.bash_profile or ~/.bashrc, depending on your environment.
``` bash
export PATH=$PATH:/usr/local/go/bin
export PATH="$PATH:$(go env GOPATH)/bin"
```

## Directory Structure
After installation, there should be a go.mod file. This defines the local go module for your implementation of KvStore. The name of the module is defined as kvstore,
and the dependencies are defined in the require section. 

The code for your control-manager and worker files are in the cmd/ directory, both as main packages.

Run all the below commands from the root directory.

### 1. Install dependencies
Install all libaries using install.sh script after installing go.

### 2. Build script
Run build.sh so that go binaries for control-manager and worker are built and docker images are generated for control-manager and worker. Run the command in root of the repository. 

### 3. Kind cluster and namespace
Run below command to create a kind cluster with namespace test-ns.
```
kind create cluster
kubectl cluster-info --context kind-kind
kubectl create ns test-ns
```

### 4. Add Helm for etcd
Install etcd and it runs as a docker container. Check the docker images generated.
```
helm install -n test-ns etcd bitnami/etcd --set auth.rbac.create=false
docker images
```

### 5. Load docker images into kind.
```
kind load docker-image control-manager:latest
kind load docker-image worker:latest
```

### 6. Apply the YAML config file for kubernetes components
```
kubectl -n test-ns apply -f cluster_setup.yaml
```

### 7. Get names of all pods to view their logs
```
kubectl get all -n test-ns
```

### 10. Cleanup kind cluster,docker containers and images
```
sh cleanup.sh
```