package content

import (
	"fmt"
	"mime"
	"path/filepath"
	"sort"
	"time"
)

// BuildManifest creates a manifest from a list of chunks
func BuildManifest(chunks []*Chunk, originalFilePath string, chunkSize uint32) (*Manifest, error) {
	if chunkSize == 0 {
		return nil, fmt.Errorf("chunk size cannot be zero")
	}

	// Calculate total file size
	var fileSize uint64 = 0
	for _, chunk := range chunks {
		fileSize += chunk.Size
	}

	// Create chunk info list
	chunkInfos := make([]ChunkInfo, len(chunks))
	for i, chunk := range chunks {
		chunkInfos[i] = ChunkInfo{
			CID:    chunk.CID,
			Size:   chunk.Size,
			Offset: chunk.Offset,
		}
	}

	// Sort chunks by offset to ensure correct order
	sort.Slice(chunkInfos, func(i, j int) bool {
		return chunkInfos[i].Offset < chunkInfos[j].Offset
	})

	// Determine content type from file extension
	contentType := ""
	filename := ""
	if originalFilePath != "" {
		filename = filepath.Base(originalFilePath)
		ext := filepath.Ext(originalFilePath)
		if ext != "" {
			contentType = mime.TypeByExtension(ext)
		}
	}

	manifest := &Manifest{
		Version:     1,
		FileSize:    fileSize,
		ChunkSize:   chunkSize,
		ChunkCount:  uint32(len(chunks)),
		Chunks:      chunkInfos,
		CreatedAt:   uint64(time.Now().UnixMilli()),
		ContentType: contentType,
		Filename:    filename,
	}

	return manifest, nil
}

// BuildManifestFromFile creates a manifest by chunking a file
func BuildManifestFromFile(filePath string, chunkSize uint32) (*Manifest, CID, error) {
	// Chunk the file
	chunks, err := ChunkFile(filePath, chunkSize)
	if err != nil {
		return nil, CID{}, fmt.Errorf("failed to chunk file: %w", err)
	}

	// Build manifest
	manifest, err := BuildManifest(chunks, filePath, chunkSize)
	if err != nil {
		return nil, CID{}, fmt.Errorf("failed to build manifest: %w", err)
	}

	// Compute manifest CID
	manifestCID, err := ComputeManifestCID(manifest)
	if err != nil {
		return nil, CID{}, fmt.Errorf("failed to compute manifest CID: %w", err)
	}

	return manifest, manifestCID, nil
}

// BuildManifestFromData creates a manifest by chunking raw data
func BuildManifestFromData(data []byte, chunkSize uint32, filename string) (*Manifest, CID, error) {
	// Chunk the data
	chunks, err := ChunkData(data, chunkSize)
	if err != nil {
		return nil, CID{}, fmt.Errorf("failed to chunk data: %w", err)
	}

	// Build manifest
	manifest, err := BuildManifest(chunks, filename, chunkSize)
	if err != nil {
		return nil, CID{}, fmt.Errorf("failed to build manifest: %w", err)
	}

	// Compute manifest CID
	manifestCID, err := ComputeManifestCID(manifest)
	if err != nil {
		return nil, CID{}, fmt.Errorf("failed to compute manifest CID: %w", err)
	}

	return manifest, manifestCID, nil
}

// VerifyManifest validates the integrity and consistency of a manifest
func VerifyManifest(manifest *Manifest) error {
	if manifest == nil {
		return fmt.Errorf("manifest is nil")
	}

	// Check version
	if manifest.Version == 0 {
		return fmt.Errorf("invalid manifest version: %d", manifest.Version)
	}

	// Check chunk size
	if manifest.ChunkSize == 0 {
		return fmt.Errorf("invalid chunk size: %d", manifest.ChunkSize)
	}

	// Check chunk count matches actual chunks
	if uint32(len(manifest.Chunks)) != manifest.ChunkCount {
		return fmt.Errorf("chunk count mismatch: manifest says %d, but has %d chunks",
			manifest.ChunkCount, len(manifest.Chunks))
	}

	// Verify chunks are in correct order and have valid offsets
	var expectedOffset uint64 = 0
	var totalSize uint64 = 0

	for i, chunk := range manifest.Chunks {
		// Check offset
		if chunk.Offset != expectedOffset {
			return fmt.Errorf("chunk %d has invalid offset: got %d, expected %d",
				i, chunk.Offset, expectedOffset)
		}

		// Check size
		if chunk.Size == 0 {
			return fmt.Errorf("chunk %d has zero size", i)
		}

		// Check CID is valid
		if !chunk.CID.IsValid() {
			return fmt.Errorf("chunk %d has invalid CID", i)
		}

		// For all chunks except the last, size should match chunk size
		if i < len(manifest.Chunks)-1 && chunk.Size != uint64(manifest.ChunkSize) {
			return fmt.Errorf("chunk %d has invalid size: got %d, expected %d",
				i, chunk.Size, manifest.ChunkSize)
		}

		// Last chunk can be smaller than chunk size
		if i == len(manifest.Chunks)-1 && chunk.Size > uint64(manifest.ChunkSize) {
			return fmt.Errorf("last chunk %d is too large: got %d, max %d",
				i, chunk.Size, manifest.ChunkSize)
		}

		expectedOffset += chunk.Size
		totalSize += chunk.Size
	}

	// Check total size matches file size
	if totalSize != manifest.FileSize {
		return fmt.Errorf("file size mismatch: manifest says %d, chunks total %d",
			manifest.FileSize, totalSize)
	}

	return nil
}

