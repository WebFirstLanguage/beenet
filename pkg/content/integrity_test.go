package content

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyContentIntegrity(t *testing.T) {
	// Create test data
	testData := []byte("This is test data for integrity verification testing")
	chunks, err := ChunkData(testData, 20)
	if err != nil {
		t.Fatalf("Failed to create test chunks: %v", err)
	}

	manifest, err := BuildManifest(chunks, "test.txt", 20)
	if err != nil {
		t.Fatalf("Failed to build manifest: %v", err)
	}

	manifestCID, err := ComputeManifestCID(manifest)
	if err != nil {
		t.Fatalf("Failed to compute manifest CID: %v", err)
	}

	// Test valid content
	report := VerifyContentIntegrity(manifest, chunks, &manifestCID)
	if !report.Valid {
		t.Errorf("Valid content reported as invalid: %v", report.Errors)
	}

	if !report.ManifestCIDValid {
		t.Error("Valid manifest CID reported as invalid")
	}

	if report.ValidChunks != report.TotalChunks {
		t.Errorf("Valid chunk count mismatch: got %d, want %d", report.ValidChunks, report.TotalChunks)
	}

	// Test with corrupted chunk
	corruptedChunks := make([]*Chunk, len(chunks))
	copy(corruptedChunks, chunks)
	if len(corruptedChunks) > 0 {
		// Corrupt the first chunk's data but keep the same CID
		corruptedChunks[0] = &Chunk{
			CID:    chunks[0].CID,
			Data:   []byte("corrupted data"),
			Size:   chunks[0].Size,
			Offset: chunks[0].Offset,
		}

		report = VerifyContentIntegrity(manifest, corruptedChunks, &manifestCID)
		if report.Valid {
			t.Error("Corrupted content reported as valid")
		}

		if report.ChunkIntegrity[0].Valid {
			t.Error("Corrupted chunk reported as valid")
		}

		if report.ChunkIntegrity[0].ExpectedCID == "" {
			t.Error("Expected CID not provided for corrupted chunk")
		}
	}

	// Test with wrong manifest CID
	wrongCID := NewCID([]byte("wrong manifest data"))
	report = VerifyContentIntegrity(manifest, chunks, &wrongCID)
	if report.ManifestCIDValid {
		t.Error("Wrong manifest CID reported as valid")
	}
}

