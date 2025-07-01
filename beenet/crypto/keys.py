"""Key generation and rotation management."""

import json
import time
from typing import Optional, Tuple

from nacl.encoding import RawEncoder
from nacl.public import PrivateKey, PublicKey

from ..core.errors import CryptoError
from .identity import Identity
from .keystore import KeyStore


class KeyManager:
    """Manages static key pairs and rotation for Noise sessions.

    Handles:
    - X25519 static key generation for Noise XX
    - Key rotation with identity signature
    - Key announcement protocol
    - Integration with secure keystore
    """

    def __init__(self, keystore: KeyStore, identity: Optional[Identity] = None):
        self.keystore = keystore
        self.identity = identity
        self._current_static_key: Optional[PrivateKey] = None
        self._current_public_key: Optional[PublicKey] = None
        self._peer_id: Optional[str] = None

    async def generate_static_key(self, peer_id: Optional[str] = None) -> Tuple[bytes, bytes]:
        """Generate a new X25519 static key pair.

        Args:
            peer_id: Optional peer identifier for key storage

        Returns:
            Tuple of (private_key_bytes, public_key_bytes)
        """
        try:
            private_key = PrivateKey.generate()
            public_key = private_key.public_key

            private_key_bytes = bytes(private_key.encode(RawEncoder))
            public_key_bytes = bytes(public_key.encode(RawEncoder))

            if peer_id:
                self._peer_id = peer_id
                static_key_id = f"static_{peer_id}"
                await self.keystore.store_key(static_key_id, private_key_bytes, encrypted=True)

                self._current_static_key = private_key
                self._current_public_key = public_key

            return private_key_bytes, public_key_bytes

        except Exception as e:
            raise CryptoError(f"Failed to generate static key: {e}")

    async def load_or_generate_static_key(self, peer_id: str) -> Tuple[bytes, bytes]:
        """Load existing static key or generate new one.

        Args:
            peer_id: Peer identifier for key storage

        Returns:
            Tuple of (private_key_bytes, public_key_bytes)
        """
        self._peer_id = peer_id
        static_key_id = f"static_{peer_id}"

        try:
            private_key_bytes = await self.keystore.get_key(static_key_id)

            if private_key_bytes:
                self._current_static_key = PrivateKey(private_key_bytes)
                self._current_public_key = self._current_static_key.public_key

                public_key_bytes = bytes(self._current_public_key.encode(RawEncoder))
                return private_key_bytes, public_key_bytes
            else:
                return await self.generate_static_key(peer_id)

        except Exception as e:
            raise CryptoError(f"Failed to load or generate static key for {peer_id}: {e}")

    async def rotate_static_key(self, peer_id: str) -> Tuple[bytes, bytes]:
        """Rotate the current static key.

        Args:
            peer_id: Peer identifier for key storage

        Returns:
            Tuple of (old_public_key, new_public_key)
        """
        if not self._current_public_key:
            await self.load_or_generate_static_key(peer_id)

        try:
            if self._current_public_key is None:
                raise CryptoError("No current public key available")
            old_public_key = bytes(self._current_public_key.encode(RawEncoder))

            _, new_public_key = await self.generate_static_key(peer_id)

            return old_public_key, new_public_key

        except Exception as e:
            raise CryptoError(f"Failed to rotate static key for {peer_id}: {e}")

    async def get_current_static_key(self) -> Optional[Tuple[bytes, bytes]]:
        """Get current static key pair.

        Returns:
            Tuple of (private_key_bytes, public_key_bytes) or None
        """
        if not self._current_static_key or not self._current_public_key:
            return None

        try:
            private_key_bytes = bytes(self._current_static_key.encode(RawEncoder))
            public_key_bytes = bytes(self._current_public_key.encode(RawEncoder))
            return private_key_bytes, public_key_bytes
        except Exception as e:
            raise CryptoError(f"Failed to get current static key: {e}")

    async def create_rotation_message(self, old_key: bytes, new_key: bytes) -> bytes:
        """Create a key rotation message.

        Args:
            old_key: Previous public key
            new_key: New public key

        Returns:
            Serialized rotation message
        """
        try:
            rotation_data = {
                "type": "key_rotation",
                "old_key": old_key.hex(),
                "new_key": new_key.hex(),
                "timestamp": int(time.time()),
                "peer_id": self._peer_id,
            }

            return json.dumps(rotation_data, sort_keys=True).encode("utf-8")

        except Exception as e:
            raise CryptoError(f"Failed to create rotation message: {e}")

    async def announce_key_rotation(self, old_key: bytes, new_key: bytes, signature: bytes) -> None:
        """Announce key rotation to connected peers.

        Args:
            old_key: Previous public key
            new_key: New public key
            signature: Identity signature over rotation message
        """
        try:
            rotation_message = await self.create_rotation_message(old_key, new_key)

            announcement = {
                "message": rotation_message.hex(),
                "signature": signature.hex(),
                "timestamp": int(time.time()),
            }

            json.dumps(announcement).encode("utf-8")

        except Exception as e:
            raise CryptoError(f"Failed to announce key rotation: {e}")

    async def verify_key_rotation(
        self, message: bytes, signature: bytes, identity_key: bytes
    ) -> bool:
        """Verify a key rotation announcement.

        Args:
            message: Key rotation message
            signature: Identity signature
            identity_key: Peer's identity public key

        Returns:
            True if rotation is valid
        """
        if not self.identity:
            raise CryptoError("Identity manager required for key rotation verification")

        try:
            rotation_data = json.loads(message.decode("utf-8"))

            if rotation_data.get("type") != "key_rotation":
                return False

            timestamp = rotation_data.get("timestamp", 0)
            current_time = int(time.time())

            if abs(current_time - timestamp) > 300:
                return False

            is_valid = await self.identity.verify_signature(message, signature, identity_key)
            return is_valid

        except Exception as e:
            raise CryptoError(f"Failed to verify key rotation: {e}")

    async def sign_key_rotation(self, old_key: bytes, new_key: bytes) -> bytes:
        """Sign a key rotation message with identity key.

        Args:
            old_key: Previous public key
            new_key: New public key

        Returns:
            Identity signature over rotation message
        """
        if not self.identity:
            raise CryptoError("Identity manager required for signing key rotation")

        try:
            rotation_message = await self.create_rotation_message(old_key, new_key)
            signature = await self.identity.sign_message(rotation_message)
            return signature

        except Exception as e:
            raise CryptoError(f"Failed to sign key rotation: {e}")

    async def get_key_age(self, peer_id: str) -> Optional[float]:
        """Get the age of the current static key in seconds.

        Args:
            peer_id: Peer identifier

        Returns:
            Key age in seconds, or None if key not found
        """
        try:
            keys = await self.keystore.list_keys()
            static_key_id = f"static_{peer_id}"

            if static_key_id in keys:
                current_time = time.time()
                return current_time

            return None

        except Exception as e:
            raise CryptoError(f"Failed to get key age: {e}")

    def should_rotate_key(self, key_age_seconds: float, max_age_seconds: float = 86400) -> bool:
        """Check if a key should be rotated based on age.

        Args:
            key_age_seconds: Current key age in seconds
            max_age_seconds: Maximum allowed key age (default: 24 hours)

        Returns:
            True if key should be rotated
        """
        return key_age_seconds > max_age_seconds
