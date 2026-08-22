package main

import (
	"fmt"
	"github.com/golang/glog"
	"google.golang.org/protobuf/encoding/protojson"
	"hash/fnv"
	pb "kvstore/protos"
	"os"
	"path/filepath"
	"strconv"
)

//------------------------------------------------------------------------------
// KV-Store related helper methods.
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
		error_str := fmt.Sprintf("Failed to create dir: %v", err)
		glog.Errorf(error_str)
		return false, error_str
	}

	// Open file with flags:
	// os.O_CREATE - create if not exists
	// os.O_WRONLY - write only
	// os.O_TRUNC  - truncate file when opened (overwrite)
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		error_str := fmt.Sprintf("failed to open file: %v", err)
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
		error_str := fmt.Sprintf("Error writing to file: %v", err)
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
	filePath := filepath.Join(mount_path, shard_id, key)
	data, err := os.ReadFile(filePath)
	if err != nil {
		error_str := fmt.Sprintf("Error reading file: %v", err)
		glog.Errorf(error_str)
		return false, error_str, nil
	}

	// Parse this into a KvStoreObject.
	var kv_object pb.KvStoreObject
	if err := protojson.Unmarshal(data, &kv_object); err != nil {
		error_str :=
			fmt.Sprintf("Failed to unmarshal proto object for key:%s with error %v",
				key, err)
		return false, error_str, nil
	}

	glog.Infof("Key: %s has been successfully read from disk", key)
	// Return success and the data fetched.
	return true, "", &kv_object
}

