"""NAT traversal with STUN/TURN and ICE support."""

import asyncio
import logging
import socket
from dataclasses import dataclass
from typing import Any, Dict, List, Optional, Tuple

try:
    import pystun3
except ImportError:
    pystun3 = None

try:
    import aioice
except ImportError:
    aioice = None

logger = logging.getLogger(__name__)


@dataclass
class NATConfig:
    """Configuration for NAT traversal."""

    enable_stun: bool = True
    enable_turn: bool = False
    enable_ice: bool = True
    stun_servers: List[str] = None
    turn_servers: List[Dict[str, Any]] = None
    ice_timeout: float = 30.0
    hole_punch_timeout: float = 5.0

    def __post_init__(self) -> None:
        if self.stun_servers is None:
            self.stun_servers = [
                "stun:stun.l.google.com:19302",
                "stun:stun1.l.google.com:19302",
                "stun:stun2.l.google.com:19302",
            ]
        if self.turn_servers is None:
            self.turn_servers = []


@dataclass
class ExternalAddress:
    """External address discovered via STUN."""

    host: str
    port: int
    nat_type: str
    local_host: str
    local_port: int


class NATTraversal:
    """NAT traversal with STUN/TURN and ICE support.

    Provides comprehensive NAT traversal capabilities:
    - STUN for external address discovery
    - TURN relay for symmetric NATs
    - ICE for peer-to-peer connection establishment
    - UDP hole punching for cone NATs
    """

    def __init__(self, config: Optional[NATConfig] = None):
        self.config = config or NATConfig()
        self._external_address: Optional[ExternalAddress] = None
        self._ice_connection: Optional[Any] = None

    async def discover_external_address(self) -> Optional[ExternalAddress]:
        """Discover external IP address and port via STUN.

        Returns:
            External address info if STUN is enabled and successful
        """
        if not self.config.enable_stun or not pystun3:
            logger.warning("STUN disabled or pystun3 not available")
            return None

        if self._external_address:
            return self._external_address

        for stun_server in self.config.stun_servers:
            try:
                host, port = self._parse_stun_server(stun_server)
                nat_type, external_ip, external_port = pystun3.get_ip_info(
                    source_ip="0.0.0.0", source_port=0, stun_host=host, stun_port=port
                )

                if external_ip and external_port:
                    # Get local address for comparison
                    local_sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
                    local_sock.connect((host, port))
                    local_host, local_port = local_sock.getsockname()
                    local_sock.close()

                    self._external_address = ExternalAddress(
                        host=external_ip,
                        port=external_port,
                        nat_type=nat_type,
                        local_host=local_host,
                        local_port=local_port,
                    )

                    logger.info(
                        f"Discovered external address: {external_ip}:{external_port} (NAT: {nat_type})"
                    )
                    return self._external_address

            except Exception as e:
                logger.debug(f"STUN discovery failed with {stun_server}: {e}")
                continue

        logger.warning("All STUN servers failed")
        return None

    async def establish_ice_connection(
        self, remote_candidates: List[Dict[str, Any]]
    ) -> Optional[Any]:
        """Establish ICE connection with remote peer.

        Args:
            remote_candidates: Remote ICE candidates

        Returns:
            ICE connection if successful
        """
        if not self.config.enable_ice or not aioice:
            logger.warning("ICE disabled or aioice not available")
            return None

        try:
            # Create ICE connection
            connection = aioice.Connection(ice_controlling=True)

            # Add STUN servers
            for stun_server in self.config.stun_servers:
                host, port = self._parse_stun_server(stun_server)
                connection.add_stun_server(host, port)

            # Add TURN servers if configured
            for turn_server in self.config.turn_servers:
                connection.add_turn_server(
                    turn_server["host"],
                    turn_server["port"],
                    turn_server.get("username"),
                    turn_server.get("password"),
                )

            # Gather local candidates
            await connection.gather_candidates()

            # Add remote candidates
            for candidate_info in remote_candidates:
                candidate = aioice.Candidate.from_sdp(candidate_info["sdp"])
                connection.add_remote_candidate(candidate)

            # Start connectivity checks
            await asyncio.wait_for(connection.connect(), timeout=self.config.ice_timeout)

            self._ice_connection = connection
            logger.info("ICE connection established")
            return connection

        except asyncio.TimeoutError:
            logger.error("ICE connection timeout")
            return None
        except Exception as e:
            logger.error(f"ICE connection failed: {e}")
            return None

    async def perform_hole_punching(
        self, peer_address: str, peer_port: int, local_port: int = 0
    ) -> Optional[Tuple[str, int]]:
        """Attempt UDP hole punching with peer.

        Args:
            peer_address: Peer's external address
            peer_port: Peer's external port
            local_port: Local port to bind (0 for random)

        Returns:
            Tuple of (local_host, local_port) if hole punching succeeded
        """
        if not self.config.enable_stun:
            return None

        try:
            # Create UDP socket for hole punching
            sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
            sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            sock.bind(("0.0.0.0", local_port))

            local_host, local_port = sock.getsockname()

            # Send hole punching packets
            hole_punch_data = b"BEENET_HOLE_PUNCH"

            async def send_punch_packets():
                for _ in range(10):  # Send multiple packets
                    try:
                        sock.sendto(hole_punch_data, (peer_address, peer_port))
                        await asyncio.sleep(0.1)
                    except Exception as e:
                        logger.debug(f"Hole punch send failed: {e}")

            # Send packets in background
            punch_task = asyncio.create_task(send_punch_packets())

            try:
                # Wait for response or timeout
                sock.settimeout(self.config.hole_punch_timeout)
                data, addr = sock.recvfrom(1024)

                if data == hole_punch_data:
                    logger.info(f"Hole punching successful with {addr}")
                    return local_host, local_port

            except socket.timeout:
                logger.debug("Hole punching timeout")
            except Exception as e:
                logger.debug(f"Hole punching failed: {e}")
            finally:
                punch_task.cancel()
                sock.close()

        except Exception as e:
            logger.error(f"Hole punching setup failed: {e}")

        return None

    async def get_local_candidates(self) -> List[Dict[str, Any]]:
        """Get local ICE candidates for sharing with remote peer.

        Returns:
            List of local ICE candidates
        """
        if not self.config.enable_ice or not aioice:
            return []

        try:
            # Create temporary connection to gather candidates
            connection = aioice.Connection(ice_controlling=False)

            # Add STUN servers
            for stun_server in self.config.stun_servers:
                host, port = self._parse_stun_server(stun_server)
                connection.add_stun_server(host, port)

            await connection.gather_candidates()

            candidates = []
            for candidate in connection.local_candidates:
                candidates.append(
                    {
                        "sdp": candidate.to_sdp(),
                        "type": candidate.type,
                        "priority": candidate.priority,
                        "host": candidate.host,
                        "port": candidate.port,
                    }
                )

            return candidates

        except Exception as e:
            logger.error(f"Failed to gather local candidates: {e}")
            return []

    def _parse_stun_server(self, stun_server: str) -> Tuple[str, int]:
        """Parse STUN server URL.

        Args:
            stun_server: STUN server URL (e.g., 'stun:host:port')

        Returns:
            Tuple of (host, port)
        """
        if stun_server.startswith("stun:"):
            stun_server = stun_server[5:]

        if ":" in stun_server:
            host, port_str = stun_server.rsplit(":", 1)
            return host, int(port_str)
        else:
            return stun_server, 3478  # Default STUN port

    def is_nat_traversal_enabled(self) -> bool:
        """Check if any NAT traversal features are enabled.

        Returns:
            True if STUN, TURN, or ICE is enabled
        """
        return self.config.enable_stun or self.config.enable_turn or self.config.enable_ice

    def is_behind_nat(self) -> bool:
        """Check if we appear to be behind NAT.

        Returns:
            True if external address differs from local address
        """
        if not self._external_address:
            return False

        return (
            self._external_address.host != self._external_address.local_host
            or self._external_address.port != self._external_address.local_port
        )

    @property
    def external_address(self) -> Optional[ExternalAddress]:
        """Get the discovered external address."""
        return self._external_address

    async def cleanup(self) -> None:
        """Cleanup NAT traversal resources."""
        if self._ice_connection:
            await self._ice_connection.close()
            self._ice_connection = None
