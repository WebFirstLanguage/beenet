"""Fuzz tests for BeeQuiet packet parser."""

import struct

import pytest
from hypothesis import given, settings
from hypothesis import strategies as st

from beenet.core.errors import ProtocolError
from beenet.discovery import BeeQuietDiscovery


class TestBeeQuietParserFuzz:
    """Fuzz tests for BeeQuiet packet parser."""

    @given(st.binary(min_size=0, max_size=1024))
    @settings(max_examples=200, deadline=1000)
    def test_parse_random_packets(self, packet_data):
        """Test parsing random packet data."""
        peer_id = "fuzz_peer"
        callback = lambda x: None
        discovery = BeeQuietDiscovery(peer_id, callback)

        try:
            discovery.parse_packet(packet_data, ("127.0.0.1", 8000))
        except (ProtocolError, ValueError, struct.error):
            pass
        except Exception as e:
            pytest.fail(f"Unexpected exception: {e}")

    @given(st.binary(min_size=1, max_size=4))
    @settings(max_examples=100)
    def test_parse_short_packets(self, packet_data):
        """Test parsing very short packets."""
        peer_id = "fuzz_peer"
        callback = lambda x: None
        discovery = BeeQuietDiscovery(peer_id, callback)

        try:
            discovery.parse_packet(packet_data, ("127.0.0.1", 8000))
        except (ProtocolError, ValueError, struct.error, IndexError):
            pass
        except Exception as e:
            pytest.fail(f"Unexpected exception for short packet: {e}")

    @given(st.binary(min_size=1000, max_size=2048))
    @settings(max_examples=50)
    def test_parse_large_packets(self, packet_data):
        """Test parsing large packets."""
        peer_id = "fuzz_peer"
        callback = lambda x: None
        discovery = BeeQuietDiscovery(peer_id, callback)

        try:
            discovery.parse_packet(packet_data, ("127.0.0.1", 8000))
        except (ProtocolError, ValueError, struct.error):
            pass
        except Exception as e:
            pytest.fail(f"Unexpected exception for large packet: {e}")

    @given(st.integers(min_value=0, max_value=0xFFFFFFFF))
    @settings(max_examples=100)
    def test_parse_packets_with_magic_numbers(self, magic_number):
        """Test parsing packets with various magic numbers."""
        import struct

        peer_id = "fuzz_peer"
        callback = lambda x: None
        discovery = BeeQuietDiscovery(peer_id, callback)

        packet_data = struct.pack(">I", magic_number) + b"random_data"

        try:
            discovery.parse_packet(packet_data, ("127.0.0.1", 8000))
        except (ProtocolError, ValueError, struct.error):
            pass
        except Exception as e:
            pytest.fail(f"Unexpected exception for magic {magic_number:08x}: {e}")

    @given(st.binary(min_size=8, max_size=64))
    @settings(max_examples=100)
    def test_parse_malformed_headers(self, header_data):
        """Test parsing packets with malformed headers."""
        peer_id = "fuzz_peer"
        callback = lambda x: None
        discovery = BeeQuietDiscovery(peer_id, callback)

        try:
            discovery.parse_packet(header_data, ("127.0.0.1", 8000))
        except (ProtocolError, ValueError, struct.error, IndexError):
            pass
        except Exception as e:
            pytest.fail(f"Unexpected exception for malformed header: {e}")

    @given(st.binary(min_size=0, max_size=16))
    @settings(max_examples=50)
    def test_decrypt_random_data(self, encrypted_data):
        """Test decrypting random data."""
        peer_id = "fuzz_peer"
        callback = lambda x: None
        discovery = BeeQuietDiscovery(peer_id, callback)

        session_key = b"session_key_32_bytes_exactly!!!"

        try:
            result = discovery.decrypt_message(encrypted_data, session_key)
            assert result is None or isinstance(result, bytes)
        except Exception as e:
            pytest.fail(f"Unexpected exception during decryption: {e}")

    @given(st.binary(min_size=32, max_size=32))
    @settings(max_examples=50)
    def test_session_key_derivation_fuzz(self, key_material):
        """Test session key derivation with random key material."""
        peer_id = "fuzz_peer"
        callback = lambda x: None
        discovery = BeeQuietDiscovery(peer_id, callback)

        nonce = key_material[:16]
        response = key_material[16:]

        try:
            session_key = discovery.derive_session_key(nonce, response)
            assert isinstance(session_key, bytes)
            assert len(session_key) == 32
        except Exception as e:
            pytest.fail(f"Unexpected exception during key derivation: {e}")