func TestVerifyReconstructedFile(t *testing.T) {
	// Create a temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	testData := []byte("This is test data for file verification")

	err := os.WriteFile(testFile, testData, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Calculate expected SHA256
	hasher := sha256.New()
	hasher.Write(testData)
	expectedSHA256 := fmt.Sprintf("%x", hasher.Sum(nil))

	// Test valid file
	result := VerifyReconstructedFile(testFile, uint64(len(testData)), expectedSHA256)
	if !result.Valid {
		t.Errorf("Valid file reported as invalid: %s", result.Error)
	}

	if result.ActualSize != result.ExpectedSize {
		t.Errorf("Size mismatch: got %d, want %d", result.ActualSize, result.ExpectedSize)
	}

	if result.ActualSHA256 != result.ExpectedSHA256 {
		t.Errorf("SHA256 mismatch: got %s, want %s", result.ActualSHA256, result.ExpectedSHA256)
	}

	// Test with wrong expected size
	result = VerifyReconstructedFile(testFile, uint64(len(testData)+10), expectedSHA256)
	if result.Valid {
		t.Error("File with wrong size reported as valid")
	}

	// Test with wrong expected hash
	result = VerifyReconstructedFile(testFile, uint64(len(testData)), "wrong_hash")
	if result.Valid {
		t.Error("File with wrong hash reported as valid")
	}

	// Test with non-existent file
	result = VerifyReconstructedFile("/non/existent/file", uint64(len(testData)), expectedSHA256)
	if result.Valid {
		t.Error("Non-existent file reported as valid")
	}
}

func TestVerifyEndToEndIntegrity(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	originalFile := filepath.Join(tempDir, "original.txt")
	reconstructedFile := filepath.Join(tempDir, "reconstructed.txt")

	// Create original test file
	testData := []byte("This is test data for end-to-end integrity verification")
	err := os.WriteFile(originalFile, testData, 0644)
	if err != nil {
		t.Fatalf("Failed to create original file: %v", err)
	}

	// Process the file
	chunks, err := ChunkData(testData, 20)
	if err != nil {
		t.Fatalf("Failed to create chunks: %v", err)
	}

	manifest, err := BuildManifest(chunks, "original.txt", 20)
	if err != nil {
		t.Fatalf("Failed to build manifest: %v", err)
	}

	manifestCID, err := ComputeManifestCID(manifest)
	if err != nil {
		t.Fatalf("Failed to compute manifest CID: %v", err)
	}

	// Reconstruct the file
	err = ReconstructFile(chunks, reconstructedFile)
	if err != nil {
		t.Fatalf("Failed to reconstruct file: %v", err)
	}

	// Perform end-to-end verification
	report, err := VerifyEndToEndIntegrity(originalFile, reconstructedFile, manifest, chunks, manifestCID)
	if err != nil {
		t.Fatalf("End-to-end verification failed: %v", err)
	}

	if !report.Valid {
		t.Errorf("Valid end-to-end process reported as invalid: %v", report.Errors)
	}

	if report.FileIntegrity == nil {
		t.Error("File integrity result is nil")
	} else if !report.FileIntegrity.Valid {
		t.Errorf("File integrity check failed: %s", report.FileIntegrity.Error)
	}

	// Test with corrupted reconstructed file
	corruptedFile := filepath.Join(tempDir, "corrupted.txt")
	err = os.WriteFile(corruptedFile, []byte("corrupted data"), 0644)
	if err != nil {
		t.Fatalf("Failed to create corrupted file: %v", err)
	}

	report, err = VerifyEndToEndIntegrity(originalFile, corruptedFile, manifest, chunks, manifestCID)
	if err != nil {
		t.Fatalf("End-to-end verification with corrupted file failed: %v", err)
	}

	if report.Valid {
		t.Error("Corrupted end-to-end process reported as valid")
	}

	if report.FileIntegrity == nil || report.FileIntegrity.Valid {
		t.Error("Corrupted file integrity check should fail")
	}
}

func TestVerifyChunkSequence(t *testing.T) {
	// Create test chunks with proper sequence
	testData := []byte("This is test data for chunk sequence verification")
	chunks, err := ChunkData(testData, 15)
	if err != nil {
		t.Fatalf("Failed to create test chunks: %v", err)
	}

	// Test valid sequence
	err = VerifyChunkSequence(chunks)
	if err != nil {
		t.Errorf("Valid chunk sequence reported as invalid: %v", err)
	}

	// Test empty sequence
	err = VerifyChunkSequence([]*Chunk{})
	if err != nil {
		t.Errorf("Empty chunk sequence should be valid: %v", err)
	}

	// Test with gap in sequence
	if len(chunks) > 1 {
		gappedChunks := make([]*Chunk, len(chunks))
		copy(gappedChunks, chunks)
		
		// Create a gap by modifying the second chunk's offset
		gappedChunks[1] = &Chunk{
			CID:    chunks[1].CID,
			Data:   chunks[1].Data,
			Size:   chunks[1].Size,
			Offset: chunks[1].Offset + 10, // Create gap
		}

		err = VerifyChunkSequence(gappedChunks)
		if err == nil {
			t.Error("Chunk sequence with gap should be invalid")
		}
	}

	// Test with zero-size chunk
	if len(chunks) > 0 {
		zeroSizeChunks := make([]*Chunk, len(chunks))
		copy(zeroSizeChunks, chunks)
		
		zeroSizeChunks[0] = &Chunk{
			CID:    chunks[0].CID,
			Data:   chunks[0].Data,
			Size:   0, // Zero size
			Offset: chunks[0].Offset,
		}

		err = VerifyChunkSequence(zeroSizeChunks)
		if err == nil {
			t.Error("Chunk sequence with zero-size chunk should be invalid")
		}
	}
}

func TestVerifyManifestChunkConsistency(t *testing.T) {
	// Create test data
	testData := []byte("This is test data for manifest-chunk consistency verification")
	chunks, err := ChunkData(testData, 20)
	if err != nil {
		t.Fatalf("Failed to create test chunks: %v", err)
	}

	manifest, err := BuildManifest(chunks, "test.txt", 20)
	if err != nil {
		t.Fatalf("Failed to build manifest: %v", err)
	}

	// Test valid consistency
	err = VerifyManifestChunkConsistency(manifest, chunks)
	if err != nil {
		t.Errorf("Valid manifest-chunk consistency reported as invalid: %v", err)
	}

	// Test with wrong number of chunks
	if len(chunks) > 1 {
		wrongChunks := chunks[:len(chunks)-1]
		err = VerifyManifestChunkConsistency(manifest, wrongChunks)
		if err == nil {
			t.Error("Manifest-chunk consistency with wrong count should be invalid")
		}
	}

	// Test with missing chunk
	if len(chunks) > 0 {
		missingChunks := make([]*Chunk, len(chunks))
		copy(missingChunks, chunks)
		
		// Replace first chunk with a different CID
		missingChunks[0] = &Chunk{
			CID:    NewCID([]byte("different data")),
			Data:   chunks[0].Data,
			Size:   chunks[0].Size,
			Offset: chunks[0].Offset,
		}

		err = VerifyManifestChunkConsistency(manifest, missingChunks)
		if err == nil {
			t.Error("Manifest-chunk consistency with missing chunk should be invalid")
		}
	}

	// Test with size mismatch
	if len(chunks) > 0 {
		sizeChunks := make([]*Chunk, len(chunks))
		copy(sizeChunks, chunks)
		
		sizeChunks[0] = &Chunk{
			CID:    chunks[0].CID,
			Data:   chunks[0].Data,
			Size:   chunks[0].Size + 10, // Wrong size
			Offset: chunks[0].Offset,
		}

		err = VerifyManifestChunkConsistency(manifest, sizeChunks)
		if err == nil {
			t.Error("Manifest-chunk consistency with size mismatch should be invalid")
		}
	}
}
