"""Enhanced Merkle tree with Reed-Solomon error correction integration."""

import hashlib
from typing import Dict, List, Optional, Tuple

from ..core.errors import TransferError
from .error_correction import ECCBlock, ECCConfig, ErrorCorrectionCodec
from .merkle import MerkleProof, MerkleTree


class EnhancedMerkleProof:
    """Enhanced Merkle proof with error correction capabilities."""

    def __init__(
        self,
        chunk_index: int,
        chunk_hash: bytes,
        proof_hashes: List[bytes],
        ecc_block: Optional[ECCBlock] = None,
    ):
        self.chunk_index = chunk_index
        self.chunk_hash = chunk_hash
        self.proof_hashes = proof_hashes
        self.ecc_block = ecc_block

    def verify(self, root_hash: bytes) -> bool:
        """Verify this proof against a root hash with error correction.

        Args:
            root_hash: Expected Merkle root hash

        Returns:
            True if proof is valid
        """
        # First try standard Merkle verification
        standard_proof = MerkleProof(self.chunk_index, self.chunk_hash, self.proof_hashes)
        if standard_proof.verify(root_hash):
            return True

        # If standard verification fails and we have ECC data, attempt recovery
        if self.ecc_block and self.ecc_block.ecc_symbols > 0:
            try:
                codec = ErrorCorrectionCodec()
                recovered_data = codec.decode_blocks([self.ecc_block])
                recovered_hash = MerkleTree.hash_chunk(recovered_data)

                # Try verification with recovered hash
                recovered_proof = MerkleProof(self.chunk_index, recovered_hash, self.proof_hashes)
                return recovered_proof.verify(root_hash)
            except Exception:
                pass

        return False

    def attempt_recovery(self) -> Optional[bytes]:
        """Attempt to recover chunk data using error correction.

        Returns:
            Recovered chunk data if successful, None otherwise
        """
        if not self.ecc_block or self.ecc_block.ecc_symbols == 0:
            return None

        try:
            codec = ErrorCorrectionCodec()
            return codec.decode_blocks([self.ecc_block])
        except Exception:
            return None


