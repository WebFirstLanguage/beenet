# Beenet Usage Guide

## Getting Started

This guide provides practical examples for using beenet in real applications, from basic peer-to-peer communication to advanced distributed systems.

## Installation and Setup

### Prerequisites

- Python 3.11 or 3.12
- Poetry for dependency management

### Installation

```bash
# Clone the repository
git clone https://github.com/your-org/beenet.git
cd beenet

# Install dependencies
poetry install

# Verify installation
poetry run python scripts/beenet_demo.py
```

## Basic Usage Examples

### Example 1: Simple File Sharing

Create a basic file sharing application between two peers:

```python
#!/usr/bin/env python3
"""Basic file sharing example."""

import asyncio
from pathlib import Path
import beenet

async def file_sender():
    """Peer that sends a file."""
    sender = beenet.Peer("file_sender")
    
    try:
        # Start peer on specific port
        await sender.start(listen_port=8080)
        print("File sender ready on port 8080")
        
        # Wait for receiver to connect
        await asyncio.sleep(2)
        
        # Send file
        file_path = Path("document.pdf")
        if file_path.exists():
            transfer_id = await sender.send_file("file_receiver", file_path)
            print(f"Started file transfer: {transfer_id}")
        else:
            print("File not found!")
            
    finally:
        await sender.stop()

async def file_receiver():
    """Peer that receives a file."""
    receiver = beenet.Peer("file_receiver")
    
    try:
        # Start peer
        await receiver.start(listen_port=8081)
        
        # Connect to sender
        connected = await receiver.connect_to_peer("file_sender", "127.0.0.1:8080")
        if connected:
            print("Connected to file sender")
        else:
            print("Failed to connect to sender")
            
        # Wait for transfer (in real app, use event handlers)
        await asyncio.sleep(10)
        
    finally:
        await receiver.stop()

# Run both peers
async def main():
    await asyncio.gather(
        file_sender(),
        file_receiver()
    )

if __name__ == "__main__":
    asyncio.run(main())
```

### Example 2: Progress Monitoring

Monitor file transfer progress with callbacks:

```python
#!/usr/bin/env python3
"""File transfer with progress monitoring."""

import asyncio
from pathlib import Path
import beenet

class ProgressTracker:
    def __init__(self, transfer_id: str):
        self.transfer_id = transfer_id
        self.last_progress = 0
        
    def update_progress(self, progress: float):
        """Called when transfer progress updates."""
        if progress - self.last_progress >= 5.0:  # Log every 5%
            print(f"Transfer {self.transfer_id}: {progress:.1f}% complete")
            self.last_progress = progress
            
        if progress >= 100.0:
            print(f"Transfer {self.transfer_id}: Complete! ✅")

async def monitored_transfer():
    sender = beenet.Peer("sender")
    receiver = beenet.Peer("receiver")
    
    try:
        # Start both peers
        await sender.start(listen_port=8080)
        await receiver.start(listen_port=8081)
        
        # Connect peers
        await receiver.connect_to_peer("sender", "127.0.0.1:8080")
        
        # Create large test file (10 MB)
        test_file = Path("large_file.bin")
        test_data = b"X" * (10 * 1024 * 1024)
        test_file.write_bytes(test_data)
        
        # Start transfer with progress tracking
        transfer_id = await sender.send_file("receiver", test_file)
        
        # Set up progress callback
        tracker = ProgressTracker(transfer_id)
        sender.set_transfer_progress_callback(transfer_id, tracker.update_progress)
        
        # Wait for completion
        await asyncio.sleep(30)
        
        # Cleanup
        test_file.unlink()
        
    finally:
        await sender.stop()
        await receiver.stop()

if __name__ == "__main__":
    asyncio.run(monitored_transfer())
```

### Example 3: Event-Driven Architecture

Use beenet's event system for reactive applications:

