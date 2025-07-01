# Beenet API Documentation

## Overview

Beenet is a secure peer-to-peer networking library for Python that provides encrypted communication channels, hybrid peer discovery, and integrity-verified data transfer. This document covers the complete API for integrating beenet into your applications.

## Core Features

- **Secure Channels**: Noise XX protocol with mutual authentication
- **Hybrid Discovery**: Kademlia DHT + BeeQuiet LAN discovery  
- **Data Integrity**: Merkle tree verification for all transfers
- **Resumable Transfers**: State persistence for interrupted transfers
- **Event-Driven**: Async event system for extensibility

## Quick Start

### Basic Setup

```python
import asyncio
from pathlib import Path
import beenet

async def basic_example():
    # Create and start a peer
    peer = beenet.Peer("my_peer_id")
    await peer.start(listen_port=8080)
    
    # Connect to another peer
    connected = await peer.connect_to_peer("other_peer", "192.168.1.100:8080")
    if connected:
        print("Connected successfully!")
    
    # Send a file
    transfer_id = await peer.send_file("other_peer", Path("myfile.txt"))
    print(f"Started transfer: {transfer_id}")
    
    # Clean shutdown
    await peer.stop()

# Run the example
asyncio.run(basic_example())
```

## Main API Classes

### Peer Class

The `Peer` class is the primary entry point for all beenet functionality.

#### Constructor

```python
Peer(peer_id: str, keystore_path: Optional[Path] = None)
```

**Parameters:**
- `peer_id`: Unique identifier for this peer
- `keystore_path`: Optional path for key storage (defaults to temp directory)

#### Core Methods

##### Starting and Stopping

```python
async def start(listen_port: int = 0, bootstrap_nodes: Optional[List[str]] = None) -> None
```
Start the peer and all discovery services.

**Parameters:**
- `listen_port`: Port to listen on (0 for random assignment)
- `bootstrap_nodes`: List of DHT bootstrap nodes in format `["host:port"]`

```python
async def stop() -> None
```
Stop the peer and cleanup all resources.

##### Connection Management

```python
async def connect_to_peer(peer_id: str, address: Optional[str] = None) -> bool
```
Connect to another peer.

**Parameters:**
- `peer_id`: Target peer identifier
- `address`: Optional direct address `"host:port"` (bypasses discovery)

**Returns:** `True` if connection succeeded

```python
async def list_peers() -> List[Dict[str, Any]]
```
Get list of discovered and connected peers.

**Returns:** List of peer information dictionaries containing:
- `peer_id`: Peer identifier
- `address`: IP address
- `port`: Port number
- `protocol`: Discovery protocol used
- `last_seen`: Timestamp of last contact

##### File Transfer

```python
async def send_file(peer_id: str, file_path: Path, transfer_id: Optional[str] = None) -> str
```
Send a file to another peer.

**Parameters:**
- `peer_id`: Target peer identifier
- `file_path`: Path to file to send
- `transfer_id`: Optional custom transfer identifier

**Returns:** Transfer identifier for tracking

```python
async def receive_file(transfer_id: str, save_path: Path) -> bool
```
Receive a file from another peer.

**Parameters:**
- `transfer_id`: Transfer identifier from sender
- `save_path`: Where to save the received file

**Returns:** `True` if transfer completed successfully

##### Progress Tracking

```python
def set_transfer_progress_callback(transfer_id: str, callback: Callable[[float], None]) -> None
```
Set a callback function to receive transfer progress updates.

**Parameters:**
- `transfer_id`: Transfer to monitor
- `callback`: Function called with progress percentage (0.0-100.0)

##### Properties

```python
@property
def is_running(self) -> bool
```
Check if peer is currently running.

```python
@property  
def public_key(self) -> Optional[bytes]
```
Get this peer's Ed25519 public identity key.

```python
async def get_peer_info(self) -> Dict[str, Any]
```
Get comprehensive information about this peer.

**Returns:** Dictionary containing:
- `peer_id`: This peer's identifier
- `public_key`: Public identity key
- `is_running`: Current running status
- `active_transfers`: Number of active transfers

## Discovery APIs

### Kademlia DHT Discovery

For global peer discovery across networks.

```python
from beenet.discovery import KademliaDiscovery

# Create and configure
kademlia = KademliaDiscovery(bootstrap_nodes=["dht.example.com:8468"])
await kademlia.start(listen_port=8468)

# Register this peer
await kademlia.register_peer("my_peer", "192.168.1.100", 8080)

# Find a specific peer
peer_info = await kademlia.find_peer("target_peer_id")

# Find peers near a target
nearby_peers = await kademlia.find_peers_near("target_id", count=10)

# Check routing table
table_size = await kademlia.get_routing_table_size()
node_id = await kademlia.get_node_id()

await kademlia.stop()
```

### BeeQuiet LAN Discovery

For local network peer discovery with secure messaging.