class EnhancedMerkleTree:
    """Enhanced Merkle tree with integrated Reed-Solomon error correction.

    Provides:
    - All standard Merkle tree functionality
    - Optional Reed-Solomon error correction for chunks
    - Automatic error recovery during verification
    - Configurable redundancy levels
    - Seamless fallback to standard verification
    """

    def __init__(
        self, chunk_hashes: Optional[List[bytes]] = None, ecc_config: Optional[ECCConfig] = None
    ):
        self.base_tree = MerkleTree(chunk_hashes)
        self.ecc_config = ecc_config or ECCConfig()
        self.ecc_codec = ErrorCorrectionCodec(self.ecc_config)

        # Storage for ECC blocks
        self._ecc_blocks: Dict[int, ECCBlock] = {}
        self._chunk_data_cache: Dict[int, bytes] = {}

    def add_chunk_with_ecc(self, chunk_index: int, chunk_data: bytes) -> None:
        """Add a chunk with error correction encoding.

        Args:
            chunk_index: Index of the chunk
            chunk_data: Raw chunk data
        """
        # Cache the original data
        self._chunk_data_cache[chunk_index] = chunk_data

        # Generate ECC block if enabled
        if self.ecc_config.enable_ecc:
            ecc_blocks = self.ecc_codec.encode_data(chunk_data)
            if ecc_blocks:
                # Store the first (and typically only) ECC block
                ecc_block = ecc_blocks[0]
                ecc_block.block_id = chunk_index  # Override with chunk index
                self._ecc_blocks[chunk_index] = ecc_block

        # Add hash to base Merkle tree
        chunk_hash = MerkleTree.hash_chunk(chunk_data)

        # Ensure we have enough space in the base tree
        while len(self.base_tree.chunk_hashes) <= chunk_index:
            # Fill with placeholder hashes
            self.base_tree.chunk_hashes.append(b"\\x00" * 32)

        # Update the specific index
        if chunk_index < len(self.base_tree.chunk_hashes):
            self.base_tree.chunk_hashes[chunk_index] = chunk_hash
        else:
            self.base_tree.add_chunk_hash(chunk_hash)

    def add_chunk_hash(self, chunk_hash: bytes) -> None:
        """Add a chunk hash to the tree (standard Merkle functionality).

        Args:
            chunk_hash: BLAKE2b hash of a data chunk
        """
        self.base_tree.add_chunk_hash(chunk_hash)

    def build_tree(self) -> bytes:
        """Build the Merkle tree and return root hash.

        Returns:
            Root hash of the Merkle tree
        """
        return self.base_tree.build_tree()

    def generate_enhanced_proof(self, chunk_index: int) -> EnhancedMerkleProof:
        """Generate an enhanced Merkle proof with ECC data.

        Args:
            chunk_index: Index of the chunk to prove

        Returns:
            Enhanced Merkle proof for the chunk
        """
        # Generate standard Merkle proof
        standard_proof = self.base_tree.generate_proof(chunk_index)

        # Get ECC block if available
        ecc_block = self._ecc_blocks.get(chunk_index)

        return EnhancedMerkleProof(
            chunk_index=standard_proof.chunk_index,
            chunk_hash=standard_proof.chunk_hash,
            proof_hashes=standard_proof.proof_hashes,
            ecc_block=ecc_block,
        )

    def generate_proof(self, chunk_index: int) -> MerkleProof:
        """Generate a standard Merkle proof (for compatibility).

        Args:
            chunk_index: Index of the chunk to prove

        Returns:
            Standard Merkle proof for the chunk
        """
        return self.base_tree.generate_proof(chunk_index)

    def verify_chunk_with_recovery(
        self, chunk_data: bytes, chunk_index: int, proof: EnhancedMerkleProof
    ) -> Tuple[bool, Optional[bytes]]:
        """Verify a chunk with automatic error recovery.

        Args:
            chunk_data: Raw chunk data (may be corrupted)
            chunk_index: Index of the chunk
            proof: Enhanced Merkle proof for the chunk

        Returns:
            Tuple of (verification_success, recovered_data)
            recovered_data is None if no recovery was needed or if recovery failed
        """
        # First try standard verification
        if self.base_tree.verify_chunk(
            chunk_data,
            chunk_index,
            MerkleProof(proof.chunk_index, proof.chunk_hash, proof.proof_hashes),
        ):
            return True, None

        # If standard verification fails, attempt recovery
        if proof.ecc_block:
            try:
                # Try to recover the data
                recovered_data = proof.attempt_recovery()
                if recovered_data:
                    # Verify the recovered data
                    if self.base_tree.verify_chunk(
                        recovered_data,
                        chunk_index,
                        MerkleProof(proof.chunk_index, proof.chunk_hash, proof.proof_hashes),
                    ):
                        return True, recovered_data
                    else:
                        # Try with recomputed hash
                        recovered_hash = MerkleTree.hash_chunk(recovered_data)
                        recovered_proof = MerkleProof(
                            proof.chunk_index, recovered_hash, proof.proof_hashes
                        )
                        if recovered_proof.verify(self.root_hash):
                            return True, recovered_data
            except Exception:
                pass

        return False, None

    def verify_chunk(self, chunk_data: bytes, chunk_index: int, proof: MerkleProof) -> bool:
        """Verify a chunk (standard Merkle functionality).

        Args:
            chunk_data: Raw chunk data
            chunk_index: Index of the chunk
            proof: Merkle proof for the chunk

        Returns:
            True if chunk is valid
        """
        return self.base_tree.verify_chunk(chunk_data, chunk_index, proof)

    def get_ecc_statistics(self) -> Dict[str, any]:
        """Get error correction statistics.

        Returns:
            Dictionary with ECC statistics
        """
        total_chunks = len(self.base_tree.chunk_hashes)
        ecc_chunks = len(self._ecc_blocks)

        ecc_info = self.ecc_codec.get_redundancy_info()

        return {
            "total_chunks": total_chunks,
            "ecc_protected_chunks": ecc_chunks,
            "ecc_coverage": ecc_chunks / total_chunks if total_chunks > 0 else 0.0,
            **ecc_info,
        }

    def simulate_corruption_and_recovery(self, error_rate: float = 0.1) -> Dict[str, int]:
        """Simulate data corruption and test recovery capabilities.

        Args:
            error_rate: Simulation error rate (0.0 to 1.0)

        Returns:
            Dictionary with recovery statistics
        """
        results = {
            "total_blocks": len(self._ecc_blocks),
            "corrupted_blocks": 0,
            "recovered_blocks": 0,
            "unrecoverable_blocks": 0,
        }

        for chunk_index, ecc_block in self._ecc_blocks.items():
            # Simulate corruption
            corrupted_blocks = self.ecc_codec.simulate_errors([ecc_block], error_rate)
            if corrupted_blocks:
                corrupted_block = corrupted_blocks[0]

                # Check if block was actually corrupted
                if corrupted_block.encoded_data != ecc_block.encoded_data:
                    results["corrupted_blocks"] += 1

                    # Attempt recovery
                    try:
                        recovered_data = self.ecc_codec.decode_blocks([corrupted_block])
                        original_data = self._chunk_data_cache.get(chunk_index)

                        if original_data and recovered_data == original_data:
                            results["recovered_blocks"] += 1
                        else:
                            results["unrecoverable_blocks"] += 1
                    except Exception:
                        results["unrecoverable_blocks"] += 1

        return results

    @property
    def root_hash(self) -> Optional[bytes]:
        """Get the root hash of the tree."""
        return self.base_tree.root_hash

    @property
    def chunk_hashes(self) -> List[bytes]:
        """Get the list of chunk hashes."""
        return self.base_tree.chunk_hashes

    def get_chunk_data(self, chunk_index: int) -> Optional[bytes]:
        """Get cached chunk data.

        Args:
            chunk_index: Index of the chunk

        Returns:
            Chunk data if cached, None otherwise
        """
        return self._chunk_data_cache.get(chunk_index)

    def clear_cache(self) -> None:
        """Clear cached chunk data to free memory."""
        self._chunk_data_cache.clear()

    def export_ecc_blocks(self) -> Dict[int, ECCBlock]:
        """Export ECC blocks for storage or transmission.

        Returns:
            Dictionary mapping chunk indices to ECC blocks
        """
        return self._ecc_blocks.copy()

    def import_ecc_blocks(self, ecc_blocks: Dict[int, ECCBlock]) -> None:
        """Import ECC blocks from storage or transmission.

        Args:
            ecc_blocks: Dictionary mapping chunk indices to ECC blocks
        """
        self._ecc_blocks.update(ecc_blocks)
