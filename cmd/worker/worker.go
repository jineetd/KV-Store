package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"hash/fnv"
	"io/ioutil"
	pb "kvstore/protos"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// Define global variables to be used throughout the worker code.
var (
	port = flag.Int("kv_worker_grpc_server_port", 50051,
		"The grpc server port for worker")
	num_kv_store_shards = flag.Int("kv_num_shards", 9,
		"Total number of shards for our kvstore. All shards will be distributed across worker nodes.")
	mount_path     = os.Getenv("MOUNT_PATH")
	master_ip      = os.Getenv("POD_IP")
	pod_namespace  = os.Getenv("POD_NAMESPACE")
	pod_name       = os.Getenv("POD_NAME")
	oracle_ts_path = os.Getenv("PERSIST_ORACLE")
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
	// Generate the oracle timestamp for this write
	shard_id := getShardFromKey(key)
	db_modified_ts := GenerateOracleTimestampForShard(shard_id)
	glog.Infof(
		"Generated oracle timestamp for shard:%s timestamp:%d", shard_id,
		db_modified_ts)
	// Persist this oracle timestamp for shard
	success := PersistOracleTimestampForShard(shard_id, db_modified_ts)
	if !success {
		error_details := fmt.Sprintf(
			"Failed to persist oracle timestamp for shard: %s", shard_id)
		return &pb.PutKeyInternalRet{
			Success:      false,
			ErrorDetails: error_details,
		}, nil
	}
	is_write_success, error_details :=
		WriteKvToDisk(key, value, shard_id, db_modified_ts)
	return &pb.PutKeyInternalRet{
		Success:      is_write_success,
		ErrorDetails: error_details,
	}, nil
}

