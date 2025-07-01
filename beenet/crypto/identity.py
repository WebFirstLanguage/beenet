"""Ed25519 identity key management for beenet peers."""

from typing import Optional, Tuple

from nacl.encoding import RawEncoder
from nacl.exceptions import BadSignatureError
from nacl.signing import SigningKey, VerifyKey

from ..core.errors import CryptoError
from .keystore import KeyStore


class Identity:
    """Manages Ed25519 identity keys for beenet peers.

    Each peer has a long-term identity key pair used for:
    - Peer identification and authentication
    - Signing key rotation announcements
    - Verifying peer signatures
    """

    def __init__(self, keystore: KeyStore):
        self.keystore = keystore
        self._signing_key: Optional[SigningKey] = None
        self._verify_key: Optional[VerifyKey] = None
        self._peer_id: Optional[str] = None

    async def load_or_generate_identity(self, peer_id: str) -> bytes:
        """Load existing identity or generate new one.

        Args:
            peer_id: Unique identifier for this peer

        Returns:
            Public key bytes for this identity
        """
        self._peer_id = peer_id
        identity_key_id = f"identity_{peer_id}"

        try:
            private_key_bytes = await self.keystore.get_key(identity_key_id)

            if private_key_bytes:
                self._signing_key = SigningKey(private_key_bytes)
                self._verify_key = self._signing_key.verify_key
            else:
                self._signing_key = SigningKey.generate()
                self._verify_key = self._signing_key.verify_key

                private_key_bytes = bytes(self._signing_key.encode(RawEncoder))
                await self.keystore.store_key(identity_key_id, private_key_bytes, encrypted=True)

            return bytes(self._verify_key.encode(RawEncoder))

        except Exception as e:
            raise CryptoError(f"Failed to load or generate identity for {peer_id}: {e}")

    async def sign_message(self, message: bytes) -> bytes:
        """Sign a message with the identity key.

        Args:
            message: Message to sign

        Returns:
            Signature bytes
        """
        if not self._signing_key:
            raise CryptoError("Identity not initialized - call load_or_generate first")

        try:
            signed_message = self._signing_key.sign(message)
            return signed_message.signature
        except Exception as e:
            raise CryptoError(f"Failed to sign message: {e}")

    async def verify_signature(self, message: bytes, signature: bytes, public_key: bytes) -> bool:
        """Verify a signature against a public key.

        Args:
            message: Original message
            signature: Signature to verify
            public_key: Public key to verify against

        Returns:
            True if signature is valid
        """
        try:
            if len(public_key) != 32:
                raise CryptoError(
                    f"Invalid public key length: {len(public_key)}, expected 32 bytes"
                )

            verify_key = VerifyKey(public_key, encoder=RawEncoder)
            verify_key.verify(message, signature)
            return True
        except BadSignatureError:
            return False
        except ValueError as e:
            if "signature must be exactly" in str(e):
                return False
            else:
                raise CryptoError(f"Failed to verify signature: {e}")
        except Exception as e:
            raise CryptoError(f"Failed to verify signature: {e}")

    def export_public_key(self) -> bytes:
        """Export the public key for this identity.

        Returns:
            Public key bytes
        """
        if not self._verify_key:
            raise CryptoError("Identity not initialized - call load_or_generate first")

        return bytes(self._verify_key.encode(RawEncoder))

    def derive_peer_id(self, public_key: bytes) -> str:
        """Generate a peer ID from a public key.

        Args:
            public_key: Ed25519 public key bytes

        Returns:
            Peer ID string
        """
        import base64
        import hashlib

        key_hash = hashlib.blake2b(public_key, digest_size=16).digest()
        peer_id = base64.b32encode(key_hash).decode("ascii").rstrip("=").lower()
        return peer_id

    async def rotate_identity(self) -> Tuple[bytes, bytes]:
        """Rotate the identity key (emergency use only).

        Returns:
            Tuple of (old_public_key, new_public_key)
        """
        if not self._peer_id:
            raise CryptoError("Identity not initialized - call load_or_generate first")

        try:
            old_public_key = self.export_public_key()

            new_signing_key = SigningKey.generate()
            new_verify_key = new_signing_key.verify_key

            identity_key_id = f"identity_{self._peer_id}"
            private_key_bytes = bytes(new_signing_key.encode(RawEncoder))
            await self.keystore.store_key(identity_key_id, private_key_bytes, encrypted=True)

            self._signing_key = new_signing_key
            self._verify_key = new_verify_key

            new_public_key = bytes(new_verify_key.encode(RawEncoder))
            return old_public_key, new_public_key

        except Exception as e:
            raise CryptoError(f"Failed to rotate identity: {e}")

    @property
    def public_key(self) -> Optional[bytes]:
        """Get the public key bytes for this identity."""
        if self._verify_key:
            return bytes(self._verify_key.encode(RawEncoder))
        return None

    @property
    def is_initialized(self) -> bool:
        """Check if identity is initialized."""
        return self._signing_key is not None and self._verify_key is not None

    @property
    def peer_id(self) -> Optional[str]:
        """Get the peer ID for this identity."""
        return self._peer_id
