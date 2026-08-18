// Package ratelimit provides a small in-memory, per-key request throttle.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter is a fixed-window request throttle: at most limit Allow() calls
// succeed for a given key within the trailing window duration. Safe for
// concurrent use.
type Limiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

// New creates a Limiter that permits at most limit attempts per key within
// window.
func New(limit int, window time.Duration) *Limiter {
	return &Limiter{attempts: make(map[string][]time.Time), limit: limit, window: window}
}

// Allow reports whether an attempt for key is currently permitted, and — if
// so — records it so it counts against the key's window going forward.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	kept := l.attempts[key][:0]
	for _, t := range l.attempts[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}

	if len(kept) >= l.limit {
		l.attempts[key] = kept
		return false
	}

	if len(kept) == 0 {
		// Nothing left in the window — drop the key instead of keeping an
		// empty slice around, so a flood of distinct one-off keys (e.g.
		// attacker-chosen emails) doesn't grow the map unbounded.
		delete(l.attempts, key)
	}
	l.attempts[key] = append(l.attempts[key], now)
	return true
}
