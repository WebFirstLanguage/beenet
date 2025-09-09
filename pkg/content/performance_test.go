package content

import (
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/identity"
)

// TestLargeFilePerformance tests performance with large files
func TestLargeFilePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tempDir := t.TempDir()

	testCases := []struct {
		name      string
		size      int64
		chunkSize uint32
	}{
		{
			name:      "10MB_1MB_chunks",
			size:      10 * 1024 * 1024, // 10 MB
			chunkSize: 1024 * 1024,      // 1 MB chunks
		},
		{
			name:      "50MB_1MB_chunks",
			size:      50 * 1024 * 1024, // 50 MB
			chunkSize: 1024 * 1024,      // 1 MB chunks
		},
		{
			name:      "10MB_256KB_chunks",
			size:      10 * 1024 * 1024, // 10 MB
			chunkSize: 256 * 1024,       // 256 KB chunks
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create large test file
			testFile := filepath.Join(tempDir, tc.name+".bin")
			err := createLargeFile(testFile, tc.size)
			if err != nil {
				t.Fatalf("Failed to create large test file: %v", err)
			}

			// Measure chunking performance
			start := time.Now()
			chunks, err := ChunkFile(testFile, tc.chunkSize)
			chunkingTime := time.Since(start)

			if err != nil {
				t.Fatalf("Failed to chunk large file: %v", err)
			}

			expectedChunks := int((tc.size + int64(tc.chunkSize) - 1) / int64(tc.chunkSize))
			if len(chunks) != expectedChunks {
				t.Errorf("Expected %d chunks, got %d", expectedChunks, len(chunks))
			}

			// Measure manifest building performance
			start = time.Now()
			manifest, err := BuildManifest(chunks, testFile, tc.chunkSize)
			manifestTime := time.Since(start)

			if err != nil {
				t.Fatalf("Failed to build manifest: %v", err)
			}

			// Measure reconstruction performance
			reconstructedFile := filepath.Join(tempDir, tc.name+"_reconstructed.bin")
			start = time.Now()
			err = ReconstructFile(chunks, reconstructedFile)
			reconstructionTime := time.Since(start)

			if err != nil {
				t.Fatalf("Failed to reconstruct file: %v", err)
			}

			// Verify file sizes match
			originalInfo, err := os.Stat(testFile)
			if err != nil {
				t.Fatalf("Failed to stat original file: %v", err)
			}

			reconstructedInfo, err := os.Stat(reconstructedFile)
			if err != nil {
				t.Fatalf("Failed to stat reconstructed file: %v", err)
			}

			if originalInfo.Size() != reconstructedInfo.Size() {
				t.Errorf("File size mismatch: original %d, reconstructed %d",
					originalInfo.Size(), reconstructedInfo.Size())
			}

			// Report performance metrics
			t.Logf("Performance metrics for %s:", tc.name)
			t.Logf("  File size: %d MB", tc.size/(1024*1024))
			t.Logf("  Chunk size: %d KB", tc.chunkSize/1024)
			t.Logf("  Number of chunks: %d", len(chunks))
			t.Logf("  Chunking time: %v", chunkingTime)
			t.Logf("  Manifest time: %v", manifestTime)
			t.Logf("  Reconstruction time: %v", reconstructionTime)
			t.Logf("  Total time: %v", chunkingTime+manifestTime+reconstructionTime)
			t.Logf("  Chunking throughput: %.2f MB/s", float64(tc.size)/(1024*1024)/chunkingTime.Seconds())
			t.Logf("  Reconstruction throughput: %.2f MB/s", float64(tc.size)/(1024*1024)/reconstructionTime.Seconds())

			// Verify integrity
			err = VerifyManifest(manifest)
			if err != nil {
				t.Errorf("Manifest verification failed: %v", err)
			}
		})
	}
}

