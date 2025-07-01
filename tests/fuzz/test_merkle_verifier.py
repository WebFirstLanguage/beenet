"""Fuzz tests for Merkle proof verifier."""

import pytest
from hypothesis import given, settings
from hypothesis import strategies as st

from beenet.core.errors import TransferError
from beenet.transfer import MerkleProof, MerkleTree


class TestMerkleVerifierFuzz:
    """Fuzz tests for Merkle proof verifier."""

    @given(
        st.binary(min_size=32, max_size=32),
        st.lists(st.binary(min_size=32, max_size=32), min_size=0, max_size=20),
        st.integers(min_value=-100, max_value=1000),
    )
    @settings(max_examples=200, deadline=1000)
    def test_verify_random_proofs(self, chunk_hash, proof_hashes, chunk_index):
        """Test verifying random Merkle proofs."""
        root_hash = b"\x00" * 32

        proof = MerkleProof(chunk_index, chunk_hash, proof_hashes)

        try:
            result = proof.verify(root_hash)
            assert isinstance(result, bool)
        except Exception as e:
            pytest.fail(f"Unexpected exception during proof verification: {e}")

    @given(st.binary(min_size=0, max_size=64))
    @settings(max_examples=100)
    def test_verify_invalid_root_hashes(self, invalid_root):
        """Test verification with invalid root hashes."""
        chunk_hash = b"\x01" * 32
        proof_hashes = [b"\x02" * 32]

        proof = MerkleProof(0, chunk_hash, proof_hashes)

        try:
            result = proof.verify(invalid_root)
            assert isinstance(result, bool)
            if len(invalid_root) != 32:
                assert result is False
        except Exception as e:
            pytest.fail(f"Unexpected exception with invalid root: {e}")

    @given(st.binary(min_size=0, max_size=64))
    @settings(max_examples=100)
    def test_verify_invalid_chunk_hashes(self, invalid_chunk_hash):
        """Test verification with invalid chunk hashes."""
        root_hash = b"\x00" * 32
        proof_hashes = [b"\x02" * 32]

        proof = MerkleProof(0, invalid_chunk_hash, proof_hashes)

        try:
            result = proof.verify(root_hash)
            assert isinstance(result, bool)
            if len(invalid_chunk_hash) != 32:
                assert result is False
        except Exception as e:
            pytest.fail(f"Unexpected exception with invalid chunk hash: {e}")

    @given(st.lists(st.binary(min_size=0, max_size=64), min_size=0, max_size=10))
    @settings(max_examples=100)
    def test_verify_invalid_proof_hashes(self, invalid_proof_hashes):
        """Test verification with invalid proof hashes."""
        chunk_hash = b"\x01" * 32
        root_hash = b"\x00" * 32

        proof = MerkleProof(0, chunk_hash, invalid_proof_hashes)

        try:
            result = proof.verify(root_hash)
            assert isinstance(result, bool)

            has_invalid_hash = any(len(h) != 32 for h in invalid_proof_hashes)
            if has_invalid_hash:
                assert result is False
        except Exception as e:
            pytest.fail(f"Unexpected exception with invalid proof hashes: {e}")

    @given(st.binary(min_size=1, max_size=1024), st.integers(min_value=-10, max_value=100))
    @settings(max_examples=100)
    def test_verify_chunk_with_random_data(self, chunk_data, chunk_index):
        """Test chunk verification with random data."""
        chunk_hash = MerkleTree.hash_chunk(chunk_data)
        proof_hashes = [b"\x02" * 32, b"\x03" * 32]

        tree = MerkleTree([chunk_hash])
        proof = MerkleProof(chunk_index, chunk_hash, proof_hashes)

        try:
            result = tree.verify_chunk(chunk_data, chunk_index, proof)
            assert isinstance(result, bool)
        except Exception as e:
            pytest.fail(f"Unexpected exception during chunk verification: {e}")

    @given(st.lists(st.binary(min_size=32, max_size=32), min_size=0, max_size=50))
    @settings(max_examples=50)
    def test_add_random_chunk_hashes(self, chunk_hashes):
        """Test adding random chunk hashes to tree."""
        tree = MerkleTree()

        for chunk_hash in chunk_hashes:
            try:
                tree.add_chunk_hash(chunk_hash)
            except TransferError:
                pass
            except Exception as e:
                pytest.fail(f"Unexpected exception adding chunk hash: {e}")

        if len(tree.chunk_hashes) > 0:
            try:
                root_hash = tree.build_tree()
                assert isinstance(root_hash, bytes)
                assert len(root_hash) == 32
                _ = root_hash
            except Exception as e:
                pytest.fail(f"Unexpected exception building tree: {e}")

    @given(st.binary(min_size=0, max_size=128))
    @settings(max_examples=100)
    def test_hash_chunk_fuzz(self, data):
        """Test chunk hashing with random data."""
        try:
            hash_result = MerkleTree.hash_chunk(data)
            assert isinstance(hash_result, bytes)
            assert len(hash_result) == 32
        except Exception as e:
            pytest.fail(f"Unexpected exception during chunk hashing: {e}")

    @given(st.binary(min_size=0, max_size=64), st.binary(min_size=0, max_size=64))
    @settings(max_examples=100)
    def test_hash_pair_fuzz(self, left, right):
        """Test pair hashing with random data."""
        try:
            hash_result = MerkleTree.hash_pair(left, right)
            assert isinstance(hash_result, bytes)
            assert len(hash_result) == 32
        except Exception as e:
            pytest.fail(f"Unexpected exception during pair hashing: {e}")

    @given(
        st.lists(st.binary(min_size=1, max_size=256), min_size=1, max_size=20),
        st.integers(min_value=-5, max_value=25),
    )
    @settings(max_examples=50)
    def test_generate_proof_fuzz(self, chunks, chunk_index):
        """Test proof generation with random parameters."""
        chunk_hashes = [MerkleTree.hash_chunk(chunk) for chunk in chunks]
        tree = MerkleTree(chunk_hashes)

        try:
            tree.build_tree()

            if 0 <= chunk_index < len(chunks):
                proof = tree.generate_proof(chunk_index)
                assert isinstance(proof, MerkleProof)
                assert proof.chunk_index == chunk_index
            else:
                with pytest.raises(TransferError):
                    tree.generate_proof(chunk_index)
        except TransferError:
            pass
        except Exception as e:
            pytest.fail(f"Unexpected exception during proof generation: {e}")
