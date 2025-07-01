"""Data chunking with size negotiation and flow control for efficient transfer."""

import asyncio
import time
from collections import deque
from dataclasses import dataclass
from io import BytesIO
from typing import AsyncIterator, Deque, Dict, Iterator, Optional, Tuple


@dataclass
class FlowControlConfig:
    """Configuration for flow control and congestion management."""

    initial_window_size: int = 4  # Initial number of chunks in flight
    max_window_size: int = 64  # Maximum window size
    min_window_size: int = 1  # Minimum window size
    congestion_threshold: float = 0.8  # Threshold for congestion detection
    rtt_history_size: int = 20  # Number of RTT samples to keep
    adaptive_chunking: bool = True  # Enable adaptive chunk sizing
    bandwidth_estimation: bool = True  # Enable bandwidth estimation


@dataclass
class TransferMetrics:
    """Metrics for transfer performance monitoring."""

    bytes_sent: int = 0
    bytes_acked: int = 0
    chunks_sent: int = 0
    chunks_acked: int = 0
    rtt_samples: Optional[Deque[float]] = None
    bandwidth_samples: Optional[Deque[float]] = None
    congestion_events: int = 0

    def __post_init__(self) -> None:
        if self.rtt_samples is None:
            self.rtt_samples = deque(maxlen=20)
        if self.bandwidth_samples is None:
            self.bandwidth_samples = deque(maxlen=10)