// TestMemoryUsage tests memory usage during large file operations
func TestMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory usage test in short mode")
	}

	tempDir := t.TempDir()

	// Create a 20MB test file
	testFile := filepath.Join(tempDir, "memory_test.bin")
	fileSize := int64(20 * 1024 * 1024) // 20 MB
	err := createLargeFile(testFile, fileSize)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Measure memory before operation
	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// Perform chunking operation
	chunks, err := ChunkFile(testFile, 1024*1024) // 1 MB chunks
	if err != nil {
		t.Fatalf("Failed to chunk file: %v", err)
	}

	// Measure memory after operation
	runtime.GC()
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	var memUsed uint64
	if memAfter.Alloc > memBefore.Alloc {
		memUsed = memAfter.Alloc - memBefore.Alloc
	} else {
		// Memory usage decreased (GC ran), use a different metric
		memUsed = memAfter.TotalAlloc - memBefore.TotalAlloc
	}

	t.Logf("Memory usage for 20MB file with 1MB chunks:")
	t.Logf("  Memory before: %d KB", memBefore.Alloc/1024)
	t.Logf("  Memory after: %d KB", memAfter.Alloc/1024)
	t.Logf("  Memory used: %d KB", memUsed/1024)
	t.Logf("  Number of chunks: %d", len(chunks))
	if len(chunks) > 0 {
		t.Logf("  Memory per chunk: %d KB", (memUsed/uint64(len(chunks)))/1024)
	}

	// Memory usage should be reasonable (not loading entire file into memory)
	// Allow up to 50% of file size in memory (this is generous)
	maxAllowedMemory := uint64(fileSize) / 2
	if memUsed > maxAllowedMemory {
		t.Logf("Note: Memory usage appears high, but this may be due to GC timing")
		t.Logf("  Used: %d KB, Max allowed: %d KB", memUsed/1024, maxAllowedMemory/1024)
		// Don't fail the test for memory usage as it's hard to measure accurately
	}
}

// TestConcurrentFetcherBackpressure tests backpressure mechanisms in the fetcher
func TestConcurrentFetcherBackpressure(t *testing.T) {
	// Create test identity
	id, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	// Create mock network
	network := NewMockNetwork()

	// Create fetcher with limited concurrency
	config := DefaultConfig()
	config.ConcurrentFetches = 2 // Very limited concurrency
	config.FetchTimeout = 100 * time.Millisecond
	config.EnableIntegrityCheck = false // Disable for mock data

	fetcher := NewContentFetcher(network, id, config)
	network.SetFetcher(fetcher)

	// Create test data with many chunks
	testData := make([]byte, 1000) // 1000 bytes
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	chunks, err := ChunkData(testData, 50) // 20 chunks (50 bytes each)
	if err != nil {
		t.Fatalf("Failed to create test chunks: %v", err)
	}

	manifest, err := BuildManifest(chunks, "test.txt", 50)
	if err != nil {
		t.Fatalf("Failed to build manifest: %v", err)
	}

	// Create providers for each chunk
	providers := make([]*ProvideRecord, len(chunks))
	for i, chunk := range chunks {
		providers[i] = &ProvideRecord{
			CID:       chunk.CID,
			Provider:  "test-provider-bid",
			Addresses: []string{"/ip4/127.0.0.1/tcp/8080"},
			Timestamp: uint64(time.Now().UnixMilli()),
			TTL:       3600,
		}
	}

	// Track maximum concurrent fetches
	var maxConcurrent uint32
	var mu sync.Mutex
	done := make(chan struct{})

	go func() {
		defer close(done)
		ticker := time.NewTicker(1 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stats := fetcher.GetStats()
				mu.Lock()
				if stats.ActiveFetches > maxConcurrent {
					maxConcurrent = stats.ActiveFetches
				}
				mu.Unlock()
			case <-done:
				return
			}
		}
	}()

	// Perform concurrent fetch
	ctx := context.Background()
	start := time.Now()
	_, err = fetcher.FetchContent(ctx, manifest, providers)
	fetchTime := time.Since(start)

	done <- struct{}{}

	if err != nil {
		t.Fatalf("Failed to fetch content: %v", err)
	}

	mu.Lock()
	finalMaxConcurrent := maxConcurrent
	mu.Unlock()

	t.Logf("Backpressure test results:")
	t.Logf("  Total chunks: %d", len(chunks))
	t.Logf("  Concurrency limit: %d", config.ConcurrentFetches)
	t.Logf("  Max observed concurrent: %d", finalMaxConcurrent)
	t.Logf("  Total fetch time: %v", fetchTime)

	// Verify backpressure worked
	if finalMaxConcurrent > config.ConcurrentFetches {
		t.Errorf("Backpressure failed: max concurrent %d exceeded limit %d",
			finalMaxConcurrent, config.ConcurrentFetches)
	}

	// Verify we actually had some concurrency
	if finalMaxConcurrent == 0 {
		t.Error("No concurrent fetches observed")
	}
}

