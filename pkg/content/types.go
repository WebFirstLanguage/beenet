// Package content implements content-addressed storage and transfer system
// using BLAKE3-256 hashing for Content Identifiers (CIDs) as specified in Phase 6.
package content

import (
	"time"
)

// CID represents a Content Identifier using BLAKE3-256 hash
type CID struct {
	Hash   []byte `cbor:"hash"`   // BLAKE3-256 hash (32 bytes)
	String string `cbor:"string"` // String representation for easy handling
}

// Chunk represents a single chunk of content data
type Chunk struct {
	CID    CID    `cbor:"cid"`    // Content identifier for this chunk
	Data   []byte `cbor:"data"`   // Raw chunk data
	Size   uint64 `cbor:"size"`   // Size of the chunk in bytes
	Offset uint64 `cbor:"offset"` // Offset within the original file
}

// ChunkInfo represents metadata about a chunk without the actual data
type ChunkInfo struct {
	CID    CID    `cbor:"cid"`    // Content identifier for this chunk
	Size   uint64 `cbor:"size"`   // Size of the chunk in bytes
	Offset uint64 `cbor:"offset"` // Offset within the original file
}

// Manifest represents a file manifest that maps chunks to their CIDs
type Manifest struct {
	Version     uint32      `cbor:"version"`      // Manifest format version
	FileSize    uint64      `cbor:"file_size"`    // Total size of the original file
	ChunkSize   uint32      `cbor:"chunk_size"`   // Size of each chunk (except possibly the last)
	ChunkCount  uint32      `cbor:"chunk_count"`  // Total number of chunks
	Chunks      []ChunkInfo `cbor:"chunks"`       // Ordered list of chunk information
	CreatedAt   uint64      `cbor:"created_at"`   // Creation timestamp (Unix milliseconds)
	ContentType string      `cbor:"content_type"` // MIME type of the original file (optional)
	Filename    string      `cbor:"filename"`     // Original filename (optional)
}

// ProvideRecord represents a record indicating that a node provides specific content
type ProvideRecord struct {
	CID       CID      `cbor:"cid"`       // Content identifier being provided
	Provider  string   `cbor:"provider"`  // BID of the providing node
	Addresses []string `cbor:"addresses"` // Network addresses where content can be fetched
	Timestamp uint64   `cbor:"timestamp"` // When this record was created (Unix milliseconds)
	TTL       uint32   `cbor:"ttl"`       // Time-to-live in seconds
	Signature []byte   `cbor:"signature"` // Ed25519 signature over the record
}

// FetchRequest represents a request to fetch a specific chunk
type FetchRequest struct {
	CID      CID    `cbor:"cid"`      // Content identifier to fetch
	Provider string `cbor:"provider"` // BID of the provider to fetch from
	Priority uint8  `cbor:"priority"` // Request priority (0-255, higher = more urgent)
	Timeout  uint32 `cbor:"timeout"`  // Timeout in milliseconds
}

// FetchResponse represents the response to a fetch request
type FetchResponse struct {
	CID   CID    `cbor:"cid"`   // Content identifier that was requested
	Data  []byte `cbor:"data"`  // The actual chunk data (nil if error)
	Error string `cbor:"error"` // Error message if fetch failed
}

// ContentStats represents statistics about content operations
type ContentStats struct {
	TotalChunks     uint64 `cbor:"total_chunks"`     // Total chunks processed
	TotalBytes      uint64 `cbor:"total_bytes"`      // Total bytes processed
	ActiveFetches   uint32 `cbor:"active_fetches"`   // Currently active fetch operations
	SuccessfulGets  uint64 `cbor:"successful_gets"`  // Number of successful get operations
	FailedGets      uint64 `cbor:"failed_gets"`      // Number of failed get operations
	SuccessfulPuts  uint64 `cbor:"successful_puts"`  // Number of successful put operations
	FailedPuts      uint64 `cbor:"failed_puts"`      // Number of failed put operations
	CacheHits       uint64 `cbor:"cache_hits"`       // Number of cache hits
	CacheMisses     uint64 `cbor:"cache_misses"`     // Number of cache misses
	NetworkErrors   uint64 `cbor:"network_errors"`   // Number of network-related errors
	IntegrityErrors uint64 `cbor:"integrity_errors"` // Number of integrity verification failures
}

// ChunkStore represents an interface for storing and retrieving chunks
type ChunkStore interface {
	// Put stores a chunk in the store
	Put(chunk *Chunk) error

	// Get retrieves a chunk by its CID
	Get(cid CID) (*Chunk, error)

	// Has checks if a chunk exists in the store
	Has(cid CID) bool

	// Delete removes a chunk from the store
	Delete(cid CID) error

	// List returns all CIDs in the store
	List() ([]CID, error)

	// Stats returns storage statistics
	Stats() (*ContentStats, error)
}

// ContentService represents the main interface for content operations
type ContentService interface {
	// Put processes a file and returns its manifest CID
	Put(filepath string) (CID, error)

	// Get retrieves content by CID and reconstructs the original file
	Get(cid CID, outputPath string) error

	// Publish announces that this node provides the given content
	Publish(cid CID) error

	// Lookup finds providers for the given content
	Lookup(cid CID) ([]*ProvideRecord, error)

	// Stats returns service statistics
	Stats() (*ContentStats, error)
}

// Config represents configuration for the content service
type Config struct {
	ChunkSize            uint32        `json:"chunk_size"`             // Size of each chunk in bytes (default: 1MiB)
	ConcurrentFetches    uint32        `json:"concurrent_fetches"`     // Max concurrent fetch operations (default: 4)
	FetchTimeout         time.Duration `json:"fetch_timeout"`          // Timeout for individual fetch operations
	ProvideRecordTTL     uint32        `json:"provide_record_ttl"`     // TTL for provide records in seconds
	MaxCacheSize         uint64        `json:"max_cache_size"`         // Maximum cache size in bytes
	EnableIntegrityCheck bool          `json:"enable_integrity_check"` // Whether to verify chunk integrity
	StorePath            string        `json:"store_path"`             // Path for local chunk storage
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		ChunkSize:            1024 * 1024, // 1 MiB
		ConcurrentFetches:    4,
		FetchTimeout:         30 * time.Second,
		ProvideRecordTTL:     3600,              // 1 hour
		MaxCacheSize:         100 * 1024 * 1024, // 100 MiB
		EnableIntegrityCheck: true,
		StorePath:            "./chunks",
	}
}
