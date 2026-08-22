import pytest

from kv_client.kv_interface import KvStoreInterface


def pytest_addoption(parser):
    parser.addoption("--host", default="localhost", help="KVStore server host")
    parser.addoption("--port", default="50052", help="KVStore server port")


@pytest.fixture(scope="session")
def kv_client(request):
    host = request.config.getoption("--host")
    port = request.config.getoption("--port")
    return KvStoreInterface(host, port)
