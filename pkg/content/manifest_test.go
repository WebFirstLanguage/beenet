package content

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildManifest(t *testing.T) {
	// Create test chunks
	testData := []byte("Hello, World! This is a test.")
	chunks, err := ChunkData(testData, 10)
	if err != nil {
		t.Fatalf("Failed to create test chunks: %v", err)
	}

	// Build manifest
	manifest, err := BuildManifest(chunks, "test.txt", 10)
	if err != nil {
		t.Fatalf("Failed to build manifest: %v", err)
	}

	// Verify manifest properties
	if manifest.Version != 1 {
		t.Errorf("Wrong version: got %d, want 1", manifest.Version)
	}

	if manifest.FileSize != uint64(len(testData)) {
		t.Errorf("Wrong file size: got %d, want %d", manifest.FileSize, len(testData))
	}

	if manifest.ChunkSize != 10 {
		t.Errorf("Wrong chunk size: got %d, want 10", manifest.ChunkSize)
	}

	if manifest.ChunkCount != uint32(len(chunks)) {
		t.Errorf("Wrong chunk count: got %d, want %d", manifest.ChunkCount, len(chunks))
	}

	if len(manifest.Chunks) != len(chunks) {
		t.Errorf("Wrong number of chunk infos: got %d, want %d", len(manifest.Chunks), len(chunks))
	}

	if manifest.Filename != "test.txt" {
		t.Errorf("Wrong filename: got %s, want test.txt", manifest.Filename)
	}

	// Verify chunks are in correct order
	for i, chunkInfo := range manifest.Chunks {
		if chunkInfo.Offset != chunks[i].Offset {
			t.Errorf("Chunk %d wrong offset: got %d, want %d", i, chunkInfo.Offset, chunks[i].Offset)
		}

		if chunkInfo.Size != chunks[i].Size {
			t.Errorf("Chunk %d wrong size: got %d, want %d", i, chunkInfo.Size, chunks[i].Size)
		}

		if !chunkInfo.CID.Equals(chunks[i].CID) {
			t.Errorf("Chunk %d CID mismatch", i)
		}
	}
}

func TestBuildManifestFromFile(t *testing.T) {
	// Create a test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	testData := []byte("This is test data for manifest building.")

	err := os.WriteFile(testFile, testData, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Build manifest from file
	manifest, manifestCID, err := BuildManifestFromFile(testFile, 15)
	if err != nil {
		t.Fatalf("Failed to build manifest from file: %v", err)
	}

	// Verify manifest
	if manifest.FileSize != uint64(len(testData)) {
		t.Errorf("Wrong file size: got %d, want %d", manifest.FileSize, len(testData))
	}

	if manifest.ChunkSize != 15 {
		t.Errorf("Wrong chunk size: got %d, want 15", manifest.ChunkSize)
	}

	if manifest.Filename != "test.txt" {
		t.Errorf("Wrong filename: got %s, want test.txt", manifest.Filename)
	}

	// Verify CID is valid
	if !manifestCID.IsValid() {
		t.Error("Manifest CID is invalid")
	}

	// Verify CID matches computed CID
	expectedCID, err := ComputeManifestCID(manifest)
	if err != nil {
		t.Fatalf("Failed to compute expected CID: %v", err)
	}

	if !manifestCID.Equals(expectedCID) {
		t.Error("Manifest CID doesn't match computed CID")
	}
}

func TestBuildManifestFromData(t *testing.T) {
	testData := []byte("Test data for manifest building from raw data.")

	manifest, manifestCID, err := BuildManifestFromData(testData, 20, "data.bin")
	if err != nil {
		t.Fatalf("Failed to build manifest from data: %v", err)
	}

	// Verify manifest
	if manifest.FileSize != uint64(len(testData)) {
		t.Errorf("Wrong file size: got %d, want %d", manifest.FileSize, len(testData))
	}

	if manifest.ChunkSize != 20 {
		t.Errorf("Wrong chunk size: got %d, want 20", manifest.ChunkSize)
	}

	if manifest.Filename != "data.bin" {
		t.Errorf("Wrong filename: got %s, want data.bin", manifest.Filename)
	}

	// Verify CID is valid
	if !manifestCID.IsValid() {
		t.Error("Manifest CID is invalid")
	}
}

func TestVerifyManifest(t *testing.T) {
	// Create a valid manifest
	testData := []byte("Test data for verification")
	chunks, err := ChunkData(testData, 10)
	if err != nil {
		t.Fatalf("Failed to create test chunks: %v", err)
	}

	manifest, err := BuildManifest(chunks, "test.txt", 10)
	if err != nil {
		t.Fatalf("Failed to build manifest: %v", err)
	}

	// Valid manifest should pass verification
	err = VerifyManifest(manifest)
	if err != nil {
		t.Errorf("Valid manifest failed verification: %v", err)
	}

	// Test invalid cases
	testCases := []struct {
		name     string
		modifier func(*Manifest)
	}{
		{
			"nil manifest",
			func(m *Manifest) { *m = Manifest{} },
		},
		{
			"zero version",
			func(m *Manifest) { m.Version = 0 },
		},
		{
			"zero chunk size",
			func(m *Manifest) { m.ChunkSize = 0 },
		},
		{
			"chunk count mismatch",
			func(m *Manifest) { m.ChunkCount = 999 },
		},
		{
			"wrong file size",
			func(m *Manifest) { m.FileSize = 999 },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a copy of the valid manifest
			invalidManifest := *manifest
			invalidManifest.Chunks = make([]ChunkInfo, len(manifest.Chunks))
			copy(invalidManifest.Chunks, manifest.Chunks)

			// Apply the modification
			tc.modifier(&invalidManifest)

			// Should fail verification
			err := VerifyManifest(&invalidManifest)
			if err == nil {
				t.Errorf("Invalid manifest (%s) passed verification", tc.name)
			}
		})
	}
}

