"""Benchmark metrics collection and reporting."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional

from rich.console import Console
from rich.panel import Panel
from rich.table import Table


def _percentile(sorted_values: list[float], pct: float) -> float:
    if not sorted_values:
        return 0.0
    index = int(round((pct / 100.0) * (len(sorted_values) - 1)))
    return sorted_values[index]


@dataclass
class PhaseMetrics:
    phase_name: str
    duration_sec: float
    parallelism: int
    total_ops: int = 0
    successful_ops: int = 0
    failed_ops: int = 0
    latencies_ms: list[float] = field(default_factory=list)
    throughput_ops_sec: float = 0.0
    elapsed_sec: float = 0.0
    latency_min_ms: float = 0.0
    latency_max_ms: float = 0.0
    latency_avg_ms: float = 0.0
    latency_p50_ms: float = 0.0
    latency_p95_ms: float = 0.0
    latency_p99_ms: float = 0.0

    def record_success(self, latency_ms: float) -> None:
        self.total_ops += 1
        self.successful_ops += 1
        self.latencies_ms.append(latency_ms)

    def record_failure(self) -> None:
        self.total_ops += 1
        self.failed_ops += 1

    def merge(self, other: PhaseMetrics) -> None:
        self.total_ops += other.total_ops
        self.successful_ops += other.successful_ops
        self.failed_ops += other.failed_ops
        self.latencies_ms.extend(other.latencies_ms)

    def finalize(self, elapsed_sec: float) -> None:
        self.elapsed_sec = elapsed_sec
        if elapsed_sec > 0:
            self.throughput_ops_sec = self.successful_ops / elapsed_sec

        if not self.latencies_ms:
            return

        sorted_latencies = sorted(self.latencies_ms)
        self.latency_min_ms = sorted_latencies[0]
        self.latency_max_ms = sorted_latencies[-1]
        self.latency_avg_ms = sum(sorted_latencies) / len(sorted_latencies)
        self.latency_p50_ms = _percentile(sorted_latencies, 50)
        self.latency_p95_ms = _percentile(sorted_latencies, 95)
        self.latency_p99_ms = _percentile(sorted_latencies, 99)


_console = Console()


def print_header(host: str, port: str, parallelism: int) -> None:
    table = Table.grid(padding=(0, 2))
    table.add_row("Target:", f"{host}:{port}")
    table.add_row("Parallelism:", str(parallelism))
    _console.print(
        Panel(table, title="KV-Store Benchmark Results", border_style="cyan")
    )


def print_phase_report(metrics: PhaseMetrics) -> None:
    phase_label = metrics.phase_name.upper()
    _console.print(
        f"\n[bold]── Phase: {phase_label} "
        f"({metrics.duration_sec:.0f} s target) ──[/bold]"
    )

    table = Table(show_header=False, box=None, padding=(0, 2))
    table.add_row(
        "Operations:",
        (f"{metrics.total_ops:,} total │ "
         f"{metrics.successful_ops:,} success │ "
         f"{metrics.failed_ops:,} errors"),
    )
    table.add_row("Throughput:", f"{metrics.throughput_ops_sec:.2f} ops/sec")
    if metrics.latencies_ms:
        table.add_row(
            "Latency (ms):",
            (f"avg {metrics.latency_avg_ms:6.1f} │ "
             f"p50 {metrics.latency_p50_ms:6.1f} │ "
             f"p95 {metrics.latency_p95_ms:6.1f} │ "
             f"p99 {metrics.latency_p99_ms:6.1f}"),
        )
    else:
        table.add_row("Latency (ms):", "n/a (no successful operations)")
    _console.print(table)


def print_combined_report(
    create_metrics: Optional[PhaseMetrics],
    update_metrics: Optional[PhaseMetrics],
) -> None:
    if create_metrics is None and update_metrics is None:
        return

    total_ops = 0
    total_time = 0.0
    total_success = 0

    if create_metrics is not None:
        total_ops += create_metrics.total_ops
        total_time += create_metrics.elapsed_sec
        total_success += create_metrics.successful_ops
    if update_metrics is not None:
        total_ops += update_metrics.total_ops
        total_time += update_metrics.elapsed_sec
        total_success += update_metrics.successful_ops

    avg_throughput = total_success / total_time if total_time > 0 else 0.0

    _console.print("\n[bold]── Combined ──[/bold]")
    _console.print(
        f"  Total ops: {total_ops:,} │ Total time: {total_time:.1f} s │ "
        f"Avg throughput: {avg_throughput:.2f} ops/sec"
    )
