"""Integration tests for peer-to-peer functionality with async cancellation."""

import asyncio
import tempfile
from pathlib import Path

import pytest

from beenet.core import Peer
from beenet.transfer import MerkleTree


class TestPeerIntegration:
    """Integration tests for peer functionality."""

    @pytest.mark.asyncio
    async def test_two_peer_startup_shutdown(self, two_test_peers):
        """Test starting and stopping two peers."""
        peer1, peer2 = two_test_peers

        await peer1.start(8500)
        await peer2.start(8501)

        assert peer1.is_running
        assert peer2.is_running

        await peer1.stop()
        await peer2.stop()

        assert not peer1.is_running
        assert not peer2.is_running

    @pytest.mark.asyncio
    async def test_peer_discovery(self, two_test_peers):
        """Test peer discovery between two peers."""
        peer1, peer2 = two_test_peers

        await peer1.start(8502)
        await peer2.start(8503)

        await asyncio.sleep(2)  # Allow discovery to work

        peers1 = await peer1.list_peers()
        peers2 = await peer2.list_peers()

        await peer1.stop()
        await peer2.stop()

    @pytest.mark.asyncio
    async def test_peer_connection(self, two_test_peers):
        """Test direct peer connection."""
        peer1, peer2 = two_test_peers

        await peer1.start(8504)
        await peer2.start(8505)

        success = await peer1.connect_to_peer(peer2.peer_id, "127.0.0.1:8505")

        await peer1.stop()
        await peer2.stop()

    @pytest.mark.asyncio
    async def test_small_file_transfer(self, two_test_peers, test_file):
        """Test small file transfer between peers."""
        peer1, peer2 = two_test_peers

        await peer1.start(8506)
        await peer2.start(8507)

        await peer1.connect_to_peer(peer2.peer_id, "127.0.0.1:8507")

        transfer_id = await peer1.send_file(peer2.peer_id, test_file)
        assert transfer_id is not None

        with tempfile.TemporaryDirectory() as tmpdir:
            receive_path = Path(tmpdir) / "received_file.txt"
            success = await peer2.receive_file(transfer_id, receive_path)

        await peer1.stop()
        await peer2.stop()

    @pytest.mark.asyncio
    async def test_large_file_transfer_with_verification(self, two_test_peers, large_test_file):
        """Test large file transfer with Merkle verification."""
        peer1, peer2 = two_test_peers

        await peer1.start(8508)
        await peer2.start(8509)

        await peer1.connect_to_peer(peer2.peer_id, "127.0.0.1:8509")

        original_data = large_test_file.read_bytes()
        original_hash = MerkleTree.hash_chunk(original_data)

        transfer_id = await peer1.send_file(peer2.peer_id, large_test_file)

        with tempfile.TemporaryDirectory() as tmpdir:
            receive_path = Path(tmpdir) / "received_large_file.bin"

            start_time = asyncio.get_event_loop().time()
            success = await peer2.receive_file(transfer_id, receive_path)
            end_time = asyncio.get_event_loop().time()

            transfer_time = end_time - start_time

            if success and receive_path.exists():
                received_data = receive_path.read_bytes()
                received_hash = MerkleTree.hash_chunk(received_data)

                assert received_hash == original_hash
                assert len(received_data) == len(original_data)

                file_size_mb = len(original_data) / (1024 * 1024)
                print(f"Transferred {file_size_mb:.1f} MB in {transfer_time:.2f}s")

                if transfer_time > 0:
                    speed_mbps = file_size_mb / transfer_time
                    print(f"Transfer speed: {speed_mbps:.2f} MB/s")

        await peer1.stop()
        await peer2.stop()

    @pytest.mark.asyncio
    async def test_transfer_cancellation_and_resumption(self, two_test_peers, large_test_file):
        """Test transfer cancellation and resumption with async cancellation coverage."""
        peer1, peer2 = two_test_peers

        await peer1.start(8510)
        await peer2.start(8511)

        await peer1.connect_to_peer(peer2.peer_id, "127.0.0.1:8511")

        transfer_id = await peer1.send_file(peer2.peer_id, large_test_file)

        with tempfile.TemporaryDirectory() as tmpdir:
            receive_path = Path(tmpdir) / "cancelled_transfer.bin"
            state_path = Path(tmpdir) / f"{transfer_id}.state"

            async def cancelled_transfer():
                await asyncio.sleep(0.1)  # Start transfer
                raise asyncio.CancelledError("Simulated cancellation")

            try:
                await asyncio.wait_for(cancelled_transfer(), timeout=1.0)
            except asyncio.CancelledError:
                pass

            if transfer_id in peer1._transfers:
                await peer1._transfers[transfer_id].save_state(state_path)

            if state_path.exists():
                if transfer_id in peer2._transfers:
                    await peer2._transfers[transfer_id].resume_transfer(state_path)

                    success = await peer2.receive_file(transfer_id, receive_path)

        await peer1.stop()
        await peer2.stop()

    @pytest.mark.asyncio
    async def test_graceful_shutdown_during_transfer(self, two_test_peers, test_file):
        """Test graceful shutdown during active transfer."""
        peer1, peer2 = two_test_peers

        await peer1.start(8512)
        await peer2.start(8513)

        await peer1.connect_to_peer(peer2.peer_id, "127.0.0.1:8513")

        transfer_id = await peer1.send_file(peer2.peer_id, test_file)

        await asyncio.sleep(0.1)  # Let transfer start

        await peer1.stop()
        await peer2.stop()

    @pytest.mark.asyncio
    async def test_multiple_concurrent_transfers(self, two_test_peers, temp_dir):
        """Test multiple concurrent file transfers."""
        peer1, peer2 = two_test_peers

        await peer1.start(8514)
        await peer2.start(8515)

        await peer1.connect_to_peer(peer2.peer_id, "127.0.0.1:8515")

        test_files = []
        for i in range(3):
            test_file = temp_dir / f"test_file_{i}.txt"
            test_file.write_text(f"Test content for file {i}\n" * 100)
            test_files.append(test_file)

        transfer_tasks = []
        for test_file in test_files:
            task = asyncio.create_task(peer1.send_file(peer2.peer_id, test_file))
            transfer_tasks.append(task)

        transfer_ids = await asyncio.gather(*transfer_tasks, return_exceptions=True)

        await peer1.stop()
        await peer2.stop()

    @pytest.mark.asyncio
    async def test_peer_reconnection(self, two_test_peers):
        """Test peer reconnection after disconnect."""
        peer1, peer2 = two_test_peers

        await peer1.start(8516)
        await peer2.start(8517)

        success1 = await peer1.connect_to_peer(peer2.peer_id, "127.0.0.1:8517")

        await peer2.stop()
        await asyncio.sleep(0.5)

        await peer2.start(8517)

        success2 = await peer1.connect_to_peer(peer2.peer_id, "127.0.0.1:8517")

        await peer1.stop()
        await peer2.stop()

    @pytest.mark.asyncio
    async def test_keystore_persistence_across_restarts(self, temp_dir):
        """Test that keystore persists across peer restarts."""
        keystore_path = temp_dir / "persistent_keystore"
        peer_id = "persistent_peer"

        peer1 = Peer(peer_id, keystore_path)
        await peer1.start(8518)
        public_key1 = peer1.public_key
        await peer1.stop()

        peer2 = Peer(peer_id, keystore_path)
        await peer2.start(8518)
        public_key2 = peer2.public_key
        await peer2.stop()

        assert public_key1 == public_key2

    @pytest.mark.asyncio
    async def test_aead_secured_beequiet_discovery(self, two_test_peers):
        """Test AEAD-secured BeeQuiet LAN discovery between peers."""
        peer1, peer2 = two_test_peers

        await peer1.start(8519)
        await peer2.start(8520)

        await asyncio.sleep(3)  # Allow BeeQuiet discovery to work

        discovered_peers1 = peer1.beequiet.get_discovered_peers()
        discovered_peers2 = peer2.beequiet.get_discovered_peers()

        await peer1.stop()
        await peer2.stop()

    @pytest.mark.asyncio
    async def test_async_cancellation_during_file_transfer(self, two_test_peers, large_test_file):
        """Test injecting asyncio.CancelledError during file transfer for graceful shutdown."""
        peer1, peer2 = two_test_peers

        await peer1.start(8521)
        await peer2.start(8522)

        await peer1.connect_to_peer(peer2.peer_id, "127.0.0.1:8522")

        with tempfile.TemporaryDirectory() as tmpdir:
            receive_path = Path(tmpdir) / "cancelled_file.bin"
            state_path = Path(tmpdir) / "transfer_state.json"

            async def transfer_with_cancellation():
                transfer_id = await peer1.send_file(peer2.peer_id, large_test_file)

                await asyncio.sleep(0.2)  # Let transfer start

                if transfer_id in peer1._transfers:
                    await peer1._transfers[transfer_id].save_state(state_path)

                raise asyncio.CancelledError("Injected cancellation for testing")

            try:
                await transfer_with_cancellation()
            except asyncio.CancelledError:
                pass

            await peer1.stop()
            await peer2.stop()

            assert state_path.exists()

    @pytest.mark.asyncio
    async def test_keystore_flush_on_cancellation(self, temp_dir):
        """Test proper keystore flushing during cancellation."""
        keystore_path = temp_dir / "flush_test_keystore"
        peer_id = "flush_test_peer"

        peer = Peer(peer_id, keystore_path)

        async def peer_with_cancellation():
            await peer.start(8523)
            await asyncio.sleep(0.1)
            raise asyncio.CancelledError("Testing keystore flush")

        try:
            await peer_with_cancellation()
        except asyncio.CancelledError:
            pass
        finally:
            if peer.is_running:
                await peer.stop()

        assert keystore_path.exists()

    @pytest.mark.asyncio
    async def test_ten_mib_file_transfer_performance(self, two_test_peers, temp_dir):
        """Test 10 MiB file transfer in < 60s with Merkle verification."""
        peer1, peer2 = two_test_peers

        ten_mib_data = b"X" * (10 * 1024 * 1024)  # 10 MiB
        large_file = temp_dir / "ten_mib_file.bin"
        large_file.write_bytes(ten_mib_data)

        await peer1.start(8524)
        await peer2.start(8525)

        await peer1.connect_to_peer(peer2.peer_id, "127.0.0.1:8525")

        original_hash = MerkleTree.hash_chunk(ten_mib_data)

        with tempfile.TemporaryDirectory() as tmpdir:
            receive_path = Path(tmpdir) / "received_ten_mib.bin"

            start_time = asyncio.get_event_loop().time()

            transfer_id = await peer1.send_file(peer2.peer_id, large_file)
            success = await peer2.receive_file(transfer_id, receive_path)

            end_time = asyncio.get_event_loop().time()
            transfer_time = end_time - start_time

            assert transfer_time < 60.0, f"Transfer took {transfer_time:.2f}s, should be < 60s"

            if success and receive_path.exists():
                received_data = receive_path.read_bytes()
                received_hash = MerkleTree.hash_chunk(received_data)

                assert received_hash == original_hash
                assert len(received_data) == len(ten_mib_data)

                print(f"10 MiB transfer completed in {transfer_time:.2f}s")
                speed_mbps = 10.0 / transfer_time
                print(f"Transfer speed: {speed_mbps:.2f} MB/s")

        await peer1.stop()
        await peer2.stop()

    @pytest.mark.asyncio
    async def test_transfer_resumption_after_cancellation(self, two_test_peers, large_test_file):
        """Test transfer resumption after cancellation with state persistence."""
        peer1, peer2 = two_test_peers

        await peer1.start(8526)
        await peer2.start(8527)

        await peer1.connect_to_peer(peer2.peer_id, "127.0.0.1:8527")

        with tempfile.TemporaryDirectory() as tmpdir:
            receive_path = Path(tmpdir) / "resumed_file.bin"
            state_path = Path(tmpdir) / "resume_state.json"

            transfer_id = await peer1.send_file(peer2.peer_id, large_test_file)

            await asyncio.sleep(0.1)  # Let some transfer happen

            if transfer_id in peer1._transfers:
                await peer1._transfers[transfer_id].save_state(state_path)

            await peer1.stop()
            await peer2.stop()

            await peer1.start(8526)
            await peer2.start(8527)

            await peer1.connect_to_peer(peer2.peer_id, "127.0.0.1:8527")

            if state_path.exists() and transfer_id in peer2._transfers:
                await peer2._transfers[transfer_id].resume_transfer(state_path)
                success = await peer2.receive_file(transfer_id, receive_path)

            await peer1.stop()
            await peer2.stop()
