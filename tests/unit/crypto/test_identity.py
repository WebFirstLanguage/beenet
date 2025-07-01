"""Unit tests for Identity functionality."""

import pytest

from beenet.core.errors import CryptoError
from beenet.crypto import Identity, KeyStore


class TestIdentity:
    """Test Identity functionality."""

    @pytest.mark.asyncio
    async def test_identity_generation(self, identity: Identity):
        """Test identity key generation."""
        assert identity.public_key is not None
        assert len(identity.public_key) == 32  # Ed25519 public key size
        assert identity.peer_id is not None

    @pytest.mark.asyncio
    async def test_identity_persistence(self, temp_dir):
        """Test identity persistence across sessions."""
        keystore_path = temp_dir / "identity_keystore"
        peer_id = "persistent_peer"

        keystore1 = KeyStore(keystore_path)
        await keystore1.open()
        identity1 = Identity(keystore1)
        await identity1.load_or_generate_identity(peer_id)
        public_key1 = identity1.public_key
        await keystore1.close()

        keystore2 = KeyStore(keystore_path)
        await keystore2.open()
        identity2 = Identity(keystore2)
        await identity2.load_or_generate_identity(peer_id)
        public_key2 = identity2.public_key
        await keystore2.close()

        assert public_key1 == public_key2

    @pytest.mark.asyncio
    async def test_sign_and_verify(self, identity: Identity):
        """Test message signing and verification."""
        message = b"Test message for signing"

        signature = await identity.sign_message(message)
        assert len(signature) == 64  # Ed25519 signature size

        is_valid = await identity.verify_signature(message, signature, identity.public_key)
        assert is_valid

    @pytest.mark.asyncio
    async def test_verify_invalid_signature(self, identity: Identity):
        """Test verification of invalid signature."""
        message = b"Test message for signing"
        invalid_signature = b"invalid_signature" + b"\x00" * 50

        is_valid = await identity.verify_signature(message, invalid_signature, identity.public_key)
        assert not is_valid

    @pytest.mark.asyncio
    async def test_verify_wrong_key(self, temp_dir):
        """Test verification with wrong public key."""
        keystore_path = temp_dir / "wrong_key_keystore"

        keystore1 = KeyStore(keystore_path)
        await keystore1.open()
        identity1 = Identity(keystore1)
        await identity1.load_or_generate_identity("peer1")

        keystore2 = KeyStore(keystore_path)
        await keystore2.open()
        identity2 = Identity(keystore2)
        await identity2.load_or_generate_identity("peer2")

        message = b"Test message"
        signature = await identity1.sign_message(message)

        is_valid = await identity2.verify_signature(message, signature, identity2.public_key)
        assert not is_valid

        await keystore1.close()
        await keystore2.close()

    @pytest.mark.asyncio
    async def test_identity_derivation(self, identity: Identity):
        """Test that peer ID is derived from public key."""
        derived_peer_id = identity.derive_peer_id(identity.public_key)
        assert len(derived_peer_id) > 0
        assert isinstance(derived_peer_id, str)
        
        derived_peer_id2 = identity.derive_peer_id(identity.public_key)
        assert derived_peer_id == derived_peer_id2

    @pytest.mark.asyncio
    async def test_identity_error_handling(self, temp_dir):
        """Test identity error handling."""
        keystore_path = temp_dir / "error_keystore"
        keystore = KeyStore(keystore_path)
        identity = Identity(keystore)

        with pytest.raises(CryptoError):
            await identity.sign_message(b"test message")

        with pytest.raises(CryptoError):
            await identity.verify_signature(b"test", b"valid_64_byte_signature" + b"\x00" * 37, b"short")
