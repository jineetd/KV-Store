import logging

import pytest

logger = logging.getLogger("testLogger")

pytestmark = pytest.mark.integration


def test_put_and_get(kv_client):
    logger.info("Put key: foo value:bar in kvstore")
    res = kv_client.put_key("foo", "bar")
    assert res.success is True

    logger.info("Fetch key: foo from kvstore")
    res = kv_client.get_key("foo")
    assert res.success is True
    logger.info("Verify if the value is bar and db_modified_ts > 0")
    assert res.value == "bar"
    assert res.db_modified_ts > 0


def test_overwrite_key(kv_client):
    logger.info("Put key:a value:1 to kvstore and verify success")
    res = kv_client.put_key("a", "1")
    assert res.success is True

    logger.info("Put key:a value:2 to kvstore and verify success")
    res = kv_client.put_key("a", "2")
    assert res.success is True

    logger.info("Check if the kvstore persists the latest value for key")
    assert kv_client.get_key("a").value == "2"


def test_empty_key_write(kv_client):
    logger.info("Try to write empty key value to kvstore")
    res = kv_client.put_key("", "some_value")
    assert res.success is False
    logger.info("Verify if kvstore throws the correct error")
    assert res.kv_error.error_details == "Cannot send empty key to kvstore."


def test_empty_value_write(kv_client):
    logger.info("Try to write empty string value to kvstore.")
    res = kv_client.put_key("some_key", "")
    assert res.success is False
    logger.info("Verify if kvstore throws the correct error")
    assert res.kv_error.error_details == "Cannot send empty value to kvstore."


def test_empty_key_get(kv_client):
    logger.info("Try to fetch empty key from kvstore.")
    res = kv_client.get_key("")
    assert res.success is False
    logger.info("Verify if kvstore throws the correct error")
    assert res.kv_error.error_details == "Cannot fetch empty key from kvstore"


def test_monotonic_increasing_db_timestamp(kv_client):
    logger.info("Write a key: b value: v1 to kvstore")
    res = kv_client.put_key("b", "v1")
    assert res.success is True

    logger.info("Fetch the key:b from kvstore")
    res = kv_client.get_key("b")
    val1 = res.value
    db_ts1 = res.db_modified_ts
    assert val1 == "v1"
    assert db_ts1 > 0

    logger.info("Write a key: b value: v2 to kvstore")
    res = kv_client.put_key("b", "v2")
    assert res.success is True

    logger.info("Fetch the key:b from kvstore")
    res = kv_client.get_key("b")
    val2 = res.value
    db_ts2 = res.db_modified_ts
    assert val2 == "v2"
    assert db_ts2 > 0

    logger.info(
        "Verify that the db_modified_ts for second update is "
        "greater than that of first update")
    assert db_ts2 > db_ts1
