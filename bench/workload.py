"""Create and update benchmark workload runners."""

from __future__ import annotations

import logging
import threading
import time
import uuid
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass

import grpc
from kv_client.kv_interface import KvStoreInterface

from bench.metrics import PhaseMetrics

logger = logging.getLogger(__name__)


@dataclass
class BenchmarkConfig:
    host: str
    port: str
    parallelism: int
    create_duration_sec: float
    update_duration_sec: float
    value_size: int
    warmup_sec: float


class KeyRegistry:
    def __init__(self) -> None:
        self._keys: list[str] = []
        self._lock = threading.Lock()

    def add(self, key: str) -> None:
        with self._lock:
            self._keys.append(key)

    def snapshot(self) -> list[str]:
        with self._lock:
            return list(self._keys)


def _make_value(value_size: int, suffix: str = "") -> str:
    if not suffix:
        return "x" * value_size
    if len(suffix) >= value_size:
        return suffix[:value_size]
    return ("x" * (value_size - len(suffix))) + suffix


def _run_put(
    client: KvStoreInterface,
    key: str,
    value: str,
    metrics: PhaseMetrics,
    record: bool,
) -> bool:
    start = time.perf_counter()
    try:
        response = client.put_key(key, value)
        latency_ms = (time.perf_counter() - start) * 1000
        if response.success:
            if record:
                metrics.record_success(latency_ms)
            return True
        if record:
            metrics.record_failure()
        logger.debug("PutKey failed for key %s: %s", key, response)
        return False
    except Exception as exc:
        if record:
            metrics.record_failure()
        logger.debug("PutKey exception for key %s: %s", key, exc)
        return False


def _create_worker(
    worker_id: int,
    config: BenchmarkConfig,
    deadline: float,
    warmup_deadline: float,
    registry: KeyRegistry,
) -> PhaseMetrics:
    # Instantiate the python client for the KV-Store server.
    client = KvStoreInterface(config.host, config.port)
    metrics = PhaseMetrics(
        phase_name="create",
        duration_sec=config.create_duration_sec,
        parallelism=config.parallelism,
    )
    value = _make_value(config.value_size)

    # Loop until and generate the keys until the deadline.
    while time.perf_counter() < deadline:
        key = str(uuid.uuid4())
        record = time.perf_counter() >= warmup_deadline
        if _run_put(client, key, value, metrics, record):
            registry.add(key)

    return metrics


def _update_worker(
    worker_id: int,
    config: BenchmarkConfig,
    keys: list[str],
    deadline: float,
    warmup_deadline: float,
) -> PhaseMetrics:
    client = KvStoreInterface(config.host, config.port)
    metrics = PhaseMetrics(
        phase_name="update",
        duration_sec=config.update_duration_sec,
        parallelism=config.parallelism,
    )
    cursor = worker_id
    update_seq = 0

    while time.perf_counter() < deadline:
        key = keys[cursor % len(keys)]
        cursor += config.parallelism
        suffix = f"{update_seq:08d}"
        value = _make_value(config.value_size, suffix=suffix)
        record = time.perf_counter() >= warmup_deadline
        _run_put(client, key, value, metrics, record)
        update_seq += 1

    return metrics


def run_create_phase(
    config: BenchmarkConfig,
) -> tuple[PhaseMetrics, list[str]]:
    registry = KeyRegistry()
    warmup_deadline = time.perf_counter() + config.warmup_sec
    deadline = warmup_deadline + config.create_duration_sec

    worker_metrics: list[PhaseMetrics] = []
    with ThreadPoolExecutor(max_workers=config.parallelism) as executor:
        futures = [
            executor.submit(
                _create_worker,
                worker_id,
                config,
                deadline,
                warmup_deadline,
                registry,
            )
            for worker_id in range(config.parallelism)
        ]
        for future in as_completed(futures):
            worker_metrics.append(future.result())

    measurement_elapsed = time.perf_counter() - warmup_deadline
    combined = PhaseMetrics(
        phase_name="create",
        duration_sec=config.create_duration_sec,
        parallelism=config.parallelism,
    )
    for metrics in worker_metrics:
        combined.merge(metrics)
    combined.finalize(measurement_elapsed)
    return combined, registry.snapshot()


def run_update_phase(
    config: BenchmarkConfig,
    keys: list[str],
) -> PhaseMetrics:
    warmup_deadline = time.perf_counter() + config.warmup_sec
    deadline = warmup_deadline + config.update_duration_sec

    worker_metrics: list[PhaseMetrics] = []
    with ThreadPoolExecutor(max_workers=config.parallelism) as executor:
        futures = [
            executor.submit(
                _update_worker,
                worker_id,
                config,
                keys,
                deadline,
                warmup_deadline,
            )
            for worker_id in range(config.parallelism)
        ]
        for future in as_completed(futures):
            worker_metrics.append(future.result())

    measurement_elapsed = time.perf_counter() - warmup_deadline
    combined = PhaseMetrics(
        phase_name="update",
        duration_sec=config.update_duration_sec,
        parallelism=config.parallelism,
    )
    for metrics in worker_metrics:
        combined.merge(metrics)
    combined.finalize(measurement_elapsed)
    return combined


def verify_connection(config: BenchmarkConfig) -> None:
    client = KvStoreInterface(config.host, config.port)
    probe_key = f"bench-probe-{uuid.uuid4()}"
    try:
        response = client.put_key(probe_key, _make_value(min(config.value_size, 8)))
    except grpc.RpcError as exc:
        raise RuntimeError(
            f"Failed to connect to KV-Store at {config.host}:{config.port}. "
            f"Is the cluster running and port-forward active? ({exc.code()})"
        ) from exc
    if not response.success:
        raise RuntimeError(
            f"Failed to connect to KV-Store at {config.host}:{config.port}. "
            f"Error: {getattr(response, 'kv_error', response)}"
        )
