"""Forward error correction using Reed-Solomon codes."""

import hashlib
import struct
from dataclasses import dataclass
from typing import List, Optional, Tuple

try:
    from reedsolo import RSCodec, ReedSolomonError
except ImportError:
    RSCodec = None
    ReedSolomonError = Exception


@dataclass
class ECCConfig:
    """Configuration for error correction coding."""

    enable_ecc: bool = True
    ecc_symbols: int = 10  # Number of Reed-Solomon error correction symbols
    fountain_mode: bool = False  # Enable fountain coding (future feature)
    max_erasures: int = 5  # Maximum number of erasures to handle
    block_size: int = 223  # RS block size (255 - ecc_symbols - safety margin)

    def __post_init__(self) -> None:
        # Validate parameters
        if self.ecc_symbols < 2:
            raise ValueError("ECC symbols must be at least 2")
        if self.ecc_symbols > 128:
            raise ValueError("ECC symbols cannot exceed 128")
        if self.block_size + self.ecc_symbols > 255:
            raise ValueError("Block size + ECC symbols cannot exceed 255")


@dataclass
class ECCBlock:
    """Error correction coded block."""

    block_id: int
    original_data: bytes
    encoded_data: bytes
    checksum: bytes
    ecc_symbols: int

    def __post_init__(self) -> None:
        if not self.checksum:
            self.checksum = hashlib.blake2b(self.original_data, digest_size=16).digest()


class ErrorCorrectionCodec:
    """Reed-Solomon error correction codec for data transfer.

    Provides forward error correction capabilities on top of Merkle trees:
    - Reed-Solomon encoding for automatic error recovery
    - Configurable redundancy levels
    - Block-based processing for streaming data
    - Integration with existing Merkle tree verification
    """

    def __init__(self, config: Optional[ECCConfig] = None):
        self.config = config or ECCConfig()

        if not self.config.enable_ecc:
            self.codec = None
        elif RSCodec is None:
            raise ImportError("reedsolo library required for error correction")
        else:
            # Initialize Reed-Solomon codec
            self.codec = RSCodec(
                self.config.ecc_symbols,
                c_exp=8,  # Galois field GF(2^8)
                prim=0x11D,  # Primitive polynomial
                generator=2,  # Generator polynomial root
                single_gen=True,  # Use single generator for all blocks
            )

    def encode_data(self, data: bytes) -> List[ECCBlock]:
        """Encode data with Reed-Solomon error correction.

        Args:
            data: Raw data to encode

        Returns:
            List of ECC blocks with redundancy
        """
        if not self.config.enable_ecc or not self.codec:
            # Return single block without ECC
            return [
                ECCBlock(
                    block_id=0, original_data=data, encoded_data=data, checksum=b"", ecc_symbols=0
                )
            ]

        blocks = []
        block_id = 0

        # Process data in blocks
        for i in range(0, len(data), self.config.block_size):
            block_data = data[i : i + self.config.block_size]

            try:
                # Encode block with Reed-Solomon
                encoded_block = self.codec.encode(block_data)

                ecc_block = ECCBlock(
                    block_id=block_id,
                    original_data=block_data,
                    encoded_data=encoded_block,
                    checksum=b"",  # Will be computed in __post_init__
                    ecc_symbols=self.config.ecc_symbols,
                )

                blocks.append(ecc_block)
                block_id += 1

            except Exception as e:
                raise RuntimeError(f"Reed-Solomon encoding failed for block {block_id}: {e}")

        return blocks

    def decode_blocks(self, blocks: List[ECCBlock], expected_length: Optional[int] = None) -> bytes:
        """Decode ECC blocks back to original data.

        Args:
            blocks: List of ECC blocks (may contain errors/erasures)
            expected_length: Expected length of decoded data for validation

        Returns:
            Recovered original data

        Raises:
            RuntimeError: If decoding fails or too many errors
        """
        if not self.config.enable_ecc or not self.codec:
            # Simple concatenation without ECC
            result = b"".join(
                block.encoded_data for block in sorted(blocks, key=lambda b: b.block_id)
            )
            if expected_length and len(result) != expected_length:
                raise RuntimeError(
                    f"Data length mismatch: got {len(result)}, expected {expected_length}"
                )
            return result

        # Sort blocks by ID
        sorted_blocks = sorted(blocks, key=lambda b: b.block_id)
        decoded_data = []

        for block in sorted_blocks:
            try:
                # Attempt to decode with error correction
                if block.ecc_symbols > 0:
                    decoded_block = self.codec.decode(block.encoded_data)[0]
                else:
                    decoded_block = block.encoded_data

                # Verify checksum if available
                if block.checksum:
                    computed_checksum = hashlib.blake2b(decoded_block, digest_size=16).digest()
                    if computed_checksum != block.checksum:
                        raise RuntimeError(f"Checksum mismatch in block {block.block_id}")

                decoded_data.append(decoded_block)

            except ReedSolomonError as e:
                raise RuntimeError(f"Reed-Solomon decoding failed for block {block.block_id}: {e}")
            except Exception as e:
                raise RuntimeError(f"Block decoding failed for block {block.block_id}: {e}")

        result = b"".join(decoded_data)

        # Validate total length if provided
        if expected_length and len(result) != expected_length:
            raise RuntimeError(
                f"Data length mismatch: got {len(result)}, expected {expected_length}"
            )

        return result

    def simulate_errors(self, blocks: List[ECCBlock], error_rate: float = 0.1) -> List[ECCBlock]:
        """Simulate transmission errors for testing purposes.

        Args:
            blocks: Original ECC blocks
            error_rate: Probability of error per byte (0.0 to 1.0)

        Returns:
            Blocks with simulated errors
        """
        import random

        corrupted_blocks = []

        for block in blocks:
            if not self.config.enable_ecc or block.ecc_symbols == 0:
                # No error correction available
                corrupted_blocks.append(block)
                continue

            encoded_data = bytearray(block.encoded_data)
            errors_introduced = 0
            max_correctable = self.config.ecc_symbols // 2

            # Introduce random errors
            for i in range(len(encoded_data)):
                if random.random() < error_rate and errors_introduced < max_correctable:
                    # Corrupt this byte
                    encoded_data[i] = random.randint(0, 255)
                    errors_introduced += 1

            corrupted_block = ECCBlock(
                block_id=block.block_id,
                original_data=block.original_data,
                encoded_data=bytes(encoded_data),
                checksum=block.checksum,
                ecc_symbols=block.ecc_symbols,
            )

            corrupted_blocks.append(corrupted_block)

        return corrupted_blocks

    def get_redundancy_info(self) -> dict:
        """Get information about error correction redundancy.

        Returns:
            Dictionary with redundancy statistics
        """
        if not self.config.enable_ecc:
            return {
                "enabled": False,
                "redundancy_ratio": 0.0,
                "max_correctable_errors": 0,
                "max_correctable_erasures": 0,
            }

        data_symbols = self.config.block_size
        total_symbols = data_symbols + self.config.ecc_symbols
        redundancy_ratio = self.config.ecc_symbols / data_symbols

        return {
            "enabled": True,
            "ecc_symbols": self.config.ecc_symbols,
            "block_size": self.config.block_size,
            "total_block_size": total_symbols,
            "redundancy_ratio": redundancy_ratio,
            "max_correctable_errors": self.config.ecc_symbols // 2,
            "max_correctable_erasures": self.config.ecc_symbols,
            "overhead_percentage": (redundancy_ratio * 100),
        }

    def estimate_recovery_probability(self, corruption_rate: float) -> float:
        """Estimate probability of successful data recovery.

        Args:
            corruption_rate: Expected corruption rate (0.0 to 1.0)

        Returns:
            Estimated recovery probability (0.0 to 1.0)
        """
        if not self.config.enable_ecc:
            return 1.0 - corruption_rate

        # Simplified model: binomial probability
        # Can correct up to ecc_symbols//2 errors per block
        max_errors = self.config.ecc_symbols // 2
        block_size = self.config.block_size + self.config.ecc_symbols

        # Probability of having <= max_errors in a block
        recovery_prob = 0.0
        for k in range(max_errors + 1):
            # Binomial coefficient
            from math import comb

            prob = (
                comb(block_size, k)
                * (corruption_rate**k)
                * ((1 - corruption_rate) ** (block_size - k))
            )
            recovery_prob += prob

        return recovery_prob


