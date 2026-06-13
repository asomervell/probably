package chat

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CacheEntry represents a cached query result
type CacheEntry struct {
	Result    *QueryResult
	ExpiresAt time.Time
}

// QueryCache provides in-memory caching for query results
type QueryCache struct {
	entries map[string]*CacheEntry
	mu      sync.RWMutex
	ttl     time.Duration
}

// NewQueryCache creates a new query cache
func NewQueryCache(ttl time.Duration) *QueryCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute // Default: 5 minutes
	}
	
	cache := &QueryCache{
		entries: make(map[string]*CacheEntry),
		ttl:     ttl,
	}
	
	// Start cleanup goroutine
	go cache.cleanup()
	
	return cache
}

// Get retrieves a cached result if available and not expired
func (c *QueryCache) Get(key string) (*QueryResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}
	
	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}
	
	return entry.Result, true
}

// Set stores a query result in the cache
func (c *QueryCache) Set(key string, result *QueryResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.entries[key] = &CacheEntry{
		Result:    result,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// cleanup periodically removes expired entries
func (c *QueryCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		now := time.Now()
		c.mu.Lock()
		
		for key, entry := range c.entries {
			if now.After(entry.ExpiresAt) {
				delete(c.entries, key)
			}
		}
		
		c.mu.Unlock()
	}
}

// CacheKey generates a cache key from SQL query and ledger ID
func CacheKey(sql string, ledgerID uuid.UUID) string {
	// Create a hash of SQL + ledger ID for the cache key
	// This ensures identical queries get the same key
	data := fmt.Sprintf("%s:%s", sql, ledgerID.String())
	hash := sha256.Sum256([]byte(data))
	return ledgerID.String() + ":" + hex.EncodeToString(hash[:])
}
