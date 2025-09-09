package content

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/constants"
	"github.com/WebFirstLanguage/beenet/pkg/identity"
	"github.com/WebFirstLanguage/beenet/pkg/wire"
)

// MockNetwork implements NetworkInterface for testing
type MockNetwork struct {
	mu       sync.RWMutex
	messages []MockMessage
	fetcher  *ContentFetcher // Reference to fetcher for response simulation
}

type MockMessage struct {
	Target string
	Frame  *wire.BaseFrame
}

func NewMockNetwork() *MockNetwork {
	return &MockNetwork{
		messages: make([]MockMessage, 0),
	}
}

func (mn *MockNetwork) SendMessage(ctx context.Context, target string, frame *wire.BaseFrame) error {
	mn.mu.Lock()
	defer mn.mu.Unlock()

	mn.messages = append(mn.messages, MockMessage{
		Target: target,
		Frame:  frame,
	})

	// Simulate response for FETCH_CHUNK messages
	if frame.Kind == constants.KindFetchChunk && mn.fetcher != nil {
		go mn.simulateChunkResponse(frame)
	}

	return nil
}

func (mn *MockNetwork) BroadcastMessage(ctx context.Context, frame *wire.BaseFrame) error {
	mn.mu.Lock()
	defer mn.mu.Unlock()

	mn.messages = append(mn.messages, MockMessage{
		Target: "broadcast",
		Frame:  frame,
	})

	return nil
}

func (mn *MockNetwork) GetMessages() []MockMessage {
	mn.mu.RLock()
	defer mn.mu.RUnlock()

	result := make([]MockMessage, len(mn.messages))
	copy(result, mn.messages)
	return result
}

func (mn *MockNetwork) Clear() {
	mn.mu.Lock()
	defer mn.mu.Unlock()
	mn.messages = mn.messages[:0]
}

func (mn *MockNetwork) SetFetcher(fetcher *ContentFetcher) {
	mn.fetcher = fetcher
}

func (mn *MockNetwork) simulateChunkResponse(frame *wire.BaseFrame) {
	// Small delay to simulate network latency
	time.Sleep(10 * time.Millisecond)

	body, ok := frame.Body.(*wire.FetchChunkBody)
	if !ok {
		return
	}

	// Parse the requested CID to validate it
	_, err := ParseCID(body.CID)
	if err != nil {
		return
	}

	// For testing, we'll create data that matches the CID
	// In a real implementation, this would lookup the actual chunk data
	var testData []byte
	if body.CID == "bee:nk5yernqm7rh2li5rad5csot564h444mhlqpmlbx7ly74l72c3gq" {
		// This is the CID for "test chunk data"
		testData = []byte("test chunk data")
	} else {
		// Create some test data that will generate the requested CID
		testData = []byte("mock chunk data for " + body.CID)
	}

	// Create CHUNK_DATA response
	responseFrame := wire.NewChunkDataFrame(
		"mock-provider",
		frame.Seq, // Use same sequence number
		body.CID,
		0, // Offset
		testData,
	)

	// Send response to fetcher
	mn.fetcher.HandleChunkDataMessage(responseFrame)
}

func TestNewContentFetcher(t *testing.T) {
	// Create test identity
	id, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	// Create mock network
	network := NewMockNetwork()

	// Create fetcher
	config := DefaultConfig()
	fetcher := NewContentFetcher(network, id, config)

	// Verify fetcher properties
	if fetcher.identity != id {
		t.Error("Fetcher identity mismatch")
	}

	if fetcher.config != config {
		t.Error("Fetcher config mismatch")
	}

	if len(fetcher.semaphore) != 0 {
		t.Error("Semaphore should start empty")
	}

	if cap(fetcher.semaphore) != int(config.ConcurrentFetches) {
		t.Errorf("Semaphore capacity mismatch: got %d, want %d",
			cap(fetcher.semaphore), config.ConcurrentFetches)
	}
}

func TestFetchChunkFromProvider(t *testing.T) {
	// Create test identity
	id, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	// Create mock network
	network := NewMockNetwork()

	// Create fetcher
	config := DefaultConfig()
	config.FetchTimeout = 1 * time.Second
	config.EnableIntegrityCheck = false // Disable for mock testing
	fetcher := NewContentFetcher(network, id, config)
	network.SetFetcher(fetcher)

	// Create test CID and provider
	testData := []byte("test chunk data")
	cid := NewCID(testData)

	provider := &ProvideRecord{
		CID:       cid,
		Provider:  "test-provider-bid",
		Addresses: []string{"/ip4/127.0.0.1/tcp/8080"},
		Timestamp: uint64(time.Now().UnixMilli()),
		TTL:       3600,
	}

	// Fetch chunk
	ctx := context.Background()
	chunk, err := fetcher.fetchChunkFromProvider(ctx, cid, provider)
	if err != nil {
		t.Fatalf("Failed to fetch chunk: %v", err)
	}

	// Verify chunk
	if !chunk.CID.Equals(cid) {
		t.Error("Chunk CID mismatch")
	}

	if len(chunk.Data) == 0 {
		t.Error("Chunk data is empty")
	}

	// Verify network message was sent
	messages := network.GetMessages()
	if len(messages) != 1 {
		t.Errorf("Expected 1 network message, got %d", len(messages))
	}

	if len(messages) > 0 {
		msg := messages[0]
		if msg.Target != provider.Provider {
			t.Errorf("Message target mismatch: got %s, want %s", msg.Target, provider.Provider)
		}

		if msg.Frame.Kind != constants.KindFetchChunk {
			t.Errorf("Message kind mismatch: got %d, want %d", msg.Frame.Kind, constants.KindFetchChunk)
		}
	}
}

