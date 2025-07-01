"""Typed error hierarchy for robust error handling in beenet."""

from typing import Any, Optional


class BeenetError(Exception):
    """Base exception for all beenet errors.

    All beenet-specific exceptions inherit from this class to enable
    easy error handling and categorization.
    """

    def __init__(
        self,
        message: str,
        error_code: Optional[str] = None,
        details: Optional[dict[str, Any]] = None,
    ):
        super().__init__(message)
        self.message = message
        self.error_code = error_code
        self.details = details or {}


class CryptoError(BeenetError):
    """Cryptographic operation errors.

    Raised for unrecoverable cryptographic failures such as:
    - Key generation failures
    - Encryption/decryption errors
    - Signature verification failures
    - Noise handshake failures
    """

    pass


class NetworkError(BeenetError):
    """Network communication errors.

    Raised for recoverable network issues such as:
    - Connection failures
    - Socket errors
    - Timeout errors
    - DNS resolution failures
    """

    pass


class ProtocolError(BeenetError):
    """Protocol violation errors.

    Raised for malformed messages or protocol violations such as:
    - Invalid message format
    - Unexpected message type
    - Protocol state violations
    - Version mismatches
    """

    pass


class DiscoveryError(BeenetError):
    """Peer discovery errors.

    Raised for discovery-related issues such as:
    - DHT bootstrap failures
    - BeeQuiet protocol errors
    - Peer lookup failures
    """

    pass


class TransferError(BeenetError):
    """Data transfer errors.

    Raised for transfer-related issues such as:
    - Merkle verification failures
    - Chunk corruption
    - Transfer timeout
    - Resume failures
    """

    pass


class KeyStoreError(BeenetError):
    """Key storage and management errors.

    Raised for keystore-related issues such as:
    - Key storage failures
    - Decryption failures
    - Key rotation errors
    - Access permission errors
    """

    pass


class ConfigurationError(BeenetError):
    """Configuration and setup errors.

    Raised for configuration issues such as:
    - Invalid configuration values
    - Missing required settings
    - Incompatible options
    """

    pass


class ValidationError(BeenetError):
    """Data validation errors.

    Raised for data validation failures such as:
    - Invalid peer IDs
    - Malformed addresses
    - Invalid chunk sizes
    - Parameter validation failures
    """

    pass
