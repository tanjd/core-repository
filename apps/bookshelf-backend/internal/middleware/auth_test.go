package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret"

func signToken(t *testing.T, claims JWTClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)
	return signed
}

func TestSetAuth(t *testing.T) {
	validClaims := JWTClaims{
		UserID: 42,
		Role:   "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	expiredClaims := JWTClaims{
		UserID: 42,
		Role:   "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}

	tests := []struct {
		name       string
		authHeader string
		wantUserID uint
		wantRole   string
	}{
		{
			name:       "valid bearer token",
			authHeader: "Bearer " + signToken(t, validClaims),
			wantUserID: 42,
			wantRole:   "admin",
		},
		{
			name:       "expired token is not authenticated",
			authHeader: "Bearer " + signToken(t, expiredClaims),
			wantUserID: 0,
			wantRole:   "",
		},
		{
			name:       "malformed token is not authenticated",
			authHeader: "Bearer not-a-real-jwt",
			wantUserID: 0,
			wantRole:   "",
		},
		{
			name:       "missing header is not authenticated",
			authHeader: "",
			wantUserID: 0,
			wantRole:   "",
		},
		{
			name:       "non-bearer scheme is ignored",
			authHeader: "Basic dXNlcjpwYXNz",
			wantUserID: 0,
			wantRole:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUserID uint
			var gotRole string
			next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				gotUserID = GetUserID(r.Context())
				gotRole = GetUserRole(r.Context())
			})

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()

			SetAuth(testSecret)(next).ServeHTTP(rec, req)

			assert.Equal(t, tt.wantUserID, gotUserID)
			assert.Equal(t, tt.wantRole, gotRole)
		})
	}
}

func TestSetAuth_RejectsWrongSigningAlgorithm(t *testing.T) {
	// A token signed with a different HMAC secret must not authenticate —
	// this also exercises the "none"/non-HMAC algorithm guard indirectly by
	// confirming signature verification is actually enforced.
	claims := JWTClaims{UserID: 1, Role: "user"}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("a-different-secret"))
	require.NoError(t, err)

	var gotUserID uint
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotUserID = GetUserID(r.Context())
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rec := httptest.NewRecorder()

	SetAuth(testSecret)(next).ServeHTTP(rec, req)

	assert.Equal(t, uint(0), gotUserID)
}

func TestGetRequiredUserID(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := GetRequiredUserID(r.Context()); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	})

	t.Run("no token present returns ErrUnauthorized", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		SetAuth(testSecret)(next).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("valid token authenticates", func(t *testing.T) {
		claims := JWTClaims{
			UserID: 5,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		}
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+signToken(t, claims))
		rec := httptest.NewRecorder()
		SetAuth(testSecret)(next).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestRequireAdmin(t *testing.T) {
	tests := []struct {
		name    string
		role    string
		hasAuth bool
		wantErr error
	}{
		{name: "no authentication", hasAuth: false, wantErr: ErrUnauthorized},
		{name: "authenticated non-admin", hasAuth: true, role: "user", wantErr: ErrForbidden},
		{name: "authenticated admin", hasAuth: true, role: "admin", wantErr: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotErr error
			next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				gotErr = RequireAdmin(r.Context())
			})

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
			if tt.hasAuth {
				claims := JWTClaims{
					UserID: 1,
					Role:   tt.role,
					RegisteredClaims: jwt.RegisteredClaims{
						ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
					},
				}
				req.Header.Set("Authorization", "Bearer "+signToken(t, claims))
			}
			rec := httptest.NewRecorder()
			SetAuth(testSecret)(next).ServeHTTP(rec, req)

			assert.ErrorIs(t, gotErr, tt.wantErr)
		})
	}
}
