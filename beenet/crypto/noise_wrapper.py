"""Async Noise XX protocol wrapper for secure peer channels."""

from typing import Any, Optional

from noise.connection import Keypair, NoiseConnection  # type: ignore[import-untyped]

from ..core.errors import CryptoError


class NoiseChannel:
    """Async wrapper for Noise XX secure channels.

    Provides mutual authentication and forward secrecy for all peer traffic.
    Uses Noise_XX_25519_ChaChaPoly_BLAKE2b configuration.
    """

    def __init__(self, is_initiator: bool = False):
        self.is_initiator = is_initiator
        self._noise: Optional[NoiseConnection] = None
        self._handshake_complete = False
        self._remote_static_key: Optional[bytes] = None

    async def start_handshake(self, static_key: Optional[bytes] = None) -> bytes:
        """Start the Noise XX handshake.

        Args:
            static_key: Optional static key for this peer

        Returns:
            First handshake message to send
        """
        try:
            protocol_name = b"Noise_XX_25519_ChaChaPoly_BLAKE2b"
            self._noise = NoiseConnection.from_name(protocol_name)

            if static_key:
                self._noise.set_keypair_from_private_bytes(Keypair.STATIC, static_key)

            self._noise.set_as_initiator() if self.is_initiator else self._noise.set_as_responder()
            self._noise.start_handshake()

            if self.is_initiator:
                message = self._noise.write_message()
                return bytes(message)
            else:
                return b""

        except Exception as e:
            raise CryptoError(f"Failed to start Noise handshake: {e}")

    async def process_handshake_message(self, message: bytes) -> Optional[bytes]:
        """Process incoming handshake message.

        Args:
            message: Handshake message from peer

        Returns:
            Response message to send, or None if handshake complete
        """
        if not self._noise:
            raise CryptoError("Handshake not started - call start_handshake first")

        try:
            self._noise.read_message(message)

            if self._noise.handshake_finished:
                self._handshake_complete = True
                self._remote_static_key = self._noise.get_remote_static_key()
                return None
            else:
                response = self._noise.write_message()
                return bytes(response)

        except Exception as e:
            raise CryptoError(f"Failed to process handshake message: {e}")

    async def encrypt(self, plaintext: bytes) -> bytes:
        """Encrypt data for transmission.

        Args:
            plaintext: Data to encrypt

        Returns:
            Encrypted data
        """
        if not self._handshake_complete:
            raise CryptoError("Handshake not complete - cannot encrypt data")

        if not self._noise:
            raise CryptoError("Noise connection not initialized")

        try:
            ciphertext = self._noise.encrypt(plaintext)
            return bytes(ciphertext)
        except Exception as e:
            raise CryptoError(f"Failed to encrypt data: {e}")

    async def decrypt(self, ciphertext: bytes) -> bytes:
        """Decrypt received data.

        Args:
            ciphertext: Encrypted data

        Returns:
            Decrypted plaintext
        """
        if not self._handshake_complete:
            raise CryptoError("Handshake not complete - cannot decrypt data")

        if not self._noise:
            raise CryptoError("Noise connection not initialized")

        try:
            plaintext = self._noise.decrypt(ciphertext)
            return bytes(plaintext)
        except Exception as e:
            raise CryptoError(f"Failed to decrypt data: {e}")

    async def rekey(self) -> None:
        """Rekey the Noise connection for forward secrecy.

        Should be called periodically to maintain forward secrecy.
        """
        if not self._handshake_complete:
            raise CryptoError("Handshake not complete - cannot rekey")

        if not self._noise:
            raise CryptoError("Noise connection not initialized")

        try:
            self._noise.rekey()
        except Exception as e:
            raise CryptoError(f"Failed to rekey connection: {e}")

    def get_handshake_hash(self) -> Optional[bytes]:
        """Get the handshake hash for channel binding.

        Returns:
            Handshake hash bytes if handshake is complete
        """
        if not self._handshake_complete or not self._noise:
            return None

        try:
            return self._noise.get_handshake_hash()
        except Exception:
            return None

    @property
    def handshake_complete(self) -> bool:
        """Check if handshake is complete."""
        return self._handshake_complete

    @property
    def remote_static_key(self) -> Optional[bytes]:
        """Get remote peer's static key after handshake."""
        return self._remote_static_key

    @property
    def cipher_state_send(self) -> Any:
        """Get the send cipher state for advanced operations."""
        if self._noise and self._handshake_complete:
            return self._noise.cipher_state_encrypt
        return None

    @property
    def cipher_state_recv(self) -> Any:
        """Get the receive cipher state for advanced operations."""
        if self._noise and self._handshake_complete:
            return self._noise.cipher_state_decrypt
        return None
