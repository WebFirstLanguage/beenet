package content

import (
	"testing"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/codec/cborcanon"
)

func TestCIDSerialization(t *testing.T) {
	// Create a test CID
	hash := make([]byte, 32)
	for i := range hash {
		hash[i] = byte(i)
	}
	
	original := CID{
		Hash:   hash,
		String: "test-cid-string",
	}

	// Test CBOR marshaling
	data, err := cborcanon.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal CID: %v", err)
	}

	// Test CBOR unmarshaling
	var unmarshaled CID
	err = cborcanon.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal CID: %v", err)
	}

	// Verify the data matches
	if len(unmarshaled.Hash) != len(original.Hash) {
		t.Errorf("Hash length mismatch: got %d, want %d", len(unmarshaled.Hash), len(original.Hash))
	}
	
	for i, b := range original.Hash {
		if unmarshaled.Hash[i] != b {
			t.Errorf("Hash byte %d mismatch: got %d, want %d", i, unmarshaled.Hash[i], b)
		}
	}
	
	if unmarshaled.String != original.String {
		t.Errorf("String mismatch: got %s, want %s", unmarshaled.String, original.String)
	}
}

func TestChunkSerialization(t *testing.T) {
	// Create a test chunk
	hash := make([]byte, 32)
	for i := range hash {
		hash[i] = byte(i % 256)
	}
	
	original := Chunk{
		CID: CID{
			Hash:   hash,
			String: "chunk-cid",
		},
		Data:   []byte("test chunk data"),
		Size:   15,
		Offset: 1024,
	}

	// Test CBOR marshaling
	data, err := cborcanon.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal Chunk: %v", err)
	}

	// Test CBOR unmarshaling
	var unmarshaled Chunk
	err = cborcanon.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal Chunk: %v", err)
	}

	// Verify the data matches
	if string(unmarshaled.Data) != string(original.Data) {
		t.Errorf("Data mismatch: got %s, want %s", string(unmarshaled.Data), string(original.Data))
	}
	
	if unmarshaled.Size != original.Size {
		t.Errorf("Size mismatch: got %d, want %d", unmarshaled.Size, original.Size)
	}
	
	if unmarshaled.Offset != original.Offset {
		t.Errorf("Offset mismatch: got %d, want %d", unmarshaled.Offset, original.Offset)
	}
}

func TestManifestSerialization(t *testing.T) {
	// Create test chunk info
	hash1 := make([]byte, 32)
	hash2 := make([]byte, 32)
	for i := range hash1 {
		hash1[i] = byte(i)
		hash2[i] = byte(i + 32)
	}
	
	original := Manifest{
		Version:     1,
		FileSize:    2048,
		ChunkSize:   1024,
		ChunkCount:  2,
		Chunks: []ChunkInfo{
			{
				CID:    CID{Hash: hash1, String: "chunk1"},
				Size:   1024,
				Offset: 0,
			},
			{
				CID:    CID{Hash: hash2, String: "chunk2"},
				Size:   1024,
				Offset: 1024,
			},
		},
		CreatedAt:   uint64(time.Now().UnixMilli()),
		ContentType: "text/plain",
		Filename:    "test.txt",
	}

	// Test CBOR marshaling
	data, err := cborcanon.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal Manifest: %v", err)
	}

	// Test CBOR unmarshaling
	var unmarshaled Manifest
	err = cborcanon.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal Manifest: %v", err)
	}

	// Verify the data matches
	if unmarshaled.Version != original.Version {
		t.Errorf("Version mismatch: got %d, want %d", unmarshaled.Version, original.Version)
	}
	
	if unmarshaled.FileSize != original.FileSize {
		t.Errorf("FileSize mismatch: got %d, want %d", unmarshaled.FileSize, original.FileSize)
	}
	
	if unmarshaled.ChunkSize != original.ChunkSize {
		t.Errorf("ChunkSize mismatch: got %d, want %d", unmarshaled.ChunkSize, original.ChunkSize)
	}
	
	if unmarshaled.ChunkCount != original.ChunkCount {
		t.Errorf("ChunkCount mismatch: got %d, want %d", unmarshaled.ChunkCount, original.ChunkCount)
	}
	
	if len(unmarshaled.Chunks) != len(original.Chunks) {
		t.Errorf("Chunks length mismatch: got %d, want %d", len(unmarshaled.Chunks), len(original.Chunks))
	}
	
	if unmarshaled.ContentType != original.ContentType {
		t.Errorf("ContentType mismatch: got %s, want %s", unmarshaled.ContentType, original.ContentType)
	}
	
	if unmarshaled.Filename != original.Filename {
		t.Errorf("Filename mismatch: got %s, want %s", unmarshaled.Filename, original.Filename)
	}
}

func TestProvideRecordSerialization(t *testing.T) {
	// Create a test provide record
	hash := make([]byte, 32)
	for i := range hash {
		hash[i] = byte(i)
	}
	
	original := ProvideRecord{
		CID: CID{
			Hash:   hash,
			String: "provide-cid",
		},
		Provider:  "test-provider-bid",
		Addresses: []string{"/ip4/127.0.0.1/tcp/8080", "/ip6/::1/tcp/8080"},
		Timestamp: uint64(time.Now().UnixMilli()),
		TTL:       3600,
		Signature: []byte("test-signature"),
	}

	// Test CBOR marshaling
	data, err := cborcanon.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal ProvideRecord: %v", err)
	}

	// Test CBOR unmarshaling
	var unmarshaled ProvideRecord
	err = cborcanon.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal ProvideRecord: %v", err)
	}

	// Verify the data matches
	if unmarshaled.Provider != original.Provider {
		t.Errorf("Provider mismatch: got %s, want %s", unmarshaled.Provider, original.Provider)
	}
	
	if len(unmarshaled.Addresses) != len(original.Addresses) {
		t.Errorf("Addresses length mismatch: got %d, want %d", len(unmarshaled.Addresses), len(original.Addresses))
	}
	
	for i, addr := range original.Addresses {
		if unmarshaled.Addresses[i] != addr {
			t.Errorf("Address %d mismatch: got %s, want %s", i, unmarshaled.Addresses[i], addr)
		}
	}
	
	if unmarshaled.TTL != original.TTL {
		t.Errorf("TTL mismatch: got %d, want %d", unmarshaled.TTL, original.TTL)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	if config.ChunkSize != 1024*1024 {
		t.Errorf("Default ChunkSize mismatch: got %d, want %d", config.ChunkSize, 1024*1024)
	}
	
	if config.ConcurrentFetches != 4 {
		t.Errorf("Default ConcurrentFetches mismatch: got %d, want %d", config.ConcurrentFetches, 4)
	}
	
	if config.FetchTimeout != 30*time.Second {
		t.Errorf("Default FetchTimeout mismatch: got %v, want %v", config.FetchTimeout, 30*time.Second)
	}
	
	if config.ProvideRecordTTL != 3600 {
		t.Errorf("Default ProvideRecordTTL mismatch: got %d, want %d", config.ProvideRecordTTL, 3600)
	}
	
	if !config.EnableIntegrityCheck {
		t.Error("Default EnableIntegrityCheck should be true")
	}
}
