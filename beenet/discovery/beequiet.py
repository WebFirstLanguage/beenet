"""BeeQuiet LAN discovery protocol with AEAD security."""

import asyncio
import json
import logging
import secrets
import socket
import struct
import time
from enum import Enum
from typing import Any, Callable, Dict, List, Optional, Tuple

from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.ciphers.aead import ChaCha20Poly1305
from cryptography.hazmat.primitives.kdf.hkdf import HKDF

from ..core.errors import DiscoveryError

logger = logging.getLogger(__name__)


class BeeQuietState(Enum):
    """BeeQuiet protocol state machine states."""

    DISCOVERING = "discovering"
    STEADY = "steady"
    LEAVING = "leaving"


class BeeQuietMessageType(Enum):
    """BeeQuiet message types."""

    WHO_IS_HERE = 0x01
    I_AM_HERE = 0x02
    HEARTBEAT = 0x03
    GOODBYE = 0x04


class BeeQuietProtocol(asyncio.DatagramProtocol):
    """UDP protocol handler for BeeQuiet messages."""

    def __init__(self, discovery: "BeeQuietDiscovery"):
        self.discovery = discovery
        self.transport: Optional[asyncio.DatagramTransport] = None

    def connection_made(self, transport: asyncio.DatagramTransport) -> None:
        self.transport = transport
        self.discovery._transport = transport

    def datagram_received(self, data: bytes, addr: Tuple[str, int]) -> None:
        asyncio.create_task(self.discovery._handle_message(data, addr))

    def error_received(self, exc: Exception) -> None:
        logger.warning(f"BeeQuiet protocol error: {exc}")


