# Peform all the gRPC related imports.
import grpc
import kv_store_interface_pb2
import kv_store_interface_pb2_grpc


class KvStoreInterface:
    def __init__(self, server_ip, server_port):
        self.server_ip = server_ip
        self.server_port = server_port
        self.channel = grpc.insecure_channel(self.server_ip + ":" + self.server_port)
        self.stub = kv_store_interface_pb2_grpc.KvStoreInterfaceStub(self.channel)

    def get_key(self, key):
        request = kv_store_interface_pb2.GetKeyArg(key=key)
        response = self.stub.GetKey(request)
        return response

    def put_key(self, key, value):
        request = kv_store_interface_pb2.PutKeyArg(key=key, value=value)
        response = self.stub.PutKey(request)
        return response