# Build the required protobufs.
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative protos/cm_worker.proto

# Build the go binaries for the services.
# Change the arch type if trying to build on windows system.
env GOOS=linux GOARCH=amd64 GOARM=7 go build cmd/control-manager/control-manager.go
env GOOS=linux GOARCH=amd64 GOARM=7 go build cmd/worker/worker.go

# Build the docker container for the services.
docker build -f docker/Dockerfile.control-manager -t control-manager:latest .
docker build -f docker/Dockerfile.worker -t worker:latest .