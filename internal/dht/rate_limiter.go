// Package dht implements rate limiting for DHT operations
package dht

import (
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	capacity int           // Maximum tokens in bucket
	refill   time.Duration // Time to refill one token
	cleanup  time.Duration // How often to clean up old buckets
	
	// Cleanup management
	lastCleanup time.Time
}

// bucket represents a token bucket for a specific key (e.g., IP address or BID)
type bucket struct {
	tokens   int
	lastSeen time.Time
}

// RateLimiterConfig holds rate limiter configuration
type RateLimiterConfig struct {
	Capacity int           // Maximum tokens (requests) per bucket
	Refill   time.Duration // Time to refill one token
	Cleanup  time.Duration // How often to clean up old buckets
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config *RateLimiterConfig) *RateLimiter {
	if config.Capacity <= 0 {
		config.Capacity = 10 // Default: 10 requests
	}
	if config.Refill <= 0 {
		config.Refill = 1 * time.Minute // Default: 1 request per minute
	}
	if config.Cleanup <= 0 {
		config.Cleanup = 10 * time.Minute // Default: cleanup every 10 minutes
	}
	
	return &RateLimiter{
		buckets:     make(map[string]*bucket),
		capacity:    config.Capacity,
		refill:      config.Refill,
		cleanup:     config.Cleanup,
		lastCleanup: time.Now(),
	}
}

// Allow checks if a request from the given key should be allowed
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	
	// Perform cleanup if needed
	if now.Sub(rl.lastCleanup) > rl.cleanup {
		rl.performCleanup(now)
		rl.lastCleanup = now
	}
	
	// Get or create bucket for this key
	b, exists := rl.buckets[key]
	if !exists {
		b = &bucket{
			tokens:   rl.capacity - 1, // Use one token for this request
			lastSeen: now,
		}
		rl.buckets[key] = b
		return true
	}
	
	// Calculate tokens to add based on time elapsed
	elapsed := now.Sub(b.lastSeen)
	tokensToAdd := int(elapsed / rl.refill)
	
	// Add tokens up to capacity
	b.tokens += tokensToAdd
	if b.tokens > rl.capacity {
		b.tokens = rl.capacity
	}
	
	b.lastSeen = now
	
	// Check if we have tokens available
	if b.tokens > 0 {
		b.tokens--
		return true
	}
	
	return false
}

// GetTokens returns the current number of tokens for a key
func (rl *RateLimiter) GetTokens(key string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	b, exists := rl.buckets[key]
	if !exists {
		return rl.capacity
	}
	
	// Calculate current tokens
	now := time.Now()
	elapsed := now.Sub(b.lastSeen)
	tokensToAdd := int(elapsed / rl.refill)
	
	tokens := b.tokens + tokensToAdd
	if tokens > rl.capacity {
		tokens = rl.capacity
	}
	
	return tokens
}

// Reset resets the rate limiter for a specific key
func (rl *RateLimiter) Reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	delete(rl.buckets, key)
}

// Clear removes all rate limiting state
func (rl *RateLimiter) Clear() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	rl.buckets = make(map[string]*bucket)
}

// GetStats returns statistics about the rate limiter
func (rl *RateLimiter) GetStats() map[string]interface{} {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	return map[string]interface{}{
		"total_buckets": len(rl.buckets),
		"capacity":      rl.capacity,
		"refill_period": rl.refill.String(),
	}
}

// performCleanup removes old buckets that haven't been used recently
func (rl *RateLimiter) performCleanup(now time.Time) {
	// Remove buckets that haven't been used in the last hour
	cutoff := now.Add(-1 * time.Hour)
	
	for key, b := range rl.buckets {
		if b.lastSeen.Before(cutoff) {
			delete(rl.buckets, key)
		}
	}
}

// DHT Security Manager
type SecurityManager struct {
	rateLimiter *RateLimiter
	
	// Blacklist management
	blacklist map[string]time.Time // BID -> expiry time
	mu        sync.RWMutex
}

// SecurityConfig holds security manager configuration
type SecurityConfig struct {
	RateLimiter *RateLimiterConfig
}

// NewSecurityManager creates a new security manager
func NewSecurityManager(config *SecurityConfig) *SecurityManager {
	rateLimiterConfig := config.RateLimiter
	if rateLimiterConfig == nil {
		rateLimiterConfig = &RateLimiterConfig{
			Capacity: 20,                // 20 requests per bucket
			Refill:   30 * time.Second,  // 1 request every 30 seconds
			Cleanup:  10 * time.Minute,  // Cleanup every 10 minutes
		}
	}
	
	return &SecurityManager{
		rateLimiter: NewRateLimiter(rateLimiterConfig),
		blacklist:   make(map[string]time.Time),
	}
}

// AllowRequest checks if a request from the given BID should be allowed
func (sm *SecurityManager) AllowRequest(bid string) bool {
	sm.mu.RLock()
	
	// Check blacklist first
	if expiry, blacklisted := sm.blacklist[bid]; blacklisted {
		if time.Now().Before(expiry) {
			sm.mu.RUnlock()
			return false // Still blacklisted
		}
		// Blacklist entry expired, remove it
		sm.mu.RUnlock()
		sm.mu.Lock()
		delete(sm.blacklist, bid)
		sm.mu.Unlock()
		sm.mu.RLock()
	}
	
	sm.mu.RUnlock()
	
	// Check rate limit
	return sm.rateLimiter.Allow(bid)
}

// BlacklistBID adds a BID to the blacklist for the specified duration
func (sm *SecurityManager) BlacklistBID(bid string, duration time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.blacklist[bid] = time.Now().Add(duration)
}

// IsBlacklisted checks if a BID is currently blacklisted
func (sm *SecurityManager) IsBlacklisted(bid string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	expiry, exists := sm.blacklist[bid]
	if !exists {
		return false
	}
	
	if time.Now().After(expiry) {
		// Expired, remove from blacklist
		go func() {
			sm.mu.Lock()
			delete(sm.blacklist, bid)
			sm.mu.Unlock()
		}()
		return false
	}
	
	return true
}

// GetStats returns security manager statistics
func (sm *SecurityManager) GetStats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	// Count active blacklist entries
	activeBlacklist := 0
	now := time.Now()
	for _, expiry := range sm.blacklist {
		if now.Before(expiry) {
			activeBlacklist++
		}
	}
	
	stats := map[string]interface{}{
		"active_blacklist": activeBlacklist,
		"total_blacklist":  len(sm.blacklist),
	}
	
	// Add rate limiter stats
	rateLimiterStats := sm.rateLimiter.GetStats()
	for k, v := range rateLimiterStats {
		stats["rate_limiter_"+k] = v
	}
	
	return stats
}

// CleanupExpired removes expired blacklist entries
func (sm *SecurityManager) CleanupExpired() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	now := time.Now()
	for bid, expiry := range sm.blacklist {
		if now.After(expiry) {
			delete(sm.blacklist, bid)
		}
	}
}
