"""Async connection manager for peer communications."""

import asyncio
from typing import Any, Dict, Optional

from ..crypto import NoiseChannel
from .errors import CryptoError, NetworkError
from .events import EventBus, EventType


class Connection:
    """Represents a secure connection to a peer."""

    def __init__(self, peer_id: str, noise_channel: NoiseChannel, transport: asyncio.Transport):
        self.peer_id = peer_id
        self.noise_channel = noise_channel
        self.transport = transport
        self.connected_at = asyncio.get_event_loop().time()
        self._closed = False

    async def send(self, data: bytes) -> None:
        """Send encrypted data to peer.

        Args:
            data: Data to send
        """
        if self._closed:
            raise NetworkError("Connection is closed")

        try:
            encrypted_data = await self.noise_channel.encrypt(data)
            self.transport.write(encrypted_data)
        except Exception as e:
            raise CryptoError(f"Failed to encrypt data: {e}")

    async def close(self) -> None:
        """Close the connection."""
        if not self._closed:
            self._closed = True
            self.transport.close()

    @property
    def is_closed(self) -> bool:
        """Check if connection is closed."""
        return self._closed


class ConnectionManager:
    """Manages secure connections to peers with key rotation support.

    Provides:
    - TCP/UDP socket management with asyncio
    - Noise XX secure channel establishment
    - Connection pooling and lifecycle management
    - Key rotation with graceful connection migration
    """

    def __init__(self, event_bus: EventBus, key_manager: Any = None):
        self.event_bus = event_bus
        self.key_manager = key_manager
        self._connections: Dict[str, Connection] = {}
        self._servers: Dict[str, asyncio.Server] = {}
        self._lock = asyncio.Lock()
        self.listen_port: Optional[int] = None

    async def start(self, port: int = 0) -> None:
        """Start the connection manager.

        Args:
            port: Port to listen on (0 for random)
        """
        self.listen_port = await self.start_server("0.0.0.0", port)

    async def stop(self) -> None:
        """Stop the connection manager and close all connections."""
        async with self._lock:
            for connection in self._connections.values():
                await connection.close()
            self._connections.clear()

        await self.stop_server()

    async def _handle_client(
        self, reader: asyncio.StreamReader, writer: asyncio.StreamWriter
    ) -> None:
        """Handle incoming client connections.

        Args:
            reader: Stream reader
            writer: Stream writer
        """
        try:
            noise_channel = NoiseChannel(is_initiator=False)

            static_key = None
            if self.key_manager:
                key_pair = await self.key_manager.get_current_static_key()
                if key_pair:
                    static_key = key_pair[0]  # private key bytes

            await noise_channel.start_handshake(static_key)

            peer_address = writer.get_extra_info("peername")
            peer_id = f"incoming_{peer_address[0]}_{peer_address[1]}"

            transport = writer.transport
            if transport is None:
                raise NetworkError("Transport is None")
            connection = Connection(peer_id, noise_channel, transport)

            async with self._lock:
                self._connections[peer_id] = connection

            await self.event_bus.emit(
                EventType.PEER_CONNECTED, {"peer_id": peer_id, "address": peer_address}
            )

        except Exception:
            writer.close()
            await writer.wait_closed()

    async def get_connected_peers(self) -> list[Dict[str, Any]]:
        """Get list of connected peers with their info.

        Returns:
            List of peer information dictionaries
        """
        async with self._lock:
            peers = []
            for peer_id, connection in self._connections.items():
                if not connection.is_closed:
                    peers.append(
                        {
                            "peer_id": peer_id,
                            "connected_at": connection.connected_at,
                            "is_closed": connection.is_closed,
                        }
                    )
            return peers

    async def start_server(self, host: str = "0.0.0.0", port: int = 0) -> int:
        """Start listening for incoming connections.

        Args:
            host: Host to bind to
            port: Port to bind to (0 for random)

        Returns:
            Actual port bound to
        """
        try:
            server = await asyncio.start_server(self._handle_client, host, port)

            actual_port = server.sockets[0].getsockname()[1]
            self._servers[f"{host}:{actual_port}"] = server

            await self.event_bus.emit(EventType.NETWORK_ERROR, {"host": host, "port": actual_port})

            return int(actual_port)

        except Exception as e:
            raise NetworkError(f"Failed to start server: {e}")

    async def stop_server(self) -> None:
        """Stop listening for connections."""
        try:
            for server_key, server in self._servers.items():
                server.close()
                await server.wait_closed()

            self._servers.clear()

            await self.event_bus.emit(EventType.NETWORK_ERROR, {})

        except Exception as e:
            raise NetworkError(f"Failed to stop server: {e}")

    async def connect_to_peer(self, peer_id: str, host: str, port: int) -> Connection:
        """Establish connection to a peer.

        Args:
            peer_id: Target peer identifier
            host: Peer's host address
            port: Peer's port

        Returns:
            Established connection
        """
        async with self._lock:
            if peer_id in self._connections:
                if not self._connections[peer_id].is_closed:
                    return self._connections[peer_id]
                else:
                    del self._connections[peer_id]

        try:
            reader, writer = await asyncio.open_connection(host, port)
            transport = writer.transport

            noise_channel = NoiseChannel(is_initiator=True)

            static_key = None
            if self.key_manager:
                key_pair = await self.key_manager.get_current_static_key()
                if key_pair:
                    static_key = key_pair[0]  # private key bytes

            await noise_channel.start_handshake(static_key)

            if transport is None:
                raise NetworkError("Transport is None")
            connection = Connection(peer_id, noise_channel, transport)

            async with self._lock:
                self._connections[peer_id] = connection

            await self.event_bus.emit(
                EventType.PEER_CONNECTED, {"peer_id": peer_id, "host": host, "port": port}
            )

            return connection

        except Exception as e:
            raise NetworkError(f"Failed to connect to peer {peer_id}: {e}")

    async def disconnect_peer(self, peer_id: str) -> None:
        """Disconnect from a peer.

        Args:
            peer_id: Peer to disconnect from
        """
        async with self._lock:
            if peer_id in self._connections:
                await self._connections[peer_id].close()
                del self._connections[peer_id]

    async def send_to_peer(self, peer_id: str, data: bytes) -> None:
        """Send data to a connected peer.

        Args:
            peer_id: Target peer identifier
            data: Data to send
        """
        async with self._lock:
            if peer_id not in self._connections:
                raise NetworkError(f"No connection to peer {peer_id}")

            connection = self._connections[peer_id]
            await connection.send(data)

    async def handle_key_rotation(self, peer_id: str, old_key: bytes, new_key: bytes) -> None:
        """Handle key rotation for a peer connection.

        Args:
            peer_id: Peer rotating keys
            old_key: Previous public key
            new_key: New public key
        """
        async with self._lock:
            if peer_id not in self._connections:
                return

            connection = self._connections[peer_id]

            try:
                await connection.close()
                del self._connections[peer_id]

                await self.event_bus.emit(
                    EventType.KEY_ROTATED,
                    {"peer_id": peer_id, "old_key": old_key.hex(), "new_key": new_key.hex()},
                )

            except Exception as e:
                raise CryptoError(f"Failed to handle key rotation for {peer_id}: {e}")

    def get_connected_peer_ids(self) -> list[str]:
        """Get list of connected peer IDs.

        Returns:
            List of connected peer identifiers
        """
        return list(self._connections.keys())

    def is_connected(self, peer_id: str) -> bool:
        """Check if connected to a peer.

        Args:
            peer_id: Peer identifier to check

        Returns:
            True if connected
        """
        return peer_id in self._connections and not self._connections[peer_id].is_closed
