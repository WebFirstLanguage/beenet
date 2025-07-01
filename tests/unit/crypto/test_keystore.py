"""Unit tests for KeyStore functionality."""

from pathlib import Path

import pytest

from beenet.core.errors import KeyStoreError
from beenet.crypto import KeyStore


class TestKeyStore:
    """Test KeyStore functionality."""

    @pytest.mark.asyncio
    async def test_keystore_creation(self, temp_dir: Path):
        """Test keystore creation and initialization."""
        keystore_path = temp_dir / "test_keystore"
        keystore = KeyStore(keystore_path)

        assert keystore.keystore_path == keystore_path
        assert not keystore.is_open

        await keystore.close()

    @pytest.mark.asyncio
    async def test_keystore_open_close(self, temp_dir: Path):
        """Test keystore opening and closing."""
        keystore_path = temp_dir / "test_keystore"
        keystore = KeyStore(keystore_path)

        await keystore.open()
        assert keystore.is_open

        await keystore.close()
        assert not keystore.is_open

    @pytest.mark.asyncio
    async def test_store_and_retrieve_key(self, keystore: KeyStore):
        """Test storing and retrieving keys."""
        test_key = b"test_key_data_32_bytes_exactly!!"
        key_id = "test_key_001"

        await keystore.store_key(key_id, test_key)
        retrieved_key = await keystore.get_key(key_id)

        assert retrieved_key == test_key

    @pytest.mark.asyncio
    async def test_key_not_found(self, keystore: KeyStore):
        """Test retrieving non-existent key."""
        retrieved_key = await keystore.get_key("non_existent_key")
        assert retrieved_key is None

    @pytest.mark.asyncio
    async def test_delete_key(self, keystore: KeyStore):
        """Test key deletion."""
        test_key = b"test_key_data_32_bytes_exactly!!"
        key_id = "test_key_to_delete"

        await keystore.store_key(key_id, test_key)
        assert await keystore.get_key(key_id) == test_key

        await keystore.delete_key(key_id)
        assert await keystore.get_key(key_id) is None

    @pytest.mark.asyncio
    async def test_list_keys(self, keystore: KeyStore):
        """Test listing stored keys."""
        key_ids = ["key1", "key2", "key3"]
        test_key = b"test_key_data_32_bytes_exactly!!"

        for key_id in key_ids:
            await keystore.store_key(key_id, test_key)

        stored_keys = await keystore.list_keys()
        for key_id in key_ids:
            assert key_id in stored_keys

    @pytest.mark.asyncio
    async def test_keystore_persistence(self, temp_dir: Path):
        """Test that keys persist across keystore sessions."""
        keystore_path = temp_dir / "persistent_keystore"
        test_key = b"persistent_key_32_bytes_exactly!"
        key_id = "persistent_key"

        keystore1 = KeyStore(keystore_path)
        await keystore1.open()
        await keystore1.store_key(key_id, test_key)
        await keystore1.close()

        keystore2 = KeyStore(keystore_path)
        await keystore2.open()
        retrieved_key = await keystore2.get_key(key_id)
        await keystore2.close()

        assert retrieved_key == test_key

    @pytest.mark.asyncio
    async def test_keystore_error_handling(self, temp_dir: Path):
        """Test keystore error handling."""
        keystore_path = temp_dir / "error_keystore"
        keystore = KeyStore(keystore_path)

        try:
            with pytest.raises(KeyStoreError):
                await keystore.store_key("test_key", b"test_data")

            with pytest.raises(KeyStoreError):
                await keystore.get_key("test_key")
        finally:
            if keystore.is_open:
                await keystore.close()

    @pytest.mark.asyncio
    async def test_key_rotation(self, keystore: KeyStore):
        """Test key rotation functionality."""
        old_key = b"old_key_data_32_bytes_exactly!!!"
        new_key = b"new_key_data_32_bytes_exactly!!!"
        key_id = "rotating_key"

        await keystore.store_key(key_id, old_key)
        await keystore.rotate_key(key_id, new_key)

        retrieved_key = await keystore.get_key(key_id)
        assert retrieved_key == new_key
        assert retrieved_key != old_key

    @pytest.mark.asyncio
    async def test_secure_deletion(self, keystore: KeyStore):
        """Test secure key deletion."""
        sensitive_key = b"sensitive_key_32_bytes_exactly!"
        key_id = "sensitive_key"

        await keystore.store_key(key_id, sensitive_key)
        await keystore.secure_delete(key_id)

        assert await keystore.get_key(key_id) is None