```python
from beenet.discovery import BeeQuietDiscovery

def on_peer_found(peer_info):
    print(f"Found peer: {peer_info['peer_id']} at {peer_info['address']}")

# Create with callback
beequiet = BeeQuietDiscovery("my_peer", on_peer_discovered=on_peer_found)
await beequiet.start(bind_port=0)  # 0 for random port

# Send discovery broadcast
await beequiet.send_who_is_here()

# Get discovered peers
peers = beequiet.get_discovered_peers()

await beequiet.stop()
```

**BeeQuiet Configuration:**
- Multicast address: `239.255.7.7:7777`
- Magic number: `0xBEEC`
- Heartbeat interval: 30 seconds
- Peer timeout: 90 seconds

## Transfer APIs

### TransferStream

For managing file transfers with resumability and verification.

```python
from beenet.transfer import TransferStream, DataChunker
from pathlib import Path

# Create transfer stream
chunker = DataChunker(chunk_size=32*1024)  # 32KB chunks
stream = TransferStream("my_transfer", chunker)

# Sender side
await stream.start_send(Path("myfile.txt"), "peer_address")

# Get chunks and proofs
for chunk_index, chunk_data in chunker.chunk_file("myfile.txt"):
    proof = stream.merkle_tree.generate_proof(chunk_index)
    await stream.send_chunk(chunk_index, chunk_data, proof)

# Receiver side
await stream.start_receive(Path("received.txt"), expected_root, total_chunks)

# Receive chunks
success = await stream.receive_chunk(chunk_index, chunk_data, proof)

# Save/resume transfer state
await stream.save_state(Path("transfer.state"))
await stream.resume_transfer(Path("transfer.state"))

# Verify completed file
is_valid = await stream.verify_complete_file(Path("received.txt"))
```

### DataChunker

For efficient data chunking.

```python
from beenet.transfer import DataChunker

# Create chunker with custom size
chunker = DataChunker(chunk_size=16*1024)  # 16KB chunks

# Chunk a file
for chunk_index, chunk_data in chunker.chunk_file("myfile.txt"):
    print(f"Chunk {chunk_index}: {len(chunk_data)} bytes")

# Chunk raw data
data = b"some large data..."
for chunk_index, chunk_data in chunker.chunk_data(data):
    process_chunk(chunk_index, chunk_data)

# Calculate chunk count
total_chunks = DataChunker.calculate_chunk_count(file_size, chunk_size)

# Negotiate chunk size with peer
optimal_size = await chunker.negotiate_chunk_size(
    proposed_size=32*1024,
    peer_max_size=64*1024
)
```

**Chunk Size Limits:**
- Minimum: 4 KiB
- Maximum: 64 KiB  
- Default: 16 KiB

### MerkleTree

For cryptographic integrity verification.

```python
from beenet.transfer import MerkleTree

# Build tree from chunk hashes
chunk_hashes = [MerkleTree.hash_chunk(chunk) for chunk in chunks]
tree = MerkleTree(chunk_hashes)
root_hash = tree.build_tree()

# Generate proofs
for i, chunk in enumerate(chunks):
    proof = tree.generate_proof(i)
    
    # Verify chunk
    is_valid = tree.verify_chunk(chunk, i, proof)
    
    # Or verify with just the proof
    is_valid = proof.verify(root_hash)
```

## Cryptography APIs

### Identity Management

```python
from beenet.crypto import Identity, KeyStore

# Create keystore and identity
keystore = KeyStore(Path("~/.beenet/keys"))
await keystore.open()

identity = Identity(keystore)
public_key = await identity.load_or_generate_identity("my_peer")

# Sign and verify messages
signature = await identity.sign_message(b"hello world")
is_valid = await identity.verify_signature(
    b"hello world", 
    signature, 
    public_key
)

# Rotate identity key
old_key, new_key = await identity.rotate_identity()
```

### Key Storage

```python
from beenet.crypto import KeyStore

# Create encrypted keystore
keystore = KeyStore(Path("keys.db"), passphrase="secret")
await keystore.open()

# Store keys
await keystore.store_key("session_key_1", key_bytes, encrypted=True)

# Retrieve keys
key_data = await keystore.get_key("session_key_1")

# List all keys
key_ids = await keystore.list_keys()

# Rotate a key
old_key = await keystore.rotate_key("session_key_1", new_key_bytes)

await keystore.close()
```

## Event System

### Event Types

```python
from beenet.core.events import EventType

# Available event types
EventType.PEER_DISCOVERED     # New peer found
EventType.PEER_CONNECTED      # Peer connection established  
EventType.PEER_DISCONNECTED   # Peer disconnected
EventType.TRANSFER_STARTED    # File transfer began
EventType.TRANSFER_PROGRESS   # Transfer progress update
EventType.TRANSFER_COMPLETED  # Transfer finished successfully
EventType.TRANSFER_FAILED     # Transfer failed
EventType.KEY_ROTATED         # Cryptographic key rotated
EventType.NETWORK_ERROR       # Network error occurred
```

### Event Handling