```python
#!/usr/bin/env python3
"""Event-driven peer network."""

import asyncio
from typing import Dict, Set
import beenet
from beenet.core.events import EventType

class NetworkManager:
    def __init__(self, peer_id: str):
        self.peer = beenet.Peer(peer_id)
        self.connected_peers: Set[str] = set()
        self.pending_transfers: Dict[str, str] = {}
        
    async def start(self, listen_port: int):
        """Start the network manager."""
        # Set up event handlers
        self.peer.event_bus.subscribe(EventType.PEER_DISCOVERED, self._on_peer_discovered)
        self.peer.event_bus.subscribe(EventType.PEER_CONNECTED, self._on_peer_connected)
        self.peer.event_bus.subscribe(EventType.PEER_DISCONNECTED, self._on_peer_disconnected)
        self.peer.event_bus.subscribe(EventType.TRANSFER_STARTED, self._on_transfer_started)
        self.peer.event_bus.subscribe(EventType.TRANSFER_COMPLETED, self._on_transfer_completed)
        self.peer.event_bus.subscribe(EventType.TRANSFER_FAILED, self._on_transfer_failed)
        
        # Start peer
        await self.peer.start(listen_port=listen_port)
        print(f"Network manager started for {self.peer.peer_id}")
        
    async def stop(self):
        """Stop the network manager."""
        await self.peer.stop()
        
    def _on_peer_discovered(self, event):
        """Handle peer discovery."""
        peer_info = event.data
        peer_id = peer_info['peer_id']
        print(f"🔍 Discovered peer: {peer_id} at {peer_info['address']}")
        
        # Auto-connect to discovered peers
        asyncio.create_task(self._auto_connect(peer_id))
        
    async def _auto_connect(self, peer_id: str):
        """Automatically connect to discovered peers."""
        if peer_id not in self.connected_peers:
            success = await self.peer.connect_to_peer(peer_id)
            if success:
                print(f"✅ Auto-connected to {peer_id}")
            
    def _on_peer_connected(self, event):
        """Handle peer connection."""
        peer_id = event.data['peer_id']
        self.connected_peers.add(peer_id)
        print(f"🔗 Connected to peer: {peer_id}")
        
    def _on_peer_disconnected(self, event):
        """Handle peer disconnection."""
        peer_id = event.data['peer_id']
        self.connected_peers.discard(peer_id)
        print(f"💔 Disconnected from peer: {peer_id}")
        
    def _on_transfer_started(self, event):
        """Handle transfer start."""
        transfer_id = event.data['transfer_id']
        peer_id = event.data.get('peer_id', 'unknown')
        direction = event.data.get('direction', 'unknown')
        
        self.pending_transfers[transfer_id] = peer_id
        print(f"📤 Transfer started: {transfer_id} ({direction}) with {peer_id}")
        
    def _on_transfer_completed(self, event):
        """Handle transfer completion."""
        transfer_id = event.data['transfer_id']
        peer_id = self.pending_transfers.pop(transfer_id, 'unknown')
        print(f"✅ Transfer completed: {transfer_id} with {peer_id}")
        
    def _on_transfer_failed(self, event):
        """Handle transfer failure."""
        transfer_id = event.data['transfer_id']
        error = event.data.get('error', 'Unknown error')
        peer_id = self.pending_transfers.pop(transfer_id, 'unknown')
        print(f"❌ Transfer failed: {transfer_id} with {peer_id} - {error}")
        
    async def broadcast_file(self, file_path: Path):
        """Broadcast a file to all connected peers."""
        if not file_path.exists():
            print(f"File not found: {file_path}")
            return
            
        print(f"📡 Broadcasting {file_path.name} to {len(self.connected_peers)} peers")
        
        for peer_id in self.connected_peers:
            try:
                transfer_id = await self.peer.send_file(peer_id, file_path)
                print(f"Started broadcast to {peer_id}: {transfer_id}")
            except Exception as e:
                print(f"Failed to send to {peer_id}: {e}")

# Example usage
async def run_network_node(peer_id: str, port: int):
    """Run a network node."""
    manager = NetworkManager(peer_id)
    
    try:
        await manager.start(port)
        
        # Keep running and handle user commands
        print("Commands: 'peers', 'broadcast <file>', 'quit'")
        
        while True:
            try:
                command = await asyncio.wait_for(
                    asyncio.to_thread(input, "> "), 
                    timeout=1.0
                )
                
                if command == "quit":
                    break
                elif command == "peers":
                    peers = await manager.peer.list_peers()
                    print(f"Connected peers: {[p['peer_id'] for p in peers]}")
                elif command.startswith("broadcast "):
                    filename = command[10:]
                    await manager.broadcast_file(Path(filename))
                    
            except asyncio.TimeoutError:
                continue
                
    finally:
        await manager.stop()

# Run multiple nodes
async def main():
    # Start multiple nodes
    await asyncio.gather(
        run_network_node("node1", 8080),
        run_network_node("node2", 8081),
        run_network_node("node3", 8082)
    )

if __name__ == "__main__":
    asyncio.run(main())
```

