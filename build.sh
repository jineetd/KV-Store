# Set required paths for golang.
export PATH=$PATH:/usr/local/go/bin
export PATH="$PATH:$(go env GOPATH)/bin"

# Build the required protobufs in golang
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative protos/cm_worker.proto
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative protos/kv_store_interface.proto

# Build the required protobufs in python for the client package.
python -m grpc_tools.protoc --proto_path=./protos --python_out=./kv_client --grpc_python_out=./kv_client kv_store_interface.proto
sed -i '' 's/^import kv_store_interface_pb2/from . import kv_store_interface_pb2/' kv_client/kv_store_interface_pb2_grpc.py

# Build the go binaries for the services.
# Change the arch type if trying to build on windows system.
env GOOS=linux GOARCH=amd64 GOARM=7 go build -o control-manager ./cmd/control-manager
env GOOS=linux GOARCH=amd64 GOARM=7 go build -o worker ./cmd/worker

# Build the docker container for the services.
docker build -f docker/Dockerfile.control-manager -t control-manager:latest .
docker build -f docker/Dockerfile.worker -t worker:latest .