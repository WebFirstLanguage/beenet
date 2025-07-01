"""Observability and monitoring for beenet with structured logging and Prometheus metrics."""

import logging
import time
from contextlib import contextmanager
from dataclasses import dataclass
from typing import Any, Dict, Generator, Optional

try:
    import structlog
    from structlog.stdlib import LoggerFactory
except ImportError:
    structlog = None

try:
    from prometheus_client import Counter, Gauge, Histogram, start_http_server
except ImportError:
    Counter = Gauge = Histogram = start_http_server = None


@dataclass
class MetricsConfig:
    """Configuration for metrics collection."""

    enable_prometheus: bool = True
    prometheus_port: int = 8000
    enable_structured_logging: bool = True
    log_level: str = "INFO"
    log_format: str = "json"  # "json" or "console"


class BeenetMetrics:
    """Prometheus metrics for beenet P2P networking."""

    def __init__(self, enabled: bool = True):
        self.enabled = enabled and Counter is not None

        if not self.enabled:
            return

        # Connection metrics
        self.connections_total = Counter(
            "beenet_connections_total",
            "Total number of connection attempts",
            ["peer_id", "result"],  # result: success, failure, timeout
        )

        self.active_connections = Gauge(
            "beenet_active_connections", "Number of currently active connections"
        )

        self.connection_duration = Histogram(
            "beenet_connection_duration_seconds",
            "Time spent establishing connections",
            buckets=(0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0, 60.0, 120.0, float("inf")),
        )

        # Transfer metrics
        self.transfers_total = Counter(
            "beenet_transfers_total",
            "Total number of transfer attempts",
            ["peer_id", "direction", "result"],  # direction: send, receive
        )

        self.transfer_bytes = Counter(
            "beenet_transfer_bytes_total", "Total bytes transferred", ["peer_id", "direction"]
        )

        self.transfer_duration = Histogram(
            "beenet_transfer_duration_seconds",
            "Time spent on transfers",
            buckets=(1.0, 5.0, 10.0, 30.0, 60.0, 300.0, 600.0, 1800.0, 3600.0, float("inf")),
        )

        self.active_transfers = Gauge(
            "beenet_active_transfers", "Number of currently active transfers"
        )

        # Network metrics
        self.network_rtt = Histogram(
            "beenet_network_rtt_seconds",
            "Round-trip time for network operations",
            ["peer_id", "operation"],
            buckets=(
                0.001,
                0.005,
                0.01,
                0.025,
                0.05,
                0.1,
                0.25,
                0.5,
                1.0,
                2.5,
                5.0,
                10.0,
                float("inf"),
            ),
        )

        self.network_errors = Counter(
            "beenet_network_errors_total", "Total network errors", ["peer_id", "error_type"]
        )

        # Discovery metrics
        self.peers_discovered = Counter(
            "beenet_peers_discovered_total",
            "Total peers discovered",
            ["discovery_method"],  # kademlia, beequiet, manual
        )

        self.active_peers = Gauge("beenet_active_peers", "Number of currently known active peers")

        # Cryptographic metrics
        self.crypto_operations = Counter(
            "beenet_crypto_operations_total",
            "Total cryptographic operations",
            ["operation", "result"],  # operation: encrypt, decrypt, sign, verify
        )

        self.crypto_duration = Histogram(
            "beenet_crypto_duration_seconds",
            "Time spent on cryptographic operations",
            ["operation"],
            buckets=(0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, float("inf")),
        )

        # Merkle tree verification metrics
        self.merkle_verifications = Counter(
            "beenet_merkle_verifications_total",
            "Total Merkle tree verifications",
            ["result"],  # success, failure, recovered
        )

        self.merkle_recovery_attempts = Counter(
            "beenet_merkle_recovery_attempts_total",
            "Reed-Solomon recovery attempts",
            ["result"],  # success, failure
        )

        # Flow control metrics
        self.congestion_events = Counter(
            "beenet_congestion_events_total", "Total congestion events", ["peer_id"]
        )

        self.window_size = Gauge(
            "beenet_window_size", "Current congestion window size", ["peer_id"]
        )

        # NAT traversal metrics
        self.nat_traversal_attempts = Counter(
            "beenet_nat_traversal_attempts_total",
            "NAT traversal attempts",
            ["method", "result"],  # method: stun, turn, ice; result: success, failure
        )

    def record_connection_attempt(
        self, peer_id: str, success: bool, duration: Optional[float] = None
    ) -> None:
        """Record a connection attempt."""
        if not self.enabled:
            return

        result = "success" if success else "failure"
        self.connections_total.labels(peer_id=peer_id, result=result).inc()

        if duration is not None:
            self.connection_duration.observe(duration)

    def record_transfer(
        self, peer_id: str, direction: str, bytes_transferred: int, duration: float, success: bool
    ) -> None:
        """Record a transfer operation."""
        if not self.enabled:
            return

        result = "success" if success else "failure"
        self.transfers_total.labels(peer_id=peer_id, direction=direction, result=result).inc()
        self.transfer_bytes.labels(peer_id=peer_id, direction=direction).inc(bytes_transferred)
        self.transfer_duration.observe(duration)

    def record_rtt(self, peer_id: str, operation: str, rtt: float) -> None:
        """Record round-trip time."""
        if not self.enabled:
            return

        self.network_rtt.labels(peer_id=peer_id, operation=operation).observe(rtt)

    def record_error(self, peer_id: str, error_type: str) -> None:
        """Record a network error."""
        if not self.enabled:
            return

        self.network_errors.labels(peer_id=peer_id, error_type=error_type).inc()

    def record_peer_discovery(self, method: str, count: int = 1) -> None:
        """Record peer discovery."""
        if not self.enabled:
            return

        self.peers_discovered.labels(discovery_method=method).inc(count)

    def record_crypto_operation(self, operation: str, duration: float, success: bool) -> None:
        """Record cryptographic operation."""
        if not self.enabled:
            return

        result = "success" if success else "failure"
        self.crypto_operations.labels(operation=operation, result=result).inc()
        self.crypto_duration.labels(operation=operation).observe(duration)

    def record_merkle_verification(self, success: bool, recovered: bool = False) -> None:
        """Record Merkle tree verification."""
        if not self.enabled:
            return

        if recovered:
            result = "recovered"
        elif success:
            result = "success"
        else:
            result = "failure"

        self.merkle_verifications.labels(result=result).inc()

    def record_merkle_recovery(self, success: bool) -> None:
        """Record Reed-Solomon recovery attempt."""
        if not self.enabled:
            return

        result = "success" if success else "failure"
        self.merkle_recovery_attempts.labels(result=result).inc()

    def record_congestion_event(self, peer_id: str) -> None:
        """Record congestion event."""
        if not self.enabled:
            return

        self.congestion_events.labels(peer_id=peer_id).inc()

    def update_window_size(self, peer_id: str, size: int) -> None:
        """Update congestion window size."""
        if not self.enabled:
            return

        self.window_size.labels(peer_id=peer_id).set(size)

    def record_nat_traversal(self, method: str, success: bool) -> None:
        """Record NAT traversal attempt."""
        if not self.enabled:
            return

        result = "success" if success else "failure"
        self.nat_traversal_attempts.labels(method=method, result=result).inc()

    def update_active_connections(self, count: int) -> None:
        """Update active connection count."""
        if not self.enabled:
            return

        self.active_connections.set(count)

    def update_active_transfers(self, count: int) -> None:
        """Update active transfer count."""
        if not self.enabled:
            return

        self.active_transfers.set(count)

    def update_active_peers(self, count: int) -> None:
        """Update active peer count."""
        if not self.enabled:
            return

        self.active_peers.set(count)


