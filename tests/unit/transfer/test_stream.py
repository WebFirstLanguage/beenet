"""Unit tests for TransferStream functionality."""

from pathlib import Path

import pytest

from beenet.core.errors import TransferError
from beenet.transfer import TransferStream


class TestTransferStream:
    """Test TransferStream functionality."""

    def test_transfer_stream_creation(self):
        """Test transfer stream creation."""
        transfer_id = "test_transfer_001"
        stream = TransferStream(transfer_id)

        assert stream.transfer_id == transfer_id
        assert stream.state is None
        assert stream.merkle_tree is None

    @pytest.mark.asyncio
    async def test_start_send(self, test_file):
        """Test starting file send."""
        transfer_id = "send_test_001"
        stream = TransferStream(transfer_id)

        await stream.start_send(test_file, "peer_address")

        assert stream.state is not None
        assert stream.state.total_chunks > 0
        assert stream.merkle_tree is not None
        assert stream.state.merkle_root is not None

    @pytest.mark.asyncio
    async def test_start_send_nonexistent_file(self):
        """Test starting send with non-existent file."""
        transfer_id = "send_error_001"
        stream = TransferStream(transfer_id)

        nonexistent_file = Path("/tmp/nonexistent_file.txt")

        with pytest.raises(TransferError):
            await stream.start_send(nonexistent_file, "peer_address")

    @pytest.mark.asyncio
    async def test_start_receive(self, temp_dir):
        """Test starting file receive."""
        transfer_id = "receive_test_001"
        stream = TransferStream(transfer_id)

        expected_root = b"\x01" * 32
        total_chunks = 10
        receive_path = temp_dir / "received_file.txt"

        await stream.start_receive(receive_path, expected_root, total_chunks)

        assert stream.state is not None
        assert stream.state.total_chunks == total_chunks
        assert stream.state.merkle_root == expected_root
        assert receive_path.exists()

    @pytest.mark.asyncio
    async def test_start_receive_invalid_root(self, temp_dir):
        """Test starting receive with invalid root hash."""
        transfer_id = "receive_error_001"
        stream = TransferStream(transfer_id)

        invalid_root = b"invalid"
        receive_path = temp_dir / "received_file.txt"

        with pytest.raises(TransferError):
            await stream.start_receive(receive_path, invalid_root, 10)

    @pytest.mark.asyncio
    async def test_start_receive_invalid_chunk_count(self, temp_dir):
        """Test starting receive with invalid chunk count."""
        transfer_id = "receive_error_002"
        stream = TransferStream(transfer_id)

        expected_root = b"\x01" * 32
        receive_path = temp_dir / "received_file.txt"

        with pytest.raises(TransferError):
            await stream.start_receive(receive_path, expected_root, 0)

    @pytest.mark.asyncio
    async def test_save_and_resume_state(self, temp_dir, test_file):
        """Test saving and resuming transfer state."""
        transfer_id = "state_test_001"
        stream = TransferStream(transfer_id)

        await stream.start_send(test_file, "peer_address")

        state_file = temp_dir / "transfer_state.json"
        await stream.save_state(state_file)

        assert state_file.exists()

        new_stream = TransferStream(transfer_id)
        await new_stream.resume_transfer(state_file)

        assert new_stream.state is not None
        assert new_stream.state.transfer_id == transfer_id
        assert new_stream.state.merkle_root == stream.state.merkle_root

    @pytest.mark.asyncio
    async def test_resume_nonexistent_state(self, temp_dir):
        """Test resuming from non-existent state file."""
        transfer_id = "resume_error_001"
        stream = TransferStream(transfer_id)

        nonexistent_state = temp_dir / "nonexistent_state.json"

        with pytest.raises(TransferError):
            await stream.resume_transfer(nonexistent_state)

    @pytest.mark.asyncio
    async def test_verify_complete_file(self, test_file):
        """Test verifying complete file."""
        transfer_id = "verify_test_001"
        stream = TransferStream(transfer_id)

        await stream.start_send(test_file, "peer_address")

        stream.state.completed_chunks = set(range(stream.state.total_chunks))

        is_valid = await stream.verify_complete_file(test_file)
        assert is_valid

    @pytest.mark.asyncio
    async def test_verify_incomplete_file(self, test_file):
        """Test verifying incomplete file."""
        transfer_id = "verify_test_002"
        stream = TransferStream(transfer_id)

        await stream.start_send(test_file, "peer_address")

        is_valid = await stream.verify_complete_file(test_file)
        assert not is_valid

    @pytest.mark.asyncio
    async def test_verify_nonexistent_file(self, temp_dir):
        """Test verifying non-existent file."""
        transfer_id = "verify_test_003"
        stream = TransferStream(transfer_id)

        nonexistent_file = temp_dir / "nonexistent.txt"

        is_valid = await stream.verify_complete_file(nonexistent_file)
        assert not is_valid

    def test_set_progress_callback(self):
        """Test setting progress callback."""
        transfer_id = "callback_test_001"
        stream = TransferStream(transfer_id)

        callback_called = False

        def progress_callback(progress):
            nonlocal callback_called
            callback_called = True
            assert 0.0 <= progress <= 100.0  # Progress is percentage, not decimal

        from beenet.transfer.stream import TransferState

        stream.state = TransferState(transfer_id, 10)
        stream.state.completed_chunks = {0, 1, 2}  # 30% progress

        stream.set_progress_callback(progress_callback)
        stream._update_progress()

        assert callback_called

    @pytest.mark.asyncio
    async def test_get_missing_chunks(self, test_file):
        """Test getting missing chunks."""
        transfer_id = "missing_test_001"
        stream = TransferStream(transfer_id)

        from beenet.transfer.stream import TransferState

        stream.state = TransferState(transfer_id, 10)
        stream.state.completed_chunks = {0, 2, 4, 6, 8}

        missing = await stream.get_missing_chunks()
        expected_missing = [1, 3, 5, 7, 9]

        assert missing == expected_missing

    @pytest.mark.asyncio
    async def test_save_state_without_transfer(self, temp_dir):
        """Test saving state without active transfer."""
        transfer_id = "save_error_001"
        stream = TransferStream(transfer_id)

        state_file = temp_dir / "error_state.json"

        with pytest.raises(TransferError):
            await stream.save_state(state_file)