class DataChunker:
    """Data chunking with negotiable chunk sizes and flow control.

    Provides:
    - Default 16 KiB chunks with negotiation up to 64 KiB
    - Efficient streaming chunking for large files
    - Chunk size negotiation protocol
    - Memory-efficient chunk iteration
    - Adaptive windowing for congestion control
    - Bandwidth-delay product (BDP) based throttling
    - RTT-aware flow control
    """

    DEFAULT_CHUNK_SIZE = 16 * 1024  # 16 KiB
    MAX_CHUNK_SIZE = 64 * 1024  # 64 KiB
    MIN_CHUNK_SIZE = 4 * 1024  # 4 KiB

    def __init__(
        self, chunk_size: int = DEFAULT_CHUNK_SIZE, flow_config: Optional[FlowControlConfig] = None
    ):
        if not (self.MIN_CHUNK_SIZE <= chunk_size <= self.MAX_CHUNK_SIZE):
            raise ValueError(
                f"Chunk size must be between {self.MIN_CHUNK_SIZE} and {self.MAX_CHUNK_SIZE}"
            )
        self.chunk_size = chunk_size
        self.flow_config = flow_config or FlowControlConfig()
        self.metrics = TransferMetrics()

        # Flow control state
        self._window_size = self.flow_config.initial_window_size
        self._in_flight = 0
        self._last_ack_time = time.time()
        self._congestion_window = self.flow_config.initial_window_size
        self._slow_start_threshold = self.flow_config.max_window_size // 2
        self._in_slow_start = True

        # Chunk acknowledgment tracking
        self._pending_chunks: Dict[int, float] = {}  # chunk_id -> send_time
        self._send_semaphore = asyncio.Semaphore(self._window_size)

    async def negotiate_chunk_size(self, proposed_size: int, peer_max_size: int) -> int:
        """Negotiate chunk size with peer.

        Args:
            proposed_size: Our proposed chunk size
            peer_max_size: Peer's maximum supported chunk size

        Returns:
            Agreed chunk size
        """
        if not (self.MIN_CHUNK_SIZE <= proposed_size <= self.MAX_CHUNK_SIZE):
            proposed_size = self.DEFAULT_CHUNK_SIZE

        if not (self.MIN_CHUNK_SIZE <= peer_max_size <= self.MAX_CHUNK_SIZE):
            peer_max_size = self.DEFAULT_CHUNK_SIZE

        agreed_size = min(proposed_size, peer_max_size)

        if agreed_size < self.MIN_CHUNK_SIZE:
            agreed_size = self.MIN_CHUNK_SIZE
        elif agreed_size > self.MAX_CHUNK_SIZE:
            agreed_size = self.MAX_CHUNK_SIZE

        self.chunk_size = agreed_size
        return agreed_size

    def chunk_data(self, data: bytes) -> Iterator[Tuple[int, bytes]]:
        """Split data into chunks.

        Args:
            data: Data to chunk

        Yields:
            Tuples of (chunk_index, chunk_data)
        """
        for i in range(0, len(data), self.chunk_size):
            chunk_index = i // self.chunk_size
            chunk_data = data[i : i + self.chunk_size]
            yield chunk_index, chunk_data

    async def chunk_stream_with_flow_control(
        self, stream: AsyncIterator[bytes]
    ) -> AsyncIterator[Tuple[int, bytes]]:
        """Chunk data from an async stream with flow control.

        Args:
            stream: Async iterator of data bytes

        Yields:
            Tuples of (chunk_index, chunk_data)
        """
        chunk_index = 0
        buffer = BytesIO()

        async for data in stream:
            buffer.write(data)

            while buffer.tell() >= self.chunk_size:
                # Wait for flow control window
                await self._send_semaphore.acquire()

                buffer.seek(0)
                chunk_data = buffer.read(self.chunk_size)

                remaining_data = buffer.read()
                buffer = BytesIO()
                buffer.write(remaining_data)

                # Track chunk for acknowledgment
                send_time = time.time()
                self._pending_chunks[chunk_index] = send_time
                self._in_flight += 1
                self.metrics.chunks_sent += 1
                self.metrics.bytes_sent += len(chunk_data)

                yield chunk_index, chunk_data
                chunk_index += 1

        if buffer.tell() > 0:
            await self._send_semaphore.acquire()

            buffer.seek(0)
            final_chunk = buffer.read()

            send_time = time.time()
            self._pending_chunks[chunk_index] = send_time
            self._in_flight += 1
            self.metrics.chunks_sent += 1
            self.metrics.bytes_sent += len(final_chunk)

            yield chunk_index, final_chunk

    async def chunk_stream(self, stream: AsyncIterator[bytes]) -> AsyncIterator[Tuple[int, bytes]]:
        """Chunk data from an async stream (legacy compatibility).

        Args:
            stream: Async iterator of data bytes

        Yields:
            Tuples of (chunk_index, chunk_data)
        """
        if self.flow_config.adaptive_chunking:
            async for chunk in self.chunk_stream_with_flow_control(stream):
                yield chunk
        else:
            # Original implementation for backward compatibility
            chunk_index = 0
            buffer = BytesIO()

            async for data in stream:
                buffer.write(data)

                while buffer.tell() >= self.chunk_size:
                    buffer.seek(0)
                    chunk_data = buffer.read(self.chunk_size)

                    remaining_data = buffer.read()
                    buffer = BytesIO()
                    buffer.write(remaining_data)

                    yield chunk_index, chunk_data
                    chunk_index += 1

            if buffer.tell() > 0:
                buffer.seek(0)
                final_chunk = buffer.read()
                yield chunk_index, final_chunk

    def chunk_file(self, file_path: str) -> Iterator[Tuple[int, bytes]]:
        """Chunk data from a file.

        Args:
            file_path: Path to file to chunk

        Yields:
            Tuples of (chunk_index, chunk_data)
        """
        chunk_index = 0
        with open(file_path, "rb") as f:
            while True:
                chunk_data = f.read(self.chunk_size)
                if not chunk_data:
                    break
                yield chunk_index, chunk_data
                chunk_index += 1

    def reassemble_chunks(self, chunks: Iterator[Tuple[int, bytes]]) -> bytes:
        """Reassemble chunks into original data.

        Args:
            chunks: Iterator of (chunk_index, chunk_data) tuples

        Returns:
            Reassembled data
        """
        chunk_dict = {}
        max_index = -1

        for chunk_index, chunk_data in chunks:
            chunk_dict[chunk_index] = chunk_data
            max_index = max(max_index, chunk_index)

        if max_index == -1:
            return b""

        result = BytesIO()
        for i in range(max_index + 1):
            if i in chunk_dict:
                result.write(chunk_dict[i])
            else:
                raise ValueError(f"Missing chunk at index {i}")

        return result.getvalue()

    async def acknowledge_chunk(self, chunk_id: int) -> None:
        """Acknowledge receipt of a chunk and update flow control.

        Args:
            chunk_id: ID of the acknowledged chunk
        """
        if chunk_id not in self._pending_chunks:
            return

        # Calculate RTT
        send_time = self._pending_chunks.pop(chunk_id)
        rtt = time.time() - send_time
        self.metrics.rtt_samples.append(rtt)

        # Update metrics
        self.metrics.chunks_acked += 1
        self._in_flight -= 1
        self._last_ack_time = time.time()

        # Release semaphore
        self._send_semaphore.release()

        # Update congestion window based on current state
        if self._in_slow_start:
            # Slow start: increase window by 1 for each ACK
            self._congestion_window += 1
            if self._congestion_window >= self._slow_start_threshold:
                self._in_slow_start = False
        else:
            # Congestion avoidance: increase window by 1/window per ACK
            self._congestion_window += 1.0 / self._congestion_window

        # Cap the window size
        self._congestion_window = min(self._congestion_window, self.flow_config.max_window_size)

        # Update semaphore if window expanded
        new_window_size = int(self._congestion_window)
        if new_window_size > self._window_size:
            for _ in range(new_window_size - self._window_size):
                self._send_semaphore.release()
            self._window_size = new_window_size

        # Estimate bandwidth if enabled
        if self.flow_config.bandwidth_estimation and len(self.metrics.rtt_samples) > 0:
            self._estimate_bandwidth()

    def handle_congestion(self) -> None:
        """Handle congestion events by reducing window size."""
        self.metrics.congestion_events += 1

        # Reduce congestion window (multiplicative decrease)
        self._slow_start_threshold = max(
            self._congestion_window // 2, self.flow_config.min_window_size
        )
        self._congestion_window = self._slow_start_threshold
        self._in_slow_start = False

        # Update window size immediately
        new_window = max(int(self._congestion_window), self.flow_config.min_window_size)
        if new_window < self._window_size:
            # Drain excess permits from semaphore
            excess = self._window_size - new_window
            for _ in range(excess):
                try:
                    if not self._send_semaphore.locked():
                        self._send_semaphore.acquire_nowait()
                except ValueError:
                    break
            self._window_size = new_window

    def _estimate_bandwidth(self) -> None:
        """Estimate available bandwidth based on recent RTT samples."""
        if not self.metrics.rtt_samples:
            return

        # Simple bandwidth estimation: bytes_acked / average_rtt
        avg_rtt = sum(self.metrics.rtt_samples) / len(self.metrics.rtt_samples)
        if avg_rtt > 0:
            estimated_bw = self.metrics.bytes_acked / avg_rtt
            self.metrics.bandwidth_samples.append(estimated_bw)

            # Adjust chunk size based on bandwidth-delay product
            if self.flow_config.adaptive_chunking and len(self.metrics.bandwidth_samples) >= 3:
                self._adapt_chunk_size(estimated_bw, avg_rtt)

    def _adapt_chunk_size(self, bandwidth: float, rtt: float) -> None:
        """Adapt chunk size based on network conditions.

        Args:
            bandwidth: Estimated bandwidth in bytes/second
            rtt: Average round-trip time in seconds
        """
        # Calculate bandwidth-delay product
        bdp = int(bandwidth * rtt)

        # Suggest optimal chunk size (1/10 of BDP, within bounds)
        optimal_chunk_size = max(self.MIN_CHUNK_SIZE, min(bdp // 10, self.MAX_CHUNK_SIZE))

        # Gradually adjust chunk size
        if optimal_chunk_size > self.chunk_size:
            self.chunk_size = min(self.chunk_size + 1024, optimal_chunk_size)
        elif optimal_chunk_size < self.chunk_size:
            self.chunk_size = max(self.chunk_size - 1024, optimal_chunk_size)

    def get_flow_control_stats(self) -> Dict[str, float]:
        """Get current flow control statistics.

        Returns:
            Dictionary with flow control metrics
        """
        avg_rtt = 0.0
        if self.metrics.rtt_samples:
            avg_rtt = sum(self.metrics.rtt_samples) / len(self.metrics.rtt_samples)

        avg_bandwidth = 0.0
        if self.metrics.bandwidth_samples:
            avg_bandwidth = sum(self.metrics.bandwidth_samples) / len(
                self.metrics.bandwidth_samples
            )

        return {
            "window_size": self._window_size,
            "congestion_window": self._congestion_window,
            "in_flight": self._in_flight,
            "in_slow_start": self._in_slow_start,
            "slow_start_threshold": self._slow_start_threshold,
            "avg_rtt": avg_rtt,
            "estimated_bandwidth": avg_bandwidth,
            "chunk_size": self.chunk_size,
            "congestion_events": self.metrics.congestion_events,
            "chunks_sent": self.metrics.chunks_sent,
            "chunks_acked": self.metrics.chunks_acked,
            "bytes_sent": self.metrics.bytes_sent,
            "bytes_acked": self.metrics.bytes_acked,
        }

    def reset_flow_control(self) -> None:
        """Reset flow control state for new transfer."""
        self._window_size = self.flow_config.initial_window_size
        self._in_flight = 0
        self._congestion_window = self.flow_config.initial_window_size
        self._slow_start_threshold = self.flow_config.max_window_size // 2
        self._in_slow_start = True
        self._pending_chunks.clear()
        self.metrics = TransferMetrics()

        # Reset semaphore
        self._send_semaphore = asyncio.Semaphore(self._window_size)

    @classmethod
    def calculate_chunk_count(cls, data_size: int, chunk_size: int = DEFAULT_CHUNK_SIZE) -> int:
        """Calculate number of chunks for given data size.

        Args:
            data_size: Total size of data
            chunk_size: Size of each chunk

        Returns:
            Number of chunks needed
        """
        return (data_size + chunk_size - 1) // chunk_size
