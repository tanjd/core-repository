package ratelimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLimiter_Allow(t *testing.T) {
	l := New(3, time.Minute)

	assert.True(t, l.Allow("a"), "1st attempt for key a")
	assert.True(t, l.Allow("a"), "2nd attempt for key a")
	assert.True(t, l.Allow("a"), "3rd attempt for key a")
	assert.False(t, l.Allow("a"), "4th attempt for key a exceeds the limit")

	assert.True(t, l.Allow("b"), "a different key has its own independent budget")
}

func TestLimiter_WindowExpires(t *testing.T) {
	l := New(1, 10*time.Millisecond)

	assert.True(t, l.Allow("a"))
	assert.False(t, l.Allow("a"), "still within the window")

	time.Sleep(20 * time.Millisecond)
	assert.True(t, l.Allow("a"), "old attempt has aged out of the window")
}
