# beenet Architecture

## System Overview

beenet is designed as a modular P2P networking library with several key components that work together to provide secure, reliable peer-to-peer communication. The architecture follows an event-driven, async-first approach using Python's asyncio framework.

## Core Components

### Peer (beenet/core/peer.py)

The `Peer` class is the main entry point for the library, integrating all components:
- Manages peer identity and lifecycle
- Coordinates discovery, connection, and transfer operations
- Provides a high-level API for applications
- Handles event dispatching and resilience

### Crypto (beenet/crypto/)

- **Identity** (identity.py): Manages Ed25519 identity keys for peer authentication
- **KeyStore** (keystore.py): Securely stores and manages cryptographic keys
- **KeyManager** (keys.py): Handles key rotation and management
- **NoiseChannel** (noise_wrapper.py): Implements the Noise XX protocol for secure channels using ChaCha20-Poly1305 and BLAKE2b

### Discovery (beenet/discovery/)

- **KademliaDiscovery** (kademlia.py): Global peer discovery using Kademlia DHT
- **BeeQuietDiscovery** (beequiet.py): Local network discovery using UDP multicast with AEAD security
- **NATTraversal** (nat_traversal.py): Handles NAT traversal using STUN and ICE

### Transfer (beenet/transfer/)

- **DataChunker** (chunker.py): Splits data into chunks with adaptive sizing and flow control
- **MerkleTree** (merkle.py): Provides cryptographic verification of data integrity
- **TransferStream** (stream.py): Manages resumable file transfers with state persistence
- **EnhancedMerkle** (enhanced_merkle.py): Extended Merkle tree implementation with additional features
- **ErrorCorrection** (error_correction.py): Implements Reed-Solomon error correction for transfers

### Core Infrastructure (beenet/core/)

- **ConnectionManager** (connection.py): Manages peer connections and connection state
- **EventBus** (events.py): Event-driven architecture for extensibility
- **PeerResilienceManager** (resilience.py): Handles reconnection and failure recovery
- **Errors** (errors.py): Custom exception hierarchy

### Observability (beenet/observability.py)

- Logging with structlog
- Metrics with Prometheus
- Performance monitoring

## Key Implementation Paths

### Secure Connection Establishment

1. Peer initialization (peer.py)
2. Identity loading/generation (identity.py)
3. Noise XX handshake (noise_wrapper.py)
4. Secure channel establishment (connection.py)

### Peer Discovery Flow

1. Peer registration in Kademlia DHT (kademlia.py)
2. Local network announcement via BeeQuiet (beequiet.py)
3. NAT traversal for connectivity (nat_traversal.py)
4. Connection establishment (connection.py)

### File Transfer Flow

1. File chunking (chunker.py)
2. Merkle tree generation (merkle.py)
3. Chunk transfer with flow control (chunker.py)
4. Integrity verification (merkle.py)
5. State persistence for resumability (stream.py)

## Design Patterns

- **Event-driven architecture**: Components communicate via events for loose coupling
- **Async-first design**: All operations are async for scalability
- **Modular components**: Clear separation of concerns
- **Cryptographic verification**: All data is verified for integrity
- **Resilience by design**: Automatic recovery from failures
- **Observability integration**: Comprehensive logging and metrics

## Source Code Paths

- `/beenet/core/`: Core peer implementation and infrastructure
- `/beenet/crypto/`: Cryptographic primitives and secure channels
- `/beenet/discovery/`: Peer discovery mechanisms
- `/beenet/transfer/`: Data transfer and integrity verification
- `/docs/`: Documentation
- `/tests/`: Test suite (unit, integration, fuzz, property)
- `/scripts/`: Demo and utility scripts