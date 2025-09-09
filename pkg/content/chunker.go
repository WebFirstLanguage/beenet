package content

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ChunkFile splits a file into chunks and returns the chunks with their CIDs
func ChunkFile(filePath string, chunkSize uint32) ([]*Chunk, error) {
	if chunkSize == 0 {
		return nil, fmt.Errorf("chunk size cannot be zero")
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	fileSize := fileInfo.Size()
	if fileSize < 0 {
		return nil, fmt.Errorf("invalid file size: %d", fileSize)
	}

	// Handle empty file
	if fileSize == 0 {
		return []*Chunk{}, nil
	}

	// Calculate number of chunks
	numChunks := (uint64(fileSize) + uint64(chunkSize) - 1) / uint64(chunkSize)
	chunks := make([]*Chunk, 0, numChunks)

	// Read and chunk the file
	buffer := make([]byte, chunkSize)
	var offset uint64 = 0

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("failed to read file at offset %d: %w", offset, err)
		}

		if n == 0 {
			break
		}

		// Create chunk data (only the bytes we actually read)
		chunkData := make([]byte, n)
		copy(chunkData, buffer[:n])

		// Generate CID for this chunk
		cid := GenerateChunkCID(chunkData)

		// Create chunk
		chunk := &Chunk{
			CID:    cid,
			Data:   chunkData,
			Size:   uint64(n),
			Offset: offset,
		}

		chunks = append(chunks, chunk)
		offset += uint64(n)

		if err == io.EOF {
			break
		}
	}

	return chunks, nil
}

// ChunkReader splits data from a reader into chunks
func ChunkReader(reader io.Reader, chunkSize uint32) ([]*Chunk, error) {
	if chunkSize == 0 {
		return nil, fmt.Errorf("chunk size cannot be zero")
	}

	var chunks []*Chunk
	buffer := make([]byte, chunkSize)
	var offset uint64 = 0

	for {
		n, err := reader.Read(buffer)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("failed to read data at offset %d: %w", offset, err)
		}

		if n == 0 {
			break
		}

		// Create chunk data (only the bytes we actually read)
		chunkData := make([]byte, n)
		copy(chunkData, buffer[:n])

		// Generate CID for this chunk
		cid := GenerateChunkCID(chunkData)

		// Create chunk
		chunk := &Chunk{
			CID:    cid,
			Data:   chunkData,
			Size:   uint64(n),
			Offset: offset,
		}

		chunks = append(chunks, chunk)
		offset += uint64(n)

		if err == io.EOF {
			break
		}
	}

	return chunks, nil
}

// ChunkData splits raw data into chunks
func ChunkData(data []byte, chunkSize uint32) ([]*Chunk, error) {
	if chunkSize == 0 {
		return nil, fmt.Errorf("chunk size cannot be zero")
	}

	// Handle empty data
	if len(data) == 0 {
		return []*Chunk{}, nil
	}

	// Calculate number of chunks
	numChunks := (len(data) + int(chunkSize) - 1) / int(chunkSize)
	chunks := make([]*Chunk, 0, numChunks)

	var offset uint64 = 0

	for i := 0; i < len(data); i += int(chunkSize) {
		end := i + int(chunkSize)
		if end > len(data) {
			end = len(data)
		}

		// Create chunk data
		chunkData := make([]byte, end-i)
		copy(chunkData, data[i:end])

		// Generate CID for this chunk
		cid := GenerateChunkCID(chunkData)

		// Create chunk
		chunk := &Chunk{
			CID:    cid,
			Data:   chunkData,
			Size:   uint64(end - i),
			Offset: offset,
		}

		chunks = append(chunks, chunk)
		offset += uint64(end - i)
	}

	return chunks, nil
}

// ReconstructFile reconstructs a file from chunks
func ReconstructFile(chunks []*Chunk, outputPath string) error {
	if len(chunks) == 0 {
		// Create empty file
		file, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create empty file: %w", err)
		}
		return file.Close()
	}

	// Ensure output directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create output file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Sort chunks by offset to ensure correct order
	// Note: In a real implementation, we might want to use a more sophisticated
	// sorting algorithm, but for now we'll assume chunks are already in order

	// Write chunks to file
	var expectedOffset uint64 = 0
	for i, chunk := range chunks {
		if chunk.Offset != expectedOffset {
			return fmt.Errorf("chunk %d has unexpected offset: got %d, want %d",
				i, chunk.Offset, expectedOffset)
		}

		// Verify chunk integrity
		if err := VerifyChunkIntegrity(chunk); err != nil {
			return fmt.Errorf("chunk %d failed integrity check: %w", i, err)
		}

		// Write chunk data
		n, err := file.Write(chunk.Data)
		if err != nil {
			return fmt.Errorf("failed to write chunk %d: %w", i, err)
		}

		if n != len(chunk.Data) {
			return fmt.Errorf("incomplete write for chunk %d: wrote %d, expected %d",
				i, n, len(chunk.Data))
		}

		expectedOffset += chunk.Size
	}

	return nil
}

// ReconstructData reconstructs data from chunks
func ReconstructData(chunks []*Chunk) ([]byte, error) {
	if len(chunks) == 0 {
		return []byte{}, nil
	}

	// Calculate total size
	var totalSize uint64 = 0
	for _, chunk := range chunks {
		totalSize += chunk.Size
	}

	// Allocate result buffer
	result := make([]byte, totalSize)

	// Copy chunk data
	var offset uint64 = 0
	for i, chunk := range chunks {
		if chunk.Offset != offset {
			return nil, fmt.Errorf("chunk %d has unexpected offset: got %d, want %d",
				i, chunk.Offset, offset)
		}

		// Verify chunk integrity
		if err := VerifyChunkIntegrity(chunk); err != nil {
			return nil, fmt.Errorf("chunk %d failed integrity check: %w", i, err)
		}

		// Copy chunk data
		copy(result[offset:offset+chunk.Size], chunk.Data)
		offset += chunk.Size
	}

	return result, nil
}
