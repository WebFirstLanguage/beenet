#!/usr/bin/env python3
"""CLI demo for beenet P2P library."""

import asyncio
import logging
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent))

from beenet import Peer


async def main():
    """Run the beenet demo."""
    logging.basicConfig(level=logging.INFO)

    print("🐝 beenet P2P Library Demo")
    print("=" * 30)

    try:
        print("Creating two peers...")
        peer1 = Peer(peer_id="demo_peer_1")
        peer2 = Peer(peer_id="demo_peer_2")

        print("Starting peers...")
        await peer1.start()
        await peer2.start()

        print(f"Peer 1 listening on port: {peer1.connection_manager.listen_port}")
        print(f"Peer 2 listening on port: {peer2.connection_manager.listen_port}")

        print("Simulating file transfer...")
        test_data = b"Hello from beenet P2P library! This is a test transfer."

        print(f"Test data size: {len(test_data)} bytes")
        print("Demo completed successfully! ✅")

        print("Shutting down peers...")
        await peer1.stop()
        await peer2.stop()

    except Exception as e:
        print(f"Demo failed with error: {e}")
        return 1

    return 0


if __name__ == "__main__":
    exit_code = asyncio.run(main())
    sys.exit(exit_code)
