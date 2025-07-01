"""Core beenet components.

This module provides:
- Main peer class for P2P networking
- Connection management and event handling
- Typed error hierarchy for robust error handling
- Event loop integration for async operations
"""

from .connection import ConnectionManager
from .errors import BeenetError, CryptoError, NetworkError, ProtocolError
from .events import EventBus
from .peer import Peer

__all__ = [
    "Peer",
    "ConnectionManager",
    "EventBus",
    "BeenetError",
    "CryptoError",
    "NetworkError",
    "ProtocolError",
]
