"""Fuzz tests for AEAD payload processing in BeeQuiet."""

import pytest
from hypothesis import given, settings
from hypothesis import strategies as st

from beenet.core.errors import ProtocolError
from beenet.discovery import BeeQuietDiscovery


class TestAEADPayloadFuzz:
    """Fuzz tests for AEAD payload processing."""

    @given(st.binary(min_size=0, max_size=512), st.binary(min_size=32, max_size=32))
    @settings(max_examples=200, deadline=1000)
    def test_encrypt_decrypt_random_payloads(self, payload, session_key):
        """Test AEAD encryption/decryption with random payloads."""
        peer_id = "fuzz_peer"
        callback = lambda x: None
        discovery = BeeQuietDiscovery(peer_id, callback)

        try:
            encrypted = discovery.encrypt_message(payload, session_key)
            assert isinstance(encrypted, bytes)
            assert len(encrypted) >= len(payload)

            decrypted = discovery.decrypt_message(encrypted, session_key)
            if decrypted is not None:
                assert decrypted == payload
        except Exception as e:
            pytest.fail(f"Unexpected exception during AEAD processing: {e}")

    @given(st.binary(min_size=1, max_size=16))
    @settings(max_examples=100)
    def test_decrypt_short_ciphertexts(self, ciphertext):
        """Test decrypting very short ciphertexts."""
        peer_id = "fuzz_peer"
        callback = lambda x: None
        discovery = BeeQuietDiscovery(peer_id, callback)
        session_key = b"session_key_32_bytes_exactly!!!"

        try:
            result = discovery.decrypt_message(ciphertext, session_key)
            assert result is None or isinstance(result, bytes)
        except Exception as e:
            pytest.fail(f"Unexpected exception for short ciphertext: {e}")

    @given(st.binary(min_size=1000, max_size=2048))
    @settings(max_examples=50)
    def test_decrypt_large_ciphertexts(self, ciphertext):
        """Test decrypting large ciphertexts."""
        peer_id = "fuzz_peer"
        callback = lambda x: None
        discovery = BeeQuietDiscovery(peer_id, callback)
        session_key = b"session_key_32_bytes_exactly!!!"

        try:
            result = discovery.decrypt_message(ciphertext, session_key)
            assert result is None or isinstance(result, bytes)
        except Exception as e:
            pytest.fail(f"Unexpected exception for large ciphertext: {e}")

    @given(st.binary(min_size=0, max_size=64))
    @settings(max_examples=100)
    def test_decrypt_with_invalid_keys(self, invalid_key):
        """Test decryption with invalid keys."""
        peer_id = "fuzz_peer"
        callback = lambda x: None
        discovery = BeeQuietDiscovery(peer_id, callback)

        valid_payload = b"test message"
        valid_key = b"session_key_32_bytes_exactly!!!"

        try:
            encrypted = discovery.encrypt_message(valid_payload, valid_key)
            result = discovery.decrypt_message(encrypted, invalid_key)

            if len(invalid_key) != 32:
                assert result is None
        except Exception as e:
            pytest.fail(f"Unexpected exception with invalid key: {e}")

    @given(st.binary(min_size=12, max_size=128))
    @settings(max_examples=100)
    def test_malformed_aead_packets(self, malformed_packet):
        """Test processing malformed AEAD packets."""
        peer_id = "fuzz_peer"
        callback = lambda x: None
        discovery = BeeQuietDiscovery(peer_id, callback)
        session_key = b"session_key_32_bytes_exactly!!!"

        try:
            result = discovery.decrypt_message(malformed_packet, session_key)
            assert result is None or isinstance(result, bytes)
        except Exception as e:
            pytest.fail(f"Unexpected exception for malformed packet: {e}")

    @given(st.binary(min_size=16, max_size=16), st.binary(min_size=16, max_size=16))
    @settings(max_examples=100)
    def test_session_key_derivation_fuzz(self, nonce, response):
        """Test session key derivation with random inputs."""
        peer_id = "fuzz_peer"
        callback = lambda x: None
        discovery = BeeQuietDiscovery(peer_id, callback)

        try:
            session_key = discovery.derive_session_key(nonce, response)
            assert isinstance(session_key, bytes)
            assert len(session_key) == 32

            session_key2 = discovery.derive_session_key(nonce, response)
            assert session_key == session_key2  # Should be deterministic
        except Exception as e:
            pytest.fail(f"Unexpected exception during key derivation: {e}")

    @given(st.binary(min_size=1, max_size=256))
    @settings(max_examples=50)
    def test_encrypt_empty_and_large_messages(self, message):
        """Test encrypting messages of various sizes."""
        peer_id = "fuzz_peer"
        callback = lambda x: None
        discovery = BeeQuietDiscovery(peer_id, callback)
        session_key = b"session_key_32_bytes_exactly!!!"

        try:
            encrypted = discovery.encrypt_message(message, session_key)
            assert isinstance(encrypted, bytes)
            assert len(encrypted) > len(message)  # Should include nonce and tag

            decrypted = discovery.decrypt_message(encrypted, session_key)
            assert decrypted == message
        except Exception as e:
            pytest.fail(f"Unexpected exception for message size {len(message)}: {e}")

    @given(st.binary(min_size=32, max_size=32))
    @settings(max_examples=50)
    def test_key_rotation_scenarios(self, new_key):
        """Test key rotation with random keys."""
        peer_id = "fuzz_peer"
        callback = lambda x: None
        discovery = BeeQuietDiscovery(peer_id, callback)

        old_key = b"old_session_key_32_bytes_exactly!"
        message = b"test message for key rotation"

        try:
            encrypted_old = discovery.encrypt_message(message, old_key)
            decrypted_old = discovery.decrypt_message(encrypted_old, old_key)
            assert decrypted_old == message

            encrypted_new = discovery.encrypt_message(message, new_key)
            decrypted_new = discovery.decrypt_message(encrypted_new, new_key)
            assert decrypted_new == message

            cross_decrypt = discovery.decrypt_message(encrypted_old, new_key)
            assert cross_decrypt is None  # Should fail with wrong key
        except Exception as e:
            pytest.fail(f"Unexpected exception during key rotation: {e}")
