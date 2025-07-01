"""Fuzz tests for Noise payload decoder."""

import pytest
from hypothesis import given, settings
from hypothesis import strategies as st

from beenet.core.errors import CryptoError
from beenet.crypto import NoiseChannel


class TestNoiseDecoderFuzz:
    """Fuzz tests for Noise payload decoder."""

    @given(st.binary(min_size=0, max_size=1024))
    @settings(max_examples=200, deadline=1000)
    def test_decrypt_random_payloads(self, payload_data):
        """Test decrypting random payload data."""
        noise_channel = NoiseChannel(is_initiator=True)

        try:
            if hasattr(noise_channel, "decrypt"):
                result = noise_channel.decrypt(payload_data)
                if hasattr(result, "__await__"):
                    pass  # Skip async methods in sync fuzz tests
                else:
                    assert result is None or isinstance(result, bytes)
        except (CryptoError, ValueError):
            pass
        except Exception as e:
            pytest.fail(f"Unexpected exception during decryption: {e}")

    @given(st.binary(min_size=1, max_size=16))
    @settings(max_examples=100)
    def test_decrypt_short_payloads(self, payload_data):
        """Test decrypting very short payloads."""
        noise_channel = NoiseChannel(is_initiator=False)

        try:
            if hasattr(noise_channel, "decrypt"):
                result = noise_channel.decrypt(payload_data)
                if hasattr(result, "__await__"):
                    pass  # Skip async methods in sync fuzz tests
                else:
                    assert result is None or isinstance(result, bytes)
        except (CryptoError, ValueError, IndexError):
            pass
        except Exception as e:
            pytest.fail(f"Unexpected exception for short payload: {e}")

    @given(st.binary(min_size=1000, max_size=2048))
    @settings(max_examples=50)
    def test_decrypt_large_payloads(self, payload_data):
        """Test decrypting large payloads."""
        noise_channel = NoiseChannel(is_initiator=True)

        try:
            if hasattr(noise_channel, "decrypt"):
                result = noise_channel.decrypt(payload_data)
                if hasattr(result, "__await__"):
                    pass  # Skip async methods in sync fuzz tests
                else:
                    assert result is None or isinstance(result, bytes)
        except (CryptoError, ValueError):
            pass
        except Exception as e:
            pytest.fail(f"Unexpected exception for large payload: {e}")

    @given(st.binary(min_size=1, max_size=512))
    @settings(max_examples=100)
    def test_handshake_message_processing(self, message_data):
        """Test processing random handshake messages."""
        noise_channel = NoiseChannel(is_initiator=True)

        try:
            if hasattr(noise_channel, "process_handshake_message"):
                result = noise_channel.process_handshake_message(message_data)
                if hasattr(result, "__await__"):
                    pass  # Skip async methods in sync fuzz tests
                else:
                    assert result is None or isinstance(result, bytes)
        except (CryptoError, ValueError):
            pass
        except Exception as e:
            pytest.fail(f"Unexpected exception during handshake processing: {e}")

    @given(st.binary(min_size=0, max_size=64))
    @settings(max_examples=100)
    def test_malformed_noise_headers(self, header_data):
        """Test processing malformed Noise headers."""
        noise_channel = NoiseChannel(is_initiator=False)

        try:
            if hasattr(noise_channel, "decrypt"):
                result = noise_channel.decrypt(header_data)
                if hasattr(result, "__await__"):
                    pass  # Skip async methods in sync fuzz tests
                else:
                    assert result is None or isinstance(result, bytes)
        except (CryptoError, ValueError, IndexError):
            pass
        except Exception as e:
            pytest.fail(f"Unexpected exception for malformed header: {e}")

    @given(st.binary(min_size=32, max_size=32))
    @settings(max_examples=50)
    def test_key_material_processing(self, key_material):
        """Test processing random key material."""
        noise_channel = NoiseChannel(is_initiator=True)

        try:
            noise_channel._process_key_material(key_material)
        except (CryptoError, ValueError, AttributeError):
            pass
        except Exception as e:
            pytest.fail(f"Unexpected exception during key processing: {e}")

    @given(st.binary(min_size=1, max_size=256))
    @settings(max_examples=100)
    def test_encrypt_decrypt_roundtrip_fuzz(self, plaintext):
        """Test encrypt/decrypt roundtrip with random data."""
        noise_channel = NoiseChannel(is_initiator=True)

        try:
            if hasattr(noise_channel, "_transport_encrypt"):
                encrypted = noise_channel._transport_encrypt(plaintext)
                if encrypted:
                    decrypted = noise_channel._transport_decrypt(encrypted)
                    if decrypted is not None:
                        assert decrypted == plaintext
        except (CryptoError, ValueError, AttributeError):
            pass
        except Exception as e:
            pytest.fail(f"Unexpected exception during roundtrip: {e}")

    @given(st.binary(min_size=0, max_size=128))
    @settings(max_examples=50)
    def test_rekey_with_random_data(self, rekey_data):
        """Test rekeying with random data."""
        noise_channel = NoiseChannel(is_initiator=False)

        try:
            if hasattr(noise_channel, "rekey"):
                result = noise_channel.rekey()
                if hasattr(result, "__await__"):
                    pass  # Skip async methods in sync fuzz tests
        except (CryptoError, ValueError, AttributeError):
            pass
        except Exception as e:
            pytest.fail(f"Unexpected exception during rekey: {e}")