## Advanced Configuration

### Custom Discovery Setup

Configure discovery services for different network topologies:

```python
#!/usr/bin/env python3
"""Custom discovery configuration."""

import asyncio
from beenet.discovery import KademliaDiscovery, BeeQuietDiscovery
import beenet

async def configure_discovery():
    """Set up custom discovery configuration."""
    
    # Configure Kademlia DHT with custom bootstrap nodes
    bootstrap_nodes = [
        "dht1.mynetwork.com:8468",
        "dht2.mynetwork.com:8468",
        "backup-dht.mynetwork.com:8468"
    ]
    
    peer = beenet.Peer("configured_peer")
    
    # Override default discovery settings
    peer.kademlia.bootstrap_nodes = bootstrap_nodes
    
    # Custom BeeQuiet callback for filtering
    def filter_discovered_peers(peer_info):
        # Only accept peers from trusted domains
        if peer_info['peer_id'].startswith('trusted_'):
            print(f"Accepting trusted peer: {peer_info['peer_id']}")
            return True
        else:
            print(f"Rejecting untrusted peer: {peer_info['peer_id']}")
            return False
    
    peer.beequiet.on_peer_discovered = filter_discovered_peers
    
    # Start with custom configuration
    await peer.start(
        listen_port=8080,
        bootstrap_nodes=bootstrap_nodes
    )
    
    # Manual peer registration
    await peer.kademlia.register_peer(
        peer_id="configured_peer",
        address="192.168.1.100", 
        port=8080
    )
    
    print("Custom discovery configured")
    return peer

if __name__ == "__main__":
    async def main():
        peer = await configure_discovery()
        await asyncio.sleep(30)  # Run for 30 seconds
        await peer.stop()
    
    asyncio.run(main())
```

### Secure Key Management

Implement proper key management for production:

```python
#!/usr/bin/env python3
"""Secure key management example."""

import asyncio
import os
from pathlib import Path
from beenet.crypto import KeyStore, Identity
import beenet

class SecureBeenetNode:
    def __init__(self, peer_id: str, keystore_path: Path):
        self.peer_id = peer_id
        self.keystore_path = keystore_path
        self.peer = None
        
    async def start_secure(self, passphrase: str):
        """Start node with encrypted keystore."""
        
        # Create secure keystore
        keystore = KeyStore(self.keystore_path, passphrase=passphrase)
        await keystore.open()
        
        # Initialize identity
        identity = Identity(keystore)
        public_key = await identity.load_or_generate_identity(self.peer_id)
        
        print(f"Node identity: {public_key.hex()[:16]}...")
        
        # Create peer with secure keystore
        self.peer = beenet.Peer(self.peer_id, keystore_path=self.keystore_path)
        await self.peer.start()
        
        print(f"Secure node {self.peer_id} started")
        
    async def rotate_keys(self):
        """Rotate cryptographic keys."""
        if self.peer and self.peer.is_running:
            # Rotate identity key
            old_key, new_key = await self.peer.identity.rotate_identity()
            print(f"Identity key rotated: {old_key.hex()[:8]}... -> {new_key.hex()[:8]}...")
            
            # Rotate static keys
            old_static, new_static = await self.peer.key_manager.rotate_static_key(self.peer_id)
            print(f"Static key rotated: {old_static.hex()[:8]}... -> {new_static.hex()[:8]}...")
            
    async def stop(self):
        """Stop the secure node."""
        if self.peer:
            await self.peer.stop()

# Usage example
async def secure_deployment():
    """Example of secure deployment."""
    
    # Get passphrase from environment or prompt
    passphrase = os.getenv('BEENET_PASSPHRASE') or input("Enter keystore passphrase: ")
    
    # Create secure nodes
    node1 = SecureBeenetNode("secure_node_1", Path("~/.beenet/node1"))
    node2 = SecureBeenetNode("secure_node_2", Path("~/.beenet/node2"))
    
    try:
        # Start nodes
        await node1.start_secure(passphrase)
        await node2.start_secure(passphrase)
        
        # Connect nodes
        await node1.peer.connect_to_peer("secure_node_2", "127.0.0.1:8081")
        
        # Simulate key rotation (every 24 hours in production)
        await asyncio.sleep(5)
        await node1.rotate_keys()
        
        # Continue operations
        await asyncio.sleep(10)
        
    finally:
        await node1.stop()
        await node2.stop()

if __name__ == "__main__":
    asyncio.run(secure_deployment())
```

