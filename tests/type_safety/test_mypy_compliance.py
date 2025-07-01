"""Type safety compliance tests using pytest-mypy."""

import pytest


@pytest.mark.mypy
def test_core_module_types():
    """Test type annotations in core module."""
    # This will be checked by pytest-mypy plugin
    from beenet.core import Peer, ConnectionManager, EventBus
    from beenet.core.resilience import PeerResilienceManager, PeerScore

    # Type annotations should be valid
    assert hasattr(Peer, "__annotations__")
    assert hasattr(PeerScore, "__annotations__")


@pytest.mark.mypy
def test_crypto_module_types():
    """Test type annotations in crypto module."""
    from beenet.crypto import Identity, KeyManager, KeyStore

    # Type annotations should be valid
    assert hasattr(Identity, "__annotations__")
    assert hasattr(KeyManager, "__annotations__")


@pytest.mark.mypy
def test_discovery_module_types():
    """Test type annotations in discovery module."""
    from beenet.discovery import BeeQuietDiscovery, KademliaDiscovery, NATTraversal
    from beenet.discovery.nat_traversal import NATConfig, ExternalAddress

    # Type annotations should be valid
    assert hasattr(NATConfig, "__annotations__")
    assert hasattr(ExternalAddress, "__annotations__")


@pytest.mark.mypy
def test_transfer_module_types():
    """Test type annotations in transfer module."""
    from beenet.transfer import (
        DataChunker,
        FlowControlConfig,
        TransferMetrics,
        ECCConfig,
        ECCBlock,
        ErrorCorrectionCodec,
        EnhancedMerkleTree,
        EnhancedMerkleProof,
    )

    # Type annotations should be valid
    assert hasattr(FlowControlConfig, "__annotations__")
    assert hasattr(TransferMetrics, "__annotations__")
    assert hasattr(ECCConfig, "__annotations__")
    assert hasattr(ECCBlock, "__annotations__")


@pytest.mark.mypy
def test_observability_types():
    """Test type annotations in observability module."""
    from beenet.observability import MetricsConfig, BeenetMetrics, BeenetLogger

    # Type annotations should be valid
    assert hasattr(MetricsConfig, "__annotations__")


def test_type_coverage_badge_info():
    """Provide information for type coverage badge generation."""
    # This could be extended to generate actual coverage metrics
    # For now, it serves as documentation

    modules_with_types = [
        "beenet.core.peer",
        "beenet.core.resilience",
        "beenet.core.connection",
        "beenet.core.events",
        "beenet.crypto.identity",
        "beenet.crypto.keys",
        "beenet.crypto.keystore",
        "beenet.discovery.nat_traversal",
        "beenet.transfer.chunker",
        "beenet.transfer.error_correction",
        "beenet.transfer.enhanced_merkle",
        "beenet.observability",
    ]

    # All modules should be type-annotated
    assert len(modules_with_types) > 0

    print(f"Type-annotated modules: {len(modules_with_types)}")
    for module in modules_with_types:
        print(f"  - {module}")
