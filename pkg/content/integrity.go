package content

import (
	"crypto/sha256"
	"fmt"
	"os"
)

// IntegrityReport represents the result of an integrity verification
type IntegrityReport struct {
	Valid            bool                   `json:"valid"`
	ManifestCIDValid bool                   `json:"manifest_cid_valid"`
	ChunkIntegrity   []ChunkIntegrityResult `json:"chunk_integrity"`
	FileIntegrity    *FileIntegrityResult   `json:"file_integrity,omitempty"`
	Errors           []string               `json:"errors,omitempty"`
	TotalChunks      int                    `json:"total_chunks"`
	ValidChunks      int                    `json:"valid_chunks"`
	TotalBytes       uint64                 `json:"total_bytes"`
	VerificationTime int64                  `json:"verification_time_ms"`
}

// ChunkIntegrityResult represents the integrity check result for a single chunk
type ChunkIntegrityResult struct {
	Index       int    `json:"index"`
	CID         string `json:"cid"`
	Valid       bool   `json:"valid"`
	ExpectedCID string `json:"expected_cid,omitempty"`
	Error       string `json:"error,omitempty"`
	Size        uint64 `json:"size"`
	Offset      uint64 `json:"offset"`
}

// FileIntegrityResult represents the integrity check result for the reconstructed file
type FileIntegrityResult struct {
	Valid          bool   `json:"valid"`
	ExpectedSHA256 string `json:"expected_sha256,omitempty"`
	ActualSHA256   string `json:"actual_sha256,omitempty"`
	ExpectedSize   uint64 `json:"expected_size"`
	ActualSize     uint64 `json:"actual_size"`
	Error          string `json:"error,omitempty"`
}

// VerifyContentIntegrity performs comprehensive integrity verification of content
func VerifyContentIntegrity(manifest *Manifest, chunks []*Chunk, expectedManifestCID *CID) *IntegrityReport {
	report := &IntegrityReport{
		Valid:          true,
		TotalChunks:    len(chunks),
		ChunkIntegrity: make([]ChunkIntegrityResult, len(chunks)),
		Errors:         make([]string, 0),
	}

	// Step 1: Verify manifest CID if provided
	if expectedManifestCID != nil {
		if err := ValidateManifestCID(manifest, *expectedManifestCID); err != nil {
			report.ManifestCIDValid = false
			report.Valid = false
			report.Errors = append(report.Errors, fmt.Sprintf("Manifest CID validation failed: %v", err))
		} else {
			report.ManifestCIDValid = true
		}
	}

	// Step 2: Verify manifest structure
	if err := VerifyManifest(manifest); err != nil {
		report.Valid = false
		report.Errors = append(report.Errors, fmt.Sprintf("Manifest verification failed: %v", err))
	}

	// Step 3: Verify each chunk integrity
	for i, chunk := range chunks {
		result := ChunkIntegrityResult{
			Index:  i,
			CID:    chunk.CID.String,
			Size:   chunk.Size,
			Offset: chunk.Offset,
		}

		if err := VerifyChunkIntegrity(chunk); err != nil {
			result.Valid = false
			result.Error = err.Error()

			// Calculate what the CID should be
			expectedCID := NewCID(chunk.Data)
			result.ExpectedCID = expectedCID.String

			report.Valid = false
		} else {
			result.Valid = true
			report.ValidChunks++
		}

		report.ChunkIntegrity[i] = result
		report.TotalBytes += chunk.Size
	}

	// Step 4: Verify manifest against chunks
	if err := VerifyManifestWithChunks(manifest, chunks); err != nil {
		report.Valid = false
		report.Errors = append(report.Errors, fmt.Sprintf("Manifest-chunk verification failed: %v", err))
	}

	return report
}

// VerifyReconstructedFile verifies that a reconstructed file matches the original
func VerifyReconstructedFile(filePath string, expectedSize uint64, originalSHA256 string) *FileIntegrityResult {
	result := &FileIntegrityResult{
		ExpectedSize:   expectedSize,
		ExpectedSHA256: originalSHA256,
	}

	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to stat file: %v", err)
		return result
	}

	result.ActualSize = uint64(fileInfo.Size())

	// Check file size
	if result.ActualSize != result.ExpectedSize {
		result.Error = fmt.Sprintf("File size mismatch: expected %d, got %d",
			result.ExpectedSize, result.ActualSize)
		return result
	}

	// Calculate SHA256 hash if expected hash is provided
	if originalSHA256 != "" {
		file, err := os.Open(filePath)
		if err != nil {
			result.Error = fmt.Sprintf("Failed to open file for hashing: %v", err)
			return result
		}
		defer file.Close()

		hasher := sha256.New()
		buffer := make([]byte, 64*1024) // 64KB buffer

		for {
			n, err := file.Read(buffer)
			if n > 0 {
				hasher.Write(buffer[:n])
			}
			if err != nil {
				if err.Error() != "EOF" {
					result.Error = fmt.Sprintf("Failed to read file for hashing: %v", err)
					return result
				}
				break
			}
		}

		result.ActualSHA256 = fmt.Sprintf("%x", hasher.Sum(nil))

		if result.ActualSHA256 != originalSHA256 {
			result.Error = fmt.Sprintf("SHA256 hash mismatch: expected %s, got %s",
				originalSHA256, result.ActualSHA256)
			return result
		}
	}

	result.Valid = true
	return result
}

