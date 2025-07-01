"""Self-healing and resilience policies for peer connections."""

import asyncio
import logging
import time
from dataclasses import dataclass, field
from enum import Enum
from typing import Any, Callable, Dict, List, Optional, Set

logger = logging.getLogger(__name__)


class PeerState(Enum):
    """Peer connection states."""

    UNKNOWN = "unknown"
    CONNECTING = "connecting"
    CONNECTED = "connected"
    DISCONNECTED = "disconnected"
    FAILED = "failed"
    BLACKLISTED = "blacklisted"


class ReconnectionReason(Enum):
    """Reasons for peer reconnection attempts."""

    INITIAL_CONNECTION = "initial_connection"
    CONNECTION_LOST = "connection_lost"
    TIMEOUT = "timeout"
    TRANSFER_FAILED = "transfer_failed"
    MANUAL_RETRY = "manual_retry"
    SCHEDULED_MAINTENANCE = "scheduled_maintenance"


@dataclass
class PeerScore:
    """Scoring system for peer reliability and performance."""

    connection_success_rate: float = 1.0  # 0.0 to 1.0
    average_latency: float = 0.0  # milliseconds
    transfer_success_rate: float = 1.0  # 0.0 to 1.0
    uptime_ratio: float = 1.0  # 0.0 to 1.0
    last_seen: float = 0.0  # timestamp

    # Connection statistics
    total_connections: int = 0
    successful_connections: int = 0
    failed_connections: int = 0

    # Transfer statistics
    total_transfers: int = 0
    successful_transfers: int = 0
    failed_transfers: int = 0

    # Latency samples
    latency_samples: List[float] = field(default_factory=list)

    def update_connection_attempt(self, success: bool, latency: Optional[float] = None) -> None:
        """Update statistics for a connection attempt.

        Args:
            success: Whether the connection succeeded
            latency: Connection latency in milliseconds
        """
        self.total_connections += 1

        if success:
            self.successful_connections += 1
            self.last_seen = time.time()

            if latency is not None:
                self.latency_samples.append(latency)
                # Keep only recent samples
                if len(self.latency_samples) > 20:
                    self.latency_samples = self.latency_samples[-20:]
                self.average_latency = sum(self.latency_samples) / len(self.latency_samples)
        else:
            self.failed_connections += 1

        # Update success rate
        if self.total_connections > 0:
            self.connection_success_rate = self.successful_connections / self.total_connections

    def update_transfer_attempt(self, success: bool) -> None:
        """Update statistics for a transfer attempt.

        Args:
            success: Whether the transfer succeeded
        """
        self.total_transfers += 1

        if success:
            self.successful_transfers += 1
        else:
            self.failed_transfers += 1

        # Update transfer success rate
        if self.total_transfers > 0:
            self.transfer_success_rate = self.successful_transfers / self.total_transfers

    def calculate_overall_score(self) -> float:
        """Calculate overall peer score (0.0 to 1.0).

        Returns:
            Weighted score based on various metrics
        """
        # Weights for different factors
        connection_weight = 0.3
        transfer_weight = 0.3
        latency_weight = 0.2
        uptime_weight = 0.2

        # Normalize latency score (lower is better)
        latency_score = 1.0
        if self.average_latency > 0:
            # Assume 100ms is excellent, 1000ms is poor
            latency_score = max(0.0, 1.0 - (self.average_latency / 1000.0))

        # Calculate weighted score
        score = (
            self.connection_success_rate * connection_weight
            + self.transfer_success_rate * transfer_weight
            + latency_score * latency_weight
            + self.uptime_ratio * uptime_weight
        )

        return max(0.0, min(1.0, score))

    def should_blacklist(self) -> bool:
        """Determine if peer should be blacklisted.

        Returns:
            True if peer should be blacklisted
        """
        # Blacklist criteria
        if self.total_connections >= 10:
            if self.connection_success_rate < 0.1:  # Less than 10% success rate
                return True

        if self.total_transfers >= 5:
            if self.transfer_success_rate < 0.2:  # Less than 20% transfer success
                return True

        # Check if peer has been offline for too long
        time_since_seen = time.time() - self.last_seen
        if time_since_seen > 86400:  # 24 hours
            return True

        return False


