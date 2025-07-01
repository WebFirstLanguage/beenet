"""beenet: A P2P networking library with Noise XX secure channels and Merkle tree data transfer.

This library provides secure peer-to-peer networking capabilities with:
- Noise XX protocol for secure channels with mutual authentication
- Hybrid discovery using Kademlia DHT and BeeQuiet LAN protocol  
- Merkle tree-based data transfer with integrity verification
- Ed25519 identity keys and rotating session keys
"""

__version__ = "0.1.0"
__author__ = "beenet contributors"
__license__ = "Apache-2.0"

from .core.errors import BeenetError
from .core.peer import Peer

__all__ = ["Peer", "BeenetError"]
