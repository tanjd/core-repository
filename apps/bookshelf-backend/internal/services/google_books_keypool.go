package services

import (
	"sync"
	"sync/atomic"
	"time"
)

// googleBooksKeyCooldown is how long a key that just got a 429 is skipped
// for, giving Google's per-key quota window a chance to reset.
const googleBooksKeyCooldown = time.Minute

// GoogleBooksKeyPool round-robins across a shared pool of Google Books API
// keys so a busy self-hosted instance can spread requests across several
// keys' separate free-tier quotas instead of hammering one. Safe for
// concurrent use.
type GoogleBooksKeyPool struct {
	keys []string
	next atomic.Uint64

	mu        sync.Mutex
	skipUntil map[string]time.Time
}

// NewGoogleBooksKeyPool creates a pool from keys (already cleaned — see
// config.cleanKeys). An empty pool is valid: Key() always returns "", which
// every caller already treats as "Google Books unavailable" the same way an
// unset single key used to.
func NewGoogleBooksKeyPool(keys []string) *GoogleBooksKeyPool {
	return &GoogleBooksKeyPool{
		keys:      keys,
		skipUntil: make(map[string]time.Time),
	}
}

// Key returns the next key to use, round-robining across the pool and
// skipping any key still cooling down from a recent 429. If every key is
// currently cooling down, it falls back to the round-robin pick anyway — a
// stale cooldown beats going fully dark on Google Books.
func (p *GoogleBooksKeyPool) Key() string {
	n := uint64(len(p.keys))
	if n == 0 {
		return ""
	}
	start := p.next.Add(1) - 1

	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	for i := range n {
		key := p.keys[(start+i)%n]
		if until, ok := p.skipUntil[key]; !ok || now.After(until) {
			return key
		}
	}
	return p.keys[start%n]
}

// MarkRateLimited puts key into cooldown so Key() skips it for a while,
// letting a burst of 429s on one key fall over to the rest of the pool. A
// no-op for a key not in the pool (e.g. a user's personal key) — harmless,
// just an unused map entry that expires on its own.
func (p *GoogleBooksKeyPool) MarkRateLimited(key string) {
	if key == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.skipUntil[key] = time.Now().Add(googleBooksKeyCooldown)
}
