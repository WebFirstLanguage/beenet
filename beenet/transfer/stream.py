"""Async resumable transfer streams with state persistence."""

import json
from pathlib import Path
from typing import Callable, Optional

from ..core.errors import TransferError
from .chunker import DataChunker
from .merkle import MerkleProof, MerkleTree


class TransferState:
    """Persistent state for resumable transfers."""

    def __init__(self, transfer_id: str, total_chunks: int):
        self.transfer_id = transfer_id
        self.total_chunks = total_chunks
        self.completed_chunks: set[int] = set()
        self.merkle_root: Optional[bytes] = None
        self.chunk_size: int = DataChunker.DEFAULT_CHUNK_SIZE

    @property
    def progress(self) -> float:
        """Get transfer progress as percentage."""
        if self.total_chunks == 0:
            return 100.0
        return (len(self.completed_chunks) / self.total_chunks) * 100.0

    @property
    def is_complete(self) -> bool:
        """Check if transfer is complete."""
        return len(self.completed_chunks) == self.total_chunks


class TransferStream:
    """Async resumable transfer stream with Merkle verification.

    Provides:
    - Resumable file transfers with state persistence
    - Merkle tree integrity verification
    - Progress tracking and callbacks
    - Async streaming interface
    """

    def __init__(self, transfer_id: str, chunker: Optional[DataChunker] = None):
        self.transfer_id = transfer_id
        self.chunker = chunker or DataChunker()
        self.state: Optional[TransferState] = None
        self.merkle_tree: Optional[MerkleTree] = None
        self._progress_callback: Optional[Callable[[float], None]] = None

    async def start_send(self, file_path: Path, peer_address: str) -> None:
        """Start sending a file to a peer.

        Args:
            file_path: Path to file to send
            peer_address: Address of receiving peer
        """
        if not file_path.exists():
            raise TransferError(f"File not found: {file_path}")

        file_size = file_path.stat().st_size
        total_chunks = DataChunker.calculate_chunk_count(file_size, self.chunker.chunk_size)

        self.state = TransferState(self.transfer_id, total_chunks)
        self.state.chunk_size = self.chunker.chunk_size

        chunk_hashes = []
        for chunk_index, chunk_data in self.chunker.chunk_file(str(file_path)):
            chunk_hash = MerkleTree.hash_chunk(chunk_data)
            chunk_hashes.append(chunk_hash)

        self.merkle_tree = MerkleTree(chunk_hashes)
        self.state.merkle_root = self.merkle_tree.build_tree()

        self._update_progress()

    async def start_receive(self, file_path: Path, expected_root: bytes, total_chunks: int) -> None:
        """Start receiving a file from a peer.

        Args:
            file_path: Path where received file will be saved
            expected_root: Expected Merkle root hash
            total_chunks: Total number of chunks to receive
        """
        if not isinstance(expected_root, bytes) or len(expected_root) != 32:
            raise TransferError("Invalid Merkle root hash")

        if total_chunks <= 0:
            raise TransferError("Invalid chunk count")

        self.state = TransferState(self.transfer_id, total_chunks)
        self.state.merkle_root = expected_root
        self.state.chunk_size = self.chunker.chunk_size

        file_path.parent.mkdir(parents=True, exist_ok=True)

        if not file_path.exists():
            with open(file_path, "wb") as f:
                f.write(b"\x00" * (total_chunks * self.chunker.chunk_size))

        self._update_progress()

    async def send_chunk(self, chunk_index: int, chunk_data: bytes, proof: MerkleProof) -> None:
        """Send a chunk with its Merkle proof.

        Args:
            chunk_index: Index of the chunk
            chunk_data: Chunk data
            proof: Merkle proof for the chunk
        """
        if not self.state:
            raise TransferError("Transfer not started")

        if chunk_index < 0 or chunk_index >= self.state.total_chunks:
            raise TransferError(f"Invalid chunk index: {chunk_index}")

        if not isinstance(chunk_data, bytes):
            raise TransferError("Chunk data must be bytes")

        if not self.merkle_tree:
            raise TransferError("Merkle tree not initialized")

        if not self.merkle_tree.verify_chunk(chunk_data, chunk_index, proof):
            raise TransferError(f"Invalid chunk or proof for index {chunk_index}")

        self.state.completed_chunks.add(chunk_index)
        self._update_progress()

    async def receive_chunk(self, chunk_index: int, chunk_data: bytes, proof: MerkleProof) -> bool:
        """Receive and verify a chunk.

        Args:
            chunk_index: Index of the chunk
            chunk_data: Chunk data
            proof: Merkle proof for the chunk

        Returns:
            True if chunk was valid and accepted
        """
        if not self.state:
            return False

        if chunk_index < 0 or chunk_index >= self.state.total_chunks:
            return False

        if chunk_index in self.state.completed_chunks:
            return True

        if not isinstance(chunk_data, bytes):
            return False  # type: ignore[unreachable]

        if not self.state.merkle_root:
            return False

        if not proof.verify(self.state.merkle_root):
            return False

        computed_hash = MerkleTree.hash_chunk(chunk_data)
        if computed_hash != proof.chunk_hash:
            return False

        self.state.completed_chunks.add(chunk_index)
        self._update_progress()
        return True

    async def resume_transfer(self, state_file: Path) -> None:
        """Resume a transfer from saved state.

        Args:
            state_file: Path to saved transfer state
        """
        if not state_file.exists():
            raise TransferError(f"State file not found: {state_file}")

        try:
            with open(state_file, "r") as f:
                state_data = json.load(f)

            if state_data.get("transfer_id") != self.transfer_id:
                raise TransferError("Transfer ID mismatch")

            total_chunks = state_data.get("total_chunks", 0)
            if total_chunks <= 0:
                raise TransferError("Invalid total chunks in state file")

            self.state = TransferState(self.transfer_id, total_chunks)
            self.state.completed_chunks = set(state_data.get("completed_chunks", []))

            merkle_root_hex = state_data.get("merkle_root")
            if merkle_root_hex:
                self.state.merkle_root = bytes.fromhex(merkle_root_hex)

            self.state.chunk_size = state_data.get("chunk_size", DataChunker.DEFAULT_CHUNK_SIZE)
            self.chunker.chunk_size = self.state.chunk_size

            self._update_progress()

        except (json.JSONDecodeError, KeyError, ValueError) as e:
            raise TransferError(f"Invalid state file format: {e}")

    async def save_state(self, state_file: Path) -> None:
        """Save transfer state for resumption.

        Args:
            state_file: Path to save state to
        """
        if not self.state:
            raise TransferError("No transfer state to save")

        try:
            state_data = {
                "transfer_id": self.transfer_id,
                "total_chunks": self.state.total_chunks,
                "completed_chunks": list(self.state.completed_chunks),
                "chunk_size": self.state.chunk_size,
                "progress": self.state.progress,
            }

            if self.state.merkle_root:
                state_data["merkle_root"] = self.state.merkle_root.hex()

            state_file.parent.mkdir(parents=True, exist_ok=True)

            with open(state_file, "w") as f:
                json.dump(state_data, f, indent=2)

        except (OSError, json.JSONDecodeError) as e:
            raise TransferError(f"Failed to save state: {e}")

    def set_progress_callback(self, callback: Callable[[float], None]) -> None:
        """Set callback for progress updates.

        Args:
            callback: Function to call with progress percentage
        """
        self._progress_callback = callback

    def _update_progress(self) -> None:
        """Update progress and call callback if set."""
        if self.state and self._progress_callback:
            self._progress_callback(self.state.progress)

    async def get_missing_chunks(self) -> list[int]:
        """Get list of missing chunk indices.

        Returns:
            List of chunk indices that still need to be received
        """
        if not self.state:
            return []

        all_chunks = set(range(self.state.total_chunks))
        missing = all_chunks - self.state.completed_chunks
        return sorted(missing)

    async def verify_complete_file(self, file_path: Path) -> bool:
        """Verify the complete file against Merkle root.

        Args:
            file_path: Path to file to verify

        Returns:
            True if file is valid
        """
        if not self.state or not self.state.merkle_root:
            return False

        if not file_path.exists():
            return False

        if not self.state.is_complete:
            return False

        try:
            chunk_hashes = []
            for chunk_index, chunk_data in self.chunker.chunk_file(str(file_path)):
                chunk_hash = MerkleTree.hash_chunk(chunk_data)
                chunk_hashes.append(chunk_hash)

            if len(chunk_hashes) != self.state.total_chunks:
                return False

            merkle_tree = MerkleTree(chunk_hashes)
            computed_root = merkle_tree.build_tree()

            return computed_root == self.state.merkle_root

        except Exception:
            return False
