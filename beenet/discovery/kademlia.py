"""Async Kademlia DHT interface for global peer discovery."""

import asyncio
import json
import logging
from typing import Any, Dict, List, Optional

from kademlia.network import Server
from kademlia.node import Node

from ..core.errors import DiscoveryError

logger = logging.getLogger(__name__)


class KademliaDiscovery:
    """Async wrapper around Kademlia DHT for global peer discovery.

    Provides:
    - Peer registration and lookup
    - Integration with beenet identity system
    - Async interface for all DHT operations
    """

    def __init__(self, bootstrap_nodes: Optional[List[str]] = None):
        self.bootstrap_nodes = bootstrap_nodes or []
        self._dht: Optional[Server] = None
        self._running = False
        self._listen_port = 8468

    async def start(self, listen_port: int = 8468) -> None:
        """Start the DHT node.

        Args:
            listen_port: Port to listen on for DHT traffic
        """
        try:
            if self._running:
                await self.stop()

            self._listen_port = listen_port
            self._dht = Server()

            await self._dht.listen(listen_port)
            logger.info(f"Kademlia DHT listening on port {listen_port}")

            if self.bootstrap_nodes:
                bootstrap_addrs = []
                for node in self.bootstrap_nodes:
                    if ":" in node:
                        host, port = node.split(":")
                        bootstrap_addrs.append((host, int(port)))
                    else:
                        bootstrap_addrs.append((node, 8468))

                await self._dht.bootstrap(bootstrap_addrs)
                logger.info(f"Bootstrapped with {len(bootstrap_addrs)} nodes")

            self._running = True

        except Exception as e:
            raise DiscoveryError(f"Failed to start Kademlia DHT: {e}")

    async def stop(self) -> None:
        """Stop the DHT node."""
        if self._dht and self._running:
            try:
                self._dht.stop()
                self._running = False
                self._dht = None  # Clear reference to allow port reuse
                logger.info("Kademlia DHT stopped")
                await asyncio.sleep(0.1)
            except Exception as e:
                raise DiscoveryError(f"Failed to stop Kademlia DHT: {e}")

    async def register_peer(self, peer_id: str, address: str, port: int) -> None:
        """Register this peer in the DHT.

        Args:
            peer_id: Unique peer identifier
            address: IP address or hostname
            port: Port number
        """
        if not self._dht or not self._running:
            raise DiscoveryError("DHT not running - call start() first")

        try:
            peer_info = {
                "peer_id": peer_id,
                "address": address,
                "port": port,
                "protocol": "beenet",
                "version": "0.1.0",
            }

            peer_data = json.dumps(peer_info).encode("utf-8")

            await self._dht.set(peer_id, peer_data)
            logger.debug(f"Registered peer {peer_id} at {address}:{port}")

        except Exception as e:
            raise DiscoveryError(f"Failed to register peer {peer_id}: {e}")

    async def find_peer(self, peer_id: str) -> Optional[Dict[str, Any]]:
        """Find a peer in the DHT.

        Args:
            peer_id: Peer identifier to search for

        Returns:
            Peer information if found, None otherwise
        """
        if not self._dht or not self._running:
            raise DiscoveryError("DHT not running - call start() first")

        try:
            peer_data = await self._dht.get(peer_id)

            if peer_data is None:
                return None

            peer_info = json.loads(peer_data.decode("utf-8"))
            logger.debug(f"Found peer {peer_id}: {peer_info}")

            return peer_info

        except Exception as e:
            logger.warning(f"Failed to find peer {peer_id}: {e}")
            return None

    async def find_peers_near(self, target_id: str, count: int = 20) -> List[Dict[str, Any]]:
        """Find peers near a target ID.

        Args:
            target_id: Target identifier
            count: Maximum number of peers to return

        Returns:
            List of peer information
        """
        if not self._dht or not self._running:
            raise DiscoveryError("DHT not running - call start() first")

        try:
            target_node = Node(target_id.encode("utf-8"))
            nearest_nodes = self._dht.protocol.router.find_neighbors(target_node, count)

            peers = []
            for node in nearest_nodes:
                try:
                    node_id = node.id.hex()
                    peer_info = await self.find_peer(node_id)
                    if peer_info:
                        peers.append(peer_info)
                except Exception as e:
                    logger.warning(f"Failed to get info for node {node.id.hex()}: {e}")
                    continue

            logger.debug(f"Found {len(peers)} peers near {target_id}")
            return peers

        except Exception as e:
            raise DiscoveryError(f"Failed to find peers near {target_id}: {e}")

    async def get_routing_table_size(self) -> int:
        """Get the number of nodes in the routing table.

        Returns:
            Number of nodes in routing table
        """
        if not self._dht or not self._running:
            return 0

        try:
            return len(self._dht.protocol.router)
        except Exception:
            return 0

    async def get_node_id(self) -> Optional[str]:
        """Get this node's ID.

        Returns:
            Node ID as hex string, or None if not running
        """
        if not self._dht or not self._running:
            return None

        try:
            return self._dht.node.id.hex()
        except Exception:
            return None

    async def bootstrap_from_known_peers(self, known_peers: List[Dict[str, Any]]) -> None:
        """Bootstrap DHT from a list of known peers.

        Args:
            known_peers: List of peer info dictionaries
        """
        if not self._dht or not self._running:
            raise DiscoveryError("DHT not running - call start() first")

        try:
            bootstrap_addrs = []
            for peer in known_peers:
                address = peer.get("address")
                port = peer.get("port", 8468)
                if address:
                    bootstrap_addrs.append((address, port))

            if bootstrap_addrs:
                await self._dht.bootstrap(bootstrap_addrs)
                logger.info(f"Bootstrapped from {len(bootstrap_addrs)} known peers")

        except Exception as e:
            raise DiscoveryError(f"Failed to bootstrap from known peers: {e}")

    @property
    def is_running(self) -> bool:
        """Check if DHT is running."""
        return self._running

    @property
    def listen_port(self) -> int:
        """Get the port this DHT is listening on."""
        return self._listen_port
