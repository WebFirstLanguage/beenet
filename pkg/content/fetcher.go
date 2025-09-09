package content

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/identity"
	"github.com/WebFirstLanguage/beenet/pkg/wire"
)

// NetworkInterface defines the interface for network operations
type NetworkInterface interface {
	SendMessage(ctx context.Context, target string, frame *wire.BaseFrame) error
	BroadcastMessage(ctx context.Context, frame *wire.BaseFrame) error
}

// ContentFetcher manages fetching content chunks from the network
type ContentFetcher struct {
	network  NetworkInterface
	identity *identity.Identity
	config   *Config
	stats    *ContentStats
	statsMu  sync.RWMutex

	// Error tracking
	errorStats *ErrorStats
	errorMu    sync.RWMutex

	// Concurrency control
	semaphore chan struct{}

	// Active fetch tracking
	activeFetches map[string]*fetchOperation
	fetchesMu     sync.RWMutex

	// Response handling
	responseHandlers map[uint64]chan *FetchResponse
	handlersMu       sync.RWMutex
	seqCounter       uint64
	seqMu            sync.Mutex
}

// fetchOperation represents an active fetch operation
type fetchOperation struct {
	CID       CID
	Provider  string
	StartTime time.Time
	Timeout   time.Duration
	Cancel    context.CancelFunc
}

// NewContentFetcher creates a new content fetcher
func NewContentFetcher(network NetworkInterface, identity *identity.Identity, config *Config) *ContentFetcher {
	if config == nil {
		config = DefaultConfig()
	}

	return &ContentFetcher{
		network:          network,
		identity:         identity,
		config:           config,
		stats:            &ContentStats{},
		errorStats:       NewErrorStats(),
		semaphore:        make(chan struct{}, config.ConcurrentFetches),
		activeFetches:    make(map[string]*fetchOperation),
		responseHandlers: make(map[uint64]chan *FetchResponse),
	}
}

// FetchContent fetches content by CID from the network
func (cf *ContentFetcher) FetchContent(ctx context.Context, manifest *Manifest, providers []*ProvideRecord) ([]*Chunk, error) {
	if manifest == nil {
		return nil, fmt.Errorf("manifest is required")
	}

	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers available")
	}

	// Verify manifest
	if err := VerifyManifest(manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	chunks := make([]*Chunk, len(manifest.Chunks))
	fetchErrors := make([]error, len(manifest.Chunks))

	// Use a wait group to track all fetch operations
	var wg sync.WaitGroup

	// Fetch each chunk concurrently with backpressure control
	for i, chunkInfo := range manifest.Chunks {
		wg.Add(1)
		go func(index int, info ChunkInfo) {
			defer wg.Done()

			chunk, err := cf.fetchChunk(ctx, info.CID, providers)
			if err != nil {
				fetchErrors[index] = err

				// Record error statistics
				var contentErr *ContentError
				if !errors.As(err, &contentErr) {
					// Wrap non-ContentError as network error
					contentErr = NewNetworkError(err.Error(), "", err)
				}
				cf.recordError(contentErr)

				cf.updateStats(func(s *ContentStats) {
					s.FailedGets++
					s.NetworkErrors++
				})
				return
			}

			// Verify chunk integrity
			if cf.config.EnableIntegrityCheck {
				if err := VerifyChunkIntegrity(chunk); err != nil {
					fetchErrors[index] = fmt.Errorf("chunk integrity verification failed: %w", err)

					// Record integrity error
					contentErr := NewIntegrityError("chunk integrity verification failed", &info.CID, err)
					cf.recordError(contentErr)

					cf.updateStats(func(s *ContentStats) {
						s.IntegrityErrors++
					})
					return
				}
			}

			chunks[index] = chunk
			cf.updateStats(func(s *ContentStats) {
				s.SuccessfulGets++
				s.TotalChunks++
				s.TotalBytes += chunk.Size
			})
		}(i, chunkInfo)
	}

	// Wait for all fetches to complete
	wg.Wait()

	// Check for errors
	var firstError error
	for i, err := range fetchErrors {
		if err != nil {
			if firstError == nil {
				firstError = fmt.Errorf("chunk %d fetch failed: %w", i, err)
			}
		}
	}

	if firstError != nil {
		return nil, firstError
	}

	return chunks, nil
}

