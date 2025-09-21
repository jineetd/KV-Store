import unittest
import sys
import os
import argparse
import logging

import kv_interface as kv

# Configure logging
logging.basicConfig(level=logging.INFO, format='[%(levelname)s] %(message)s')
logger = logging.getLogger("testLogger")

# Define host and port variables
HOST = "localhost"
PORT = "50052"

class TestKVStore(unittest.TestCase):
    @classmethod
    def setUpClass(self):
        logger.info("====== SETUP CLASS ======")
        logger.info("Initialize the kv store interface")
        self.kv = kv.KvStoreInterface(HOST, PORT)

    @classmethod
    def tearDownClass(self):
        logger.info("====== TEARDOWN CLASS ======")

    def setUp(self):
        logger.info(f"--- START test: {self._testMethodName} ---")

    def tearDown(self):
        logger.info(f"--- END test: {self._testMethodName} ---")

    def test_put_and_get(self):
        # Perform put and verify success.
        logger.info("Put key: foo value:bar in kvstore")
        res = self.kv.put_key("foo", "bar")
        self.assertEqual(res.success, True)

        # Perform get and verify the value
        logger.info("Fetch key: foo from kvstore")
        res = self.kv.get_key("foo")
        self.assertEqual(res.success, True)
        val = res.value
        db_modified_ts = res.db_modified_ts
        logger.info("Verify if the value is bar and db_modified_ts > 0")
        self.assertEqual(val, "bar")
        self.assertGreater(db_modified_ts, 0)

    def test_overwrite_key(self):
        logger.info("Put key:a value:1 to kvstore and verify success")
        res = self.kv.put_key("a", "1")
        self.assertEqual(res.success, True)
        logger.info("Put key:a value:2 to kvstore and verify success")
        res = self.kv.put_key("a", "2")
        self.assertEqual(res.success, True)
        logger.info("Check if the kvstore persists the latest value for key")
        self.assertEqual(self.kv.get_key("a").value, "2")

    def test_empty_key_write(self):
        logger.info("Try to write empty key value to kvstore")
        res = self.kv.put_key("", "some_value")
        self.assertEqual(res.success, False)
        logger.info("Verify if kvstore throws the correct error")
        self.assertEqual(
            res.kv_error.error_details,
            "Cannot send empty key to kvstore.")

    def test_empty_value_write(self):
        logger.info("Try to write empty string value to kvstore.")
        res = self.kv.put_key("some_key", "")
        self.assertEqual(res.success, False)
        logger.info("Verify if kvstore throws the correct error")
        self.assertEqual(
            res.kv_error.error_details,
            "Cannot send empty value to kvstore.")

    def test_empty_key_get(self):
        logger.info("Try to fetch empty key from kvstore.")
        res = self.kv.get_key("")
        self.assertEqual(res.success, False)
        logger.info("Verify if kvstore throws the correct error")
        self.assertEqual(
            res.kv_error.error_details,
            "Cannot fetch empty key from kvstore")

    def test_monotonic_increasing_db_timestamp(self):
        logger.info("Write a key: b value: v1 to kvstore")
        res = self.kv.put_key("b", "v1")
        self.assertEqual(res.success, True)
        logger.info("Fetch the key:b from kvstore")
        res = self.kv.get_key("b")
        val1 = res.value
        db_ts1 = res.db_modified_ts
        self.assertEqual(val1, "v1")
        self.assertGreater(db_ts1, 0)

        logger.info("Write a key: b value: v2 to kvstore")
        res = self.kv.put_key("b", "v2")
        self.assertEqual(res.success, True)
        logger.info("Fetch the key:b from kvstore")
        res = self.kv.get_key("b")
        val2 = res.value
        db_ts2 = res.db_modified_ts
        self.assertEqual(val2, "v2")
        self.assertGreater(db_ts2, 0)

        logger.info("Verify that the db_modified_ts for second update is " +
                    "greater than that of first update")
        self.assertGreater(db_ts2, db_ts1)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument('--host', type=str, default="localhost", help="Host/IP of KVStore server")
    parser.add_argument('--port', type=str, default="50052", help="Port of KVStore server")

    # Parse known args so unittest doesn't complain about extra ones
    args, _ = parser.parse_known_args()

    # Store in the class before tests run
    HOST = args.host
    PORT = args.port
    unittest.main()