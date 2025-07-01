#!/usr/bin/env python3
"""CLI demo for beenet P2P library."""

import asyncio
import logging
import sys
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent))

from beenet import Peer
from beenet.transfer import TransferStream, DataChunker, MerkleTree


async def main():
    """Run the beenet demo."""
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
    )

    print("🐝 beenet P2P Library Demo")
    print("=" * 30)

    try:
        # Create peers
        print("Creating two peers...")
        peer1 = Peer(peer_id="demo_peer_1")
        peer2 = Peer(peer_id="demo_peer_2")

        # Start peers
        print("Starting peers...")
        await peer1.start(listen_port=0)
        await peer2.start(listen_port=0)

        print(f"Peer 1 listening on port: {peer1.connection_manager.listen_port}")
        print(f"Peer 2 listening on port: {peer2.connection_manager.listen_port}")

        # Wait a bit for discovery services to stabilize
        await asyncio.sleep(2)

        # Connect peer2 to peer1
        print("\nConnecting peers...")
        peer1_addr = f"127.0.0.1:{peer1.connection_manager.listen_port}"
        connected = await peer2.connect_to_peer("demo_peer_1", peer1_addr)
        
        if not connected:
            print("Failed to connect peers!")
            return 1

        print("Peers connected successfully!")

        # Create a 10 MiB test file
        print("\nCreating 10 MiB test file...")
        test_data = b"X" * (10 * 1024 * 1024)  # 10 MiB
        test_file = Path("/tmp/beenet_test_file.bin")
        test_file.write_bytes(test_data)
        print(f"Test file created: {test_file} ({len(test_data):,} bytes)")

        # Demonstrate file transfer using TransferStream components
        print("\nDemonstrating file transfer components...")
        
        # Create chunker and merkle tree
        chunker = DataChunker(chunk_size=64 * 1024)  # 64 KB chunks (max allowed)
        chunks = list(chunker.chunk_file(test_file))
        print(f"File split into {len(chunks)} chunks")
        
        # Build merkle tree
        merkle_tree = MerkleTree()
        chunk_data_list = []
        for chunk_index, chunk_data in chunks:
            chunk_hash = MerkleTree.hash_chunk(chunk_data)
            merkle_tree.add_chunk_hash(chunk_hash)
            chunk_data_list.append(chunk_data)
        
        merkle_root = merkle_tree.build_tree()
        print(f"Merkle root: {merkle_root.hex()[:16]}...")
        
        # Simulate transfer using the connection
        print("\nSimulating chunk-based transfer...")
        start_time = time.time()
        
        # Create transfer streams
        transfer_id = "demo_transfer_001"
        sender_stream = TransferStream(transfer_id, chunker)
        receiver_stream = TransferStream(transfer_id, chunker)
        
        # Initialize receiver
        receive_file = Path("/tmp/beenet_received_file.bin")
        await receiver_stream.start_receive(receive_file, merkle_root, len(chunks))
        
        # Transfer chunks
        transferred_bytes = 0
        for i, chunk_data in enumerate(chunk_data_list):
            proof = merkle_tree.generate_proof(i)
            success = await receiver_stream.receive_chunk(i, chunk_data, proof)
            if success:
                transferred_bytes += len(chunk_data)
                if (i + 1) % 5 == 0:  # Progress every 5 chunks
                    progress = receiver_stream.state.progress if receiver_stream.state else 0
                    print(f"Transfer progress: {progress:.1f}% ({transferred_bytes:,} bytes)")
        
        transfer_time = time.time() - start_time
        
        # Verify transfer
        print("\nVerifying transfer...")
        if receive_file.exists():
            received_data = receive_file.read_bytes()
            if received_data[:transferred_bytes] == test_data[:transferred_bytes]:
                print("✅ File transfer successful! Data integrity verified.")
                transfer_rate = transferred_bytes / transfer_time / 1024 / 1024
                print(f"Transfer rate: {transfer_rate:.2f} MB/s")
            else:
                print("❌ Data verification failed!")
        else:
            print("❌ Received file not found!")

        # Show peer connection status
        print("\nPeer connection details:")
        connected_peers = await peer1.connection_manager.get_connected_peers()
        print(f"Peer 1 has {len(connected_peers)} connections")
        for peer_info in connected_peers:
            print(f"  - Connected to: {peer_info['peer_id']}")
        
        connected_peers = await peer2.connection_manager.get_connected_peers()
        print(f"Peer 2 has {len(connected_peers)} connections")
        for peer_info in connected_peers:
            print(f"  - Connected to: {peer_info['peer_id']}")

        # Cleanup
        print("\nCleaning up...")
        test_file.unlink()
        if receive_file.exists():
            receive_file.unlink()
        
        # Shutdown peers
        print("Shutting down peers...")
        await peer1.stop()
        await peer2.stop()

        print("\nDemo completed successfully! ✅")

    except Exception as e:
        print(f"Demo failed with error: {e}")
        import traceback
        traceback.print_exc()
        return 1

    return 0


if __name__ == "__main__":
    exit_code = asyncio.run(main())
    sys.exit(exit_code)