// fetchChunk fetches a single chunk from providers
func (cf *ContentFetcher) fetchChunk(ctx context.Context, cid CID, providers []*ProvideRecord) (*Chunk, error) {
	// Acquire semaphore for backpressure control
	select {
	case cf.semaphore <- struct{}{}:
		defer func() { <-cf.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	cf.updateStats(func(s *ContentStats) {
		s.ActiveFetches++
	})
	defer cf.updateStats(func(s *ContentStats) {
		s.ActiveFetches--
	})

	// Try each provider until one succeeds
	for _, provider := range providers {
		chunk, err := cf.fetchChunkFromProvider(ctx, cid, provider)
		if err == nil {
			return chunk, nil
		}
		// Log error but continue to next provider
	}

	return nil, fmt.Errorf("failed to fetch chunk from any provider")
}

// fetchChunkFromProvider fetches a chunk from a specific provider
func (cf *ContentFetcher) fetchChunkFromProvider(ctx context.Context, cid CID, provider *ProvideRecord) (*Chunk, error) {
	// Create fetch context with timeout
	fetchCtx, cancel := context.WithTimeout(ctx, cf.config.FetchTimeout)
	defer cancel()

	// Track this fetch operation
	fetchKey := fmt.Sprintf("%s:%s", cid.String, provider.Provider)
	operation := &fetchOperation{
		CID:       cid,
		Provider:  provider.Provider,
		StartTime: time.Now(),
		Timeout:   cf.config.FetchTimeout,
		Cancel:    cancel,
	}

	cf.fetchesMu.Lock()
	cf.activeFetches[fetchKey] = operation
	cf.fetchesMu.Unlock()

	defer func() {
		cf.fetchesMu.Lock()
		delete(cf.activeFetches, fetchKey)
		cf.fetchesMu.Unlock()
	}()

	// Generate sequence number for this request
	seq := cf.getNextSeq()

	// Create response channel
	responseChan := make(chan *FetchResponse, 1)
	cf.handlersMu.Lock()
	cf.responseHandlers[seq] = responseChan
	cf.handlersMu.Unlock()

	defer func() {
		cf.handlersMu.Lock()
		delete(cf.responseHandlers, seq)
		cf.handlersMu.Unlock()
		close(responseChan)
	}()

	// Create FETCH_CHUNK message
	frame := wire.NewFetchChunkFrame(cf.identity.BID(), seq, cid.String, nil)

	// Send fetch request
	if err := cf.network.SendMessage(fetchCtx, provider.Provider, frame); err != nil {
		return nil, NewNetworkError("failed to send fetch request", provider.Provider, err)
	}

	// Wait for response
	select {
	case response := <-responseChan:
		if response.Error != "" {
			return nil, NewChunkNotFoundError(&cid, provider.Provider)
		}

		// Create chunk from response
		chunk := &Chunk{
			CID:    response.CID,
			Data:   response.Data,
			Size:   uint64(len(response.Data)),
			Offset: 0, // Will be set correctly by the caller based on manifest
		}

		return chunk, nil

	case <-fetchCtx.Done():
		if fetchCtx.Err() == context.DeadlineExceeded {
			return nil, NewTimeoutError("fetch request timed out", &cid, provider.Provider)
		}
		return nil, NewNetworkError("fetch request cancelled", provider.Provider, fetchCtx.Err())
	}
}

// HandleChunkDataMessage handles incoming CHUNK_DATA messages
func (cf *ContentFetcher) HandleChunkDataMessage(frame *wire.BaseFrame) error {
	// Extract chunk data body
	body, ok := frame.Body.(*wire.ChunkDataBody)
	if !ok {
		return NewInvalidRequestError("invalid CHUNK_DATA message body", nil)
	}

	// Find the response handler for this sequence
	cf.handlersMu.RLock()
	responseChan, exists := cf.responseHandlers[frame.Seq]
	cf.handlersMu.RUnlock()

	if !exists {
		// No handler waiting for this response, ignore
		return nil
	}

	// Parse CID
	cid, err := ParseCID(body.CID)
	if err != nil {
		// Send error response
		response := &FetchResponse{
			CID:   CID{},
			Data:  nil,
			Error: fmt.Sprintf("invalid CID: %v", err),
		}
		select {
		case responseChan <- response:
		default:
		}
		return nil
	}

	// Validate chunk data integrity if enabled
	if cf.config.EnableIntegrityCheck && len(body.Data) > 0 {
		expectedCID := NewCID(body.Data)
		if !expectedCID.Equals(cid) {
			// Data doesn't match CID - corrupted chunk
			response := &FetchResponse{
				CID:   cid,
				Data:  nil,
				Error: "chunk data integrity verification failed",
			}
			select {
			case responseChan <- response:
			default:
			}

			// Record corruption error
			corruptionErr := NewCorruptedDataError("received chunk data doesn't match CID", &cid, nil)
			cf.recordError(corruptionErr)

			return nil
		}
	}

	// Create successful response
	response := &FetchResponse{
		CID:   cid,
		Data:  body.Data,
		Error: "",
	}

	// Send response to waiting goroutine
	select {
	case responseChan <- response:
	default:
		// Channel full or closed, ignore
	}

	return nil
}

// GetStats returns current fetcher statistics
func (cf *ContentFetcher) GetStats() *ContentStats {
	cf.statsMu.RLock()
	defer cf.statsMu.RUnlock()

	// Return a copy
	return &ContentStats{
		TotalChunks:     cf.stats.TotalChunks,
		TotalBytes:      cf.stats.TotalBytes,
		ActiveFetches:   cf.stats.ActiveFetches,
		SuccessfulGets:  cf.stats.SuccessfulGets,
		FailedGets:      cf.stats.FailedGets,
		SuccessfulPuts:  cf.stats.SuccessfulPuts,
		FailedPuts:      cf.stats.FailedPuts,
		CacheHits:       cf.stats.CacheHits,
		CacheMisses:     cf.stats.CacheMisses,
		NetworkErrors:   cf.stats.NetworkErrors,
		IntegrityErrors: cf.stats.IntegrityErrors,
	}
}

// GetActiveFetches returns information about currently active fetches
func (cf *ContentFetcher) GetActiveFetches() map[string]*fetchOperation {
	cf.fetchesMu.RLock()
	defer cf.fetchesMu.RUnlock()

	// Return a copy
	result := make(map[string]*fetchOperation)
	for k, v := range cf.activeFetches {
		result[k] = &fetchOperation{
			CID:       v.CID,
			Provider:  v.Provider,
			StartTime: v.StartTime,
			Timeout:   v.Timeout,
			// Don't copy Cancel function
		}
	}

	return result
}

// updateStats safely updates statistics
func (cf *ContentFetcher) updateStats(updater func(*ContentStats)) {
	cf.statsMu.Lock()
	defer cf.statsMu.Unlock()
	updater(cf.stats)
}

// getNextSeq returns the next sequence number
func (cf *ContentFetcher) getNextSeq() uint64 {
	cf.seqMu.Lock()
	defer cf.seqMu.Unlock()
	cf.seqCounter++
	return cf.seqCounter
}

// recordError records an error in the error statistics
func (cf *ContentFetcher) recordError(err *ContentError) {
	cf.errorMu.Lock()
	defer cf.errorMu.Unlock()
	cf.errorStats.RecordError(err)
}

// GetErrorStats returns current error statistics
func (cf *ContentFetcher) GetErrorStats() *ErrorStats {
	cf.errorMu.RLock()
	defer cf.errorMu.RUnlock()

	// Return a copy to avoid race conditions
	stats := &ErrorStats{
		NetworkErrors:    cf.errorStats.NetworkErrors,
		TimeoutErrors:    cf.errorStats.TimeoutErrors,
		IntegrityErrors:  cf.errorStats.IntegrityErrors,
		ProviderErrors:   cf.errorStats.ProviderErrors,
		CorruptionErrors: cf.errorStats.CorruptionErrors,
		RateLimitErrors:  cf.errorStats.RateLimitErrors,
		ErrorsByProvider: make(map[string]uint64),
		LastError:        cf.errorStats.LastError,
		LastErrorTime:    cf.errorStats.LastErrorTime,
	}

	// Copy provider error map
	for provider, count := range cf.errorStats.ErrorsByProvider {
		stats.ErrorsByProvider[provider] = count
	}

	return stats
}
