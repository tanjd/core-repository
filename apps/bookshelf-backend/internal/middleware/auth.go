// Package middleware provides HTTP middleware for the bookshelf API.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

type contextKey string

const userIDKey contextKey = "userID"
const roleKey contextKey = "role"

// JWTClaims are the custom claims embedded in every issued token.
type JWTClaims struct {
	UserID uint   `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// ErrUnauthorized is returned by GetRequiredUserID when no authenticated user
// is present in the context.
var ErrUnauthorized = errors.New("authentication required")

// SetAuth returns a middleware that parses the Bearer JWT from the
// Authorization header and stores the user ID and role in the request context
// when valid. Requests with a missing or invalid token are not rejected — routes
// that require authentication should call GetRequiredUserID.
func SetAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header != "" && strings.HasPrefix(header, "Bearer ") {
				tokenStr := strings.TrimPrefix(header, "Bearer ")
				claims := &JWTClaims{}
				token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
					if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, jwt.ErrSignatureInvalid
					}
					return []byte(secret), nil
				})
				if err == nil && token.Valid {
					ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
					ctx = context.WithValue(ctx, roleKey, claims.Role)
					r = r.WithContext(ctx)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GetUserID retrieves the authenticated user's ID from the context.
// Returns 0 if no valid JWT was present on the request.
func GetUserID(ctx context.Context) uint {
	v, _ := ctx.Value(userIDKey).(uint)
	return v
}

// GetRequiredUserID returns the authenticated user ID from ctx.
// Returns 0 and ErrUnauthorized if no authenticated user is present.
func GetRequiredUserID(ctx context.Context) (uint, error) {
	v, _ := ctx.Value(userIDKey).(uint)
	if v == 0 {
		return 0, ErrUnauthorized
	}
	return v, nil
}

// GetUserRole retrieves the authenticated user's role from the context.
// Returns empty string if no valid JWT was present on the request.
func GetUserRole(ctx context.Context) string {
	v, _ := ctx.Value(roleKey).(string)
	return v
}

// RequireAdmin returns ErrUnauthorized if no token is present or ErrForbidden
// if the token's role is not "admin".
func RequireAdmin(ctx context.Context) error {
	_, err := GetRequiredUserID(ctx)
	if err != nil {
		return ErrUnauthorized
	}
	if GetUserRole(ctx) != "admin" {
		return ErrForbidden
	}
	return nil
}

// ErrForbidden is returned by RequireAdmin when the user is authenticated but
// lacks admin privileges.
var ErrForbidden = errors.New("forbidden")

// RequireActiveUser returns middleware that re-checks the authenticated
// user's live database state on every request. SetAuth only verifies the
// JWT's signature and expiry — its role/user_id claims are baked in at
// issuance time and stay valid for the token's full 24h lifetime even after
// an admin suspends the account, revokes approval, or demotes the user's
// role. This closes that gap by looking the user up on each request and
// rejecting (or downgrading) stale sessions immediately. Requests with no
// authenticated user pass through unchanged, since many routes are public.
func RequireActiveUser(users repository.UserRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserID(r.Context())
			if userID == 0 {
				next.ServeHTTP(w, r)
				return
			}

			user, err := users.FindByID(userID)
			if err != nil {
				writeSessionError(w, http.StatusUnauthorized, "this session is no longer valid")
				return
			}
			if user.Suspended || user.PendingApproval {
				writeSessionError(w, http.StatusForbidden, "this session is no longer valid")
				return
			}

			// Use the live DB role rather than the one baked into the JWT at
			// issuance time, so a demoted admin loses admin access immediately
			// instead of waiting out the token's expiry.
			ctx := context.WithValue(r.Context(), roleKey, user.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeSessionError(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"detail":"` + detail + `"}`))
}
