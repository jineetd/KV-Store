"""KV-Store performance benchmark CLI."""

from __future__ import annotations

import argparse
import logging
import sys

from bench.metrics import print_combined_report, print_header, print_phase_report
from bench.workload import (
    BenchmarkConfig,
    run_create_phase,
    run_update_phase,
    verify_connection,
)


def _parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Benchmark KV-Store create and update throughput/latency."
    )
    parser.add_argument("--host", default="localhost", help="KVStore server host")
    parser.add_argument("--port", default="50052", help="KVStore server port")
    parser.add_argument(
        "--parallelism",
        type=int,
        default=4,
        help="Number of concurrent worker threads",
    )
    parser.add_argument(
        "--create-duration",
        type=float,
        default=100.0,
        help="Create phase duration in seconds",
    )
    parser.add_argument(
        "--update-duration",
        type=float,
        default=100.0,
        help="Update phase duration in seconds",
    )
    parser.add_argument(
        "--value-size",
        type=int,
        default=64,
        help="Payload size in bytes for values",
    )
    parser.add_argument(
        "--warmup",
        type=float,
        default=0.0,
        help="Warmup seconds before metrics collection (per phase)",
    )
    return parser.parse_args()


def main() -> int:
    logging.basicConfig(level=logging.WARNING)
    args = _parse_args()

    if args.parallelism < 1:
        print("Error: --parallelism must be at least 1", file=sys.stderr)
        return 1
    if args.value_size < 1:
        print("Error: --value-size must be at least 1", file=sys.stderr)
        return 1

    config = BenchmarkConfig(
        host=args.host,
        port=args.port,
        parallelism=args.parallelism,
        create_duration_sec=args.create_duration,
        update_duration_sec=args.update_duration,
        value_size=args.value_size,
        warmup_sec=args.warmup,
    )

    create_metrics = None
    update_metrics = None

    try:
        print("Verify the connection to KV-Store server !", file=sys.stdout)
        verify_connection(config)
    except RuntimeError as exc:
        print(f"Error: {exc}", file=sys.stderr)
        return 1

    print_header(config.host, config.port, config.parallelism)

    try:
        create_metrics, keys = run_create_phase(config)
        print_phase_report(create_metrics)

        if not keys:
            print(
                "Error: create phase produced zero keys; aborting update phase.",
                file=sys.stderr,
            )
            return 1

        update_metrics = run_update_phase(config, keys)
        print_phase_report(update_metrics)
    except KeyboardInterrupt:
        print("\nBenchmark interrupted.", file=sys.stderr)
    finally:
        print_combined_report(create_metrics, update_metrics)

    return 0


if __name__ == "__main__":
    sys.exit(main())
