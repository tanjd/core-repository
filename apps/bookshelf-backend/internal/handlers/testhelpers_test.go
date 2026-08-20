package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/services"
)

// testJWTSecret is the signing secret handlers in this package's tests are
// constructed with, so fakeAuthedCtx-produced tokens verify correctly.
const testJWTSecret = "test-jwt-secret"

// noopEmail returns an EmailService with no SMTP host configured, so
// SendEmail always no-ops instead of touching the network — see
// EmailService.SendEmail's "if s.host == ”" branch.
func noopEmail() *services.EmailService {
	return services.NewEmailService("", "", "", "", "", "", "", "http://localhost:3000")
}

// noopSMS returns the same MockSMSService production uses — there's no real
// SMS provider yet, so this is already the correct fake, not a test-only stub.
func noopSMS() *services.MockSMSService {
	return services.NewMockSMSService()
}

// fakeAuthedCtx returns a context carrying userID/role exactly as
// middleware.SetAuth would attach them for a valid request, by
// round-tripping a signed JWT through the real middleware — this avoids
// needing access to middleware's unexported context-key type.
func fakeAuthedCtx(t *testing.T, userID uint, role string) context.Context {
	t.Helper()
	claims := middleware.JWTClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testJWTSecret))
	require.NoError(t, err)

	var captured context.Context
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = r.Context()
	})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	middleware.SetAuth(testJWTSecret)(next).ServeHTTP(httptest.NewRecorder(), req)
	return captured
}

// fakeAuthedCtxNone returns a context as middleware.SetAuth would produce
// for a request with no Authorization header — i.e. unauthenticated.
func fakeAuthedCtxNone() context.Context {
	var captured context.Context
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = r.Context()
	})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	middleware.SetAuth(testJWTSecret)(next).ServeHTTP(httptest.NewRecorder(), req)
	return captured
}

// assertStatus asserts that err is a huma.StatusError with the given HTTP status code.
func assertStatus(t *testing.T, err error, want int) {
	t.Helper()
	var statusErr huma.StatusError
	require.ErrorAs(t, err, &statusErr)
	assert.Equal(t, want, statusErr.GetStatus())
}
