package content

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"lukechampine.com/blake3"
)

func TestNewCID(t *testing.T) {
	testData := []byte("hello world")

	cid := NewCID(testData)

	// Verify hash is correct length
	if len(cid.Hash) != HashSize {
		t.Errorf("Hash length mismatch: got %d, want %d", len(cid.Hash), HashSize)
	}

	// Verify hash is correct
	expectedHash := blake3.Sum256(testData)
	if !bytes.Equal(cid.Hash, expectedHash[:]) {
		t.Errorf("Hash mismatch: got %x, want %x", cid.Hash, expectedHash[:])
	}

	// Verify string representation is not empty
	if cid.String == "" {
		t.Error("CID string representation is empty")
	}

	// Verify string has correct prefix
	if !bytes.HasPrefix([]byte(cid.String), []byte(CIDPrefix+":")) {
		t.Errorf("CID string missing prefix: got %s", cid.String)
	}
}

func TestNewCIDFromHash(t *testing.T) {
	// Test with valid hash
	validHash := make([]byte, HashSize)
	for i := range validHash {
		validHash[i] = byte(i)
	}

	cid, err := NewCIDFromHash(validHash)
	if err != nil {
		t.Fatalf("Failed to create CID from valid hash: %v", err)
	}

	if !bytes.Equal(cid.Hash, validHash) {
		t.Errorf("Hash mismatch: got %x, want %x", cid.Hash, validHash)
	}

	// Test with invalid hash size
	invalidHash := make([]byte, 16) // Wrong size
	_, err = NewCIDFromHash(invalidHash)
	if err == nil {
		t.Error("Expected error for invalid hash size, got nil")
	}
}

