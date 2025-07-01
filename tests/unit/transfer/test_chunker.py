"""Unit tests for DataChunker functionality."""

import pytest

from beenet.transfer import DataChunker


class TestDataChunker:
    """Test DataChunker functionality."""

    def test_chunker_creation(self):
        """Test chunker creation with default settings."""
        chunker = DataChunker()

        assert chunker.chunk_size == DataChunker.DEFAULT_CHUNK_SIZE
        assert chunker.chunk_size == 16 * 1024

    def test_chunker_with_custom_size(self):
        """Test chunker creation with custom chunk size."""
        chunk_size = 32 * 1024
        chunker = DataChunker(chunk_size)

        assert chunker.chunk_size == chunk_size

    def test_invalid_chunk_size(self):
        """Test chunker creation with invalid chunk size."""
        with pytest.raises(ValueError):
            DataChunker(1024)  # Too small

        with pytest.raises(ValueError):
            DataChunker(128 * 1024)  # Too large

    @pytest.mark.asyncio
    async def test_chunk_size_negotiation(self):
        """Test chunk size negotiation."""
        chunker = DataChunker()

        proposed_size = 32 * 1024
        peer_max_size = 48 * 1024

        agreed_size = await chunker.negotiate_chunk_size(proposed_size, peer_max_size)

        assert agreed_size == proposed_size
        assert chunker.chunk_size == agreed_size

    @pytest.mark.asyncio
    async def test_chunk_size_negotiation_peer_limit(self):
        """Test chunk size negotiation with peer limit."""
        chunker = DataChunker()

        proposed_size = 48 * 1024
        peer_max_size = 32 * 1024

        agreed_size = await chunker.negotiate_chunk_size(proposed_size, peer_max_size)

        assert agreed_size == peer_max_size
        assert chunker.chunk_size == agreed_size

    @pytest.mark.asyncio
    async def test_chunk_size_negotiation_invalid_inputs(self):
        """Test chunk size negotiation with invalid inputs."""
        chunker = DataChunker()

        agreed_size = await chunker.negotiate_chunk_size(1024, 128 * 1024)

        assert DataChunker.MIN_CHUNK_SIZE <= agreed_size <= DataChunker.MAX_CHUNK_SIZE

    def test_chunk_data_small(self):
        """Test chunking small data."""
        chunker = DataChunker(4096)
        data = b"small test data"

        chunks = list(chunker.chunk_data(data))

        assert len(chunks) == 1
        assert chunks[0] == (0, data)

    def test_chunk_data_exact_size(self):
        """Test chunking data that exactly fits chunk size."""
        chunk_size = 4096
        chunker = DataChunker(chunk_size)
        data = b"x" * chunk_size

        chunks = list(chunker.chunk_data(data))

        assert len(chunks) == 1
        assert chunks[0] == (0, data)

    def test_chunk_data_multiple_chunks(self):
        """Test chunking data into multiple chunks."""
        chunk_size = 4096
        chunker = DataChunker(chunk_size)
        data = b"x" * (chunk_size * 2 + 500)  # 2.5 chunks worth of data

        chunks = list(chunker.chunk_data(data))

        assert len(chunks) == 3
        assert chunks[0] == (0, b"x" * chunk_size)
        assert chunks[1] == (1, b"x" * chunk_size)
        assert chunks[2] == (2, b"x" * 500)

    def test_chunk_file(self, test_file):
        """Test chunking data from file."""
        chunker = DataChunker(4096)

        chunks = list(chunker.chunk_file(str(test_file)))

        assert len(chunks) > 0
        assert all(isinstance(chunk_index, int) for chunk_index, _ in chunks)
        assert all(isinstance(chunk_data, bytes) for _, chunk_data in chunks)

    @pytest.mark.asyncio
    async def test_chunk_stream(self):
        """Test chunking data from async stream."""
        chunker = DataChunker(4096)

        async def data_stream():
            yield b"first"
            yield b"second"
            yield b"third"
            yield b"fourth"

        chunks = []
        async for chunk in chunker.chunk_stream(data_stream()):
            chunks.append(chunk)

        assert len(chunks) >= 1
        assert all(isinstance(chunk_index, int) for chunk_index, _ in chunks)
        assert all(isinstance(chunk_data, bytes) for _, chunk_data in chunks)

    def test_reassemble_chunks(self):
        """Test reassembling chunks back to original data."""
        chunker = DataChunker(4096)
        original_data = b"this is test data for reassembly" * 200  # Make it larger

        chunks = list(chunker.chunk_data(original_data))
        reassembled = chunker.reassemble_chunks(iter(chunks))

        assert reassembled == original_data

    def test_reassemble_chunks_out_of_order(self):
        """Test reassembling chunks provided out of order."""
        chunker = DataChunker(4096)
        original_data = b"x" * (4096 * 4)  # 4 chunks worth

        chunks = list(chunker.chunk_data(original_data))
        if len(chunks) >= 4:
            shuffled_chunks = [chunks[2], chunks[0], chunks[3], chunks[1]]
        else:
            shuffled_chunks = chunks[::-1]  # Just reverse if fewer chunks

        reassembled = chunker.reassemble_chunks(iter(shuffled_chunks))

        assert reassembled == original_data

    def test_reassemble_chunks_missing_chunk(self):
        """Test reassembling with missing chunk."""
        chunker = DataChunker(4096)
        original_data = b"x" * (4096 * 4)  # 4 chunks worth

        chunks = list(chunker.chunk_data(original_data))
        incomplete_chunks = [chunks[0], chunks[1], chunks[3]]  # Missing chunk 2

        with pytest.raises(ValueError):
            chunker.reassemble_chunks(iter(incomplete_chunks))

    def test_calculate_chunk_count(self):
        """Test calculating chunk count."""
        assert DataChunker.calculate_chunk_count(1000, 100) == 10
        assert DataChunker.calculate_chunk_count(1001, 100) == 11
        assert DataChunker.calculate_chunk_count(99, 100) == 1
        assert DataChunker.calculate_chunk_count(0, 100) == 0

    def test_empty_data_chunking(self):
        """Test chunking empty data."""
        chunker = DataChunker()
        data = b""

        chunks = list(chunker.chunk_data(data))

        assert len(chunks) == 0

    def test_large_data_chunking(self, large_sample_data):
        """Test chunking large data."""
        chunker = DataChunker()

        chunks = list(chunker.chunk_data(large_sample_data))

        expected_chunks = DataChunker.calculate_chunk_count(
            len(large_sample_data), chunker.chunk_size
        )
        assert len(chunks) == expected_chunks

        reassembled = chunker.reassemble_chunks(iter(chunks))
        assert reassembled == large_sample_data
