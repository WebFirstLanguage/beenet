"""Data transfer layer for beenet.

This module provides:
- Merkle tree-based data integrity verification
- Chunked data transfer with size negotiation and flow control
- Resumable transfer streams with state persistence
- Forward error correction using Reed-Solomon codes
- Enhanced Merkle trees with automatic error recovery
"""

from .chunker import DataChunker, FlowControlConfig, TransferMetrics
from .error_correction import ECCBlock, ECCConfig, ErrorCorrectionCodec
from .enhanced_merkle import EnhancedMerkleProof, EnhancedMerkleTree
from .merkle import MerkleProof, MerkleTree
from .stream import TransferStream

__all__ = [
    "MerkleTree",
    "MerkleProof",
    "DataChunker",
    "TransferStream",
    "FlowControlConfig",
    "TransferMetrics",
    "ECCConfig",
    "ECCBlock",
    "ErrorCorrectionCodec",
    "EnhancedMerkleTree",
    "EnhancedMerkleProof",
]
