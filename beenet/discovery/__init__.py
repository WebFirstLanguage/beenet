"""Discovery layer for beenet peers.

This module provides:
- Kademlia DHT for global peer discovery
- BeeQuiet protocol for LAN peer discovery with AEAD security
- NAT traversal capabilities (feature-flagged)
"""

from .beequiet import BeeQuietDiscovery
from .kademlia import KademliaDiscovery
from .nat_traversal import NATTraversal

__all__ = ["KademliaDiscovery", "BeeQuietDiscovery", "NATTraversal"]
