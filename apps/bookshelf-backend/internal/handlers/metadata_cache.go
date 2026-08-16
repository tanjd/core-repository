package handlers

import (
	"context"
	"sync"
	"time"
)

// MetadataCache is the interface for caching metadata search results.
// Implement this interface to swap in Redis or another backend.
type MetadataCache interface {
	Get(key string) ([]BookMetadataResult, bool)
	Set(key string, results []BookMetadataResult)
}

type cacheEntry struct {
	results   []BookMetadataResult
	expiresAt time.Time
}

type inMemoryMetadataCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
	ttl     time.Duration
}

// NewInMemoryMetadataCache returns a MetadataCache backed by an in-memory map.
// A background goroutine evicts expired entries every 10 minutes; it stops when
// ctx is cancelled.
func NewInMemoryMetadataCache(ctx context.Context, ttl time.Duration) MetadataCache {
	c := &inMemoryMetadataCache{
		entries: make(map[string]cacheEntry),
		ttl:     ttl,
	}
	go c.runEviction(ctx)
	return c
}

func (c *inMemoryMetadataCache) Get(key string) ([]BookMetadataResult, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.results, true
}

func (c *inMemoryMetadataCache) Set(key string, results []BookMetadataResult) {
	c.mu.Lock()
	c.entries[key] = cacheEntry{
		results:   results,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}

func (c *inMemoryMetadataCache) evictExpired() {
	now := time.Now()
	c.mu.Lock()
	for k, e := range c.entries {
		if now.After(e.expiresAt) {
			delete(c.entries, k)
		}
	}
	c.mu.Unlock()
}

func (c *inMemoryMetadataCache) runEviction(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.evictExpired()
		case <-ctx.Done():
			return
		}
	}
}
