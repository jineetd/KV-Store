package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/golang/glog"
	"github.com/google/uuid"
	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"hash/fnv"
	pb "kvstore/protos"
	"net"
	"os"
	"strconv"
	"time"
)

// Define global variables to be used throughout the worker code.
var (
	port = flag.Int("kv_worker_grpc_server_port", 50051,
		"The grpc server port for worker")
	num_workers = flag.Int("kv_num_worker_pods", 3,
		"Number of worker pods for our distributed kv-store")
	num_kv_store_shards = flag.Int("kv_num_shards", 9,
		"Total number of shards for our kvstore. All shards will be distributed across worker nodes.")
	server_port = flag.Int("kv_control_manager_grpc_server_port", 50052,
		"The grpc server port for control manager to get client requests.")
)

// Define all global variables related to pod environment.
var (
	master_ip     = os.Getenv("POD_IP")
	pod_namespace = os.Getenv("POD_NAMESPACE")
	pod_name      = os.Getenv("POD_NAME")
)

// Map from worker DNS name to RPC client
var rpcClients map[string]pb.KvStoreServiceClient

//------------------------------------------------------------------------------
// METHODS TO INITIALIZE THE CONTROL MANAGER FOR KV STORE.
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

// Helper method to initialize the Rpc clients for worker nodes.
func initRpcClients() {
	rpcClients = make(map[string]pb.KvStoreServiceClient)
	for ii := 0; ii < *num_workers; ii++ {
		worker_pod := "worker-" + strconv.Itoa(ii)
		rpc_client, err := getRpcClientForWorkerPod(worker_pod)
		if err != nil {
			glog.Errorf("Could not initialize rpc client for %s with error:%v", worker_pod, err)
			rpcClients[worker_pod] = nil
		}
		rpcClients[worker_pod] = rpc_client
	}
}

// Helper method for performing the active control manager node.
// We simply rely on the etcd leader election to do so.
func PerformLeaderElection() {
	// Write the code for leader election.
	etcd_dns_url := "etcd." + pod_namespace + ".svc.cluster.local:2379"
	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{etcd_dns_url}})
	if err != nil {
		glog.Fatal(err)
	}
	defer cli.Close()
	// Create a session to elect a Leader
	glog.Info("Create a session to elect a new leader.")
	s, err := concurrency.NewSession(cli)
	if err != nil {
		glog.Fatal(err)
	}
	defer s.Close()
	e := concurrency.NewElection(s, "/leader-election/")
	ctx := context.Background()
	// Elect a leader (or wait that the leader resign)
	glog.Info("Elect a leader or wait for leader resign.")
	if err := e.Campaign(ctx, "e"); err != nil {
		glog.Fatal(err)
	}
	glog.Info("Leader elected: ", pod_name)
}

//------------------------------------------------------------------------------
// HELPER METHODS FOR PUT KEY AND GET KEY
//------------------------------------------------------------------------------

func PutKeyInternal(req_id string, key string, value string,
	error_msg *pb.KvError) {
	// Get the worker pod based on the shard of this key.
	worker_pod := getWorkerNodeForKey(key)
	// Make RPC call to the worker pod.
	rpc_client := rpcClients[worker_pod]
	if rpc_client == nil {
		error_msg.ErrorType = pb.ErrorCode_kInternalError
		error_msg.ErrorDetails =
			fmt.Sprintf("RPC client not initialized: %s", worker_pod)
		return
	}
	glog.Infof("Call PutKeyInternal request_id: %s for worker node: %s ",
		req_id, worker_pod)
	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	r, err := rpc_client.PutKeyInternal(
		ctx, &pb.PutKeyInternalArg{ReqId: req_id, Key: key, Value: value})
	if err != nil {
		error_msg.ErrorType = pb.ErrorCode_kInternalError
		error_msg.ErrorDetails =
			fmt.Sprintf("No response from worker server")
		return
	}
	// Check if we did not receive any errors from writing onto backend disk.
	// All these errors are perceived as kBackend errors.
	if !r.GetSuccess() {
		error_msg.ErrorType = pb.ErrorCode_kBackendError
		error_msg.ErrorDetails = r.GetErrorDetails()
		return
	}
	glog.Infof(
		"Received Response PutKeyInternal request_id: %s from worker node: %s is %s",
		req_id,
		worker_pod,
		r.GetSuccess(),
	)
	// In case of success return kNoError. Let error details be empty.
	error_msg.ErrorType = pb.ErrorCode_kNoError
}

func getRpcClientForWorkerPod(worker_pod string) (pb.KvStoreServiceClient, error) {
	ip_addr_port := worker_pod + ".worker." + pod_namespace + ".svc.cluster.local:" + strconv.Itoa(*port)
	conn, err := grpc.Dial(ip_addr_port, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		glog.Errorf("Error creating RPC client: %v", err)
		return nil, err
	}
	return pb.NewKvStoreServiceClient(conn), nil
}

func getWorkerNodeForKey(key string) string {
	// FNV-1a: fast, decent distribution
	h := fnv.New32a()
	// hash the key
	h.Write([]byte(key))
	shard_id := int(h.Sum32()) % (*num_kv_store_shards)
	node_id := shard_id % (*num_workers)
	return "worker-" + strconv.Itoa(node_id)
}