class BeeQuietDiscovery:
    """BeeQuiet LAN discovery with AEAD-wrapped payloads.

    Protocol flow:
    1. WHO_IS_HERE with nonce challenge
    2. I_AM_HERE response derives ChaCha20-Poly1305 session key
    3. All subsequent messages (HEARTBEAT, GOODBYE) are AEAD-wrapped

    Uses UDP multicast/unicast on 239.255.7.7:7777 with magic 0xBEEC.
    """

    MULTICAST_ADDR = "239.255.7.7"
    MULTICAST_GROUP = "239.255.7.7"  # Alias for tests
    MULTICAST_PORT = 7777
    MAGIC_NUMBER = 0xBEEC
    HEARTBEAT_INTERVAL = 30.0
    PEER_TIMEOUT = 90.0
    
    WHO_IS_HERE = BeeQuietMessageType.WHO_IS_HERE
    I_AM_HERE = BeeQuietMessageType.I_AM_HERE
    HEARTBEAT = BeeQuietMessageType.HEARTBEAT
    GOODBYE = BeeQuietMessageType.GOODBYE

    def __init__(self, peer_id: str, on_peer_discovered: Optional[Callable] = None):
        self.peer_id = peer_id
        self.on_peer_discovered = on_peer_discovered
        self.peer_discovered_callback = on_peer_discovered  # Alias for tests
        self.state = BeeQuietState.DISCOVERING
        self._transport: Optional[asyncio.DatagramTransport] = None
        self._session_keys: Dict[str, bytes] = {}
        self._discovered_peers: Dict[str, Dict[str, Any]] = {}
        self._heartbeat_task: Optional[asyncio.Task] = None
        self._cleanup_task: Optional[asyncio.Task] = None
        self._bind_port = 0
        self._running = False

    async def start(self, bind_port: int = 0) -> None:
        """Start BeeQuiet discovery.

        Args:
            bind_port: Port to bind to (0 for random)
        """
        try:
            self._bind_port = bind_port

            loop = asyncio.get_event_loop()

            sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
            sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            sock.setsockopt(socket.SOL_SOCKET, socket.SO_BROADCAST, 1)

            mreq = struct.pack("4sl", socket.inet_aton(self.MULTICAST_ADDR), socket.INADDR_ANY)
            sock.setsockopt(socket.IPPROTO_IP, socket.IP_ADD_MEMBERSHIP, mreq)

            sock.bind(("", bind_port))

            transport, protocol = await loop.create_datagram_endpoint(
                lambda: BeeQuietProtocol(self), sock=sock
            )

            self._transport = transport

            self._heartbeat_task = asyncio.create_task(self._heartbeat_loop())
            self._cleanup_task = asyncio.create_task(self._cleanup_loop())
            
            self._running = True

            await self.send_who_is_here()

            logger.info(f"BeeQuiet discovery started on port {bind_port}")

        except Exception as e:
            raise DiscoveryError(f"Failed to start BeeQuiet discovery: {e}")

    async def stop(self) -> None:
        """Stop BeeQuiet discovery and send GOODBYE."""
        try:
            self.state = BeeQuietState.LEAVING

            for peer_addr in list(self._discovered_peers.keys()):
                await self._send_goodbye_to_peer(peer_addr)

            if self._heartbeat_task:
                self._heartbeat_task.cancel()
                try:
                    await self._heartbeat_task
                except asyncio.CancelledError:
                    pass

            if self._cleanup_task:
                self._cleanup_task.cancel()
                try:
                    await self._cleanup_task
                except asyncio.CancelledError:
                    pass

            if self._transport:
                self._transport.close()
            
            self._running = False

            logger.info("BeeQuiet discovery stopped")

        except Exception as e:
            raise DiscoveryError(f"Failed to stop BeeQuiet discovery: {e}")

    async def send_who_is_here(self) -> None:
        """Send WHO_IS_HERE discovery message with nonce."""
        if not self._transport:
            raise DiscoveryError("Transport not available")

        try:
            nonce = secrets.token_bytes(16)

            message_data = {
                "peer_id": self.peer_id,
                "nonce": nonce.hex(),
                "timestamp": int(time.time()),
            }

            payload = json.dumps(message_data).encode("utf-8")
            packet = self._create_packet(BeeQuietMessageType.WHO_IS_HERE, payload)

            self._transport.sendto(packet, (self.MULTICAST_ADDR, self.MULTICAST_PORT))
            logger.debug(f"Sent WHO_IS_HERE from {self.peer_id}")

        except Exception as e:
            raise DiscoveryError(f"Failed to send WHO_IS_HERE: {e}")

    async def send_i_am_here(self, nonce: bytes, peer_address: tuple) -> None:
        """Send I_AM_HERE response and derive session key.

        Args:
            nonce: Nonce from WHO_IS_HERE message
            peer_address: Address tuple (host, port) to send response to
        """
        if not self._transport:
            raise DiscoveryError("Transport not available")

        try:
            response_data = secrets.token_bytes(16)

            session_key = self._derive_session_key(nonce, response_data)
            target_addr = f"{peer_address[0]}:{peer_address[1]}"
            self._session_keys[target_addr] = session_key

            message_data = {
                "peer_id": self.peer_id,
                "response": response_data.hex(),
                "timestamp": int(time.time()),
            }

            payload = json.dumps(message_data).encode("utf-8")
            packet = self._create_packet(BeeQuietMessageType.I_AM_HERE, payload)

            self._transport.sendto(packet, peer_address)

            logger.debug(f"Sent I_AM_HERE to {target_addr}")

        except Exception as e:
            raise DiscoveryError(f"Failed to send I_AM_HERE: {e}")

    async def send_heartbeat(self, peer_id: str, session_key: bytes, peer_address: tuple) -> None:
        """Send AEAD-wrapped HEARTBEAT message.

        Args:
            peer_id: Target peer ID
            session_key: Session key for encryption
            peer_address: Target peer address tuple (host, port)
        """
        if not self._transport:
            raise DiscoveryError("Transport not available")

        try:
            message_data = {"peer_id": self.peer_id, "timestamp": int(time.time())}

            payload = json.dumps(message_data).encode("utf-8")
            encrypted_payload = self._encrypt_payload(payload, session_key)

            packet = self._create_packet(BeeQuietMessageType.HEARTBEAT, encrypted_payload)
            self._transport.sendto(packet, peer_address)

            logger.debug(f"Sent HEARTBEAT to {peer_id} at {peer_address}")

        except Exception as e:
            logger.warning(f"Failed to send heartbeat to {peer_id}: {e}")
            
    async def _send_heartbeat_to_peer(self, target_addr: str) -> None:
        """Internal method to send heartbeat to discovered peer.

        Args:
            target_addr: Target peer address string
        """
        if target_addr not in self._session_keys:
            logger.warning(f"No session key for {target_addr}, skipping heartbeat")
            return

        try:
            session_key = self._session_keys[target_addr]
            host, port = (
                target_addr.split(":") if ":" in target_addr else (target_addr, self.MULTICAST_PORT)
            )
            await self.send_heartbeat("unknown", session_key, (host, int(port)))

        except Exception as e:
            logger.warning(f"Failed to send heartbeat to {target_addr}: {e}")

    async def send_goodbye(self, peer_id: str, session_key: bytes, peer_address: tuple) -> None:
        """Send AEAD-wrapped GOODBYE message.

        Args:
            peer_id: Target peer ID
            session_key: Session key for encryption
            peer_address: Target peer address tuple (host, port)
        """
        if not self._transport:
            return

        try:
            message_data = {"peer_id": self.peer_id, "timestamp": int(time.time())}

            payload = json.dumps(message_data).encode("utf-8")
            encrypted_payload = self._encrypt_payload(payload, session_key)

            packet = self._create_packet(BeeQuietMessageType.GOODBYE, encrypted_payload)
            self._transport.sendto(packet, peer_address)

            logger.debug(f"Sent GOODBYE to {peer_id} at {peer_address}")

        except Exception as e:
            logger.warning(f"Failed to send goodbye to {peer_id}: {e}")
            
    async def _send_goodbye_to_peer(self, target_addr: str) -> None:
        """Internal method to send goodbye to discovered peer.

        Args:
            target_addr: Target peer address string
        """
        if target_addr not in self._session_keys:
            return

        try:
            session_key = self._session_keys[target_addr]
            host, port = (
                target_addr.split(":") if ":" in target_addr else (target_addr, self.MULTICAST_PORT)
            )
            await self.send_goodbye("unknown", session_key, (host, int(port)))

        except Exception as e:
            logger.warning(f"Failed to send goodbye to {target_addr}: {e}")

    def _derive_session_key(self, nonce: bytes, response: bytes) -> bytes:
        """Derive ChaCha20-Poly1305 session key from challenge-response.

        Args:
            nonce: Challenge nonce
            response: Response data

        Returns:
            Derived session key
        """
        try:
            if isinstance(nonce, str):
                nonce = nonce.encode('utf-8')
            if isinstance(response, str):
                response = response.encode('utf-8')
                
            hkdf = HKDF(
                algorithm=hashes.BLAKE2b(64),
                length=32,  # ChaCha20Poly1305 requires exactly 32 bytes
                salt=nonce,
                info=b"beenet-beequiet-session-key",
            )

            session_key = hkdf.derive(response)
            return session_key

        except Exception as e:
            raise DiscoveryError(f"Failed to derive session key: {e}")

    def _encrypt_payload(self, payload: bytes, session_key: bytes) -> bytes:
        """Encrypt payload with ChaCha20-Poly1305 AEAD.

        Args:
            payload: Plaintext payload
            session_key: Session key for encryption

        Returns:
            Encrypted payload
        """
        try:
            # Ensure session key is exactly 32 bytes
            if len(session_key) != 32:
                raise ValueError(f"Session key must be exactly 32 bytes, got {len(session_key)}")
                
            cipher = ChaCha20Poly1305(session_key)
            nonce = secrets.token_bytes(12)
            ciphertext = cipher.encrypt(nonce, payload, None)

            return nonce + ciphertext

        except Exception as e:
            raise DiscoveryError(f"Failed to encrypt payload: {e}")

    def _decrypt_payload(self, encrypted_payload: bytes, session_key: bytes) -> bytes:
        """Decrypt AEAD payload.

        Args:
            encrypted_payload: Encrypted payload
            session_key: Session key for decryption

        Returns:
            Decrypted payload
        """
        try:
            if len(encrypted_payload) < 12:
                raise ValueError("Encrypted payload too short")

            # Ensure session key is exactly 32 bytes
            if len(session_key) != 32:
                raise ValueError(f"Session key must be exactly 32 bytes, got {len(session_key)}")

            nonce = encrypted_payload[:12]
            ciphertext = encrypted_payload[12:]

            cipher = ChaCha20Poly1305(session_key)
            plaintext = cipher.decrypt(nonce, ciphertext, None)

            return plaintext

        except Exception as e:
            raise DiscoveryError(f"Failed to decrypt payload: {e}")

    def _create_packet(self, msg_type: BeeQuietMessageType, payload: bytes) -> bytes:
        """Create a BeeQuiet protocol packet.

        Args:
            msg_type: Message type
            payload: Message payload

        Returns:
            Formatted packet bytes
        """
        header = struct.pack(">HBH", self.MAGIC_NUMBER, msg_type.value, len(payload))
        return header + payload

    def _parse_packet(self, packet: bytes) -> Tuple[BeeQuietMessageType, bytes]:
        """Parse a BeeQuiet protocol packet.

        Args:
            packet: Raw packet bytes

        Returns:
            Tuple of (message_type, payload)
        """
        if len(packet) < 5:
            raise ValueError("Packet too short")

        magic, msg_type_val, payload_len = struct.unpack(">HBH", packet[:5])

        if magic != self.MAGIC_NUMBER:
            raise ValueError(f"Invalid magic number: {magic}")

        if len(packet) != 5 + payload_len:
            raise ValueError("Packet length mismatch")

        try:
            msg_type = BeeQuietMessageType(msg_type_val)
        except ValueError:
            raise ValueError(f"Unknown message type: {msg_type_val}")

        payload = packet[5:]
        return msg_type, payload

    async def _handle_message(self, data: bytes, addr: Tuple[str, int]) -> None:
        """Handle incoming BeeQuiet message.

        Args:
            data: Raw message data
            addr: Sender address
        """
        try:
            msg_type, payload = self._parse_packet(data)
            peer_addr = f"{addr[0]}:{addr[1]}"

            if msg_type == BeeQuietMessageType.WHO_IS_HERE:
                await self._handle_who_is_here(payload, peer_addr)
            elif msg_type == BeeQuietMessageType.I_AM_HERE:
                await self._handle_i_am_here(payload, peer_addr)
            elif msg_type == BeeQuietMessageType.HEARTBEAT:
                await self._handle_heartbeat(payload, peer_addr)
            elif msg_type == BeeQuietMessageType.GOODBYE:
                await self._handle_goodbye(payload, peer_addr)

        except Exception as e:
            logger.warning(f"Failed to handle message from {addr}: {e}")

    async def _handle_who_is_here(self, payload: bytes, peer_addr: str) -> None:
        """Handle WHO_IS_HERE message."""
        try:
            message_data = json.loads(payload.decode("utf-8"))
            peer_id = message_data.get("peer_id")
            nonce_hex = message_data.get("nonce")

            if not peer_id or not nonce_hex or peer_id == self.peer_id:
                return

            nonce = bytes.fromhex(nonce_hex)
            await self.send_i_am_here(peer_addr, nonce)

        except Exception as e:
            logger.warning(f"Failed to handle WHO_IS_HERE from {peer_addr}: {e}")

    async def _handle_i_am_here(self, payload: bytes, peer_addr: str) -> None:
        """Handle I_AM_HERE message."""
        try:
            message_data = json.loads(payload.decode("utf-8"))
            peer_id = message_data.get("peer_id")
            response_hex = message_data.get("response")

            if not peer_id or not response_hex or peer_id == self.peer_id:
                return

            response = bytes.fromhex(response_hex)

            peer_info = {
                "peer_id": peer_id,
                "address": peer_addr.split(":")[0],
                "port": int(peer_addr.split(":")[1]),
                "last_seen": time.time(),
                "protocol": "beenet-beequiet",
            }

            self._discovered_peers[peer_addr] = peer_info

            if self.on_peer_discovered:
                await self.on_peer_discovered(peer_info)

            logger.info(f"Discovered peer {peer_id} at {peer_addr}")

        except Exception as e:
            logger.warning(f"Failed to handle I_AM_HERE from {peer_addr}: {e}")

    async def _handle_heartbeat(self, payload: bytes, peer_addr: str) -> None:
        """Handle HEARTBEAT message."""
        if peer_addr not in self._session_keys:
            return

        try:
            session_key = self._session_keys[peer_addr]
            decrypted_payload = self._decrypt_payload(payload, session_key)
            message_data = json.loads(decrypted_payload.decode("utf-8"))

            peer_id = message_data.get("peer_id")
            if not peer_id:
                return

            if peer_addr in self._discovered_peers:
                self._discovered_peers[peer_addr]["last_seen"] = time.time()

            logger.debug(f"Received heartbeat from {peer_id} at {peer_addr}")

        except Exception as e:
            logger.warning(f"Failed to handle heartbeat from {peer_addr}: {e}")

    async def _handle_goodbye(self, payload: bytes, peer_addr: str) -> None:
        """Handle GOODBYE message."""
        if peer_addr not in self._session_keys:
            return

        try:
            session_key = self._session_keys[peer_addr]
            decrypted_payload = self._decrypt_payload(payload, session_key)
            message_data = json.loads(decrypted_payload.decode("utf-8"))

            peer_id = message_data.get("peer_id")
            if not peer_id:
                return

            if peer_addr in self._discovered_peers:
                del self._discovered_peers[peer_addr]

            if peer_addr in self._session_keys:
                del self._session_keys[peer_addr]

            logger.info(f"Peer {peer_id} at {peer_addr} said goodbye")

        except Exception as e:
            logger.warning(f"Failed to handle goodbye from {peer_addr}: {e}")

    async def _heartbeat_loop(self) -> None:
        """Periodic heartbeat loop."""
        while self.state != BeeQuietState.LEAVING:
            try:
                await asyncio.sleep(self.HEARTBEAT_INTERVAL)

                for peer_addr in list(self._discovered_peers.keys()):
                    await self._send_heartbeat_to_peer(peer_addr)

            except asyncio.CancelledError:
                break
            except Exception as e:
                logger.warning(f"Error in heartbeat loop: {e}")

    async def _cleanup_loop(self) -> None:
        """Periodic cleanup of stale peers."""
        while self.state != BeeQuietState.LEAVING:
            try:
                await asyncio.sleep(30.0)

                current_time = time.time()
                stale_peers = []

                for peer_addr, peer_info in self._discovered_peers.items():
                    if current_time - peer_info["last_seen"] > self.PEER_TIMEOUT:
                        stale_peers.append(peer_addr)

                for peer_addr in stale_peers:
                    del self._discovered_peers[peer_addr]
                    if peer_addr in self._session_keys:
                        del self._session_keys[peer_addr]
                    logger.info(f"Removed stale peer {peer_addr}")

            except asyncio.CancelledError:
                break
            except Exception as e:
                logger.warning(f"Error in cleanup loop: {e}")

    def get_discovered_peers(self) -> List[Dict[str, Any]]:
        """Get list of currently discovered peers.

        Returns:
            List of peer information dictionaries
        """
        return list(self._discovered_peers.values())
    
    def derive_session_key(self, nonce: bytes, response: bytes) -> bytes:
        """Public method to derive session key for tests.
        
        Args:
            nonce: Challenge nonce
            response: Response data
            
        Returns:
            Derived session key
        """
        return self._derive_session_key(nonce, response)
    
    def encrypt_message(self, message: bytes, session_key: bytes) -> bytes:
        """Public method to encrypt message for tests.
        
        Args:
            message: Message to encrypt
            session_key: Session key for encryption
            
        Returns:
            Encrypted message
        """
        return self._encrypt_payload(message, session_key)
    
    def decrypt_message(self, encrypted_message: bytes, session_key: bytes) -> bytes:
        """Public method to decrypt message for tests.
        
        Args:
            encrypted_message: Encrypted message
            session_key: Session key for decryption
            
        Returns:
            Decrypted message or None if decryption fails
        """
        try:
            return self._decrypt_payload(encrypted_message, session_key)
        except Exception:
            return None  # Fail gracefully for invalid decryption
    
    async def _on_peer_discovered(self, peer_info: dict) -> None:
        """Internal callback for peer discovery (for tests).
        
        Args:
            peer_info: Discovered peer information
        """
        if self.on_peer_discovered:
            await self.on_peer_discovered(peer_info)

    @property
    def is_running(self) -> bool:
        """Check if BeeQuiet discovery is running."""
        return self._running