```python
def handle_peer_discovered(event):
    peer_info = event.data
    print(f"Discovered peer: {peer_info['peer_id']}")

def handle_transfer_progress(event):
    progress = event.data['progress']
    transfer_id = event.data['transfer_id']
    print(f"Transfer {transfer_id}: {progress:.1f}%")

# Subscribe to specific events
peer.event_bus.subscribe(EventType.PEER_DISCOVERED, handle_peer_discovered)
peer.event_bus.subscribe(EventType.TRANSFER_PROGRESS, handle_transfer_progress)

# Subscribe to all events
def handle_all_events(event):
    print(f"Event: {event.event_type.name} from {event.source}")

peer.event_bus.subscribe_all(handle_all_events)

# Emit custom events
await peer.event_bus.emit(
    EventType.NETWORK_ERROR,
    {"error": "Connection timeout", "peer_id": "failed_peer"}
)
```

## Configuration

### Environment Variables

```bash
# Key storage encryption
export BEENET_KEYSTORE_PASSPHRASE="your_secure_passphrase"

# Default ports
export BEENET_LISTEN_PORT=8080
export BEENET_DHT_PORT=8468

# Discovery settings  
export BEENET_BOOTSTRAP_NODES="dht1.example.com:8468,dht2.example.com:8468"

# Transfer settings
export BEENET_DEFAULT_CHUNK_SIZE=16384  # 16KB
export BEENET_TRANSFER_TIMEOUT=60       # seconds
```

### Performance Tuning

```python
# Optimize chunk size for your network
chunker = DataChunker(chunk_size=64*1024)  # Larger chunks for fast networks

# Adjust timeouts
peer = Peer("my_peer")
await peer.start(listen_port=8080)

# Set aggressive discovery
await peer.beequiet.send_who_is_here()  # Manual discovery broadcast

# Monitor performance
def track_progress(progress):
    if progress % 10 == 0:  # Log every 10%
        print(f"Transfer progress: {progress}%")

peer.set_transfer_progress_callback(transfer_id, track_progress)
```

## Error Handling

### Exception Hierarchy

All beenet exceptions inherit from `beenet.BeenetError`:

```python
try:
    await peer.connect_to_peer("unreachable_peer", "192.168.1.999:8080")
except beenet.NetworkError as e:
    print(f"Network error: {e}")
except beenet.ProtocolError as e:
    print(f"Protocol error: {e}")
except beenet.BeenetError as e:
    print(f"General beenet error: {e}")
```

**Specific Exception Types:**
- `CryptoError`: Cryptographic failures
- `NetworkError`: Connection/communication issues
- `ProtocolError`: Protocol violations
- `DiscoveryError`: Peer discovery problems
- `TransferError`: File transfer issues
- `KeyStoreError`: Key storage problems
- `ConfigurationError`: Invalid configuration
- `ValidationError`: Data validation failures

### Best Practices

1. **Always use async context management:**
```python
async def managed_peer():
    peer = beenet.Peer("my_peer")
    try:
        await peer.start()
        # Use peer
    finally:
        await peer.stop()
```

2. **Handle transfer failures gracefully:**
```python
try:
    transfer_id = await peer.send_file("other_peer", file_path)
    # Monitor transfer with callbacks
except beenet.TransferError as e:
    print(f"Transfer failed: {e}")
    # Implement retry logic
```

3. **Validate peer connections:**
```python
if await peer.connect_to_peer("target", address):
    print("Connected successfully")
else:
    print("Connection failed - peer may be offline")
```

## Advanced Usage

### Custom Discovery

```python
# Implement custom peer discovery callback
def custom_peer_handler(peer_info):
    # Custom logic for discovered peers
    if peer_info['peer_id'].startswith('trusted_'):
        # Auto-connect to trusted peers
        asyncio.create_task(peer.connect_to_peer(peer_info['peer_id']))

beequiet = BeeQuietDiscovery("my_peer", on_peer_discovered=custom_peer_handler)
```

### Resumable Transfers

```python
# Save transfer state periodically
async def monitored_transfer(peer, target_peer, file_path):
    transfer_id = await peer.send_file(target_peer, file_path)
    
    # Set up periodic state saving
    async def save_state():
        while transfer_id in peer._transfers:
            await asyncio.sleep(10)  # Save every 10 seconds
            state_file = Path(f"{transfer_id}.state")
            await peer._transfers[transfer_id].save_state(state_file)
    
    asyncio.create_task(save_state())
    return transfer_id

# Resume interrupted transfer
async def resume_transfer(transfer_id, state_file):
    if state_file.exists():
        stream = TransferStream(transfer_id)
        await stream.resume_transfer(state_file)
        # Continue transfer from where it left off
```

### Multi-Peer Broadcasting

```python
# Send file to multiple peers
async def broadcast_file(peer, file_path, target_peers):
    transfers = []
    
    for target_peer in target_peers:
        if await peer.connect_to_peer(target_peer):
            transfer_id = await peer.send_file(target_peer, file_path)
            transfers.append(transfer_id)
    
    return transfers
```

This completes the comprehensive beenet API documentation. The library provides a complete toolkit for secure P2P networking with strong cryptographic guarantees and flexible configuration options.