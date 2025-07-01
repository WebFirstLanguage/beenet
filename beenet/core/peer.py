"""Main peer class for beenet P2P networking."""

import asyncio
import tempfile
import time
from pathlib import Path
from typing import Any, Callable, Dict, Optional

from ..crypto import Identity, KeyManager, KeyStore
from ..discovery import BeeQuietDiscovery, KademliaDiscovery, NATConfig, NATTraversal
from ..transfer import TransferStream
from .connection import ConnectionManager
from .events import EventBus, EventType
from .resilience import PeerResilienceManager, ReconnectionPolicy, ReconnectionReason
from ..observability import get_logger, get_metrics, MetricsConfig, setup_observability


class Peer:
    """Main beenet peer class providing P2P networking capabilities.

    Integrates all beenet components:
    - Secure identity and key management
    - Hybrid discovery (DHT + LAN)
    - Encrypted data transfer with integrity verification
    - Event-driven architecture for extensibility
    """

    def __init__(
        self,
        peer_id: str,
        keystore_path: Optional[Path] = None,
        nat_config: Optional[NATConfig] = None,
        reconnection_policy: Optional[ReconnectionPolicy] = None,
        metrics_config: Optional[MetricsConfig] = None,
    ):
        self.peer_id = peer_id

        # Set up observability first
        self.observability = setup_observability(metrics_config)
        self.logger = get_logger("peer")
        self.metrics = get_metrics()

        self.keystore = KeyStore(keystore_path)
        self.identity = Identity(self.keystore)
        self.key_manager = KeyManager(self.keystore)
        self.event_bus = EventBus()
        self.connection_manager = ConnectionManager(self.event_bus, self.key_manager)

        self.kademlia = KademliaDiscovery()
        self.beequiet = BeeQuietDiscovery(peer_id, None)
        self.nat_traversal = NATTraversal(nat_config)
        self.resilience_manager = PeerResilienceManager(reconnection_policy)

        self._running = False
        self._transfers: Dict[str, TransferStream] = {}
        self._state_dir = Path(tempfile.gettempdir()) / "beenet" / peer_id
        self._state_dir.mkdir(parents=True, exist_ok=True)

        # Set up resilience callbacks
        self._setup_resilience_callbacks()

        # Start metrics server
        self.observability.start_metrics_server()

        self.logger.info("Peer initialized", peer_id=peer_id)

    async def start(
        self, listen_port: int = 0, bootstrap_nodes: Optional[list[str]] = None
    ) -> None:
        """Start the peer and all discovery services.

        Args:
            listen_port: Port to listen on (0 for random)
            bootstrap_nodes: DHT bootstrap nodes
        """
        if self._running:
            return

        start_time = time.time()
        self.logger.info("Starting peer", listen_port=listen_port, bootstrap_nodes=bootstrap_nodes)

        try:
            with self.observability.time_operation("peer_startup", self.peer_id):
                await self.keystore.open()
                await self.identity.load_or_generate_identity(self.peer_id)
                await self.key_manager.load_or_generate_static_key(self.peer_id)

                await self.connection_manager.start(listen_port)
                actual_port = self.connection_manager.listen_port
                if actual_port is None:
                    raise RuntimeError("Failed to get listen port from connection manager")

                if bootstrap_nodes:
                    self.kademlia.bootstrap_nodes = bootstrap_nodes
                await self.kademlia.start(actual_port + 1)

                await self.beequiet.start()

                # Perform NAT traversal discovery if enabled
                if self.nat_traversal.is_nat_traversal_enabled():
                    with self.observability.time_operation("nat_discovery", self.peer_id):
                        external_addr = await self.nat_traversal.discover_external_address()
                        if external_addr:
                            # Register external address with Kademlia
                            await self.kademlia.register_peer(
                                self.peer_id, external_addr.host, external_addr.port
                            )
                            self.metrics.record_nat_traversal("stun", True)
                            self.logger.info(
                                "NAT traversal successful",
                                external_address=f"{external_addr.host}:{external_addr.port}",
                            )
                        else:
                            # Fallback to local address
                            await self.kademlia.register_peer(
                                self.peer_id, "127.0.0.1", actual_port
                            )
                            self.metrics.record_nat_traversal("stun", False)
                            self.logger.warning("NAT traversal failed, using local address")
                else:
                    await self.kademlia.register_peer(self.peer_id, "127.0.0.1", actual_port)

                # Start resilience manager
                await self.resilience_manager.start()

                self._running = True
                startup_duration = time.time() - start_time

                self.logger.info(
                    "Peer started successfully",
                    actual_port=actual_port,
                    startup_duration=startup_duration,
                )

                await self.event_bus.emit(
                    EventType.PEER_CONNECTED, {"peer_id": self.peer_id, "listen_port": actual_port}
                )

        except Exception as e:
            await self.stop()
            raise e

    async def stop(self) -> None:
        """Stop the peer and cleanup resources."""
        if not self._running:
            return

        try:
            for transfer in self._transfers.values():
                state_file = self._state_dir / f"{transfer.transfer_id}.state"
                await transfer.save_state(state_file)

            await self.beequiet.stop()
            await self.kademlia.stop()
            await self.connection_manager.stop()
            await self.nat_traversal.cleanup()
            await self.resilience_manager.stop()

            self._transfers.clear()
            self._running = False

            await self.event_bus.emit(EventType.PEER_DISCONNECTED, {"peer_id": self.peer_id})

        except Exception as e:
            self._running = False
            raise e

    async def connect_to_peer(self, peer_id: str, address: Optional[str] = None) -> bool:
        """Connect to another peer.

        Args:
            peer_id: Target peer identifier
            address: Optional direct address (bypasses discovery)

        Returns:
            True if connection succeeded
        """
        if not self._running:
            return False

        try:
            if address:
                host, port = address.split(":")
                peer_info = {"peer_id": peer_id, "address": host, "port": int(port)}
            else:
                peer_info_result = await self.kademlia.find_peer(peer_id)
                if not peer_info_result:
                    return False
                peer_info = dict(peer_info_result)

            port_value = peer_info.get("port", 0)
            if isinstance(port_value, int):
                port_int = port_value
            elif isinstance(port_value, str):
                port_int = int(port_value)
            else:
                port_int = 0
            connection = await self.connection_manager.connect_to_peer(
                peer_id, str(peer_info["address"]), port_int
            )

            if connection:
                await self.event_bus.emit(
                    EventType.PEER_CONNECTED,
                    {
                        "peer_id": peer_id,
                        "address": peer_info["address"],
                        "port": peer_info["port"],
                    },
                )

            return connection is not None

        except Exception as e:
            print(f"Connection error: {e}")
            return False

    async def send_file(
        self, peer_id: str, file_path: Path, transfer_id: Optional[str] = None
    ) -> str:
        """Send a file to another peer.

        Args:
            peer_id: Target peer identifier
            file_path: Path to file to send
            transfer_id: Optional transfer identifier

        Returns:
            Transfer identifier
        """
        if not self._running:
            raise RuntimeError("Peer not running")

        if not file_path.exists():
            raise FileNotFoundError(f"File not found: {file_path}")

        if not transfer_id:
            transfer_id = (
                f"{self.peer_id}_{peer_id}_{file_path.name}_{asyncio.get_event_loop().time()}"
            )

        if not self.connection_manager.is_connected(peer_id):
            connected = await self.connect_to_peer(peer_id)
            if not connected:
                raise ConnectionError(f"Failed to connect to peer {peer_id}")

        transfer_stream = TransferStream(transfer_id)
        await transfer_stream.start_send(file_path, f"{peer_id}:address")

        self._transfers[transfer_id] = transfer_stream

        await self.event_bus.emit(
            EventType.TRANSFER_STARTED,
            {
                "transfer_id": transfer_id,
                "peer_id": peer_id,
                "file_path": str(file_path),
                "direction": "send",
            },
        )

        return transfer_id

    async def receive_file(self, transfer_id: str, save_path: Path) -> bool:
        """Receive a file from another peer.

        Args:
            transfer_id: Transfer identifier
            save_path: Path to save received file

        Returns:
            True if transfer completed successfully
        """
        if not self._running:
            return False

        try:
            if transfer_id not in self._transfers:
                return False

            transfer_stream = self._transfers[transfer_id]

            await self.event_bus.emit(
                EventType.TRANSFER_STARTED,
                {"transfer_id": transfer_id, "save_path": str(save_path), "direction": "receive"},
            )

            success = await transfer_stream.verify_complete_file(save_path)

            if success:
                await self.event_bus.emit(
                    EventType.TRANSFER_COMPLETED,
                    {"transfer_id": transfer_id, "save_path": str(save_path)},
                )

            return success

        except Exception:
            return False

    async def list_peers(self) -> list[Dict[str, Any]]:
        """List discovered peers.

        Returns:
            List of peer information
        """
        if not self._running:
            return []

        try:
            discovered_peers = []

            beequiet_peers = self.beequiet.get_discovered_peers()
            discovered_peers.extend(beequiet_peers)

            connected_peers = await self.connection_manager.get_connected_peers()
            for peer_info in connected_peers:
                if not any(p.get("peer_id") == peer_info.get("peer_id") for p in discovered_peers):
                    discovered_peers.append(peer_info)

            return discovered_peers

        except Exception:
            return []

    def set_transfer_progress_callback(
        self, transfer_id: str, callback: Callable[[float], None]
    ) -> None:
        """Set progress callback for a transfer.

        Args:
            transfer_id: Transfer identifier
            callback: Progress callback function
        """
        if transfer_id in self._transfers:
            self._transfers[transfer_id].set_progress_callback(callback)

    async def _on_peer_discovered(self, peer_info: Dict[str, Any]) -> None:
        """Handle peer discovery events.

        Args:
            peer_info: Discovered peer information
        """
        pass

    @property
    def is_running(self) -> bool:
        """Check if peer is running."""
        return self._running

    @property
    def public_key(self) -> Optional[bytes]:
        """Get this peer's public identity key."""
        return self.identity.public_key

    async def get_peer_info(self) -> Dict[str, Any]:
        """Get information about this peer.

        Returns:
            Peer information dictionary
        """
        return {
            "peer_id": self.peer_id,
            "public_key": self.public_key,
            "is_running": self.is_running,
            "active_transfers": len(self._transfers),
            "resilience_stats": self.resilience_manager.get_statistics() if self._running else {},
        }

    def _setup_resilience_callbacks(self) -> None:
        """Set up callbacks for resilience manager integration."""

        def should_reconnect_callback(peer_id: str, score, reason) -> bool:
            """Custom policy for reconnection decisions."""
            # Don't reconnect to ourselves
            if peer_id == self.peer_id:
                return False

            # Always allow initial connections
            if reason == ReconnectionReason.INITIAL_CONNECTION:
                return True

            # Use default scoring for other cases
            return (
                score.calculate_overall_score()
                >= self.resilience_manager.policy.min_score_for_retry
            )

        def score_update_callback(peer_id: str, score) -> None:
            """Handle peer score updates."""
            # Emit event for score updates
            asyncio.create_task(
                self.event_bus.emit(
                    EventType.PEER_DISCOVERED,  # Reuse existing event type
                    {
                        "peer_id": peer_id,
                        "score": score.calculate_overall_score(),
                        "connection_success_rate": score.connection_success_rate,
                        "transfer_success_rate": score.transfer_success_rate,
                    },
                )
            )

        def blacklist_callback(peer_id: str, score) -> None:
            """Handle peer blacklisting."""
            # Emit event and disconnect if connected
            asyncio.create_task(
                self.event_bus.emit(
                    EventType.PEER_DISCONNECTED,
                    {
                        "peer_id": peer_id,
                        "reason": "blacklisted",
                        "score": score.calculate_overall_score(),
                    },
                )
            )

        # Set callbacks
        self.resilience_manager.set_policy_callbacks(
            should_reconnect=should_reconnect_callback,
            score_update=score_update_callback,
            blacklist=blacklist_callback,
        )
