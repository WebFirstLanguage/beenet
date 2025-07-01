"""Unit tests for BeeQuiet discovery functionality."""

import asyncio
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from beenet.core.errors import DiscoveryError
from beenet.discovery import BeeQuietDiscovery


class TestBeeQuietDiscovery:
    """Test BeeQuiet discovery functionality."""

    @pytest.mark.asyncio
    async def test_discovery_creation(self):
        """Test BeeQuiet discovery creation."""
        peer_id = "test_peer"
        callback = AsyncMock()

        discovery = BeeQuietDiscovery(peer_id, callback)

        assert discovery.peer_id == peer_id
        assert discovery.peer_discovered_callback == callback
        assert not discovery.is_running

    @pytest.mark.asyncio
    async def test_start_discovery(self, beequiet_discovery):
        """Test starting BeeQuiet discovery."""
        await beequiet_discovery.start()

        assert beequiet_discovery.is_running

        await beequiet_discovery.stop()

    @pytest.mark.asyncio
    async def test_stop_discovery(self, beequiet_discovery):
        """Test stopping BeeQuiet discovery."""
        await beequiet_discovery.start()
        assert beequiet_discovery.is_running

        await beequiet_discovery.stop()
        assert not beequiet_discovery.is_running

    @pytest.mark.asyncio
    async def test_send_who_is_here(self, beequiet_discovery):
        """Test sending WHO_IS_HERE message."""
        await beequiet_discovery.start()

        await beequiet_discovery.send_who_is_here()

        await beequiet_discovery.stop()

    @pytest.mark.asyncio
    async def test_send_i_am_here(self, beequiet_discovery):
        """Test sending I_AM_HERE message."""
        await beequiet_discovery.start()

        nonce = b"test_nonce_16bytes"
        peer_address = ("127.0.0.1", 8000)

        await beequiet_discovery.send_i_am_here(nonce, peer_address)

        await beequiet_discovery.stop()

    @pytest.mark.asyncio
    async def test_send_heartbeat(self, beequiet_discovery):
        """Test sending HEARTBEAT message."""
        await beequiet_discovery.start()

        peer_id = "target_peer"
        session_key = b"session_key_32_bytes_exactly!!!!"
        peer_address = ("127.0.0.1", 8000)

        await beequiet_discovery.send_heartbeat(peer_id, session_key, peer_address)

        await beequiet_discovery.stop()

    @pytest.mark.asyncio
    async def test_send_goodbye(self, beequiet_discovery):
        """Test sending GOODBYE message."""
        await beequiet_discovery.start()

        peer_id = "target_peer"
        session_key = b"session_key_32_bytes_exactly!!!!"
        peer_address = ("127.0.0.1", 8000)

        await beequiet_discovery.send_goodbye(peer_id, session_key, peer_address)

        await beequiet_discovery.stop()

    @pytest.mark.asyncio
    async def test_get_discovered_peers(self, beequiet_discovery):
        """Test getting discovered peers."""
        await beequiet_discovery.start()

        peers = beequiet_discovery.get_discovered_peers()
        assert isinstance(peers, list)

        await beequiet_discovery.stop()

    @pytest.mark.asyncio
    async def test_derive_session_key(self, beequiet_discovery):
        """Test session key derivation."""
        nonce = b"test_nonce_16bytes"
        response = b"test_response_data"

        session_key = beequiet_discovery.derive_session_key(nonce, response)

        assert isinstance(session_key, bytes)
        assert len(session_key) == 32  # ChaCha20-Poly1305 key size

    @pytest.mark.asyncio
    async def test_encrypt_decrypt_message(self, beequiet_discovery):
        """Test message encryption and decryption."""
        session_key = b"session_key_32_bytes_exactly!!!!"
        message = b"test message data"

        encrypted = beequiet_discovery.encrypt_message(message, session_key)
        assert encrypted != message
        assert len(encrypted) > len(message)  # Includes nonce and tag

        decrypted = beequiet_discovery.decrypt_message(encrypted, session_key)
        assert decrypted == message

    @pytest.mark.asyncio
    async def test_invalid_decryption(self, beequiet_discovery):
        """Test decryption with wrong key."""
        session_key1 = b"session_key1_32_bytes_exactly!!!"
        session_key2 = b"session_key2_32_bytes_exactly!!!"
        message = b"test message data"

        encrypted = beequiet_discovery.encrypt_message(message, session_key1)

        decrypted = beequiet_discovery.decrypt_message(encrypted, session_key2)
        assert decrypted is None  # Should fail gracefully

    @pytest.mark.asyncio
    async def test_protocol_constants(self, beequiet_discovery):
        """Test protocol constants."""
        assert beequiet_discovery.MULTICAST_GROUP == "239.255.7.7"
        assert beequiet_discovery.MULTICAST_PORT == 7777
        assert beequiet_discovery.MAGIC_NUMBER == 0xBEEC

    @pytest.mark.asyncio
    async def test_message_types(self, beequiet_discovery):
        """Test message type constants."""
        assert hasattr(beequiet_discovery, "WHO_IS_HERE")
        assert hasattr(beequiet_discovery, "I_AM_HERE")
        assert hasattr(beequiet_discovery, "HEARTBEAT")
        assert hasattr(beequiet_discovery, "GOODBYE")

    @pytest.mark.asyncio
    async def test_operations_without_start(self):
        """Test operations without starting discovery."""
        peer_id = "test_peer"
        callback = AsyncMock()
        discovery = BeeQuietDiscovery(peer_id, callback)

        with pytest.raises(DiscoveryError):
            await discovery.send_who_is_here()

        with pytest.raises(DiscoveryError):
            await discovery.send_i_am_here(b"nonce", ("127.0.0.1", 8000))

    @pytest.mark.asyncio
    async def test_double_start_stop(self, beequiet_discovery):
        """Test double start/stop operations."""
        await beequiet_discovery.start()
        await beequiet_discovery.start()  # Should not error

        await beequiet_discovery.stop()
        await beequiet_discovery.stop()  # Should not error

    @pytest.mark.asyncio
    async def test_peer_discovery_callback(self):
        """Test peer discovery callback invocation."""
        callback = AsyncMock()
        peer_id = "test_peer"
        discovery = BeeQuietDiscovery(peer_id, callback)

        await discovery.start()

        peer_info = {"peer_id": "discovered_peer", "address": "127.0.0.1", "port": 8000}

        await discovery._on_peer_discovered(peer_info)
        callback.assert_called_once_with(peer_info)

        await discovery.stop()
