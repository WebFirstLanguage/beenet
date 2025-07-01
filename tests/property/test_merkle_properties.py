"""Property-based tests for Merkle tree using Hypothesis."""

import pytest
from hypothesis import assume, given
from hypothesis import strategies as st

from beenet.transfer import MerkleTree


class TestMerkleTreeProperties:
    """Property-based tests for Merkle tree."""

    @given(st.lists(st.binary(min_size=1, max_size=1024), min_size=1, max_size=100))
    def test_tree_deterministic(self, chunks):
        """Test that tree construction is deterministic."""
        chunk_hashes = [MerkleTree.hash_chunk(chunk) for chunk in chunks]

        tree1 = MerkleTree(chunk_hashes.copy())
        tree2 = MerkleTree(chunk_hashes.copy())

        root1 = tree1.build_tree()
        root2 = tree2.build_tree()

        assert root1 == root2

    @given(st.lists(st.binary(min_size=1, max_size=1024), min_size=1, max_size=50))
    def test_all_proofs_verify(self, chunks):
        """Test that all generated proofs verify correctly."""
        chunk_hashes = [MerkleTree.hash_chunk(chunk) for chunk in chunks]

        tree = MerkleTree(chunk_hashes)
        root_hash = tree.build_tree()

        for i in range(len(chunks)):
            proof = tree.generate_proof(i)
            assert proof.verify(root_hash)
            assert tree.verify_chunk(chunks[i], i, proof)

    @given(st.binary(min_size=1, max_size=1024))
    def test_hash_consistency(self, data):
        """Test that hash function is consistent."""
        hash1 = MerkleTree.hash_chunk(data)
        hash2 = MerkleTree.hash_chunk(data)

        assert hash1 == hash2
        assert len(hash1) == 32  # BLAKE2b digest size

    @given(st.binary(min_size=32, max_size=32), st.binary(min_size=32, max_size=32))
    def test_hash_pair_consistency(self, left, right):
        """Test that hash pair function is consistent."""
        hash1 = MerkleTree.hash_pair(left, right)
        hash2 = MerkleTree.hash_pair(left, right)

        assert hash1 == hash2
        assert len(hash1) == 32

        hash3 = MerkleTree.hash_pair(right, left)
        if left != right:
            assert hash1 != hash3  # Order matters

    @given(st.lists(st.binary(min_size=1, max_size=512), min_size=2, max_size=20))
    def test_tree_modification_changes_root(self, chunks):
        """Test that modifying tree changes root hash."""
        assume(len(set(chunks)) > 1)  # Ensure chunks are different

        chunk_hashes = [MerkleTree.hash_chunk(chunk) for chunk in chunks]

        tree1 = MerkleTree(chunk_hashes.copy())
        root1 = tree1.build_tree()

        modified_hashes = chunk_hashes.copy()
        modified_hashes[0] = MerkleTree.hash_chunk(chunks[-1])  # Change first chunk

        tree2 = MerkleTree(modified_hashes)
        root2 = tree2.build_tree()

        if chunk_hashes[0] != modified_hashes[0]:
            assert root1 != root2

    @given(
        st.lists(st.binary(min_size=1, max_size=256), min_size=1, max_size=30),
        st.integers(min_value=0),
    )
    def test_proof_index_bounds(self, chunks, index):
        """Test proof generation with various indices."""
        chunk_hashes = [MerkleTree.hash_chunk(chunk) for chunk in chunks]
        tree = MerkleTree(chunk_hashes)
        tree.build_tree()

        if 0 <= index < len(chunks):
            proof = tree.generate_proof(index)
            assert proof.chunk_index == index
            assert proof.chunk_hash == chunk_hashes[index]
        else:
            with pytest.raises(Exception):
                tree.generate_proof(index)

    @given(st.binary(min_size=1, max_size=1024))
    def test_single_chunk_tree_properties(self, chunk):
        """Test properties of single-chunk trees."""
        chunk_hash = MerkleTree.hash_chunk(chunk)
        tree = MerkleTree([chunk_hash])
        root_hash = tree.build_tree()

        assert root_hash == chunk_hash

        proof = tree.generate_proof(0)
        assert proof.verify(root_hash)
        assert tree.verify_chunk(chunk, 0, proof)

    @given(st.lists(st.binary(min_size=1, max_size=256), min_size=1, max_size=20))
    def test_proof_corruption_detection(self, chunks):
        """Test that corrupted proofs are detected."""
        chunk_hashes = [MerkleTree.hash_chunk(chunk) for chunk in chunks]
        tree = MerkleTree(chunk_hashes)
        root_hash = tree.build_tree()

        for i in range(len(chunks)):
            proof = tree.generate_proof(i)

            corrupted_chunk = chunks[i] + b"corruption"
            assert not tree.verify_chunk(corrupted_chunk, i, proof)

            fake_root = b"\x00" * 32
            assert not proof.verify(fake_root)