def create_redundant_chunks(data: bytes, redundancy_factor: float = 0.2) -> List[Tuple[int, bytes]]:
    """Create redundant chunks using fountain coding principles.

    This is a simplified fountain code implementation for future enhancement.

    Args:
        data: Original data to encode
        redundancy_factor: Amount of redundancy (0.0 to 1.0)

    Returns:
        List of (chunk_id, chunk_data) tuples with redundancy
    """
    chunk_size = 1024  # Fixed chunk size for simplicity
    chunks = []

    # Create original chunks
    for i in range(0, len(data), chunk_size):
        chunk_id = i // chunk_size
        chunk_data = data[i : i + chunk_size]
        chunks.append((chunk_id, chunk_data))

    # Create redundant chunks (simple XOR combination)
    num_redundant = int(len(chunks) * redundancy_factor)

    for i in range(num_redundant):
        redundant_id = len(chunks) + i

        # XOR multiple original chunks to create redundant chunk
        redundant_data = bytearray(chunk_size)

        # Use deterministic selection of chunks to XOR
        import random

        random.seed(redundant_id)  # Deterministic seed
        selected_chunks = random.sample(range(len(chunks)), min(3, len(chunks)))

        for chunk_idx in selected_chunks:
            chunk_data = chunks[chunk_idx][1]
            for j in range(min(len(chunk_data), chunk_size)):
                redundant_data[j] ^= chunk_data[j]

        chunks.append((redundant_id, bytes(redundant_data)))

    return chunks


def recover_from_redundant_chunks(
    chunks: List[Tuple[int, bytes]], original_length: int
) -> Optional[bytes]:
    """Attempt to recover data from redundant chunks.

    This is a simplified recovery algorithm for fountain coding.

    Args:
        chunks: Available chunks (may be incomplete)
        original_length: Expected length of original data

    Returns:
        Recovered data if successful, None otherwise
    """
    chunk_size = 1024
    num_original_chunks = (original_length + chunk_size - 1) // chunk_size

    # Separate original and redundant chunks
    original_chunks = {}
    redundant_chunks = []

    for chunk_id, chunk_data in chunks:
        if chunk_id < num_original_chunks:
            original_chunks[chunk_id] = chunk_data
        else:
            redundant_chunks.append((chunk_id, chunk_data))

    # Check if we have all original chunks
    if len(original_chunks) == num_original_chunks:
        # Simple case: reconstruct from original chunks
        result = bytearray()
        for i in range(num_original_chunks):
            if i in original_chunks:
                result.extend(original_chunks[i])
            else:
                return None  # Missing chunk

        return bytes(result[:original_length])

    # TODO: Implement sophisticated fountain code recovery
    # For now, just return None if we don't have all original chunks
    return None