class BeenetLogger:
    """Structured logger for beenet with contextual information."""

    def __init__(self, config: MetricsConfig):
        self.config = config
        self.logger = self._setup_logger()

    def _setup_logger(self) -> logging.Logger:
        """Set up structured logging."""
        logger = logging.getLogger("beenet")
        logger.setLevel(getattr(logging, self.config.log_level.upper()))

        if not self.config.enable_structured_logging or structlog is None:
            # Fall back to standard logging
            handler = logging.StreamHandler()
            formatter = logging.Formatter("%(asctime)s - %(name)s - %(levelname)s - %(message)s")
            handler.setFormatter(formatter)
            logger.addHandler(handler)
            return logger

        # Configure structlog
        if self.config.log_format == "json":
            processors = [
                structlog.stdlib.filter_by_level,
                structlog.stdlib.add_logger_name,
                structlog.stdlib.add_log_level,
                structlog.stdlib.PositionalArgumentsFormatter(),
                structlog.processors.TimeStamper(fmt="iso"),
                structlog.processors.StackInfoRenderer(),
                structlog.processors.format_exc_info,
                structlog.processors.UnicodeDecoder(),
                structlog.processors.JSONRenderer(),
            ]
        else:
            processors = [
                structlog.stdlib.filter_by_level,
                structlog.stdlib.add_logger_name,
                structlog.stdlib.add_log_level,
                structlog.stdlib.PositionalArgumentsFormatter(),
                structlog.processors.TimeStamper(fmt="%Y-%m-%d %H:%M:%S"),
                structlog.dev.ConsoleRenderer(),
            ]

        structlog.configure(
            processors=processors,
            context_class=dict,
            logger_factory=LoggerFactory(),
            wrapper_class=structlog.stdlib.BoundLogger,
            cache_logger_on_first_use=True,
        )

        return structlog.get_logger("beenet")

    def with_context(self, **kwargs) -> "BeenetLogger":
        """Create logger with additional context."""
        if hasattr(self.logger, "bind"):
            new_logger = BeenetLogger(self.config)
            new_logger.logger = self.logger.bind(**kwargs)
            return new_logger
        else:
            # Fallback for standard logging
            return self

    def info(self, message: str, **kwargs) -> None:
        """Log info message."""
        if hasattr(self.logger, "bind"):
            self.logger.info(message, **kwargs)
        else:
            self.logger.info(f"{message} {kwargs}")

    def warning(self, message: str, **kwargs) -> None:
        """Log warning message."""
        if hasattr(self.logger, "bind"):
            self.logger.warning(message, **kwargs)
        else:
            self.logger.warning(f"{message} {kwargs}")

    def error(self, message: str, **kwargs) -> None:
        """Log error message."""
        if hasattr(self.logger, "bind"):
            self.logger.error(message, **kwargs)
        else:
            self.logger.error(f"{message} {kwargs}")

    def debug(self, message: str, **kwargs) -> None:
        """Log debug message."""
        if hasattr(self.logger, "bind"):
            self.logger.debug(message, **kwargs)
        else:
            self.logger.debug(f"{message} {kwargs}")