// Implement the GetKeyInternal RPC method
func (s *server) GetKeyInternal(ctx context.Context, in *pb.GetKeyInternalArg) (*pb.GetKeyInternalRet, error) {
	key := in.GetKey()
	req_id := in.GetReqId()
	glog.Infof("Received RPC GetKeyInternal request_id:%s for key: %s", req_id, key)
	is_read_success, error_details, kv_object := GetValueFromDisk(key)
	return &pb.GetKeyInternalRet{
		Success:      is_read_success,
		KvObject:     kv_object,
		ErrorDetails: error_details}, nil
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
func WriteKvToDisk(key string, value string, shard_id string, db_modified_ts int64) (bool, string) {
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

	// Prepare the kv store object to be flushed to disk
	kv_object := &pb.KvStoreObject{
		Value:        value,
		DbModifiedTs: db_modified_ts,
	}
	// Convert the object to Json string.
	json_str := protojson.Format(kv_object)

	if _, err := file.WriteString(json_str); err != nil {
		error_str := fmt.Sprintf("Error writing to file:", err)
		glog.Errorf(error_str)
		return false, error_str
	}

	glog.Infof(
		"Key: %s Value: %s successfully written onto the disk with db_modified_ts: %d",
		key, value, db_modified_ts)
	// Return true in case of success. Let error string be empty.
	return true, ""
}

// Helper method to fetch the key value pair from disk. Function returns true
// if the disk read was successful, else returns false.
// Returns (is_read_success, error_details, value)
func GetValueFromDisk(key string) (bool, string, *pb.KvStoreObject) {
	shard_id := getShardFromKey(key)
	filePath := mount_path + "/" + shard_id + "/" + key
	data, err := os.ReadFile(filePath)
	if err != nil {
		error_str := fmt.Sprintf("Error reading file:", err)
		glog.Errorf(error_str)
		return false, error_str, nil
	}

	// Parse this into a KvStoreObject.
	var kv_object pb.KvStoreObject
	if err := protojson.Unmarshal(data, &kv_object); err != nil {
		error_str :=
			fmt.Sprintf("Failed to unmarshal proto object for key:%s with error %w",
				key, err)
		return false, error_str, nil
	}

	glog.Infof("Key: %s has been successfully read from disk", key)
	// Return success and the data fetched.
	return true, "", &kv_object
}

//------------------------------------------------------------------------------
// ORACLE TIMESTAMP RELATED STRUCTS AND METHODS
//------------------------------------------------------------------------------

type OracleTimestampMap struct {
	oracle_timestamp_lock sync.RWMutex
	// Key should be the shard for the worker. Value should be the associated
	// oracle timestamp of the shard.
	timestamp_map map[string]int64
}

// Declare a global variable for the oracle timestamp map
var ShardOracleTimestampMap *OracleTimestampMap

// Helper method to instantiate a new oracle timestamp map.
func CreateOracleTimestampMap() *OracleTimestampMap {
	return &OracleTimestampMap{
		timestamp_map: make(map[string]int64),
	}
}

// Helper method to generate new oracle timestamp for a shard write.
// This oracle time will be used as db_modified_ts for entity write.
// We shall generate the oracle time and try to persist it to disk before every
// entity write. If oracle timestamp write as well as entity write to disk
// both are successful we shall claim that oracle time as db_modified_ts.
func GenerateOracleTimestampForShard(shard_id string) int64 {
	// Take a read lock on oracle timestamp map.
	ShardOracleTimestampMap.oracle_timestamp_lock.Lock()
	prev_ts, exists := ShardOracleTimestampMap.timestamp_map[shard_id]
	var oracle_time int64
	if exists {
		// Previous timestamp exists, generate new oracle timestamp based on
		// that
		current_time := time.Now().Unix()
		oracle_time = max(current_time, prev_ts) + 1
	} else {
		// If no entry for prev db_modified_ts exists, use the current time.
		oracle_time = time.Now().Unix()
	}
	ShardOracleTimestampMap.timestamp_map[shard_id] = oracle_time
	ShardOracleTimestampMap.oracle_timestamp_lock.Unlock()
	return oracle_time
}

// Helper method to persist the Oracle Timestamp to disk
func PersistOracleTimestampForShard(shard_id string, oracle_ts int64) bool {
	// Write oracle timestamp for each shard in a separate file.
	dirPath := oracle_ts_path
	filePath := filepath.Join(oracle_ts_path, shard_id)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		error_str := fmt.Sprintf("Failed to create dir: %w", err)
		glog.Errorf(error_str)
		return false
	}
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		error_str := fmt.Sprintf("failed to open file: %w", err)
		glog.Errorf(error_str)
		return false
	}
	defer file.Close()

	if _, err := file.WriteString(strconv.FormatInt(oracle_ts, 10)); err != nil {
		error_str := fmt.Sprintf("Error writing to file:", err)
		glog.Errorf(error_str)
		return false
	}
	// Oracle timestamp successfully persisted to disk.
	return true
}

// Helper method to init the DbModifiedTimestampMap. Make sure this method is
// called before any oracle timestamp method is called.
func InitShardOracleTimestampMap() {
	ShardOracleTimestampMap = CreateOracleTimestampMap()
	// TODO: Logic to read the persisted db modified timestamp for shards
	// goes here.
	ShardOracleTimestampMap.oracle_timestamp_lock.Lock()
	// Read directory contents
	files, err := ioutil.ReadDir(oracle_ts_path)
	if err != nil {
		glog.Errorf(
			"Error reading directory: %v. May be this is a first time bootup of kvstore.", err)
	} else {
		for _, file := range files {
			if file.Mode().IsRegular() {
				shard_id := file.Name()
				glog.Infof("Reading oracle timestamp for shard: %s", shard_id)
				// Read file content
				content, err := os.ReadFile(filepath.Join(oracle_ts_path, file.Name()))
				if err != nil {
					glog.Errorf("Error reading file:", err)
					continue
				}

				// Convert content to int64
				oracle_ts, err := strconv.ParseInt(string(content), 10, 64)
				if err != nil {
					glog.Errorf("Invalid integer in file:", err)
					continue
				}

				ShardOracleTimestampMap.timestamp_map[shard_id] = oracle_ts
			}
		}
	}
	ShardOracleTimestampMap.oracle_timestamp_lock.Unlock()

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

	// Init the oracle timestamp map
	InitShardOracleTimestampMap()

	// Start the gRPC server.
	MayBeStartGrpcServer()
}