@dataclass
class ReconnectionPolicy:
    """Policy for peer reconnection behavior."""

    max_attempts: int = 10  # Maximum reconnection attempts
    initial_delay: float = 1.0  # Initial delay in seconds
    max_delay: float = 300.0  # Maximum delay in seconds (5 minutes)
    backoff_multiplier: float = 2.0  # Exponential backoff multiplier
    jitter: bool = True  # Add random jitter to delays

    # Scoring thresholds
    min_score_for_retry: float = 0.1  # Minimum score to retry connection
    blacklist_threshold: float = 0.05  # Score below which peer is blacklisted

    # Time windows
    failure_window: float = 3600.0  # Window for counting failures (1 hour)
    blacklist_duration: float = 86400.0  # How long to blacklist peer (24 hours)

    def calculate_delay(self, attempt: int) -> float:
        """Calculate delay for reconnection attempt.

        Args:
            attempt: Attempt number (0-based)

        Returns:
            Delay in seconds
        """
        if attempt >= self.max_attempts:
            return float("inf")  # No more attempts

        delay = self.initial_delay * (self.backoff_multiplier**attempt)
        delay = min(delay, self.max_delay)

        if self.jitter:
            import random

            # Add ±25% jitter
            jitter_factor = 0.75 + (random.random() * 0.5)
            delay *= jitter_factor

        return delay


# Type aliases for policy callbacks
PolicyCallback = Callable[[str, PeerScore, ReconnectionReason], bool]
ScoreUpdateCallback = Callable[[str, PeerScore], None]
BlacklistCallback = Callable[[str, PeerScore], None]


