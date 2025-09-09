package content

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestChunkData(t *testing.T) {
	testCases := []struct {
		name       string
		data       []byte
		chunkSize  uint32
		wantChunks int
	}{
		{"empty data", []byte{}, 1024, 0},
		{"single byte", []byte{42}, 1024, 1},
		{"exact chunk size", make([]byte, 1024), 1024, 1},
		{"two chunks", make([]byte, 2048), 1024, 2},
		{"partial last chunk", make([]byte, 1500), 1024, 2},
		{"small chunk size", []byte("hello world"), 5, 3},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			chunks, err := ChunkData(tc.data, tc.chunkSize)
			if err != nil {
				t.Fatalf("ChunkData failed: %v", err)
			}

			if len(chunks) != tc.wantChunks {
				t.Errorf("Wrong number of chunks: got %d, want %d", len(chunks), tc.wantChunks)
			}

			// Verify chunk properties
			var totalSize uint64 = 0
			for i, chunk := range chunks {
				// Check offset
				if chunk.Offset != totalSize {
					t.Errorf("Chunk %d has wrong offset: got %d, want %d", i, chunk.Offset, totalSize)
				}

				// Check size matches data length
				if chunk.Size != uint64(len(chunk.Data)) {
					t.Errorf("Chunk %d size mismatch: got %d, want %d", i, chunk.Size, len(chunk.Data))
				}

				// Check CID is valid
				if !chunk.CID.IsValid() {
					t.Errorf("Chunk %d has invalid CID", i)
				}

				// Verify integrity
				if err := VerifyChunkIntegrity(chunk); err != nil {
					t.Errorf("Chunk %d failed integrity check: %v", i, err)
				}

				totalSize += chunk.Size
			}

			// Total size should match original data
			if totalSize != uint64(len(tc.data)) {
				t.Errorf("Total chunk size mismatch: got %d, want %d", totalSize, len(tc.data))
			}
		})
	}
}

func TestChunkDataZeroChunkSize(t *testing.T) {
	_, err := ChunkData([]byte("test"), 0)
	if err == nil {
		t.Error("Expected error for zero chunk size, got nil")
	}
}

func TestChunkReader(t *testing.T) {
	testData := []byte("The quick brown fox jumps over the lazy dog")
	reader := bytes.NewReader(testData)

	chunks, err := ChunkReader(reader, 10)
	if err != nil {
		t.Fatalf("ChunkReader failed: %v", err)
	}

	// Should have 5 chunks (44 bytes / 10 = 4.4, rounded up to 5)
	expectedChunks := 5
	if len(chunks) != expectedChunks {
		t.Errorf("Wrong number of chunks: got %d, want %d", len(chunks), expectedChunks)
	}

	// Verify we can reconstruct the original data
	reconstructed, err := ReconstructData(chunks)
	if err != nil {
		t.Fatalf("Failed to reconstruct data: %v", err)
	}

	if !bytes.Equal(reconstructed, testData) {
		t.Error("Reconstructed data doesn't match original")
	}
}

func TestChunkFile(t *testing.T) {
	// Create a temporary file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	testData := []byte("This is a test file for chunking")

	err := os.WriteFile(testFile, testData, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Chunk the file
	chunks, err := ChunkFile(testFile, 10)
	if err != nil {
		t.Fatalf("ChunkFile failed: %v", err)
	}

	// Should have 4 chunks (32 bytes / 10 = 3.2, rounded up to 4)
	expectedChunks := 4
	if len(chunks) != expectedChunks {
		t.Errorf("Wrong number of chunks: got %d, want %d", len(chunks), expectedChunks)
	}

	// Verify we can reconstruct the file
	outputFile := filepath.Join(tempDir, "reconstructed.txt")
	err = ReconstructFile(chunks, outputFile)
	if err != nil {
		t.Fatalf("Failed to reconstruct file: %v", err)
	}

	// Read reconstructed file and compare
	reconstructedData, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read reconstructed file: %v", err)
	}

	if !bytes.Equal(reconstructedData, testData) {
		t.Error("Reconstructed file doesn't match original")
	}
}

func TestChunkFileEmpty(t *testing.T) {
	// Create an empty temporary file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "empty.txt")

	err := os.WriteFile(testFile, []byte{}, 0644)
	if err != nil {
		t.Fatalf("Failed to create empty test file: %v", err)
	}

	// Chunk the empty file
	chunks, err := ChunkFile(testFile, 1024)
	if err != nil {
		t.Fatalf("ChunkFile failed for empty file: %v", err)
	}

	// Should have 0 chunks
	if len(chunks) != 0 {
		t.Errorf("Empty file should produce 0 chunks, got %d", len(chunks))
	}
}