// VerifyManifestWithChunks verifies a manifest against actual chunk data
func VerifyManifestWithChunks(manifest *Manifest, chunks []*Chunk) error {
	// First verify the manifest itself
	if err := VerifyManifest(manifest); err != nil {
		return fmt.Errorf("manifest verification failed: %w", err)
	}

	// Check chunk count matches
	if len(chunks) != len(manifest.Chunks) {
		return fmt.Errorf("chunk count mismatch: manifest has %d, provided %d",
			len(manifest.Chunks), len(chunks))
	}

	// Sort chunks by offset for comparison
	sortedChunks := make([]*Chunk, len(chunks))
	copy(sortedChunks, chunks)
	sort.Slice(sortedChunks, func(i, j int) bool {
		return sortedChunks[i].Offset < sortedChunks[j].Offset
	})

	// Verify each chunk matches the manifest
	for i, manifestChunk := range manifest.Chunks {
		actualChunk := sortedChunks[i]

		// Check CID matches
		if !manifestChunk.CID.Equals(actualChunk.CID) {
			return fmt.Errorf("chunk %d CID mismatch: manifest has %s, chunk has %s",
				i, manifestChunk.CID.String, actualChunk.CID.String)
		}

		// Check size matches
		if manifestChunk.Size != actualChunk.Size {
			return fmt.Errorf("chunk %d size mismatch: manifest has %d, chunk has %d",
				i, manifestChunk.Size, actualChunk.Size)
		}

		// Check offset matches
		if manifestChunk.Offset != actualChunk.Offset {
			return fmt.Errorf("chunk %d offset mismatch: manifest has %d, chunk has %d",
				i, manifestChunk.Offset, actualChunk.Offset)
		}

		// Verify chunk integrity
		if err := VerifyChunkIntegrity(actualChunk); err != nil {
			return fmt.Errorf("chunk %d integrity verification failed: %w", i, err)
		}
	}

	return nil
}

// GetManifestStats returns statistics about a manifest
func GetManifestStats(manifest *Manifest) map[string]interface{} {
	if manifest == nil {
		return map[string]interface{}{"error": "manifest is nil"}
	}

	stats := map[string]interface{}{
		"version":      manifest.Version,
		"file_size":    manifest.FileSize,
		"chunk_size":   manifest.ChunkSize,
		"chunk_count":  manifest.ChunkCount,
		"created_at":   manifest.CreatedAt,
		"content_type": manifest.ContentType,
		"filename":     manifest.Filename,
	}

	// Calculate compression ratio (if applicable)
	if manifest.FileSize > 0 {
		// This is a simple calculation - in a real implementation,
		// we might want to consider the actual storage overhead
		overhead := uint64(len(manifest.Chunks)) * 64 // Approximate overhead per chunk
		stats["storage_overhead"] = overhead
		stats["efficiency"] = float64(manifest.FileSize) / float64(manifest.FileSize+overhead)
	}

	return stats
}

// ValidateManifestCID verifies that a manifest matches its claimed CID
func ValidateManifestCID(manifest *Manifest, expectedCID CID) error {
	actualCID, err := ComputeManifestCID(manifest)
	if err != nil {
		return fmt.Errorf("failed to compute manifest CID: %w", err)
	}

	if !actualCID.Equals(expectedCID) {
		return fmt.Errorf("manifest CID mismatch: expected %s, got %s",
			expectedCID.String, actualCID.String)
	}

	return nil
}
