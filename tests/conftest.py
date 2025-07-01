"""Pytest configuration and shared fixtures for beenet tests."""

import asyncio
import tempfile
from pathlib import Path
from typing import AsyncGenerator, Generator

import pytest

from beenet.core import EventBus, Peer
from beenet.crypto import Identity, KeyManager, KeyStore
from beenet.discovery import BeeQuietDiscovery, KademliaDiscovery


@pytest.fixture(scope="session")
def event_loop():
    """Create an instance of the default event loop for the test session."""
    loop = asyncio.get_event_loop_policy().new_event_loop()
    yield loop
    loop.close()


@pytest.fixture
def temp_dir() -> Generator[Path, None, None]:
    """Create a temporary directory for tests."""
    with tempfile.TemporaryDirectory() as tmpdir:
        yield Path(tmpdir)


@pytest.fixture
async def keystore(temp_dir: Path) -> AsyncGenerator[KeyStore, None]:
    """Create a test keystore."""
    keystore_path = temp_dir / "test_keystore"
    keystore = KeyStore(keystore_path)
    await keystore.open()
    yield keystore
    await keystore.close()


@pytest.fixture
async def identity(keystore: KeyStore) -> AsyncGenerator[Identity, None]:
    """Create a test identity."""
    identity = Identity(keystore)
    await identity.load_or_generate_identity("test_peer")
    yield identity


@pytest.fixture
async def key_manager(keystore: KeyStore) -> AsyncGenerator[KeyManager, None]:
    """Create a test key manager."""
    key_manager = KeyManager(keystore)
    await key_manager.load_or_generate_static_key("test_peer")
    yield key_manager


@pytest.fixture
def event_bus() -> EventBus:
    """Create a test event bus."""
    return EventBus()


@pytest.fixture
async def test_peer(temp_dir: Path) -> AsyncGenerator[Peer, None]:
    """Create a test peer."""
    keystore_path = temp_dir / "peer_keystore"
    peer = Peer("test_peer_001", keystore_path)
    yield peer
    if peer.is_running:
        await peer.stop()


@pytest.fixture
async def two_test_peers(temp_dir: Path) -> AsyncGenerator[tuple[Peer, Peer], None]:
    """Create two test peers for integration testing."""
    peer1_keystore = temp_dir / "peer1_keystore"
    peer2_keystore = temp_dir / "peer2_keystore"

    peer1 = Peer("test_peer_001", peer1_keystore)
    peer2 = Peer("test_peer_002", peer2_keystore)

    yield peer1, peer2

    if peer1.is_running:
        await peer1.stop()
    if peer2.is_running:
        await peer2.stop()


@pytest.fixture
def sample_data() -> bytes:
    """Generate sample data for testing."""
    return b"Hello, beenet! " * 1000  # ~15KB of test data


@pytest.fixture
def large_sample_data() -> bytes:
    """Generate large sample data for integration testing."""
    return b"Large test data chunk. " * (10 * 1024 * 1024 // 23)  # ~10MB


@pytest.fixture
async def kademlia_discovery() -> AsyncGenerator[KademliaDiscovery, None]:
    """Create a test Kademlia discovery instance."""
    discovery = KademliaDiscovery()
    yield discovery
    if discovery.is_running:
        await discovery.stop()


@pytest.fixture
async def beequiet_discovery() -> AsyncGenerator[BeeQuietDiscovery, None]:
    """Create a test BeeQuiet discovery instance."""
    discovery = BeeQuietDiscovery("test_peer", lambda x: None)
    yield discovery
    if discovery.is_running:
        await discovery.stop()


@pytest.fixture
def mock_peer_info() -> dict:
    """Mock peer information for testing."""
    return {
        "peer_id": "mock_peer_123",
        "address": "127.0.0.1",
        "port": 8000,
        "protocol": "beenet",
        "version": "0.1.0",
    }


@pytest.fixture
def test_file_content() -> bytes:
    """Test file content for transfer testing."""
    return b"This is test file content for beenet transfer testing.\n" * 100


@pytest.fixture
def test_file(temp_dir: Path, test_file_content: bytes) -> Path:
    """Create a test file for transfer testing."""
    test_file_path = temp_dir / "test_file.txt"
    test_file_path.write_bytes(test_file_content)
    return test_file_path


@pytest.fixture
def large_test_file(temp_dir: Path, large_sample_data: bytes) -> Path:
    """Create a large test file for integration testing."""
    large_file_path = temp_dir / "large_test_file.bin"
    large_file_path.write_bytes(large_sample_data)
    return large_file_path