// VerifyEndToEndIntegrity performs complete end-to-end integrity verification
func VerifyEndToEndIntegrity(originalFilePath, reconstructedFilePath string, manifest *Manifest, chunks []*Chunk, manifestCID CID) (*IntegrityReport, error) {
	// Start with content integrity verification
	report := VerifyContentIntegrity(manifest, chunks, &manifestCID)

	// Calculate original file hash for comparison
	var originalSHA256 string
	if originalFilePath != "" {
		if _, err := os.Stat(originalFilePath); err == nil {
			if file, err := os.Open(originalFilePath); err == nil {
				defer file.Close()
				hasher := sha256.New()
				buffer := make([]byte, 64*1024)

				for {
					n, err := file.Read(buffer)
					if n > 0 {
						hasher.Write(buffer[:n])
					}
					if err != nil {
						if err.Error() == "EOF" {
							break
						}
						return report, fmt.Errorf("failed to hash original file: %w", err)
					}
				}

				originalSHA256 = fmt.Sprintf("%x", hasher.Sum(nil))
			}
		}
	}

	// Verify reconstructed file if path is provided
	if reconstructedFilePath != "" {
		fileResult := VerifyReconstructedFile(reconstructedFilePath, manifest.FileSize, originalSHA256)
		report.FileIntegrity = fileResult

		if !fileResult.Valid {
			report.Valid = false
			if fileResult.Error != "" {
				report.Errors = append(report.Errors, fmt.Sprintf("File integrity check failed: %s", fileResult.Error))
			}
		}
	}

	return report, nil
}

// VerifyChunkSequence verifies that chunks form a valid sequence
func VerifyChunkSequence(chunks []*Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Sort chunks by offset
	sortedChunks := make([]*Chunk, len(chunks))
	copy(sortedChunks, chunks)

	// Simple bubble sort by offset (fine for small arrays)
	for i := 0; i < len(sortedChunks)-1; i++ {
		for j := 0; j < len(sortedChunks)-i-1; j++ {
			if sortedChunks[j].Offset > sortedChunks[j+1].Offset {
				sortedChunks[j], sortedChunks[j+1] = sortedChunks[j+1], sortedChunks[j]
			}
		}
	}

	// Verify sequence
	var expectedOffset uint64 = 0
	for i, chunk := range sortedChunks {
		if chunk.Offset != expectedOffset {
			return fmt.Errorf("chunk %d has invalid offset: expected %d, got %d",
				i, expectedOffset, chunk.Offset)
		}

		if chunk.Size == 0 {
			return fmt.Errorf("chunk %d has zero size", i)
		}

		expectedOffset += chunk.Size
	}

	return nil
}

// VerifyManifestChunkConsistency verifies that manifest chunk info matches actual chunks
func VerifyManifestChunkConsistency(manifest *Manifest, chunks []*Chunk) error {
	if len(manifest.Chunks) != len(chunks) {
		return fmt.Errorf("chunk count mismatch: manifest has %d, provided %d",
			len(manifest.Chunks), len(chunks))
	}

	// Create a map of chunks by CID for quick lookup
	chunkMap := make(map[string]*Chunk)
	for _, chunk := range chunks {
		chunkMap[chunk.CID.String] = chunk
	}

	// Verify each manifest chunk has a corresponding actual chunk
	for i, manifestChunk := range manifest.Chunks {
		actualChunk, exists := chunkMap[manifestChunk.CID.String]
		if !exists {
			return fmt.Errorf("manifest chunk %d (CID: %s) not found in provided chunks",
				i, manifestChunk.CID.String)
		}

		if manifestChunk.Size != actualChunk.Size {
			return fmt.Errorf("chunk %d size mismatch: manifest says %d, chunk has %d",
				i, manifestChunk.Size, actualChunk.Size)
		}

		if manifestChunk.Offset != actualChunk.Offset {
			return fmt.Errorf("chunk %d offset mismatch: manifest says %d, chunk has %d",
				i, manifestChunk.Offset, actualChunk.Offset)
		}
	}

	return nil
}