## Production Deployment

### Docker Configuration

```dockerfile
# Dockerfile for beenet application
FROM python:3.12-slim

WORKDIR /app

# Install Poetry
RUN pip install poetry

# Copy dependency files
COPY pyproject.toml poetry.lock ./

# Install dependencies
RUN poetry config virtualenvs.create false && \
    poetry install --no-dev

# Copy application
COPY . .

# Create non-root user
RUN useradd -m beenet
USER beenet

# Expose ports
EXPOSE 8080 8468 7777/udp

# Run application
CMD ["poetry", "run", "python", "your_app.py"]
```

### Docker Compose Setup

```yaml
# docker-compose.yml
version: '3.8'

services:
  beenet-node1:
    build: .
    ports:
      - "8080:8080"
      - "8468:8468" 
      - "7777:7777/udp"
    environment:
      - BEENET_PEER_ID=node1
      - BEENET_LISTEN_PORT=8080
      - BEENET_KEYSTORE_PASSPHRASE=${KEYSTORE_PASSPHRASE}
    volumes:
      - ./data/node1:/app/data
    command: poetry run python production_node.py
    
  beenet-node2:
    build: .
    ports:
      - "8081:8080"
      - "8469:8468"
      - "7778:7777/udp"
    environment:
      - BEENET_PEER_ID=node2
      - BEENET_LISTEN_PORT=8080
      - BEENET_BOOTSTRAP_NODES=beenet-node1:8468
      - BEENET_KEYSTORE_PASSPHRASE=${KEYSTORE_PASSPHRASE}
    volumes:
      - ./data/node2:/app/data
    command: poetry run python production_node.py
    depends_on:
      - beenet-node1
```

### Production Application Template

```python
#!/usr/bin/env python3
"""Production beenet application template."""

import asyncio
import logging
import os
import signal
from pathlib import Path
from typing import Optional
import beenet

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

class BeenetService:
    def __init__(self):
        self.peer: Optional[beenet.Peer] = None
        self.running = False
        
    async def start(self):
        """Start the beenet service."""
        try:
            # Get configuration from environment
            peer_id = os.getenv('BEENET_PEER_ID', 'default_peer')
            listen_port = int(os.getenv('BEENET_LISTEN_PORT', '8080'))
            keystore_path = Path(os.getenv('BEENET_KEYSTORE_PATH', './data/keys'))
            
            # Bootstrap nodes
            bootstrap_env = os.getenv('BEENET_BOOTSTRAP_NODES', '')
            bootstrap_nodes = [node.strip() for node in bootstrap_env.split(',') if node.strip()]
            
            # Create peer
            keystore_path.mkdir(parents=True, exist_ok=True)
            self.peer = beenet.Peer(peer_id, keystore_path=keystore_path)
            
            # Set up event handlers
            self._setup_event_handlers()
            
            # Start peer
            await self.peer.start(
                listen_port=listen_port,
                bootstrap_nodes=bootstrap_nodes if bootstrap_nodes else None
            )
            
            self.running = True
            logger.info(f"Beenet service started: {peer_id} on port {listen_port}")
            
        except Exception as e:
            logger.error(f"Failed to start beenet service: {e}")
            raise
            
    async def stop(self):
        """Stop the beenet service."""
        self.running = False
        if self.peer:
            await self.peer.stop()
            logger.info("Beenet service stopped")
            
    def _setup_event_handlers(self):
        """Set up event handlers for monitoring."""
        from beenet.core.events import EventType
        
        def log_peer_events(event):
            logger.info(f"Peer event: {event.event_type.name} - {event.data}")
            
        def log_transfer_events(event):
            if event.event_type == EventType.TRANSFER_PROGRESS:
                # Only log significant progress updates
                progress = event.data.get('progress', 0)
                if progress % 20 == 0:  # Every 20%
                    logger.info(f"Transfer progress: {progress}%")
            else:
                logger.info(f"Transfer event: {event.event_type.name} - {event.data}")
        
        # Subscribe to events
        self.peer.event_bus.subscribe(EventType.PEER_DISCOVERED, log_peer_events)
        self.peer.event_bus.subscribe(EventType.PEER_CONNECTED, log_peer_events)
        self.peer.event_bus.subscribe(EventType.PEER_DISCONNECTED, log_peer_events)
        
        self.peer.event_bus.subscribe(EventType.TRANSFER_STARTED, log_transfer_events)
        self.peer.event_bus.subscribe(EventType.TRANSFER_PROGRESS, log_transfer_events)
        self.peer.event_bus.subscribe(EventType.TRANSFER_COMPLETED, log_transfer_events)
        self.peer.event_bus.subscribe(EventType.TRANSFER_FAILED, log_transfer_events)
        
    async def run_forever(self):
        """Run the service until stopped."""
        while self.running:
            try:
                await asyncio.sleep(1)
            except asyncio.CancelledError:
                break
                
        await self.stop()

# Signal handling for graceful shutdown
def setup_signal_handlers(service: BeenetService):
    """Set up signal handlers for graceful shutdown."""
    def signal_handler():
        logger.info("Received shutdown signal")
        service.running = False
        
    if os.name != 'nt':  # Unix-like systems
        loop = asyncio.get_event_loop()
        for sig in (signal.SIGTERM, signal.SIGINT):
            loop.add_signal_handler(sig, signal_handler)

async def main():
    """Main entry point."""
    service = BeenetService()
    
    try:
        await service.start()
        setup_signal_handlers(service)
        await service.run_forever()
    except KeyboardInterrupt:
        logger.info("Received keyboard interrupt")
    except Exception as e:
        logger.error(f"Service error: {e}")
    finally:
        await service.stop()

if __name__ == "__main__":
    asyncio.run(main())
```

