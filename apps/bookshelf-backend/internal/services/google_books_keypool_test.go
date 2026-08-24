package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoogleBooksKeyPool_EmptyPoolReturnsEmptyKey(t *testing.T) {
	pool := NewGoogleBooksKeyPool(nil)

	assert.Empty(t, pool.Key())
}

func TestGoogleBooksKeyPool_SingleKeyBehavesLikeTheOldSingularConfig(t *testing.T) {
	pool := NewGoogleBooksKeyPool([]string{"only-key"})

	for range 3 {
		assert.Equal(t, "only-key", pool.Key())
	}
}

func TestGoogleBooksKeyPool_RoundRobinsAcrossKeys(t *testing.T) {
	pool := NewGoogleBooksKeyPool([]string{"a", "b", "c"})

	got := []string{pool.Key(), pool.Key(), pool.Key(), pool.Key()}

	assert.Equal(t, []string{"a", "b", "c", "a"}, got)
}

func TestGoogleBooksKeyPool_SkipsRateLimitedKey(t *testing.T) {
	pool := NewGoogleBooksKeyPool([]string{"a", "b"})

	pool.Key()                // consumes "a"
	pool.MarkRateLimited("b") // next pick would otherwise be "b"
	assert.Equal(t, "a", pool.Key(), "should skip the cooling-down key and wrap back to the healthy one")
	assert.Equal(t, "a", pool.Key(), "with b still cooling down, a is the only key available")
}

func TestGoogleBooksKeyPool_MarkRateLimitedOnUnknownKeyIsANoop(t *testing.T) {
	pool := NewGoogleBooksKeyPool([]string{"a"})

	pool.MarkRateLimited("not-in-pool")

	assert.Equal(t, "a", pool.Key(), "marking a key the pool never handed out must not affect the real key")
}