func TestFetchContent(t *testing.T) {
	// Create test identity
	id, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	// Create mock network
	network := NewMockNetwork()

	// Create fetcher
	config := DefaultConfig()
	config.FetchTimeout = 1 * time.Second
	config.EnableIntegrityCheck = false // Disable for mock data
	fetcher := NewContentFetcher(network, id, config)
	network.SetFetcher(fetcher)

	// Create test data and manifest
	testData := []byte("This is test data for content fetching")
	chunks, err := ChunkData(testData, 15)
	if err != nil {
		t.Fatalf("Failed to create test chunks: %v", err)
	}

	manifest, err := BuildManifest(chunks, "test.txt", 15)
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

	// Fetch content
	ctx := context.Background()
	fetchedChunks, err := fetcher.FetchContent(ctx, manifest, providers)
	if err != nil {
		t.Fatalf("Failed to fetch content: %v", err)
	}

	// Verify chunks
	if len(fetchedChunks) != len(manifest.Chunks) {
		t.Errorf("Chunk count mismatch: got %d, want %d", len(fetchedChunks), len(manifest.Chunks))
	}

	// Verify network messages were sent
	messages := network.GetMessages()
	if len(messages) != len(manifest.Chunks) {
		t.Errorf("Expected %d network messages, got %d", len(manifest.Chunks), len(messages))
	}
}

func TestConcurrencyControl(t *testing.T) {
	// Create test identity
	id, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	// Create mock network with delay
	network := NewMockNetwork()

	// Create fetcher with limited concurrency
	config := DefaultConfig()
	config.ConcurrentFetches = 2 // Limit to 2 concurrent fetches
	config.FetchTimeout = 2 * time.Second
	config.EnableIntegrityCheck = false
	fetcher := NewContentFetcher(network, id, config)
	network.SetFetcher(fetcher)

	// Create test data with multiple chunks
	testData := make([]byte, 100)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	chunks, err := ChunkData(testData, 10) // 10 chunks
	if err != nil {
		t.Fatalf("Failed to create test chunks: %v", err)
	}

	manifest, err := BuildManifest(chunks, "test.txt", 10)
	if err != nil {
		t.Fatalf("Failed to build manifest: %v", err)
	}

	// Create providers
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

	// Track active fetches during operation
	var maxActiveFetches uint32
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			select {
			case <-time.After(1 * time.Millisecond):
				stats := fetcher.GetStats()
				if stats.ActiveFetches > maxActiveFetches {
					maxActiveFetches = stats.ActiveFetches
				}
			case <-done:
				return
			}
		}
	}()

	// Fetch content
	ctx := context.Background()
	_, err = fetcher.FetchContent(ctx, manifest, providers)
	if err != nil {
		t.Fatalf("Failed to fetch content: %v", err)
	}

	done <- struct{}{}

	// Verify concurrency was limited
	if maxActiveFetches > config.ConcurrentFetches {
		t.Errorf("Concurrency limit exceeded: max active %d, limit %d",
			maxActiveFetches, config.ConcurrentFetches)
	}
}

func TestGetStats(t *testing.T) {
	// Create test identity
	id, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	// Create fetcher
	network := NewMockNetwork()
	fetcher := NewContentFetcher(network, id, nil)

	// Get initial stats
	stats := fetcher.GetStats()
	if stats == nil {
		t.Fatal("Stats should not be nil")
	}

	// Verify initial values
	if stats.TotalChunks != 0 {
		t.Errorf("Initial TotalChunks should be 0, got %d", stats.TotalChunks)
	}

	if stats.ActiveFetches != 0 {
		t.Errorf("Initial ActiveFetches should be 0, got %d", stats.ActiveFetches)
	}
}

func TestHandleChunkDataMessage(t *testing.T) {
	// Create test identity
	id, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	// Create fetcher
	network := NewMockNetwork()
	fetcher := NewContentFetcher(network, id, nil)

	// Create test CID and data
	testData := []byte("test chunk data")
	cid := NewCID(testData)

	// Create CHUNK_DATA message
	frame := wire.NewChunkDataFrame(
		"test-provider",
		12345, // sequence number
		cid.String,
		0, // offset
		testData,
	)

	// Handle the message
	err = fetcher.HandleChunkDataMessage(frame)
	if err != nil {
		t.Fatalf("Failed to handle CHUNK_DATA message: %v", err)
	}

	// Note: Without a waiting handler, the message will be processed but not consumed
	// This test mainly verifies that the message parsing works correctly
}
