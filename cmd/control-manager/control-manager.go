package main

import (
	"context"
	"errors"
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

func PutKeyInternal(key string, value string) error {
	// Get the worker pod based on the shard of this key.
	worker_pod := getWorkerNodeForKey(key)
	// Make RPC call to the worker pod.
	rpc_client := rpcClients[worker_pod]
	if rpc_client == nil {
		glog.Errorf("RPC client not initialized: %s", worker_pod)
		return errors.New("Rpc client does not exists.")
	}
	// Generate internal request id
	req_id := uuid.New().String()
	glog.Infof("Call PutKeyInternal request_id: %s for worker node: %s ",
		req_id, worker_pod)
	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	r, err := rpc_client.PutKeyInternal(
		ctx, &pb.PutKeyInternalArg{Key: key, Value: value})
	if err != nil {
		glog.Infof("No response from server: %v", err)
		return err
	}
	glog.Infof("Response PutKeyInternal request_id: %s from worker node: %s is %s", req_id, worker_pod, r.GetSuccess())
	return nil
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

func GetKeyInternal(key string) (string, error) {
	// Get the worker pod based on the shard of this key.
	worker_pod := getWorkerNodeForKey(key)
	// Make RPC call to the worker pod.
	rpc_client := rpcClients[worker_pod]
	if rpc_client == nil {
		glog.Errorf("RPC client not initialized: %s", worker_pod)
		return "", errors.New("Rpc client does not exists.")
	}
	// Generate internal request id
	req_id := uuid.New().String()
	glog.Infof("Call GetKeyInternal request_id: %s for worker node: %s ",
		req_id, worker_pod)
	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	r, err := rpc_client.GetKeyInternal(
		ctx, &pb.GetKeyInternalArg{Key: key})
	if err != nil {
		glog.Infof("No response from server: %v", err)
		return "", err
	}
	glog.Infof("Response GetKeyInternal request_id: %s from worker node: %s is %s", req_id, worker_pod, r.String())
	return r.GetValue(), nil
}

//------------------------------------------------------------------------------
// GRPC Service Implementations
//------------------------------------------------------------------------------
// Implememt the MapReduceServiceServer
type server struct {
	pb.UnimplementedKvStoreInterfaceServer
}

// Implement the PutKey RPC method.
func (s *server) PutKey(ctx context.Context, in *pb.PutKeyArg) (*pb.PutKeyRet, error) {
	key := in.GetKey()
	value := in.GetValue()
	glog.Infof("Received PutKey request for key: %s", key)
	err := PutKeyInternal(key, value)
	var is_write_success bool
	if err != nil {
		is_write_success = false
	} else {
		is_write_success = true
	}
	return &pb.PutKeyRet{Success: is_write_success}, nil
}

// Implement the GetKey RPC method
func (s *server) GetKey(ctx context.Context, in *pb.GetKeyArg) (*pb.GetKeyRet, error) {
	key := in.GetKey()
	glog.Infof("Received GetKeyInternal request for key: %s", key)
	value, err := GetKeyInternal(key)
	var is_read_success bool
	if err != nil {
		is_read_success = false
	} else {
		is_read_success = true
	}
	return &pb.GetKeyRet{Success: is_read_success, Value: value}, nil
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

	// Start the gRPC server in a separate go routine thread.
	go MayBeStartGrpcServer()

	// Init the RPC clients to workers.
	initRpcClients()

	PutKeyInternal("test_key", "test_value")
	GetKeyInternal("test_key")

	glog.Info("Sleep the current master node for 2 mins.")
	time.Sleep(120 * time.Second)

}
