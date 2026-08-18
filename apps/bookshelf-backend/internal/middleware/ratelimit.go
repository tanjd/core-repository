package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"golang.org/x/time/rate"
)

// RateLimiter enforces a per-key token-bucket limit in memory. Bookshelf
// runs as a single instance (no shared store across replicas needed), and
// at its scale — a single community, per the models' own comments — the
// set of keys ever seen (registered users, or distinct IPs once a reverse
// proxy sits in front of it) stays small enough that entries are kept for
// the life of the process rather than evicted.
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	r        rate.Limit
	burst    int
}

// NewRateLimiter creates a limiter allowing burst requests immediately per
// key, refilling one token every 1/r.
func NewRateLimiter(r rate.Limit, burst int) *RateLimiter {
	return &RateLimiter{limiters: make(map[string]*rate.Limiter), r: r, burst: burst}
}

// Allow reports whether a request for key is within the limit.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	l, ok := rl.limiters[key]
	if !ok {
		l = rate.NewLimiter(rl.r, rl.burst)
		rl.limiters[key] = l
	}
	return l.Allow()
}

// ClientIP extracts the originating client IP from a request, trusting the
// first address in X-Forwarded-For when present and falling back to the
// direct connection's remote address otherwise.
//
// Bookshelf's frontend proxies every API call server-side (see
// apps/bookshelf's src/app/api/[...path]/route.ts) — until a reverse proxy
// sits in front of *that* and sets X-Forwarded-For, every request reaching
// this backend appears to come from the frontend container's IP, so this
// degrades to one shared bucket rather than a true per-client limit. Still
// strictly better than no limit: a burst still gets throttled, just at
// coarser granularity.
func ClientIP(ctx huma.Context) string {
	if fwd := ctx.Header("X-Forwarded-For"); fwd != "" {
		if idx := strings.Index(fwd, ","); idx != -1 {
			fwd = fwd[:idx]
		}
		return strings.TrimSpace(fwd)
	}
	host, _, err := net.SplitHostPort(ctx.RemoteAddr())
	if err != nil {
		return ctx.RemoteAddr()
	}
	return host
}

// UserOrIP keys by the authenticated user ID (set by SetAuth, which runs
// ahead of huma's own middleware chain), falling back to ClientIP for the
// rare case this fires with no valid token — accurate regardless of
// whatever sits in front of the app, unlike IP-based keying.
func UserOrIP(ctx huma.Context) string {
	if userID := GetUserID(ctx.Context()); userID != 0 {
		return "user:" + strconv.FormatUint(uint64(userID), 10)
	}
	return "ip:" + ClientIP(ctx)
}

// RateLimit returns huma operation middleware that rejects requests beyond
// rl's limit (keyed by keyFunc) with 429 Too Many Requests.
func RateLimit(api huma.API, rl *RateLimiter, keyFunc func(huma.Context) string) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if !rl.Allow(keyFunc(ctx)) {
			ctx.SetHeader("Retry-After", "60")
			_ = huma.WriteErr(api, ctx, http.StatusTooManyRequests, "too many requests — please try again later")
			return
		}
		next(ctx)
	}
}