## Testing and Development

### Unit Testing Example

```python
#!/usr/bin/env python3
"""Unit testing example for beenet applications."""

import asyncio
import pytest
from pathlib import Path
import tempfile
import beenet

class TestBeenetIntegration:
    
    @pytest.fixture
    async def peers(self):
        """Create test peers."""
        with tempfile.TemporaryDirectory() as temp_dir:
            peer1 = beenet.Peer("test_peer_1", keystore_path=Path(temp_dir) / "peer1")
            peer2 = beenet.Peer("test_peer_2", keystore_path=Path(temp_dir) / "peer2")
            
            await peer1.start(listen_port=0)  # Random port
            await peer2.start(listen_port=0)
            
            yield peer1, peer2
            
            await peer1.stop()
            await peer2.stop()
    
    @pytest.mark.asyncio
    async def test_peer_connection(self, peers):
        """Test basic peer connection."""
        peer1, peer2 = peers
        
        # Connect peer2 to peer1
        peer1_addr = f"127.0.0.1:{peer1.connection_manager.listen_port}"
        connected = await peer2.connect_to_peer("test_peer_1", peer1_addr)
        
        assert connected, "Peers should connect successfully"
        
        # Verify connection from both sides
        peer1_connections = await peer1.connection_manager.get_connected_peers()
        peer2_connections = await peer2.connection_manager.get_connected_peers()
        
        assert len(peer1_connections) > 0, "Peer1 should have connections"
        assert len(peer2_connections) > 0, "Peer2 should have connections"
    
    @pytest.mark.asyncio
    async def test_file_transfer(self, peers):
        """Test file transfer between peers."""
        peer1, peer2 = peers
        
        # Connect peers
        peer1_addr = f"127.0.0.1:{peer1.connection_manager.listen_port}"
        await peer2.connect_to_peer("test_peer_1", peer1_addr)
        
        # Create test file
        with tempfile.NamedTemporaryFile(delete=False) as temp_file:
            test_data = b"Hello, beenet!" * 1000  # 14KB
            temp_file.write(test_data)
            temp_file_path = Path(temp_file.name)
        
        try:
            # Send file
            transfer_id = await peer1.send_file("test_peer_2", temp_file_path)
            assert transfer_id, "Transfer should return an ID"
            
            # Wait for transfer completion
            await asyncio.sleep(5)
            
            # Verify transfer (in real implementation, use proper completion detection)
            
        finally:
            temp_file_path.unlink()

# Run tests
if __name__ == "__main__":
    pytest.main([__file__, "-v"])
```

This comprehensive usage guide covers everything from basic file sharing to production deployment with Docker. The examples demonstrate real-world patterns for building distributed applications with beenet.