class ObservabilityManager:
    """Central manager for beenet observability and monitoring."""

    def __init__(self, config: Optional[MetricsConfig] = None):
        self.config = config or MetricsConfig()
        self.metrics = BeenetMetrics(self.config.enable_prometheus)
        self.logger = BeenetLogger(self.config)
        self._http_server_started = False

    def start_metrics_server(self) -> None:
        """Start Prometheus metrics HTTP server."""
        if (
            not self.config.enable_prometheus
            or self._http_server_started
            or start_http_server is None
        ):
            return

        try:
            start_http_server(self.config.prometheus_port)
            self._http_server_started = True
            self.logger.info("Prometheus metrics server started", port=self.config.prometheus_port)
        except Exception as e:
            self.logger.error("Failed to start metrics server", error=str(e))

    @contextmanager
    def time_operation(
        self, operation: str, peer_id: Optional[str] = None
    ) -> Generator[Dict[str, Any], None, None]:
        """Context manager to time operations and record metrics."""
        start_time = time.time()
        context = {"operation": operation, "start_time": start_time}
        if peer_id:
            context["peer_id"] = peer_id

        try:
            yield context
            duration = time.time() - start_time
            context["duration"] = duration
            context["success"] = True

            self.logger.debug("Operation completed", **context)

        except Exception as e:
            duration = time.time() - start_time
            context["duration"] = duration
            context["success"] = False
            context["error"] = str(e)

            self.logger.error("Operation failed", **context)
            raise

    def get_logger(self, component: str) -> BeenetLogger:
        """Get logger for specific component."""
        return self.logger.with_context(component=component)


# Global observability instance
_observability: Optional[ObservabilityManager] = None


def setup_observability(config: Optional[MetricsConfig] = None) -> ObservabilityManager:
    """Set up global observability manager."""
    global _observability
    _observability = ObservabilityManager(config)
    return _observability


def get_observability() -> ObservabilityManager:
    """Get global observability manager."""
    global _observability
    if _observability is None:
        _observability = setup_observability()
    return _observability


def get_metrics() -> BeenetMetrics:
    """Get global metrics instance."""
    return get_observability().metrics


def get_logger(component: str = "beenet") -> BeenetLogger:
    """Get logger for component."""
    return get_observability().get_logger(component)
