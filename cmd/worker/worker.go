package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/golang/glog"
	"google.golang.org/grpc"
	"hash/fnv"
	pb "kvstore/protos"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

// Define global variables to be used throughout the worker code.
var (
	port = flag.Int("kv_worker_grpc_server_port", 50051,
		"The grpc server port for worker")
	num_kv_store_shards = flag.Int("kv_num_shards", 9,
		"Total number of shards for our kvstore. All shards will be distributed across worker nodes.")
	mount_path    = os.Getenv("MOUNT_PATH")
	master_ip     = os.Getenv("POD_IP")
	pod_namespace = os.Getenv("POD_NAMESPACE")
	pod_name      = os.Getenv("POD_NAME")
)

//------------------------------------------------------------------------------
// GRPC Service Implementations
//------------------------------------------------------------------------------
// Implememt the MapReduceServiceServer
type server struct {
	pb.UnimplementedKvStoreServiceServer
}

// Implement the PutKeyInternal RPC method.
func (s *server) PutKeyInternal(ctx context.Context, in *pb.PutKeyInternalArg) (*pb.PutKeyInternalRet, error) {
	key := in.GetKey()
	value := in.GetValue()
	req_id := in.GetReqId()
	glog.Infof("Received RPC PutKeyInternal request_id:%s for key: %s",
		req_id, key)
	is_write_success, error_details := WriteKvToDisk(key, value)
	return &pb.PutKeyInternalRet{
		Success:      is_write_success,
		ErrorDetails: error_details,
	}, nil
}

// Implement the GetKeyInternal RPC method
func (s *server) GetKeyInternal(ctx context.Context, in *pb.GetKeyInternalArg) (*pb.GetKeyInternalRet, error) {
	key := in.GetKey()
	glog.Infof("Received GetKeyInternal request for key: %s", key)
	is_read_success, value := GetValueFromDisk(key)
	return &pb.GetKeyInternalRet{Success: is_read_success, Value: value}, nil
}

//------------------------------------------------------------------------------
// KV-Store related methods.
//------------------------------------------------------------------------------

// Helper method to get shard from key
func getShardFromKey(key string) string {
	h := fnv.New32a()
	// hash the key
	h.Write([]byte(key))
	shard_id := int(h.Sum32()) % (*num_kv_store_shards)
	return strconv.Itoa(shard_id)
}

// Helper method to write KV to pod disk. Function returns true if the write
// was successful, else returns false.
func WriteKvToDisk(key string, value string) (bool, string) {
	shard_id := getShardFromKey(key)
	dirPath := filepath.Join(mount_path, shard_id)
	filePath := filepath.Join(dirPath, key)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		error_str := fmt.Sprintf("Failed to create dir: %w", err)
		glog.Errorf(error_str)
		return false, error_str
	}

	// Open file with flags:
	// os.O_CREATE - create if not exists
	// os.O_WRONLY - write only
	// os.O_TRUNC  - truncate file when opened (overwrite)
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		error_str := fmt.Sprintf("failed to open file: %w", err)
		glog.Errorf(error_str)
		return false, error_str
	}
	defer file.Close()

	if _, err := file.WriteString(value); err != nil {
		error_str := fmt.Sprintf("Error writing to file:", err)
		glog.Errorf(error_str)
		return false, error_str
	}

	glog.Infof("Key: %s Value: %s successfully written onto the disk", key, value)
	// Return true in case of success. Let error string be empty.
	return true, ""
}

func GetValueFromDisk(key string) (bool, string) {
	shard_id := getShardFromKey(key)
	filePath := mount_path + "/" + shard_id + "/" + key
	data, err := os.ReadFile(filePath)
	if err != nil {
		glog.Errorf("Error reading file:", err)
		return false, ""
	}
	// Return success and the data fetched.
	return true, string(data)
}

//------------------------------------------------------------------------------

// Helper method to set the appropriate parameters for gflags.
// Method to be called from the main() function.
func SetGflagSettings() {
	glog.Info("Parse and set the appropriate gflags for worker service.")
	flag.Parse()
	defer glog.Flush()
	// Set default value for logtostderr
	flag.Set("logtostderr", "true")
}

// Helper method to Init the gRPC server in order to receive calls from
// control manager leader.
func MayBeStartGrpcServer() {
	// Start the grpc server to receive calls from control-manager.
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		glog.Fatalf("Failed to listen: %v", err)
	}
	worker_server := grpc.NewServer()
	pb.RegisterKvStoreServiceServer(worker_server, &server{})
	glog.Infof("Worker grpc service listening at %v", lis.Addr())
	if err := worker_server.Serve(lis); err != nil {
		glog.Fatalf("Failed to server: %v", err)
	}
}

func main() {
	// Call the method to set appropriate gflag parameters.
	SetGflagSettings()

	glog.Infof("Starting worker pod: %s at IP:%s pod_namespace:%s",
		pod_name, master_ip, pod_namespace)

	// Start the gRPC server.
	MayBeStartGrpcServer()
}
