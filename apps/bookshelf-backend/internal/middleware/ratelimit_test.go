package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(rate.Every(time.Hour), 2)

	assert.True(t, rl.Allow("a"), "first request within burst")
	assert.True(t, rl.Allow("a"), "second request within burst")
	assert.False(t, rl.Allow("a"), "third request exceeds burst")

	assert.True(t, rl.Allow("b"), "a different key has its own bucket")
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name         string
		remoteAddr   string
		forwardedFor string
		want         string
	}{
		{name: "no forwarded header falls back to remote addr", remoteAddr: "203.0.113.5:54321", want: "203.0.113.5"},
		{name: "single forwarded IP wins over remote addr", remoteAddr: "10.0.0.1:1234", forwardedFor: "198.51.100.7", want: "198.51.100.7"},
		{name: "first IP in a comma-separated chain is used", remoteAddr: "10.0.0.1:1234", forwardedFor: "198.51.100.7, 10.0.0.1", want: "198.51.100.7"},
		{name: "malformed remote addr with no port is returned as-is", remoteAddr: "not-a-host-port", want: "not-a-host-port"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/register", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.forwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.forwardedFor)
			}
			ctx := humatest.NewContext(&huma.Operation{}, req, httptest.NewRecorder())

			assert.Equal(t, tt.want, ClientIP(ctx))
		})
	}
}

func TestUserOrIP(t *testing.T) {
	t.Run("authenticated request keys by user ID", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/send-otp", nil)
		req.RemoteAddr = "203.0.113.5:54321"
		req = req.WithContext(context.WithValue(req.Context(), userIDKey, uint(42)))
		ctx := humatest.NewContext(&huma.Operation{}, req, httptest.NewRecorder())

		assert.Equal(t, "user:42", UserOrIP(ctx))
	})

	t.Run("unauthenticated request falls back to IP", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/send-otp", nil)
		req.RemoteAddr = "203.0.113.5:54321"
		ctx := humatest.NewContext(&huma.Operation{}, req, httptest.NewRecorder())

		assert.Equal(t, "ip:203.0.113.5", UserOrIP(ctx))
	})
}

func TestRateLimit(t *testing.T) {
	_, api := humatest.New(t)
	rl := NewRateLimiter(rate.Every(time.Hour), 1)
	mw := RateLimit(api, rl, func(huma.Context) string { return "shared-key" })

	t.Run("first request passes through", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/register", nil)
		rec := httptest.NewRecorder()
		ctx := humatest.NewContext(&huma.Operation{}, req, rec)

		var nextCalled bool
		mw(ctx, func(huma.Context) { nextCalled = true })

		assert.True(t, nextCalled)
		assert.NotEqual(t, http.StatusTooManyRequests, rec.Code)
	})

	t.Run("second request within the window is rejected", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/register", nil)
		rec := httptest.NewRecorder()
		ctx := humatest.NewContext(&huma.Operation{}, req, rec)

		var nextCalled bool
		mw(ctx, func(huma.Context) { nextCalled = true })

		assert.False(t, nextCalled, "next must not run once the limit is exceeded")
		assert.Equal(t, http.StatusTooManyRequests, rec.Code)
		assert.Equal(t, "60", rec.Header().Get("Retry-After"))
	})
}