class PeerResilienceManager:
    """Manager for peer resilience and self-healing policies.

    Provides:
    - Exponential backoff for reconnection attempts
    - Peer scoring and reliability tracking
    - Configurable blacklisting policies
    - Policy callbacks for customization
    - Automatic connection health monitoring
    """

    def __init__(self, policy: Optional[ReconnectionPolicy] = None):
        self.policy = policy or ReconnectionPolicy()
        self.peer_scores: Dict[str, PeerScore] = {}
        self.peer_states: Dict[str, PeerState] = {}
        self.reconnection_tasks: Dict[str, asyncio.Task] = {}
        self.blacklisted_peers: Dict[str, float] = {}  # peer_id -> blacklist_time

        # Policy callbacks
        self.should_reconnect_callback: Optional[PolicyCallback] = None
        self.score_update_callback: Optional[ScoreUpdateCallback] = None
        self.blacklist_callback: Optional[BlacklistCallback] = None

        # Background tasks
        self._cleanup_task: Optional[asyncio.Task] = None
        self._running = False

    async def start(self) -> None:
        """Start the resilience manager."""
        if self._running:
            return

        self._running = True
        self._cleanup_task = asyncio.create_task(self._cleanup_loop())
        logger.info("Peer resilience manager started")

    async def stop(self) -> None:
        """Stop the resilience manager."""
        if not self._running:
            return

        self._running = False

        # Cancel all reconnection tasks
        for task in self.reconnection_tasks.values():
            task.cancel()
        self.reconnection_tasks.clear()

        # Cancel cleanup task
        if self._cleanup_task:
            self._cleanup_task.cancel()
            self._cleanup_task = None

        logger.info("Peer resilience manager stopped")

    def set_policy_callbacks(
        self,
        should_reconnect: Optional[PolicyCallback] = None,
        score_update: Optional[ScoreUpdateCallback] = None,
        blacklist: Optional[BlacklistCallback] = None,
    ) -> None:
        """Set policy callbacks for customization.

        Args:
            should_reconnect: Callback to determine if reconnection should occur
            score_update: Callback called when peer scores are updated
            blacklist: Callback called when peers are blacklisted
        """
        self.should_reconnect_callback = should_reconnect
        self.score_update_callback = score_update
        self.blacklist_callback = blacklist

    def get_peer_score(self, peer_id: str) -> PeerScore:
        """Get or create peer score.

        Args:
            peer_id: Peer identifier

        Returns:
            Peer score object
        """
        if peer_id not in self.peer_scores:
            self.peer_scores[peer_id] = PeerScore()
        return self.peer_scores[peer_id]

    def update_peer_state(self, peer_id: str, state: PeerState) -> None:
        """Update peer connection state.

        Args:
            peer_id: Peer identifier
            state: New peer state
        """
        old_state = self.peer_states.get(peer_id, PeerState.UNKNOWN)
        self.peer_states[peer_id] = state

        logger.debug(f"Peer {peer_id} state: {old_state} -> {state}")

        # Update score based on state change
        score = self.get_peer_score(peer_id)

        if state == PeerState.CONNECTED and old_state in [
            PeerState.CONNECTING,
            PeerState.DISCONNECTED,
        ]:
            score.update_connection_attempt(True)
        elif state == PeerState.FAILED:
            score.update_connection_attempt(False)

        # Check for blacklisting
        if (
            score.should_blacklist()
            and score.calculate_overall_score() < self.policy.blacklist_threshold
        ):
            self._blacklist_peer(peer_id, score)

        # Trigger score update callback
        if self.score_update_callback:
            self.score_update_callback(peer_id, score)

    def record_connection_attempt(
        self, peer_id: str, success: bool, latency: Optional[float] = None
    ) -> None:
        """Record a connection attempt result.

        Args:
            peer_id: Peer identifier
            success: Whether the connection succeeded
            latency: Connection latency in milliseconds
        """
        score = self.get_peer_score(peer_id)
        score.update_connection_attempt(success, latency)

        if success:
            self.update_peer_state(peer_id, PeerState.CONNECTED)
        else:
            self.update_peer_state(peer_id, PeerState.FAILED)

        # Trigger score update callback
        if self.score_update_callback:
            self.score_update_callback(peer_id, score)

    def record_transfer_attempt(self, peer_id: str, success: bool) -> None:
        """Record a transfer attempt result.

        Args:
            peer_id: Peer identifier
            success: Whether the transfer succeeded
        """
        score = self.get_peer_score(peer_id)
        score.update_transfer_attempt(success)

        # Trigger score update callback
        if self.score_update_callback:
            self.score_update_callback(peer_id, score)

    def is_blacklisted(self, peer_id: str) -> bool:
        """Check if a peer is blacklisted.

        Args:
            peer_id: Peer identifier

        Returns:
            True if peer is blacklisted
        """
        if peer_id not in self.blacklisted_peers:
            return False

        blacklist_time = self.blacklisted_peers[peer_id]
        if time.time() - blacklist_time > self.policy.blacklist_duration:
            # Blacklist expired
            del self.blacklisted_peers[peer_id]
            logger.info(f"Peer {peer_id} removed from blacklist")
            return False

        return True

    async def schedule_reconnection(self, peer_id: str, reason: ReconnectionReason) -> bool:
        """Schedule a reconnection attempt.

        Args:
            peer_id: Peer identifier
            reason: Reason for reconnection

        Returns:
            True if reconnection was scheduled
        """
        if self.is_blacklisted(peer_id):
            logger.debug(f"Skipping reconnection to blacklisted peer {peer_id}")
            return False

        score = self.get_peer_score(peer_id)

        # Check if reconnection should proceed
        should_reconnect = True
        if self.should_reconnect_callback:
            should_reconnect = self.should_reconnect_callback(peer_id, score, reason)
        elif score.calculate_overall_score() < self.policy.min_score_for_retry:
            should_reconnect = False

        if not should_reconnect:
            logger.debug(f"Reconnection to {peer_id} skipped by policy")
            return False

        # Cancel existing reconnection task
        if peer_id in self.reconnection_tasks:
            self.reconnection_tasks[peer_id].cancel()

        # Schedule new reconnection
        task = asyncio.create_task(self._reconnection_loop(peer_id, reason))
        self.reconnection_tasks[peer_id] = task

        return True

    def cancel_reconnection(self, peer_id: str) -> None:
        """Cancel scheduled reconnection for a peer.

        Args:
            peer_id: Peer identifier
        """
        if peer_id in self.reconnection_tasks:
            self.reconnection_tasks[peer_id].cancel()
            del self.reconnection_tasks[peer_id]

    def get_statistics(self) -> Dict[str, Any]:
        """Get resilience manager statistics.

        Returns:
            Dictionary with statistics
        """
        total_peers = len(self.peer_scores)
        blacklisted_count = len(self.blacklisted_peers)
        active_reconnections = len(self.reconnection_tasks)

        # Calculate average scores
        if self.peer_scores:
            avg_score = sum(
                score.calculate_overall_score() for score in self.peer_scores.values()
            ) / len(self.peer_scores)
            avg_connection_rate = sum(
                score.connection_success_rate for score in self.peer_scores.values()
            ) / len(self.peer_scores)
            avg_transfer_rate = sum(
                score.transfer_success_rate for score in self.peer_scores.values()
            ) / len(self.peer_scores)
        else:
            avg_score = avg_connection_rate = avg_transfer_rate = 0.0

        return {
            "total_peers": total_peers,
            "blacklisted_peers": blacklisted_count,
            "active_reconnections": active_reconnections,
            "average_peer_score": avg_score,
            "average_connection_success_rate": avg_connection_rate,
            "average_transfer_success_rate": avg_transfer_rate,
            "policy": {
                "max_attempts": self.policy.max_attempts,
                "max_delay": self.policy.max_delay,
                "blacklist_threshold": self.policy.blacklist_threshold,
                "blacklist_duration": self.policy.blacklist_duration,
            },
        }

    def _blacklist_peer(self, peer_id: str, score: PeerScore) -> None:
        """Blacklist a peer.

        Args:
            peer_id: Peer identifier
            score: Peer score
        """
        self.blacklisted_peers[peer_id] = time.time()
        self.update_peer_state(peer_id, PeerState.BLACKLISTED)

        # Cancel any ongoing reconnection
        self.cancel_reconnection(peer_id)

        logger.warning(f"Peer {peer_id} blacklisted (score: {score.calculate_overall_score():.3f})")

        # Trigger blacklist callback
        if self.blacklist_callback:
            self.blacklist_callback(peer_id, score)

    async def _reconnection_loop(self, peer_id: str, reason: ReconnectionReason) -> None:
        """Reconnection loop with exponential backoff.

        Args:
            peer_id: Peer identifier
            reason: Reason for reconnection
        """
        attempt = 0

        while attempt < self.policy.max_attempts and not self.is_blacklisted(peer_id):
            if attempt > 0:
                delay = self.policy.calculate_delay(attempt - 1)
                if delay == float("inf"):
                    break

                logger.debug(
                    f"Waiting {delay:.1f}s before reconnection attempt {attempt + 1} to {peer_id}"
                )
                try:
                    await asyncio.sleep(delay)
                except asyncio.CancelledError:
                    return

            # Update state to connecting
            self.update_peer_state(peer_id, PeerState.CONNECTING)

            # Attempt reconnection (this would be implemented by the connection manager)
            # For now, just simulate the attempt
            logger.info(
                f"Attempting reconnection {attempt + 1}/{self.policy.max_attempts} to {peer_id} (reason: {reason.value})"
            )

            # TODO: Actually attempt connection here
            # success = await connection_manager.connect_to_peer(peer_id)

            # For simulation, assume attempt fails but don't update score here
            # The actual connection manager should call record_connection_attempt()

            attempt += 1

        # Clean up task reference
        if peer_id in self.reconnection_tasks:
            del self.reconnection_tasks[peer_id]

        logger.info(f"Reconnection attempts to {peer_id} completed")

    async def _cleanup_loop(self) -> None:
        """Background cleanup loop."""
        while self._running:
            try:
                # Clean up completed reconnection tasks
                completed_tasks = [
                    peer_id for peer_id, task in self.reconnection_tasks.items() if task.done()
                ]

                for peer_id in completed_tasks:
                    del self.reconnection_tasks[peer_id]

                # Clean up expired blacklist entries
                current_time = time.time()
                expired_blacklists = [
                    peer_id
                    for peer_id, blacklist_time in self.blacklisted_peers.items()
                    if current_time - blacklist_time > self.policy.blacklist_duration
                ]

                for peer_id in expired_blacklists:
                    del self.blacklisted_peers[peer_id]
                    logger.info(f"Peer {peer_id} removed from blacklist")

                # Sleep for cleanup interval
                await asyncio.sleep(60)  # Clean up every minute

            except asyncio.CancelledError:
                break
            except Exception as e:
                logger.error(f"Error in cleanup loop: {e}")
