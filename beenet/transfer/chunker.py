"""Data chunking with size negotiation for efficient transfer."""

from io import BytesIO
from typing import AsyncIterator, Iterator, Tuple


class DataChunker:
    """Data chunking with negotiable chunk sizes.

    Provides:
    - Default 16 KiB chunks with negotiation up to 64 KiB
    - Efficient streaming chunking for large files
    - Chunk size negotiation protocol
    - Memory-efficient chunk iteration
    """

    DEFAULT_CHUNK_SIZE = 16 * 1024  # 16 KiB
    MAX_CHUNK_SIZE = 64 * 1024  # 64 KiB
    MIN_CHUNK_SIZE = 4 * 1024  # 4 KiB

    def __init__(self, chunk_size: int = DEFAULT_CHUNK_SIZE):
        if not (self.MIN_CHUNK_SIZE <= chunk_size <= self.MAX_CHUNK_SIZE):
            raise ValueError(
                f"Chunk size must be between {self.MIN_CHUNK_SIZE} and {self.MAX_CHUNK_SIZE}"
            )
        self.chunk_size = chunk_size

    async def negotiate_chunk_size(self, proposed_size: int, peer_max_size: int) -> int:
        """Negotiate chunk size with peer.

        Args:
            proposed_size: Our proposed chunk size
            peer_max_size: Peer's maximum supported chunk size

        Returns:
            Agreed chunk size
        """
        if not (self.MIN_CHUNK_SIZE <= proposed_size <= self.MAX_CHUNK_SIZE):
            proposed_size = self.DEFAULT_CHUNK_SIZE

        if not (self.MIN_CHUNK_SIZE <= peer_max_size <= self.MAX_CHUNK_SIZE):
            peer_max_size = self.DEFAULT_CHUNK_SIZE

        agreed_size = min(proposed_size, peer_max_size)

        if agreed_size < self.MIN_CHUNK_SIZE:
            agreed_size = self.MIN_CHUNK_SIZE
        elif agreed_size > self.MAX_CHUNK_SIZE:
            agreed_size = self.MAX_CHUNK_SIZE

        self.chunk_size = agreed_size
        return agreed_size

    def chunk_data(self, data: bytes) -> Iterator[Tuple[int, bytes]]:
        """Split data into chunks.

        Args:
            data: Data to chunk

        Yields:
            Tuples of (chunk_index, chunk_data)
        """
        for i in range(0, len(data), self.chunk_size):
            chunk_index = i // self.chunk_size
            chunk_data = data[i : i + self.chunk_size]
            yield chunk_index, chunk_data

    async def chunk_stream(self, stream: AsyncIterator[bytes]) -> AsyncIterator[Tuple[int, bytes]]:
        """Chunk data from an async stream.

        Args:
            stream: Async iterator of data bytes

        Yields:
            Tuples of (chunk_index, chunk_data)
        """
        chunk_index = 0
        buffer = BytesIO()

        async for data in stream:
            buffer.write(data)

            while buffer.tell() >= self.chunk_size:
                buffer.seek(0)
                chunk_data = buffer.read(self.chunk_size)

                remaining_data = buffer.read()
                buffer = BytesIO()
                buffer.write(remaining_data)

                yield chunk_index, chunk_data
                chunk_index += 1

        if buffer.tell() > 0:
            buffer.seek(0)
            final_chunk = buffer.read()
            yield chunk_index, final_chunk

    def chunk_file(self, file_path: str) -> Iterator[Tuple[int, bytes]]:
        """Chunk data from a file.

        Args:
            file_path: Path to file to chunk

        Yields:
            Tuples of (chunk_index, chunk_data)
        """
        chunk_index = 0
        with open(file_path, "rb") as f:
            while True:
                chunk_data = f.read(self.chunk_size)
                if not chunk_data:
                    break
                yield chunk_index, chunk_data
                chunk_index += 1

    def reassemble_chunks(self, chunks: Iterator[Tuple[int, bytes]]) -> bytes:
        """Reassemble chunks into original data.

        Args:
            chunks: Iterator of (chunk_index, chunk_data) tuples

        Returns:
            Reassembled data
        """
        chunk_dict = {}
        max_index = -1

        for chunk_index, chunk_data in chunks:
            chunk_dict[chunk_index] = chunk_data
            max_index = max(max_index, chunk_index)

        if max_index == -1:
            return b""

        result = BytesIO()
        for i in range(max_index + 1):
            if i in chunk_dict:
                result.write(chunk_dict[i])
            else:
                raise ValueError(f"Missing chunk at index {i}")

        return result.getvalue()

    @classmethod
    def calculate_chunk_count(cls, data_size: int, chunk_size: int = DEFAULT_CHUNK_SIZE) -> int:
        """Calculate number of chunks for given data size.

        Args:
            data_size: Total size of data
            chunk_size: Size of each chunk

        Returns:
            Number of chunks needed
        """
        return (data_size + chunk_size - 1) // chunk_size