// Returns the Value from Kv Store.
func GetKeyInternal(req_id string, key string, error_msg *pb.KvError) string {
	// Get the worker pod based on the shard of this key.
	worker_pod := getWorkerNodeForKey(key)
	// Make RPC call to the worker pod.
	rpc_client := rpcClients[worker_pod]
	if rpc_client == nil {
		error_msg.ErrorType = pb.ErrorCode_kInternalError
		error_msg.ErrorDetails =
			fmt.Sprintf("RPC client not initialized: %s", worker_pod)
		return ""
	}
	glog.Infof("Call GetKeyInternal request_id: %s for worker node: %s ",
		req_id, worker_pod)
	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	r, err := rpc_client.GetKeyInternal(
		ctx, &pb.GetKeyInternalArg{ReqId: req_id, Key: key})
	if err != nil {
		error_msg.ErrorType = pb.ErrorCode_kInternalError
		error_msg.ErrorDetails = fmt.Sprintf("No response from server: %v", err)
		return ""
	}
	glog.Infof(
		"Response GetKeyInternal request_id: %s from worker node: %s is %t",
		req_id, worker_pod, r.GetSuccess())
	if !r.GetSuccess() {
		error_msg.ErrorType = pb.ErrorCode_kNotFound
		error_msg.ErrorDetails = r.GetErrorDetails()
		return ""
	}
	// In case of success return kNoError, let error details be empty.
	// Return the value received from disk.
	error_msg.ErrorType = pb.ErrorCode_kNoError
	return r.GetValue()
}

//------------------------------------------------------------------------------
// GRPC Service Implementations
//------------------------------------------------------------------------------
// Implememt the MapReduceServiceServer
type server struct {
	pb.UnimplementedKvStoreInterfaceServer
}

// Add validation for PutKey.
// Returns true if arg is valid, else returns false along with error details.
func ValidatePutKeyArg(in *pb.PutKeyArg) (bool, string) {
	key := in.GetKey()
	value := in.GetValue()
	if key == "" {
		return false, "Cannot send empty key to kvstore."
	}
	if value == "" {
		return false, "Cannot send empty value to kvstore."
	}
	return true, ""
}

// Add validation for GetKey
// Returns true if arg is valid, else returns false along with error details.
func ValidateGetKeyArg(in *pb.GetKeyArg) (bool, string) {
	key := in.GetKey()
	if key == "" {
		return false, "Cannot fetch empty key from kvstore"
	}
	return true, ""
}

// Implement the PutKey RPC method.
func (s *server) PutKey(ctx context.Context, in *pb.PutKeyArg) (*pb.PutKeyRet, error) {
	// Validate the PutArg.
	is_valid_arg, error_details := ValidatePutKeyArg(in)
	if is_valid_arg == false {
		return &pb.PutKeyRet{
			Success: false,
			KvError: &pb.KvError{
				ErrorType:    pb.ErrorCode_kInvalidArgument,
				ErrorDetails: error_details,
			}}, nil
	}
	key := in.GetKey()
	value := in.GetValue()
	// Generate internal request id
	req_id := uuid.New().String()
	glog.Infof("Received RPC PutKey request_id: %s for key: %s", req_id, key)
	var error_msg pb.KvError
	PutKeyInternal(req_id, key, value, &error_msg)
	is_write_success := (error_msg.ErrorType == pb.ErrorCode_kNoError)
	return &pb.PutKeyRet{Success: is_write_success,
		KvError: &error_msg}, nil
}

// Implement the GetKey RPC method
func (s *server) GetKey(ctx context.Context, in *pb.GetKeyArg) (*pb.GetKeyRet, error) {
	// Valid the GetArg
	is_valid_arg, error_details := ValidateGetKeyArg(in)
	if is_valid_arg == false {
		return &pb.GetKeyRet{
			Success: false,
			Value:   "",
			KvError: &pb.KvError{
				ErrorType:    pb.ErrorCode_kInvalidArgument,
				ErrorDetails: error_details,
			}}, nil
	}
	key := in.GetKey()
	// Generate internal request id.
	req_id := uuid.New().String()
	glog.Infof("Received RPC GetKey request_id: %s for key: %s", req_id, key)
	var error_msg pb.KvError
	value := GetKeyInternal(req_id, key, &error_msg)
	is_read_success := (error_msg.ErrorType == pb.ErrorCode_kNoError &&
		value != "")
	return &pb.GetKeyRet{
		Success: is_read_success,
		Value:   value,
		KvError: &error_msg}, nil
}

// Helper method to Init the gRPC server in order to receive calls from
// kv store clients.
func MayBeStartGrpcServer() {
	// Start the grpc server to receive calls from control-manager.
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *server_port))
	if err != nil {
		glog.Fatalf("Failed to listen: %v", err)
	}
	worker_server := grpc.NewServer()
	pb.RegisterKvStoreInterfaceServer(worker_server, &server{})
	glog.Infof("Worker grpc service listening at %v", lis.Addr())
	if err := worker_server.Serve(lis); err != nil {
		glog.Fatalf("Failed to server: %v", err)
	}
}

//------------------------------------------------------------------------------

// Main method goes here.
func main() {
	// Parse and set the appropriate gflags.
	SetGflagSettings()

	glog.Infof("Pod name: %s is spawned at pod IP: %s", pod_name, master_ip)
	glog.Infof("Control manager pod namespace: %s", pod_namespace)

	// Call the method to perform leader election.
	PerformLeaderElection()

	// Init the RPC clients to workers.
	initRpcClients()

	// Start the gRPC server in a separate go routine thread.
	MayBeStartGrpcServer()

}
