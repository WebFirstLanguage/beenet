"""Data transfer layer for beenet.

This module provides:
- Merkle tree-based data integrity verification
- Chunked data transfer with size negotiation
- Resumable transfer streams with state persistence
"""

from .chunker import DataChunker
from .merkle import MerkleProof, MerkleTree
from .stream import TransferStream

__all__ = ["MerkleTree", "MerkleProof", "DataChunker", "TransferStream"]