func TestParseCID(t *testing.T) {
	// Create a test CID
	testData := []byte("test data for parsing")
	originalCID := NewCID(testData)

	// Parse the CID string
	parsedCID, err := ParseCID(originalCID.String)
	if err != nil {
		t.Fatalf("Failed to parse CID: %v", err)
	}

	// Verify they match
	if !originalCID.Equals(parsedCID) {
		t.Errorf("Parsed CID doesn't match original: got %s, want %s",
			parsedCID.String, originalCID.String)
	}

	// Test invalid cases
	testCases := []struct {
		name   string
		cidStr string
	}{
		{"empty string", ""},
		{"wrong prefix", "wrong:abcdef"},
		{"no prefix", "abcdef"},
		{"invalid base32", "bee:invalid!@#"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCID(tc.cidStr)
			if err == nil {
				t.Errorf("Expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestCIDIsValid(t *testing.T) {
	// Valid CID
	validCID := NewCID([]byte("test data"))
	if !validCID.IsValid() {
		t.Error("Valid CID reported as invalid")
	}

	// Invalid CID - wrong hash size
	invalidCID := CID{
		Hash:   make([]byte, 16), // Wrong size
		String: "bee:test",
	}
	if invalidCID.IsValid() {
		t.Error("Invalid CID (wrong hash size) reported as valid")
	}

	// Invalid CID - empty string
	invalidCID2 := CID{
		Hash:   make([]byte, HashSize),
		String: "",
	}
	if invalidCID2.IsValid() {
		t.Error("Invalid CID (empty string) reported as valid")
	}

	// Invalid CID - mismatched string
	validHash := make([]byte, HashSize)
	invalidCID3 := CID{
		Hash:   validHash,
		String: "bee:wrongstring",
	}
	if invalidCID3.IsValid() {
		t.Error("Invalid CID (mismatched string) reported as valid")
	}
}

func TestCIDEquals(t *testing.T) {
	testData := []byte("test data")

	cid1 := NewCID(testData)
	cid2 := NewCID(testData)
	cid3 := NewCID([]byte("different data"))

	// Same data should produce equal CIDs
	if !cid1.Equals(cid2) {
		t.Error("CIDs from same data should be equal")
	}

	// Different data should produce different CIDs
	if cid1.Equals(cid3) {
		t.Error("CIDs from different data should not be equal")
	}

	// Test with different hash sizes
	invalidCID := CID{Hash: make([]byte, 16)}
	if cid1.Equals(invalidCID) {
		t.Error("CIDs with different hash sizes should not be equal")
	}
}

func TestCIDBytes(t *testing.T) {
	testData := []byte("test data")
	cid := NewCID(testData)

	cidBytes := cid.Bytes()

	// Should return a copy, not the original
	if &cidBytes[0] == &cid.Hash[0] {
		t.Error("Bytes() should return a copy, not the original slice")
	}

	// Content should match
	if !bytes.Equal(cidBytes, cid.Hash) {
		t.Error("Bytes() content doesn't match hash")
	}
}

func TestCIDHexString(t *testing.T) {
	testData := []byte("test data")
	cid := NewCID(testData)

	hexStr := cid.HexString()

	// Should be valid hex
	decoded, err := hex.DecodeString(hexStr)
	if err != nil {
		t.Fatalf("HexString() produced invalid hex: %v", err)
	}

	// Should match the hash
	if !bytes.Equal(decoded, cid.Hash) {
		t.Error("HexString() doesn't match hash")
	}
}

func TestComputeManifestCID(t *testing.T) {
	// Create a test manifest
	hash1 := make([]byte, HashSize)
	hash2 := make([]byte, HashSize)
	for i := range hash1 {
		hash1[i] = byte(i)
		hash2[i] = byte(i + 32)
	}

	manifest := &Manifest{
		Version:    1,
		FileSize:   2048,
		ChunkSize:  1024,
		ChunkCount: 2,
		Chunks: []ChunkInfo{
			{CID: CID{Hash: hash1}, Size: 1024, Offset: 0},
			{CID: CID{Hash: hash2}, Size: 1024, Offset: 1024},
		},
		CreatedAt:   uint64(time.Now().UnixMilli()),
		ContentType: "text/plain",
		Filename:    "test.txt",
	}

	cid, err := ComputeManifestCID(manifest)
	if err != nil {
		t.Fatalf("Failed to compute manifest CID: %v", err)
	}

	if !cid.IsValid() {
		t.Error("Computed manifest CID is invalid")
	}

	// Computing the same manifest should produce the same CID
	cid2, err := ComputeManifestCID(manifest)
	if err != nil {
		t.Fatalf("Failed to compute manifest CID second time: %v", err)
	}

	if !cid.Equals(cid2) {
		t.Error("Same manifest should produce same CID")
	}
}

func TestVerifyChunkIntegrity(t *testing.T) {
	testData := []byte("chunk data")
	correctCID := NewCID(testData)

	// Valid chunk
	validChunk := &Chunk{
		CID:  correctCID,
		Data: testData,
		Size: uint64(len(testData)),
	}

	err := VerifyChunkIntegrity(validChunk)
	if err != nil {
		t.Errorf("Valid chunk failed integrity check: %v", err)
	}

	// Invalid chunk - wrong CID
	wrongCID := NewCID([]byte("wrong data"))
	invalidChunk := &Chunk{
		CID:  wrongCID,
		Data: testData,
		Size: uint64(len(testData)),
	}

	err = VerifyChunkIntegrity(invalidChunk)
	if err == nil {
		t.Error("Invalid chunk passed integrity check")
	}
}

func TestGenerateChunkCID(t *testing.T) {
	testData := []byte("chunk data")

	cid := GenerateChunkCID(testData)
	expectedCID := NewCID(testData)

	if !cid.Equals(expectedCID) {
		t.Error("GenerateChunkCID doesn't match NewCID")
	}
}

func TestCIDStringRoundTrip(t *testing.T) {
	// Test multiple different data inputs
	testCases := [][]byte{
		[]byte(""),
		[]byte("a"),
		[]byte("hello world"),
		[]byte("The quick brown fox jumps over the lazy dog"),
		make([]byte, 1024), // Large data
	}

	for i, testData := range testCases {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			// Create CID
			original := NewCID(testData)

			// Parse it back
			parsed, err := ParseCID(original.String)
			if err != nil {
				t.Fatalf("Failed to parse CID: %v", err)
			}

			// Should be equal
			if !original.Equals(parsed) {
				t.Error("Round-trip failed: CIDs don't match")
			}
		})
	}
}