func TestChunkFileNonExistent(t *testing.T) {
	_, err := ChunkFile("/nonexistent/file.txt", 1024)
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestReconstructData(t *testing.T) {
	originalData := []byte("Hello, World! This is a test of data reconstruction.")

	// Chunk the data
	chunks, err := ChunkData(originalData, 10)
	if err != nil {
		t.Fatalf("Failed to chunk data: %v", err)
	}

	// Reconstruct the data
	reconstructed, err := ReconstructData(chunks)
	if err != nil {
		t.Fatalf("Failed to reconstruct data: %v", err)
	}

	// Should match original
	if !bytes.Equal(reconstructed, originalData) {
		t.Error("Reconstructed data doesn't match original")
	}
}

func TestReconstructDataEmpty(t *testing.T) {
	// Empty chunks should produce empty data
	reconstructed, err := ReconstructData([]*Chunk{})
	if err != nil {
		t.Fatalf("Failed to reconstruct empty data: %v", err)
	}

	if len(reconstructed) != 0 {
		t.Errorf("Empty chunks should produce empty data, got %d bytes", len(reconstructed))
	}
}

func TestReconstructFile(t *testing.T) {
	tempDir := t.TempDir()
	originalData := []byte("This is test data for file reconstruction.")

	// Chunk the data
	chunks, err := ChunkData(originalData, 15)
	if err != nil {
		t.Fatalf("Failed to chunk data: %v", err)
	}

	// Reconstruct to file
	outputFile := filepath.Join(tempDir, "output.txt")
	err = ReconstructFile(chunks, outputFile)
	if err != nil {
		t.Fatalf("Failed to reconstruct file: %v", err)
	}

	// Read and verify
	reconstructedData, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read reconstructed file: %v", err)
	}

	if !bytes.Equal(reconstructedData, originalData) {
		t.Error("Reconstructed file doesn't match original data")
	}
}

func TestReconstructFileEmpty(t *testing.T) {
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "empty_output.txt")

	// Reconstruct empty file
	err := ReconstructFile([]*Chunk{}, outputFile)
	if err != nil {
		t.Fatalf("Failed to reconstruct empty file: %v", err)
	}

	// Verify file exists and is empty
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read empty reconstructed file: %v", err)
	}

	if len(data) != 0 {
		t.Errorf("Empty reconstruction should produce empty file, got %d bytes", len(data))
	}
}

func TestChunkIntegrityError(t *testing.T) {
	originalData := []byte("test data")
	chunks, err := ChunkData(originalData, 5)
	if err != nil {
		t.Fatalf("Failed to chunk data: %v", err)
	}

	// Corrupt a chunk
	if len(chunks) > 0 {
		chunks[0].Data[0] = ^chunks[0].Data[0] // Flip bits
	}

	// Reconstruction should fail
	_, err = ReconstructData(chunks)
	if err == nil {
		t.Error("Expected integrity error, got nil")
	}
}

func TestLargeFileChunking(t *testing.T) {
	// Test with a larger file to ensure chunking works correctly
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "large.txt")

	// Create 10KB of test data
	testData := make([]byte, 10*1024)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	err := os.WriteFile(testFile, testData, 0644)
	if err != nil {
		t.Fatalf("Failed to create large test file: %v", err)
	}

	// Chunk with 1KB chunks
	chunks, err := ChunkFile(testFile, 1024)
	if err != nil {
		t.Fatalf("Failed to chunk large file: %v", err)
	}

	// Should have exactly 10 chunks
	if len(chunks) != 10 {
		t.Errorf("Expected 10 chunks, got %d", len(chunks))
	}

	// Verify each chunk (except possibly the last) is exactly 1KB
	for i, chunk := range chunks {
		expectedSize := uint64(1024)
		if chunk.Size != expectedSize {
			t.Errorf("Chunk %d has wrong size: got %d, want %d", i, chunk.Size, expectedSize)
		}
	}

	// Reconstruct and verify
	outputFile := filepath.Join(tempDir, "large_reconstructed.txt")
	err = ReconstructFile(chunks, outputFile)
	if err != nil {
		t.Fatalf("Failed to reconstruct large file: %v", err)
	}

	reconstructedData, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read reconstructed large file: %v", err)
	}

	if !bytes.Equal(reconstructedData, testData) {
		t.Error("Large file reconstruction failed")
	}
}
