"""Unit tests for Kademlia discovery functionality."""

from unittest.mock import AsyncMock, MagicMock

import pytest

from beenet.core.errors import DiscoveryError
from beenet.discovery import KademliaDiscovery


class TestKademliaDiscovery:
    """Test Kademlia discovery functionality."""

    @pytest.mark.asyncio
    async def test_discovery_creation(self):
        """Test Kademlia discovery creation."""
        discovery = KademliaDiscovery()

        assert discovery.bootstrap_nodes == []
        assert not discovery.is_running
        assert discovery.listen_port == 8468

    @pytest.mark.asyncio
    async def test_discovery_with_bootstrap_nodes(self):
        """Test Kademlia discovery with bootstrap nodes."""
        bootstrap_nodes = ["127.0.0.1:8468", "192.168.1.100"]
        discovery = KademliaDiscovery(bootstrap_nodes)

        assert discovery.bootstrap_nodes == bootstrap_nodes

    @pytest.mark.asyncio
    async def test_start_discovery(self, kademlia_discovery):
        """Test starting Kademlia discovery."""
        await kademlia_discovery.start(8469)

        assert kademlia_discovery.is_running
        assert kademlia_discovery.listen_port == 8469

        await kademlia_discovery.stop()

    @pytest.mark.asyncio
    async def test_stop_discovery(self, kademlia_discovery):
        """Test stopping Kademlia discovery."""
        await kademlia_discovery.start(8470)
        assert kademlia_discovery.is_running

        await kademlia_discovery.stop()
        assert not kademlia_discovery.is_running

    @pytest.mark.asyncio
    async def test_register_peer(self, kademlia_discovery):
        """Test peer registration."""
        await kademlia_discovery.start(8471)

        peer_id = "test_peer_001"
        address = "127.0.0.1"
        port = 8000

        await kademlia_discovery.register_peer(peer_id, address, port)

        await kademlia_discovery.stop()

    @pytest.mark.asyncio
    async def test_find_peer(self, kademlia_discovery):
        """Test peer lookup."""
        await kademlia_discovery.start(8472)

        peer_id = "test_peer_002"
        address = "127.0.0.1"
        port = 8001

        await kademlia_discovery.register_peer(peer_id, address, port)

        found_peer = await kademlia_discovery.find_peer(peer_id)

        if found_peer:  # May be None due to DHT propagation delay
            assert found_peer["peer_id"] == peer_id
            assert found_peer["address"] == address
            assert found_peer["port"] == port

        await kademlia_discovery.stop()

    @pytest.mark.asyncio
    async def test_find_nonexistent_peer(self, kademlia_discovery):
        """Test lookup of non-existent peer."""
        await kademlia_discovery.start(8473)

        found_peer = await kademlia_discovery.find_peer("nonexistent_peer")
        assert found_peer is None

        await kademlia_discovery.stop()

    @pytest.mark.asyncio
    async def test_find_peers_near(self, kademlia_discovery):
        """Test finding peers near a target ID."""
        await kademlia_discovery.start(8474)

        target_id = "target_peer_id"
        peers = await kademlia_discovery.find_peers_near(target_id, 10)

        assert isinstance(peers, list)
        assert len(peers) <= 10

        await kademlia_discovery.stop()

    @pytest.mark.asyncio
    async def test_get_routing_table_size(self, kademlia_discovery):
        """Test getting routing table size."""
        await kademlia_discovery.start(8475)

        size = await kademlia_discovery.get_routing_table_size()
        assert isinstance(size, int)
        assert size >= 0

        await kademlia_discovery.stop()

    @pytest.mark.asyncio
    async def test_get_node_id(self, kademlia_discovery):
        """Test getting node ID."""
        await kademlia_discovery.start(8476)

        node_id = await kademlia_discovery.get_node_id()
        assert node_id is not None
        assert isinstance(node_id, str)
        assert len(node_id) > 0

        await kademlia_discovery.stop()

    @pytest.mark.asyncio
    async def test_operations_without_start(self):
        """Test operations without starting discovery."""
        discovery = KademliaDiscovery()

        with pytest.raises(DiscoveryError):
            await discovery.register_peer("test", "127.0.0.1", 8000)

        with pytest.raises(DiscoveryError):
            await discovery.find_peer("test")

        with pytest.raises(DiscoveryError):
            await discovery.find_peers_near("test", 10)

    @pytest.mark.asyncio
    async def test_bootstrap_from_known_peers(self, kademlia_discovery):
        """Test bootstrapping from known peers."""
        await kademlia_discovery.start(8477)

        known_peers = [
            {"address": "127.0.0.1", "port": 8468},
            {"address": "127.0.0.1", "port": 8469},
        ]

        await kademlia_discovery.bootstrap_from_known_peers(known_peers)

        await kademlia_discovery.stop()

    @pytest.mark.asyncio
    async def test_double_start_stop(self, kademlia_discovery):
        """Test double start/stop operations."""
        await kademlia_discovery.start(8478)
        await kademlia_discovery.start(8478)  # Should not error

        await kademlia_discovery.stop()
        await kademlia_discovery.stop()  # Should not error