func TestVerifyManifestWithChunks(t *testing.T) {
	// Create test data and chunks
	testData := []byte("Test data for chunk verification")
	chunks, err := ChunkData(testData, 12)
	if err != nil {
		t.Fatalf("Failed to create test chunks: %v", err)
	}

	// Build manifest
	manifest, err := BuildManifest(chunks, "test.txt", 12)
	if err != nil {
		t.Fatalf("Failed to build manifest: %v", err)
	}

	// Valid manifest and chunks should pass verification
	err = VerifyManifestWithChunks(manifest, chunks)
	if err != nil {
		t.Errorf("Valid manifest and chunks failed verification: %v", err)
	}

	// Test with wrong number of chunks
	wrongChunks := chunks[:len(chunks)-1]
	err = VerifyManifestWithChunks(manifest, wrongChunks)
	if err == nil {
		t.Error("Verification should fail with wrong number of chunks")
	}

	// Test with corrupted chunk
	if len(chunks) > 0 {
		corruptedChunks := make([]*Chunk, len(chunks))
		copy(corruptedChunks, chunks)

		// Corrupt the first chunk's data
		corruptedChunks[0] = &Chunk{
			CID:    chunks[0].CID, // Keep same CID but corrupt data
			Data:   []byte("corrupted"),
			Size:   chunks[0].Size,
			Offset: chunks[0].Offset,
		}

		err = VerifyManifestWithChunks(manifest, corruptedChunks)
		if err == nil {
			t.Error("Verification should fail with corrupted chunk")
		}
	}
}

func TestGetManifestStats(t *testing.T) {
	// Test with nil manifest
	stats := GetManifestStats(nil)
	if stats["error"] == nil {
		t.Error("Expected error for nil manifest")
	}

	// Test with valid manifest
	testData := []byte("Test data for stats")
	chunks, err := ChunkData(testData, 8)
	if err != nil {
		t.Fatalf("Failed to create test chunks: %v", err)
	}

	manifest, err := BuildManifest(chunks, "test.txt", 8)
	if err != nil {
		t.Fatalf("Failed to build manifest: %v", err)
	}

	stats = GetManifestStats(manifest)

	// Check expected fields
	expectedFields := []string{
		"version", "file_size", "chunk_size", "chunk_count",
		"created_at", "content_type", "filename", "storage_overhead", "efficiency",
	}

	for _, field := range expectedFields {
		if _, exists := stats[field]; !exists {
			t.Errorf("Missing field in stats: %s", field)
		}
	}

	// Check values
	if stats["version"] != manifest.Version {
		t.Errorf("Wrong version in stats: got %v, want %d", stats["version"], manifest.Version)
	}

	if stats["file_size"] != manifest.FileSize {
		t.Errorf("Wrong file_size in stats: got %v, want %d", stats["file_size"], manifest.FileSize)
	}
}

func TestValidateManifestCID(t *testing.T) {
	// Create test manifest
	testData := []byte("Test data for CID validation")
	chunks, err := ChunkData(testData, 10)
	if err != nil {
		t.Fatalf("Failed to create test chunks: %v", err)
	}

	manifest, err := BuildManifest(chunks, "test.txt", 10)
	if err != nil {
		t.Fatalf("Failed to build manifest: %v", err)
	}

	// Compute correct CID
	correctCID, err := ComputeManifestCID(manifest)
	if err != nil {
		t.Fatalf("Failed to compute manifest CID: %v", err)
	}

	// Validation with correct CID should pass
	err = ValidateManifestCID(manifest, correctCID)
	if err != nil {
		t.Errorf("Validation failed with correct CID: %v", err)
	}

	// Validation with wrong CID should fail
	wrongCID := NewCID([]byte("wrong data"))
	err = ValidateManifestCID(manifest, wrongCID)
	if err == nil {
		t.Error("Validation should fail with wrong CID")
	}
}

func TestManifestTimestamp(t *testing.T) {
	// Create manifest
	testData := []byte("Test data")
	chunks, err := ChunkData(testData, 5)
	if err != nil {
		t.Fatalf("Failed to create test chunks: %v", err)
	}

	beforeTime := time.Now().UnixMilli()
	manifest, err := BuildManifest(chunks, "test.txt", 5)
	if err != nil {
		t.Fatalf("Failed to build manifest: %v", err)
	}
	afterTime := time.Now().UnixMilli()

	// Timestamp should be within reasonable range
	if manifest.CreatedAt < uint64(beforeTime) || manifest.CreatedAt > uint64(afterTime) {
		t.Errorf("Manifest timestamp out of range: got %d, expected between %d and %d",
			manifest.CreatedAt, beforeTime, afterTime)
	}
}

func TestManifestContentType(t *testing.T) {
	testCases := []struct {
		filename     string
		expectedType string
	}{
		{"test.txt", "text/plain; charset=utf-8"},
		{"image.jpg", "image/jpeg"},
		{"data.json", "application/json"},
		{"unknown.xyz", ""}, // Unknown extension
		{"", ""},            // No filename
	}

	testData := []byte("test")
	chunks, err := ChunkData(testData, 10)
	if err != nil {
		t.Fatalf("Failed to create test chunks: %v", err)
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			manifest, err := BuildManifest(chunks, tc.filename, 10)
			if err != nil {
				t.Fatalf("Failed to build manifest: %v", err)
			}

			if manifest.ContentType != tc.expectedType {
				t.Errorf("Wrong content type: got %s, want %s",
					manifest.ContentType, tc.expectedType)
			}
		})
	}
}
