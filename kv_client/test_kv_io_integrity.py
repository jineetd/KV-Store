import unittest
import sys
import os

import kv_interface as kv

class TestKVStore(unittest.TestCase):

    def setUp(self):
        self.kv = kv.KvStoreInterface("localhost", "50052")

    def test_put_and_get(self):
        # Perform put and verify success.
        res = self.kv.put_key("foo", "bar")
        self.assertEqual(res.success, True)

        # Perform get and verify the value
        res = self.kv.get_key("foo")
        self.assertEqual(res.success, True)
        val = res.value
        self.assertEqual(val, "bar")

    def test_get_nonexistent_key_raises(self):
        # Change this test post error handling.
        res = self.kv.get_key("missing_key")
        self.assertEqual(res.success, True)

    def test_overwrite_key(self):
        self.kv.put_key("a", "1")
        self.kv.put_key("a", "2")
        self.assertEqual(self.kv.get_key("a").value, "2")

if __name__ == "__main__":
    unittest.main()