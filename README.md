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
```
jineetdesai@Jineets-Air KV-Store % sh build.sh
```

### 3. Run the script to deploy the kv store in k8s cluster.
```
jineetdesai@Jineets-Air KV-Store % sh deploy_cluster.sh
```

### 4. Get names of all pods to view their logs
```
kubectl get all -n test-ns
```

### 5. Cleanup kind cluster,docker containers and images
```
sh cleanup.sh
```

## Accessing the KV Store via Python Client
Once the KV store cluster is up and running, you may need to forward the gRPC port for the control-manager service to a local port in order to test or interact with the KV store using a Python client.
```
# Forward the gRPC port to local port for python client.
kubectl port-forward service/grpc-service 50052:50052 -n test-ns
```
### Python Client Interface
The Python client interface for the KV store is located in the kv_client directory:
```
jineetdesai@Jineets-Air kv_client % ls
__init__.py		__pycache__		kv_store_interface.py
jineetdesai@Jineets-Air kv_client %
```

### Writing your own python client
You can implement your own Python client by importing the provided interface:
```
from kv_client import kv_store_interface
```
This interface provides the necessary method definitions to interact with the KV store.

### Running the IO integrety test.
```
(grpc-env) jineetdesai@Jineets-Air KV-Store % python kv_client/test_kv_io_integrity.py 
...
----------------------------------------------------------------------
Ran 3 tests in 0.105s

OK
(grpc-env) jineetdesai@Jineets-Air KV-Store %
```
