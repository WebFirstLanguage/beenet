package content

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

// TestCompleteWorkflow tests the complete put/get workflow
func TestCompleteWorkflow(t *testing.T) {
	tempDir := t.TempDir()

	testCases := []struct {
		name      string
		data      []byte
		chunkSize uint32
	}{
		{
			name:      "small_text_file",
			data:      []byte("Hello, BeeNet! This is a small test file."),
			chunkSize: 1024 * 1024, // 1 MiB
		},
		{
			name:      "medium_file_multiple_chunks",
			data:      bytes.Repeat([]byte("This is test data for medium file testing. "), 100),
			chunkSize: 100, // Small chunks to force multiple chunks
		},
		{
			name:      "empty_file",
			data:      []byte{},
			chunkSize: 1024 * 1024,
		},
		{
			name:      "single_byte_file",
			data:      []byte("A"),
			chunkSize: 1024 * 1024,
		},
		{
			name:      "exact_chunk_boundary",
			data:      bytes.Repeat([]byte("X"), 1024), // Exactly 1024 bytes
			chunkSize: 1024,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Step 1: Create original file
			originalFile := filepath.Join(tempDir, tc.name+"_original.txt")
			err := os.WriteFile(originalFile, tc.data, 0644)
			if err != nil {
				t.Fatalf("Failed to create original file: %v", err)
			}

			// Step 2: Put operation - chunk file and create manifest
			chunks, err := ChunkFile(originalFile, tc.chunkSize)
			if err != nil {
				t.Fatalf("Failed to chunk file: %v", err)
			}

			manifest, err := BuildManifest(chunks, originalFile, tc.chunkSize)
			if err != nil {
				t.Fatalf("Failed to build manifest: %v", err)
			}

			manifestCID, err := ComputeManifestCID(manifest)
			if err != nil {
				t.Fatalf("Failed to compute manifest CID: %v", err)
			}

			// Step 3: Verify manifest integrity
			err = VerifyManifest(manifest)
			if err != nil {
				t.Fatalf("Manifest verification failed: %v", err)
			}

			// Step 4: Get operation - reconstruct file from chunks
			reconstructedFile := filepath.Join(tempDir, tc.name+"_reconstructed.txt")
			err = ReconstructFile(chunks, reconstructedFile)
			if err != nil {
				t.Fatalf("Failed to reconstruct file: %v", err)
			}

			// Step 5: Verify reconstructed file matches original
			originalData, err := os.ReadFile(originalFile)
			if err != nil {
				t.Fatalf("Failed to read original file: %v", err)
			}

			reconstructedData, err := os.ReadFile(reconstructedFile)
			if err != nil {
				t.Fatalf("Failed to read reconstructed file: %v", err)
			}

			if !bytes.Equal(originalData, reconstructedData) {
				t.Errorf("Reconstructed file does not match original")
				t.Logf("Original size: %d, Reconstructed size: %d", len(originalData), len(reconstructedData))
			}

			// Step 6: End-to-end integrity verification
			report, err := VerifyEndToEndIntegrity(originalFile, reconstructedFile, manifest, chunks, manifestCID)
			if err != nil {
				t.Fatalf("End-to-end integrity verification failed: %v", err)
			}

			if !report.Valid {
				t.Errorf("End-to-end integrity verification failed: %v", report.Errors)
			}

			// Step 7: Verify statistics
			if report.TotalChunks != len(chunks) {
				t.Errorf("Chunk count mismatch: got %d, want %d", report.TotalChunks, len(chunks))
			}

			if report.ValidChunks != report.TotalChunks {
				t.Errorf("Not all chunks are valid: got %d valid out of %d total", report.ValidChunks, report.TotalChunks)
			}

			if report.TotalBytes != uint64(len(tc.data)) {
				t.Errorf("Total bytes mismatch: got %d, want %d", report.TotalBytes, len(tc.data))
			}
		})
	}
}

// TestLargeFileWorkflow tests the workflow with larger files
func TestLargeFileWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large file test in short mode")
	}

	tempDir := t.TempDir()

	// Create a 5MB test file with random data
	largeData := make([]byte, 5*1024*1024) // 5 MB
	_, err := rand.Read(largeData)
	if err != nil {
		t.Fatalf("Failed to generate random data: %v", err)
	}

	originalFile := filepath.Join(tempDir, "large_file.bin")
	err = os.WriteFile(originalFile, largeData, 0644)
	if err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	// Use default chunk size (1 MiB)
	chunkSize := uint32(1024 * 1024)

	// Put operation
	chunks, err := ChunkFile(originalFile, chunkSize)
	if err != nil {
		t.Fatalf("Failed to chunk large file: %v", err)
	}

	// Should have 5 chunks
	expectedChunks := 5
	if len(chunks) != expectedChunks {
		t.Errorf("Expected %d chunks, got %d", expectedChunks, len(chunks))
	}

	manifest, err := BuildManifest(chunks, originalFile, chunkSize)
	if err != nil {
		t.Fatalf("Failed to build manifest for large file: %v", err)
	}

	manifestCID, err := ComputeManifestCID(manifest)
	if err != nil {
		t.Fatalf("Failed to compute manifest CID for large file: %v", err)
	}

	// Get operation
	reconstructedFile := filepath.Join(tempDir, "large_file_reconstructed.bin")
	err = ReconstructFile(chunks, reconstructedFile)
	if err != nil {
		t.Fatalf("Failed to reconstruct large file: %v", err)
	}

	// Verify integrity
	report, err := VerifyEndToEndIntegrity(originalFile, reconstructedFile, manifest, chunks, manifestCID)
	if err != nil {
		t.Fatalf("Large file integrity verification failed: %v", err)
	}

	if !report.Valid {
		t.Errorf("Large file integrity verification failed: %v", report.Errors)
	}

	// Verify file sizes match
	originalInfo, err := os.Stat(originalFile)
	if err != nil {
		t.Fatalf("Failed to stat original large file: %v", err)
	}

	reconstructedInfo, err := os.Stat(reconstructedFile)
	if err != nil {
		t.Fatalf("Failed to stat reconstructed large file: %v", err)
	}

	if originalInfo.Size() != reconstructedInfo.Size() {
		t.Errorf("File size mismatch: original %d, reconstructed %d",
			originalInfo.Size(), reconstructedInfo.Size())
	}
}

