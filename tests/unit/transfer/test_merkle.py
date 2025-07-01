"""Unit tests for Merkle tree functionality."""

import hashlib

import pytest

from beenet.core.errors import TransferError
from beenet.transfer import MerkleProof, MerkleTree


class TestMerkleTree:
    """Test Merkle tree functionality."""

    def test_empty_tree_error(self):
        """Test that empty tree raises error."""
        tree = MerkleTree()

        with pytest.raises(TransferError):
            tree.build_tree()

    def test_single_chunk_tree(self):
        """Test tree with single chunk."""
        chunk_data = b"single chunk data"
        chunk_hash = MerkleTree.hash_chunk(chunk_data)

        tree = MerkleTree([chunk_hash])
        root_hash = tree.build_tree()

        assert root_hash == chunk_hash
        assert tree.root_hash == root_hash

    def test_two_chunk_tree(self):
        """Test tree with two chunks."""
        chunk1_data = b"first chunk data"
        chunk2_data = b"second chunk data"

        chunk1_hash = MerkleTree.hash_chunk(chunk1_data)
        chunk2_hash = MerkleTree.hash_chunk(chunk2_data)

        tree = MerkleTree([chunk1_hash, chunk2_hash])
        root_hash = tree.build_tree()

        expected_root = MerkleTree.hash_pair(chunk1_hash, chunk2_hash)
        assert root_hash == expected_root

    def test_odd_number_chunks(self):
        """Test tree with odd number of chunks."""
        chunks = [b"chunk1", b"chunk2", b"chunk3"]
        chunk_hashes = [MerkleTree.hash_chunk(chunk) for chunk in chunks]

        tree = MerkleTree(chunk_hashes)
        root_hash = tree.build_tree()

        assert root_hash is not None
        assert len(root_hash) == 32  # BLAKE2b digest size

    def test_large_tree(self):
        """Test tree with many chunks."""
        chunks = [f"chunk_{i}".encode() for i in range(100)]
        chunk_hashes = [MerkleTree.hash_chunk(chunk) for chunk in chunks]

        tree = MerkleTree(chunk_hashes)
        root_hash = tree.build_tree()

        assert root_hash is not None
        assert len(root_hash) == 32

    def test_proof_generation(self):
        """Test Merkle proof generation."""
        chunks = [b"chunk1", b"chunk2", b"chunk3", b"chunk4"]
        chunk_hashes = [MerkleTree.hash_chunk(chunk) for chunk in chunks]

        tree = MerkleTree(chunk_hashes)
        tree.build_tree()

        proof = tree.generate_proof(0)
        assert proof.chunk_index == 0
        assert proof.chunk_hash == chunk_hashes[0]
        assert len(proof.proof_hashes) > 0

    def test_proof_verification(self):
        """Test Merkle proof verification."""
        chunks = [b"chunk1", b"chunk2", b"chunk3", b"chunk4"]
        chunk_hashes = [MerkleTree.hash_chunk(chunk) for chunk in chunks]

        tree = MerkleTree(chunk_hashes)
        root_hash = tree.build_tree()

        for i in range(len(chunks)):
            proof = tree.generate_proof(i)
            assert proof.verify(root_hash)

    def test_chunk_verification(self):
        """Test chunk data verification."""
        chunks = [b"chunk1", b"chunk2", b"chunk3", b"chunk4"]
        chunk_hashes = [MerkleTree.hash_chunk(chunk) for chunk in chunks]

        tree = MerkleTree(chunk_hashes)
        tree.build_tree()

        for i, chunk in enumerate(chunks):
            proof = tree.generate_proof(i)
            assert tree.verify_chunk(chunk, i, proof)

    def test_invalid_proof_verification(self):
        """Test verification of invalid proofs."""
        chunks = [b"chunk1", b"chunk2", b"chunk3", b"chunk4"]
        chunk_hashes = [MerkleTree.hash_chunk(chunk) for chunk in chunks]

        tree = MerkleTree(chunk_hashes)
        root_hash = tree.build_tree()

        proof = tree.generate_proof(0)

        fake_root = b"\x00" * 32
        assert not proof.verify(fake_root)

        corrupted_chunk = b"corrupted chunk data"
        assert not tree.verify_chunk(corrupted_chunk, 0, proof)

    def test_hash_functions(self):
        """Test hash function consistency."""
        data = b"test data for hashing"

        hash1 = MerkleTree.hash_chunk(data)
        hash2 = MerkleTree.hash_chunk(data)
        assert hash1 == hash2

        hash3 = hashlib.blake2b(data, digest_size=32).digest()
        assert hash1 == hash3

    def test_add_chunk_hash(self):
        """Test adding chunk hashes dynamically."""
        tree = MerkleTree()

        chunk_data = b"new chunk data"
        chunk_hash = MerkleTree.hash_chunk(chunk_data)

        tree.add_chunk_hash(chunk_hash)
        assert len(tree.chunk_hashes) == 1
        assert tree.chunk_hashes[0] == chunk_hash

    def test_invalid_chunk_hash(self):
        """Test adding invalid chunk hash."""
        tree = MerkleTree()

        with pytest.raises(TransferError):
            tree.add_chunk_hash(b"invalid_hash")  # Wrong length

        with pytest.raises(TransferError):
            tree.add_chunk_hash("not_bytes")  # Wrong type


class TestMerkleProof:
    """Test Merkle proof functionality."""

    def test_proof_creation(self):
        """Test proof object creation."""
        chunk_hash = b"\x01" * 32
        proof_hashes = [b"\x02" * 32, b"\x03" * 32]

        proof = MerkleProof(0, chunk_hash, proof_hashes)

        assert proof.chunk_index == 0
        assert proof.chunk_hash == chunk_hash
        assert proof.proof_hashes == proof_hashes

    def test_proof_verification_edge_cases(self):
        """Test proof verification edge cases."""
        chunk_hash = b"\x01" * 32
        proof_hashes = [b"\x02" * 32]
        root_hash = b"\x03" * 32

        proof = MerkleProof(0, chunk_hash, proof_hashes)

        assert not proof.verify(b"invalid_root")  # Wrong length
        assert not proof.verify(None)  # None root

        invalid_proof = MerkleProof(-1, chunk_hash, proof_hashes)
        assert not invalid_proof.verify(root_hash)  # Negative index
