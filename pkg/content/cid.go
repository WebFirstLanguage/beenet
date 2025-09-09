package content

import (
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"strings"

	"lukechampine.com/blake3"
)

const (
	// CIDPrefix is the prefix for BeeNet Content Identifiers
	CIDPrefix = "bee"

	// HashSize is the size of BLAKE3-256 hash in bytes
	HashSize = 32

	// CIDVersion is the current CID format version
	CIDVersion = 1
)

// NewCID creates a new CID from data using BLAKE3-256 hashing
func NewCID(data []byte) CID {
	hash := blake3.Sum256(data)
	return CID{
		Hash:   hash[:],
		String: encodeCIDString(hash[:]),
	}
}

// NewCIDFromHash creates a CID from an existing BLAKE3-256 hash
func NewCIDFromHash(hash []byte) (CID, error) {
	if len(hash) != HashSize {
		return CID{}, fmt.Errorf("invalid hash size: got %d, want %d", len(hash), HashSize)
	}

	hashCopy := make([]byte, HashSize)
	copy(hashCopy, hash)

	return CID{
		Hash:   hashCopy,
		String: encodeCIDString(hashCopy),
	}, nil
}

// ParseCID parses a CID string and returns a CID struct
func ParseCID(cidStr string) (CID, error) {
	if cidStr == "" {
		return CID{}, fmt.Errorf("empty CID string")
	}

	// Check prefix
	if !strings.HasPrefix(cidStr, CIDPrefix+":") {
		return CID{}, fmt.Errorf("invalid CID prefix: expected %s:", CIDPrefix)
	}

	// Remove prefix
	withoutPrefix := strings.TrimPrefix(cidStr, CIDPrefix+":")

	// Parse the hash part (base32 encoded)
	hash, err := decodeCIDString(withoutPrefix)
	if err != nil {
		return CID{}, fmt.Errorf("failed to decode CID hash: %w", err)
	}

	if len(hash) != HashSize {
		return CID{}, fmt.Errorf("invalid hash size in CID: got %d, want %d", len(hash), HashSize)
	}

	return CID{
		Hash:   hash,
		String: cidStr,
	}, nil
}

// IsValid checks if a CID is valid
func (c CID) IsValid() bool {
	if len(c.Hash) != HashSize {
		return false
	}

	if c.String == "" {
		return false
	}

	// Verify that the string representation matches the hash
	expectedString := encodeCIDString(c.Hash)
	return c.String == expectedString
}

// Equals checks if two CIDs are equal
func (c CID) Equals(other CID) bool {
	if len(c.Hash) != len(other.Hash) {
		return false
	}

	for i, b := range c.Hash {
		if other.Hash[i] != b {
			return false
		}
	}

	return true
}

// Bytes returns the raw hash bytes
func (c CID) Bytes() []byte {
	result := make([]byte, len(c.Hash))
	copy(result, c.Hash)
	return result
}

// HexString returns the hash as a hex string
func (c CID) HexString() string {
	return hex.EncodeToString(c.Hash)
}

// encodeCIDString encodes a hash as a CID string using base32
func encodeCIDString(hash []byte) string {
	// Use base32 encoding without padding for compact representation
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(hash)
	return fmt.Sprintf("%s:%s", CIDPrefix, strings.ToLower(encoded))
}

// decodeCIDString decodes a CID string (without prefix) back to hash bytes
func decodeCIDString(encoded string) ([]byte, error) {
	// Convert to uppercase for base32 decoding
	upperEncoded := strings.ToUpper(encoded)

	// Decode from base32
	hash, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(upperEncoded)
	if err != nil {
		return nil, fmt.Errorf("base32 decode error: %w", err)
	}

	return hash, nil
}

// ComputeManifestCID computes the CID for a manifest
func ComputeManifestCID(manifest *Manifest) (CID, error) {
	// We need to serialize the manifest to compute its CID
	// For now, we'll use a simple approach - in a full implementation,
	// we might want to use a canonical serialization format

	// Create a deterministic representation of the manifest
	var data []byte

	// Add version
	data = append(data, byte(manifest.Version>>24), byte(manifest.Version>>16),
		byte(manifest.Version>>8), byte(manifest.Version))

	// Add file size
	data = append(data, byte(manifest.FileSize>>56), byte(manifest.FileSize>>48),
		byte(manifest.FileSize>>40), byte(manifest.FileSize>>32),
		byte(manifest.FileSize>>24), byte(manifest.FileSize>>16),
		byte(manifest.FileSize>>8), byte(manifest.FileSize))

	// Add chunk size
	data = append(data, byte(manifest.ChunkSize>>24), byte(manifest.ChunkSize>>16),
		byte(manifest.ChunkSize>>8), byte(manifest.ChunkSize))

	// Add chunk count
	data = append(data, byte(manifest.ChunkCount>>24), byte(manifest.ChunkCount>>16),
		byte(manifest.ChunkCount>>8), byte(manifest.ChunkCount))

	// Add all chunk hashes in order
	for _, chunk := range manifest.Chunks {
		data = append(data, chunk.CID.Hash...)
	}

	// Add content type and filename if present
	if manifest.ContentType != "" {
		data = append(data, []byte(manifest.ContentType)...)
	}
	if manifest.Filename != "" {
		data = append(data, []byte(manifest.Filename)...)
	}

	return NewCID(data), nil
}

// VerifyChunkIntegrity verifies that chunk data matches its CID
func VerifyChunkIntegrity(chunk *Chunk) error {
	expectedCID := NewCID(chunk.Data)
	if !chunk.CID.Equals(expectedCID) {
		return fmt.Errorf("chunk integrity verification failed: expected CID %s, got %s",
			expectedCID.String, chunk.CID.String)
	}
	return nil
}

// GenerateChunkCID generates a CID for chunk data
func GenerateChunkCID(data []byte) CID {
	return NewCID(data)
}