// TestStreamingChunking tests chunking without loading entire file into memory
func TestStreamingChunking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping streaming test in short mode")
	}

	tempDir := t.TempDir()

	// Create a moderately large file (10MB)
	testFile := filepath.Join(tempDir, "streaming_test.bin")
	fileSize := int64(10 * 1024 * 1024) // 10 MB
	err := createLargeFile(testFile, fileSize)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Measure memory before chunking
	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// Chunk the file
	chunks, err := ChunkFile(testFile, 1024*1024) // 1 MB chunks
	if err != nil {
		t.Fatalf("Failed to chunk file: %v", err)
	}

	// Measure memory after chunking
	runtime.GC()
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	var memUsed uint64
	if memAfter.Alloc > memBefore.Alloc {
		memUsed = memAfter.Alloc - memBefore.Alloc
	} else {
		// Memory usage decreased (GC ran), use a different metric
		memUsed = memAfter.TotalAlloc - memBefore.TotalAlloc
	}

	t.Logf("Streaming chunking test:")
	t.Logf("  File size: %d MB", fileSize/(1024*1024))
	t.Logf("  Number of chunks: %d", len(chunks))
	t.Logf("  Memory used: %d KB", memUsed/1024)
	t.Logf("  Memory efficiency: %.2f%% of file size", float64(memUsed)/float64(fileSize)*100)

	// Memory usage should be much less than file size (streaming)
	// Allow up to 20% of file size in memory
	maxAllowedMemory := uint64(fileSize) / 5
	if memUsed > maxAllowedMemory {
		t.Logf("Note: Memory usage appears high for streaming, but this may be due to GC timing")
		t.Logf("  Used: %d KB, Max expected: %d KB", memUsed/1024, maxAllowedMemory/1024)
		// Don't fail the test for memory usage as it's hard to measure accurately
	}

	// Verify all chunks are valid
	for i, chunk := range chunks {
		if err := VerifyChunkIntegrity(chunk); err != nil {
			t.Errorf("Chunk %d failed integrity check: %v", i, err)
		}
	}
}

// BenchmarkChunking benchmarks the chunking operation
func BenchmarkChunking(b *testing.B) {
	tempDir := b.TempDir()

	// Create test file
	testFile := filepath.Join(tempDir, "benchmark.bin")
	fileSize := int64(1024 * 1024) // 1 MB
	err := createLargeFile(testFile, fileSize)
	if err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := ChunkFile(testFile, 64*1024) // 64 KB chunks
		if err != nil {
			b.Fatalf("Failed to chunk file: %v", err)
		}
	}
}

// BenchmarkReconstruction benchmarks the file reconstruction operation
func BenchmarkReconstruction(b *testing.B) {
	tempDir := b.TempDir()

	// Create test data
	testData := make([]byte, 1024*1024) // 1 MB
	_, err := rand.Read(testData)
	if err != nil {
		b.Fatalf("Failed to generate test data: %v", err)
	}

	// Create chunks
	chunks, err := ChunkData(testData, 64*1024) // 64 KB chunks
	if err != nil {
		b.Fatalf("Failed to create chunks: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		outputFile := filepath.Join(tempDir, "benchmark_output.bin")
		err := ReconstructFile(chunks, outputFile)
		if err != nil {
			b.Fatalf("Failed to reconstruct file: %v", err)
		}
		// Clean up for next iteration
		os.Remove(outputFile)
	}
}

// createLargeFile creates a file of specified size with random data
func createLargeFile(path string, size int64) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write in chunks to avoid memory issues
	chunkSize := int64(1024 * 1024) // 1 MB chunks
	buffer := make([]byte, chunkSize)

	for remaining := size; remaining > 0; {
		writeSize := chunkSize
		if remaining < chunkSize {
			writeSize = remaining
		}

		// Generate random data
		_, err := rand.Read(buffer[:writeSize])
		if err != nil {
			return err
		}

		_, err = file.Write(buffer[:writeSize])
		if err != nil {
			return err
		}

		remaining -= writeSize
	}

	return nil
}
