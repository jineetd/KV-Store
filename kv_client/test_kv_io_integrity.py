import unittest
import sys
import os
import argparse

import kv_interface as kv

class TestKVStore(unittest.TestCase):
    @classmethod
    def setUp(self):
        self.kv = kv.KvStoreInterface(self.host, self.port)

    def test_put_and_get(self):
        print("*** START test_put_and_get ***")
        # Perform put and verify success.
        res = self.kv.put_key("foo", "bar")
        self.assertEqual(res.success, True)

        # Perform get and verify the value
        res = self.kv.get_key("foo")
        self.assertEqual(res.success, True)
        val = res.value
        self.assertEqual(val, "bar")
        print("*** PASSED test_put_and_get ***")

    def test_overwrite_key(self):
        print("*** START test_overwrite_key ***")
        res = self.kv.put_key("a", "1")
        self.assertEqual(res.success, True)
        res = self.kv.put_key("a", "2")
        self.assertEqual(res.success, True)
        self.assertEqual(self.kv.get_key("a").value, "2")
        print("*** PASSED test_overwrite_key ***")

    def test_empty_key_write(self):
        print("*** START test_empty_key_write ***")
        res = self.kv.put_key("", "some_value")
        self.assertEqual(res.success, False)
        self.assertEqual(
            res.kv_error.error_details,
            "Cannot send empty key to kvstore.")
        print("*** PASSED test_empty_key_write ***")

    def test_empty_value_write(self):
        print("*** START test_empty_value_write ***")
        res = self.kv.put_key("some_key", "")
        self.assertEqual(res.success, False)
        self.assertEqual(
            res.kv_error.error_details,
            "Cannot send empty value to kvstore.")
        print("*** PASSED test_empty_value_write ***")

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument('--host', type=str, default="localhost", help="Host/IP of KVStore server")
    parser.add_argument('--port', type=str, default="50052", help="Port of KVStore server")

    # Parse known args so unittest doesn't complain about extra ones
    args, _ = parser.parse_known_args()

    # Store in the class before tests run
    TestKVStore.host = args.host
    TestKVStore.port = args.port
    unittest.main()