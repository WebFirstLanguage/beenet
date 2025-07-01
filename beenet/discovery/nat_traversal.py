"""NAT traversal placeholder for future STUN/TURN support."""

from typing import Any, Dict, List, Optional

USE_STUN = False
USE_TURN = False


class NATTraversal:
    """Placeholder for future NAT traversal implementation.

    This module provides a feature-flagged interface for NAT traversal
    capabilities that will be implemented in v0.2. Currently disabled
    but provides extension points for STUN/TURN integration.
    """

    def __init__(
        self, stun_servers: Optional[List[str]] = None, turn_servers: Optional[List[str]] = None
    ):
        self.stun_servers = stun_servers or []
        self.turn_servers = turn_servers or []

    async def discover_external_address(self) -> Optional[Dict[str, Any]]:
        """Discover external IP address and port via STUN.

        Returns:
            External address info if STUN is enabled, None otherwise
        """
        if not USE_STUN:
            return None
        raise NotImplementedError("STUN support planned for v0.2")

    async def establish_relay(self) -> Optional[Dict[str, Any]]:
        """Establish TURN relay for NAT traversal.

        Returns:
            Relay connection info if TURN is enabled, None otherwise
        """
        if not USE_TURN:
            return None
        raise NotImplementedError("TURN support planned for v0.2")

    async def perform_hole_punching(self, peer_address: str, peer_port: int) -> bool:
        """Attempt UDP hole punching with peer.

        Args:
            peer_address: Peer's external address
            peer_port: Peer's external port

        Returns:
            True if hole punching succeeded
        """
        if not (USE_STUN or USE_TURN):
            return False
        raise NotImplementedError("Hole punching planned for v0.2")

    def is_nat_traversal_enabled(self) -> bool:
        """Check if any NAT traversal features are enabled.

        Returns:
            True if STUN or TURN is enabled
        """
        return USE_STUN or USE_TURN
