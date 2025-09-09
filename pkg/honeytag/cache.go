// Package honeytag implements caching for the resolver as specified in §12.5
package honeytag

import (
	"sync"
	"time"

	"github.com/WebFirstLanguage/beenet/internal/dht"
)

// ResolverCache implements caching for honeytag resolution
type ResolverCache struct {
	mu              sync.RWMutex
	handleIndexes   map[string]*CachedHandleIndex
	presenceRecords map[string]*CachedPresenceRecord
	nameRecords     map[string]*CachedNameRecord
}

// CachedHandleIndex represents a cached HandleIndex with expiration
type CachedHandleIndex struct {
	Record    *HandleIndex
	CachedAt  time.Time
	ExpiresAt time.Time
}

// CachedPresenceRecord represents a cached PresenceRecord with expiration
type CachedPresenceRecord struct {
	Record    *dht.PresenceRecord
	CachedAt  time.Time
	ExpiresAt time.Time
}

// CachedNameRecord represents a cached NameRecord with expiration
type CachedNameRecord struct {
	Record    *NameRecord
	CachedAt  time.Time
	ExpiresAt time.Time
}

// NewResolverCache creates a new resolver cache
func NewResolverCache() *ResolverCache {
	cache := &ResolverCache{
		handleIndexes:   make(map[string]*CachedHandleIndex),
		presenceRecords: make(map[string]*CachedPresenceRecord),
		nameRecords:     make(map[string]*CachedNameRecord),
	}

	// Start cleanup goroutine
	go cache.cleanupLoop()

	return cache
}

// GetHandleIndex retrieves a cached HandleIndex if valid
func (c *ResolverCache) GetHandleIndex(key string) *HandleIndex {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, exists := c.handleIndexes[key]
	if !exists {
		return nil
	}

	// Check if expired
	if time.Now().After(cached.ExpiresAt) {
		return nil
	}

	return cached.Record
}

// PutHandleIndex caches a HandleIndex with its natural expiration
func (c *ResolverCache) PutHandleIndex(key string, record *HandleIndex) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expiresAt := time.UnixMilli(int64(record.Expire))

	c.handleIndexes[key] = &CachedHandleIndex{
		Record:    record,
		CachedAt:  now,
		ExpiresAt: expiresAt,
	}
}

// GetPresenceRecord retrieves a cached PresenceRecord if valid
func (c *ResolverCache) GetPresenceRecord(key string) *dht.PresenceRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, exists := c.presenceRecords[key]
	if !exists {
		return nil
	}

	// Check if expired
	if time.Now().After(cached.ExpiresAt) {
		return nil
	}

	return cached.Record
}

// PutPresenceRecord caches a PresenceRecord with its natural expiration
func (c *ResolverCache) PutPresenceRecord(key string, record *dht.PresenceRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expiresAt := time.UnixMilli(int64(record.Expire))

	c.presenceRecords[key] = &CachedPresenceRecord{
		Record:    record,
		CachedAt:  now,
		ExpiresAt: expiresAt,
	}
}

// GetNameRecord retrieves a cached NameRecord if valid
func (c *ResolverCache) GetNameRecord(key string) *NameRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, exists := c.nameRecords[key]
	if !exists {
		return nil
	}

	// Check if expired or needs revalidation (≤10% remaining lease)
	now := time.Now()
	if now.After(cached.ExpiresAt) {
		return nil
	}

	// Check if we're in the revalidation window (≤10% remaining lease)
	leaseRemaining := cached.ExpiresAt.Sub(now)
	totalLease := cached.ExpiresAt.Sub(time.UnixMilli(int64(cached.Record.TS)))
	if leaseRemaining <= totalLease/10 {
		return nil // Force revalidation
	}

	return cached.Record
}

// PutNameRecord caches a NameRecord with its lease expiration
func (c *ResolverCache) PutNameRecord(key string, record *NameRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expiresAt := time.UnixMilli(int64(record.Lease))

	c.nameRecords[key] = &CachedNameRecord{
		Record:    record,
		CachedAt:  now,
		ExpiresAt: expiresAt,
	}
}

// InvalidateHandleIndex removes a HandleIndex from cache
func (c *ResolverCache) InvalidateHandleIndex(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.handleIndexes, key)
}

// InvalidatePresenceRecord removes a PresenceRecord from cache
func (c *ResolverCache) InvalidatePresenceRecord(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.presenceRecords, key)
}

// InvalidateNameRecord removes a NameRecord from cache
func (c *ResolverCache) InvalidateNameRecord(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.nameRecords, key)
}

// Clear removes all cached entries
func (c *ResolverCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.handleIndexes = make(map[string]*CachedHandleIndex)
	c.presenceRecords = make(map[string]*CachedPresenceRecord)
	c.nameRecords = make(map[string]*CachedNameRecord)
}

// cleanupLoop periodically removes expired entries
func (c *ResolverCache) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute) // Cleanup every 5 minutes
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup removes expired entries from all caches
func (c *ResolverCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	// Cleanup HandleIndexes
	for key, cached := range c.handleIndexes {
		if now.After(cached.ExpiresAt) {
			delete(c.handleIndexes, key)
		}
	}

	// Cleanup PresenceRecords
	for key, cached := range c.presenceRecords {
		if now.After(cached.ExpiresAt) {
			delete(c.presenceRecords, key)
		}
	}

	// Cleanup NameRecords
	for key, cached := range c.nameRecords {
		if now.After(cached.ExpiresAt) {
			delete(c.nameRecords, key)
		}
	}
}

// Stats returns cache statistics
func (c *ResolverCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CacheStats{
		HandleIndexes:   len(c.handleIndexes),
		PresenceRecords: len(c.presenceRecords),
		NameRecords:     len(c.nameRecords),
	}
}

// CacheStats represents cache statistics
type CacheStats struct {
	HandleIndexes   int
	PresenceRecords int
	NameRecords     int
}