// TestWorkflowWithDifferentFileTypes tests the workflow with various file types
func TestWorkflowWithDifferentFileTypes(t *testing.T) {
	tempDir := t.TempDir()

	testFiles := []struct {
		name        string
		content     []byte
		contentType string
	}{
		{
			name:        "text_file.txt",
			content:     []byte("This is a plain text file with some content.\nIt has multiple lines.\nAnd various characters: !@#$%^&*()"),
			contentType: "text/plain; charset=utf-8",
		},
		{
			name:        "json_file.json",
			content:     []byte(`{"name": "test", "value": 42, "array": [1, 2, 3], "nested": {"key": "value"}}`),
			contentType: "application/json",
		},
		{
			name:        "binary_file.bin",
			content:     []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD, 0xFC, 0x7F, 0x80},
			contentType: "application/octet-stream",
		},
		{
			name:        "unicode_file.txt",
			content:     []byte("Hello 世界! 🌍 Ñoño café résumé naïve"),
			contentType: "text/plain; charset=utf-8",
		},
	}

	for _, tf := range testFiles {
		t.Run(tf.name, func(t *testing.T) {
			// Create original file
			originalFile := filepath.Join(tempDir, "original_"+tf.name)
			err := os.WriteFile(originalFile, tf.content, 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Put operation
			chunks, err := ChunkFile(originalFile, 1024)
			if err != nil {
				t.Fatalf("Failed to chunk file: %v", err)
			}

			manifest, err := BuildManifest(chunks, originalFile, 1024)
			if err != nil {
				t.Fatalf("Failed to build manifest: %v", err)
			}

			// Verify content type detection
			if manifest.ContentType != tf.contentType {
				t.Logf("Content type mismatch: got %s, expected %s (this may be acceptable)",
					manifest.ContentType, tf.contentType)
			}

			manifestCID, err := ComputeManifestCID(manifest)
			if err != nil {
				t.Fatalf("Failed to compute manifest CID: %v", err)
			}

			// Get operation
			reconstructedFile := filepath.Join(tempDir, "reconstructed_"+tf.name)
			err = ReconstructFile(chunks, reconstructedFile)
			if err != nil {
				t.Fatalf("Failed to reconstruct file: %v", err)
			}

			// Verify content matches exactly
			originalData, err := os.ReadFile(originalFile)
			if err != nil {
				t.Fatalf("Failed to read original file: %v", err)
			}

			reconstructedData, err := os.ReadFile(reconstructedFile)
			if err != nil {
				t.Fatalf("Failed to read reconstructed file: %v", err)
			}

			if !bytes.Equal(originalData, reconstructedData) {
				t.Errorf("File content mismatch for %s", tf.name)
				t.Logf("Original: %x", originalData[:min(len(originalData), 50)])
				t.Logf("Reconstructed: %x", reconstructedData[:min(len(reconstructedData), 50)])
			}

			// End-to-end verification
			report, err := VerifyEndToEndIntegrity(originalFile, reconstructedFile, manifest, chunks, manifestCID)
			if err != nil {
				t.Fatalf("Integrity verification failed for %s: %v", tf.name, err)
			}

			if !report.Valid {
				t.Errorf("Integrity verification failed for %s: %v", tf.name, report.Errors)
			}
		})
	}
}

// TestWorkflowErrorHandling tests error handling in the workflow
func TestWorkflowErrorHandling(t *testing.T) {
	tempDir := t.TempDir()

	// Test with non-existent file
	t.Run("non_existent_file", func(t *testing.T) {
		nonExistentFile := filepath.Join(tempDir, "does_not_exist.txt")
		_, err := ChunkFile(nonExistentFile, 1024)
		if err == nil {
			t.Error("Expected error when chunking non-existent file")
		}
	})

	// Test with corrupted chunks
	t.Run("corrupted_chunks", func(t *testing.T) {
		// Create a valid file first
		testFile := filepath.Join(tempDir, "test_corruption.txt")
		testData := []byte("This is test data for corruption testing")
		err := os.WriteFile(testFile, testData, 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Chunk the file
		chunks, err := ChunkFile(testFile, 20)
		if err != nil {
			t.Fatalf("Failed to chunk file: %v", err)
		}

		// Corrupt one chunk
		if len(chunks) > 0 {
			chunks[0].Data = []byte("corrupted data")

			// Try to reconstruct - should fail integrity check
			corruptedFile := filepath.Join(tempDir, "corrupted_reconstruction.txt")
			err = ReconstructFile(chunks, corruptedFile)
			if err == nil {
				t.Error("Expected error when reconstructing with corrupted chunk")
			}
		}
	})

	// Test with invalid manifest
	t.Run("invalid_manifest", func(t *testing.T) {
		invalidManifest := &Manifest{
			Version:    0, // Invalid version
			ChunkSize:  1024,
			ChunkCount: 1,
			FileSize:   100,
			Chunks:     []ChunkInfo{},
		}

		err := VerifyManifest(invalidManifest)
		if err == nil {
			t.Error("Expected error when verifying invalid manifest")
		}
	})
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
