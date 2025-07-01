"""BLAKE2b-based Merkle tree implementation for data integrity."""

import hashlib
import math
from typing import Any, List, Optional, Tuple

from ..core.errors import TransferError


class MerkleProof:
    """Merkle proof for chunk verification."""

    def __init__(self, chunk_index: int, chunk_hash: bytes, proof_hashes: List[bytes]):
        self.chunk_index = chunk_index
        self.chunk_hash = chunk_hash
        self.proof_hashes = proof_hashes

    def verify(self, root_hash: bytes) -> bool:
        """Verify this proof against a root hash.

        Args:
            root_hash: Expected Merkle root hash

        Returns:
            True if proof is valid
        """
        try:
            if not isinstance(root_hash, bytes) or len(root_hash) != 32:
                return False

            if not isinstance(self.chunk_hash, bytes) or len(self.chunk_hash) != 32:
                return False

            if self.chunk_index < 0:
                return False

            for proof_hash in self.proof_hashes:
                if not isinstance(proof_hash, bytes) or len(proof_hash) != 32:
                    return False

            current_hash = self.chunk_hash
            current_index = self.chunk_index

            for proof_hash in self.proof_hashes:
                if current_index % 2 == 0:
                    current_hash = MerkleTree.hash_pair(current_hash, proof_hash)
                else:
                    current_hash = MerkleTree.hash_pair(proof_hash, current_hash)

                current_index //= 2

            return current_hash == root_hash

        except Exception:
            return False


class MerkleTree:
    """BLAKE2b-based Merkle tree for data integrity verification.

    Provides:
    - Efficient tree construction from data chunks
    - Proof generation for individual chunks
    - Fuzz-resistant proof verification
    - BLAKE2b hashing for cryptographic security
    """

    def __init__(self, chunk_hashes: Optional[List[bytes]] = None):
        self.chunk_hashes = chunk_hashes or []
        self._tree_levels: List[List[bytes]] = []
        self._root_hash: Optional[bytes] = None

    def add_chunk_hash(self, chunk_hash: bytes) -> None:
        """Add a chunk hash to the tree.

        Args:
            chunk_hash: BLAKE2b hash of a data chunk
        """
        if not isinstance(chunk_hash, bytes) or len(chunk_hash) != 32:
            raise TransferError("Chunk hash must be 32 bytes")

        self.chunk_hashes.append(chunk_hash)
        self._invalidate_tree()

    def build_tree(self) -> bytes:
        """Build the Merkle tree and return root hash.

        Returns:
            Root hash of the Merkle tree
        """
        if not self.chunk_hashes:
            raise TransferError("Cannot build tree with no chunk hashes")

        try:
            self._tree_levels = []
            current_level = self.chunk_hashes.copy()
            self._tree_levels.append(current_level)

            while len(current_level) > 1:
                next_level = []

                for i in range(0, len(current_level), 2):
                    left = current_level[i]

                    if i + 1 < len(current_level):
                        right = current_level[i + 1]
                    else:
                        right = left

                    parent_hash = self.hash_pair(left, right)
                    next_level.append(parent_hash)

                self._tree_levels.append(next_level)
                current_level = next_level

            self._root_hash = current_level[0]
            return self._root_hash

        except Exception as e:
            raise TransferError(f"Failed to build Merkle tree: {e}")

    def generate_proof(self, chunk_index: int) -> MerkleProof:
        """Generate a Merkle proof for a specific chunk.

        Args:
            chunk_index: Index of the chunk to prove

        Returns:
            Merkle proof for the chunk
        """
        if chunk_index < 0 or chunk_index >= len(self.chunk_hashes):
            raise TransferError(f"Invalid chunk index: {chunk_index}")

        if not self._tree_levels:
            self.build_tree()

        try:
            proof_hashes = []
            current_index = chunk_index

            for level in self._tree_levels[:-1]:
                if current_index % 2 == 0:
                    sibling_index = current_index + 1
                else:
                    sibling_index = current_index - 1

                if sibling_index < len(level):
                    proof_hashes.append(level[sibling_index])
                else:
                    proof_hashes.append(level[current_index])

                current_index //= 2

            chunk_hash = self.chunk_hashes[chunk_index]
            return MerkleProof(chunk_index, chunk_hash, proof_hashes)

        except Exception as e:
            raise TransferError(f"Failed to generate proof for chunk {chunk_index}: {e}")

    def verify_chunk(self, chunk_data: bytes, chunk_index: int, proof: MerkleProof) -> bool:
        """Verify a chunk against its Merkle proof.

        Args:
            chunk_data: Raw chunk data
            chunk_index: Index of the chunk
            proof: Merkle proof for the chunk

        Returns:
            True if chunk is valid
        """
        try:
            if not isinstance(chunk_data, bytes):
                return False

            if chunk_index != proof.chunk_index:
                return False

            computed_hash = self.hash_chunk(chunk_data)

            if computed_hash != proof.chunk_hash:
                return False

            if not self._root_hash:
                self.build_tree()

            return proof.verify(self._root_hash)

        except Exception:
            return False

    @property
    def root_hash(self) -> Optional[bytes]:
        """Get the root hash of the tree."""
        if self._root_hash is None and self.chunk_hashes:
            self._root_hash = self.build_tree()
        return self._root_hash

    def _invalidate_tree(self) -> None:
        """Invalidate cached tree data."""
        self._tree_levels.clear()
        self._root_hash = None

    @staticmethod
    def hash_chunk(data: bytes) -> bytes:
        """Hash a data chunk with BLAKE2b.

        Args:
            data: Chunk data to hash

        Returns:
            BLAKE2b hash of the data
        """
        return hashlib.blake2b(data, digest_size=32).digest()

    @staticmethod
    def hash_pair(left: bytes, right: bytes) -> bytes:
        """Hash a pair of hashes for tree construction.

        Args:
            left: Left hash
            right: Right hash

        Returns:
            BLAKE2b hash of the concatenated pair
        """
        return hashlib.blake2b(left + right, digest_size=32).digest()
