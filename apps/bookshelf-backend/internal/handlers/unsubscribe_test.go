package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
)

func newUnsubscribeHandler() (*UnsubscribeHandler, *repotest.UserRepository) {
	users := repotest.NewUserRepository()
	return NewUnsubscribeHandler(users, testJWTSecret, "dev"), users
}

func TestUnsubscribeDigest(t *testing.T) {
	t.Run("valid token flips MonthlyDigestEnabled to false", func(t *testing.T) {
		h, users := newUnsubscribeHandler()
		user := &models.User{Name: "Ada", Email: "ada@example.com", Password: "x", MonthlyDigestEnabled: true}
		require.NoError(t, users.Create(user))

		token, err := h.issueUnsubscribeToken(user.ID)
		require.NoError(t, err)

		input := &unsubscribeDigestInput{}
		input.Body.Token = token
		out, err := h.unsubscribeDigest(context.Background(), input)

		require.NoError(t, err)
		assert.Equal(t, "ada@example.com", out.Body.Email)

		reloaded, err := users.FindByID(user.ID)
		require.NoError(t, err)
		assert.False(t, reloaded.MonthlyDigestEnabled)
	})

	t.Run("expired token returns 400", func(t *testing.T) {
		h, users := newUnsubscribeHandler()
		user := &models.User{Name: "Ada", Email: "ada@example.com", Password: "x", MonthlyDigestEnabled: true}
		require.NoError(t, users.Create(user))

		// Mint a token that expired 1 second ago.
		claims := unsubscribeClaims{
			Purpose: unsubscribePurpose,
			UserID:  user.ID,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Second)),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			},
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		expired, err := tok.SignedString([]byte(testJWTSecret))
		require.NoError(t, err)

		input := &unsubscribeDigestInput{}
		input.Body.Token = expired
		_, err = h.unsubscribeDigest(context.Background(), input)

		assertStatus(t, err, 400)
	})

	t.Run("token with wrong purpose returns 400", func(t *testing.T) {
		h, users := newUnsubscribeHandler()
		user := &models.User{Name: "Ada", Email: "ada@example.com", Password: "x", MonthlyDigestEnabled: true}
		require.NoError(t, users.Create(user))

		// Mint a token using a different purpose (email_change).
		claims := unsubscribeClaims{
			Purpose: otpLinkPurposeEmailChange,
			UserID:  user.ID,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		wrongPurpose, err := tok.SignedString([]byte(testJWTSecret))
		require.NoError(t, err)

		input := &unsubscribeDigestInput{}
		input.Body.Token = wrongPurpose
		_, err = h.unsubscribeDigest(context.Background(), input)

		assertStatus(t, err, 400)
	})

	t.Run("already unsubscribed member returns 200 and flag stays false", func(t *testing.T) {
		h, users := newUnsubscribeHandler()
		user := &models.User{Name: "Ada", Email: "ada@example.com", Password: "x", MonthlyDigestEnabled: false}
		require.NoError(t, users.Create(user))

		token, err := h.issueUnsubscribeToken(user.ID)
		require.NoError(t, err)

		input := &unsubscribeDigestInput{}
		input.Body.Token = token
		out, err := h.unsubscribeDigest(context.Background(), input)

		require.NoError(t, err)
		assert.Equal(t, "ada@example.com", out.Body.Email)

		reloaded, err := users.FindByID(user.ID)
		require.NoError(t, err)
		assert.False(t, reloaded.MonthlyDigestEnabled)
	})

	t.Run("unknown user ID returns 404", func(t *testing.T) {
		h, _ := newUnsubscribeHandler()

		token, err := h.issueUnsubscribeToken(9999)
		require.NoError(t, err)

		input := &unsubscribeDigestInput{}
		input.Body.Token = token
		_, err = h.unsubscribeDigest(context.Background(), input)

		assertStatus(t, err, 404)
	})
}
