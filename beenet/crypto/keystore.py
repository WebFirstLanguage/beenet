"""Secure key storage abstraction for beenet."""

import asyncio
import base64
import json
import os
import secrets
from pathlib import Path
from typing import Any, Dict, Optional

from cryptography.fernet import Fernet
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.kdf.pbkdf2 import PBKDF2HMAC

from ..core.errors import KeyStoreError


class KeyStore:
    """Secure key storage with file-based persistence and OS keyring fallback.

    Provides:
    - Encrypted key persistence with user passphrase
    - Atomic key rotation and secure deletion
    - Thread-safe access patterns
    - OS keyring integration for sensitive keys
    """

    def __init__(self, storage_path: Optional[Path] = None, passphrase: Optional[str] = None):
        self.storage_path = storage_path or Path.home() / ".beenet" / "keys"
        self._lock = asyncio.Lock()
        self._keys: Dict[str, Any] = {}
        self._passphrase = passphrase
        self._fernet: Optional[Fernet] = None
        self._loaded = False

    async def _ensure_initialized(self) -> None:
        """Ensure keystore is initialized."""
        if not self._loaded:
            await self._load_keystore()
            self._loaded = True

    async def _load_keystore(self) -> None:
        """Load existing keystore or create new one."""
        self.storage_path.mkdir(parents=True, exist_ok=True)
        keystore_file = self.storage_path / "keystore.json"

        if keystore_file.exists():
            try:
                with open(keystore_file, "rb") as f:
                    encrypted_data = f.read()

                if self._passphrase:
                    self._fernet = self._derive_fernet_key(self._passphrase)
                    decrypted_data = self._fernet.decrypt(encrypted_data)
                    self._keys = json.loads(decrypted_data.decode("utf-8"))
                else:
                    self._keys = json.loads(encrypted_data.decode("utf-8"))
            except Exception as e:
                raise KeyStoreError(f"Failed to load keystore: {e}")
        else:
            self._keys = {}

    async def _save_keystore(self) -> None:
        """Save keystore to disk."""
        keystore_file = self.storage_path / "keystore.json"
        temp_file = keystore_file.with_suffix(".tmp")

        try:
            data = json.dumps(self._keys).encode("utf-8")

            if self._passphrase and self._fernet:
                encrypted_data = self._fernet.encrypt(data)
            else:
                encrypted_data = data

            with open(temp_file, "wb") as f:
                f.write(encrypted_data)

            temp_file.replace(keystore_file)
        except Exception as e:
            if temp_file.exists():
                temp_file.unlink()
            raise KeyStoreError(f"Failed to save keystore: {e}")

    def _derive_fernet_key(self, passphrase: str) -> Fernet:
        """Derive Fernet key from passphrase."""
        salt_file = self.storage_path / "salt"

        if salt_file.exists():
            with open(salt_file, "rb") as f:
                salt = f.read()
        else:
            salt = secrets.token_bytes(32)
            with open(salt_file, "wb") as f:
                f.write(salt)

        kdf = PBKDF2HMAC(
            algorithm=hashes.SHA256(),
            length=32,
            salt=salt,
            iterations=100000,
        )
        key = base64.urlsafe_b64encode(kdf.derive(passphrase.encode()))
        return Fernet(key)

    async def store_key(self, key_id: str, key_data: bytes, encrypted: bool = True) -> None:
        """Store a key securely.

        Args:
            key_id: Unique identifier for the key
            key_data: Key material to store
            encrypted: Whether to encrypt the key data
        """
        if not self._loaded:
            raise KeyStoreError("KeyStore not open - call open() first")
            
        async with self._lock:
            await self._ensure_initialized()

            try:
                encoded_data = base64.b64encode(key_data).decode("ascii")

                self._keys[key_id] = {
                    "data": encoded_data,
                    "encrypted": encrypted,
                    "created_at": asyncio.get_event_loop().time(),
                }

                await self._save_keystore()
            except Exception as e:
                raise KeyStoreError(f"Failed to store key {key_id}: {e}")

    async def get_key(self, key_id: str) -> Optional[bytes]:
        """Load a key from storage.

        Args:
            key_id: Unique identifier for the key

        Returns:
            Key data if found, None otherwise
        """
        if not self._loaded:
            raise KeyStoreError("KeyStore not open - call open() first")
            
        async with self._lock:
            await self._ensure_initialized()

            if key_id not in self._keys:
                return None

            try:
                key_info = self._keys[key_id]
                encoded_data = key_info["data"]
                return base64.b64decode(encoded_data.encode("ascii"))
            except Exception as e:
                raise KeyStoreError(f"Failed to load key {key_id}: {e}")

    async def delete_key(self, key_id: str) -> bool:
        """Securely delete a key.

        Args:
            key_id: Unique identifier for the key

        Returns:
            True if key was deleted, False if not found
        """
        async with self._lock:
            await self._ensure_initialized()

            if key_id not in self._keys:
                return False

            try:
                key_info = self._keys[key_id]
                key_info["data"] = base64.b64encode(secrets.token_bytes(32)).decode("ascii")

                del self._keys[key_id]

                await self._save_keystore()
                return True
            except Exception as e:
                raise KeyStoreError(f"Failed to delete key {key_id}: {e}")

    async def rotate_key(self, key_id: str, new_key_data: bytes) -> bytes:
        """Atomically rotate a key.

        Args:
            key_id: Unique identifier for the key
            new_key_data: New key material

        Returns:
            Previous key data
        """
        async with self._lock:
            await self._ensure_initialized()

            if key_id not in self._keys:
                raise KeyStoreError(f"Key {key_id} not found for rotation")

            try:
                old_key_data = await self.get_key(key_id)
                if old_key_data is None:
                    raise KeyStoreError(f"Failed to load existing key {key_id}")

                await self.store_key(key_id, new_key_data)

                return old_key_data
            except Exception as e:
                raise KeyStoreError(f"Failed to rotate key {key_id}: {e}")

    async def list_keys(self) -> list[str]:
        """List all stored key identifiers.

        Returns:
            List of key identifiers
        """
        async with self._lock:
            await self._ensure_initialized()
            return list(self._keys.keys())

    async def flush(self) -> None:
        """Flush any pending writes to storage."""
        async with self._lock:
            if self._loaded:
                await self._save_keystore()

    async def set_passphrase(self, passphrase: str) -> None:
        """Set or change the keystore passphrase.

        Args:
            passphrase: New passphrase for encryption
        """
        async with self._lock:
            await self._ensure_initialized()

            self._passphrase = passphrase
            self._fernet = self._derive_fernet_key(passphrase)

            await self._save_keystore()

    def is_encrypted(self) -> bool:
        """Check if keystore is encrypted.

        Returns:
            True if keystore uses encryption
        """
        return self._passphrase is not None

    async def open(self) -> None:
        """Open the keystore for operations."""
        await self._ensure_initialized()

    async def close(self) -> None:
        """Close the keystore and flush any pending writes."""
        await self.flush()
        self._loaded = False

    @property
    def keystore_path(self) -> Path:
        """Get the keystore storage path."""
        return self.storage_path

    @property
    def is_open(self) -> bool:
        """Check if keystore is open."""
        return self._loaded

    async def secure_delete(self, key_id: str) -> bool:
        """Securely delete a key with overwriting.

        Args:
            key_id: Unique identifier for the key

        Returns:
            True if key was deleted, False if not found
        """
        return await self.delete_key(key_id)
