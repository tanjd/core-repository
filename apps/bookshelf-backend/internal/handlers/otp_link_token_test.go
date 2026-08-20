package handlers

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOTPLinkToken(t *testing.T) {
	h, _, _ := newAuthHandler()

	t.Run("round trip returns the identifier and code it was minted with", func(t *testing.T) {
		token, err := h.issueOTPLinkToken(otpLinkPurposeResetPassword, "ada@example.com", "123456")
		require.NoError(t, err)

		identifier, code, err := h.verifyOTPLinkToken(token, otpLinkPurposeResetPassword)
		require.NoError(t, err)
		assert.Equal(t, "ada@example.com", identifier)
		assert.Equal(t, "123456", code)
	})

	t.Run("rejects a token checked against the wrong purpose", func(t *testing.T) {
		token, err := h.issueOTPLinkToken(otpLinkPurposeResetPassword, "ada@example.com", "123456")
		require.NoError(t, err)

		_, _, err = h.verifyOTPLinkToken(token, otpLinkPurposeEmailChange)
		assert.Error(t, err, "a reset-password link must not be usable as an email-change confirmation")
	})

	t.Run("rejects an expired token", func(t *testing.T) {
		claims := otpLinkClaims{
			Purpose:    otpLinkPurposeResetPassword,
			Identifier: "ada@example.com",
			Code:       "123456",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-otpLinkTokenTTL)),
			},
		}
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testJWTSecret))
		require.NoError(t, err)

		_, _, err = h.verifyOTPLinkToken(token, otpLinkPurposeResetPassword)
		assert.Error(t, err)
	})

	t.Run("rejects a token signed with a different secret", func(t *testing.T) {
		claims := otpLinkClaims{
			Purpose:    otpLinkPurposeResetPassword,
			Identifier: "ada@example.com",
			Code:       "123456",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(otpLinkTokenTTL)),
			},
		}
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("a-different-secret"))
		require.NoError(t, err)

		_, _, err = h.verifyOTPLinkToken(token, otpLinkPurposeResetPassword)
		assert.Error(t, err)
	})
}

func TestResolveCode(t *testing.T) {
	h, _, _ := newAuthHandler()

	t.Run("returns the body code when no token is given", func(t *testing.T) {
		code, err := h.resolveCode("", otpLinkPurposeOTPVerify, "123456")
		require.NoError(t, err)
		assert.Equal(t, "123456", code)
	})

	t.Run("returns the token's embedded code when a token is given", func(t *testing.T) {
		token, err := h.issueOTPLinkToken(otpLinkPurposeOTPVerify, "42", "654321")
		require.NoError(t, err)

		code, err := h.resolveCode(token, otpLinkPurposeOTPVerify, "")
		require.NoError(t, err)
		assert.Equal(t, "654321", code)
	})

	t.Run("errors when neither token nor code is given", func(t *testing.T) {
		_, err := h.resolveCode("", otpLinkPurposeOTPVerify, "")
		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("errors on a token minted for a different purpose", func(t *testing.T) {
		token, err := h.issueOTPLinkToken(otpLinkPurposeEmailChange, "42", "654321")
		require.NoError(t, err)

		_, err = h.resolveCode(token, otpLinkPurposeOTPVerify, "")
		require.Error(t, err)
		assertStatus(t, err, 400)
	})
}

func TestResolveEmailAndCode(t *testing.T) {
	h, _, _ := newAuthHandler()

	t.Run("returns the body email/code when no token is given", func(t *testing.T) {
		email, code, err := h.resolveEmailAndCode("", otpLinkPurposeResetPassword, "ada@example.com", "123456")
		require.NoError(t, err)
		assert.Equal(t, "ada@example.com", email)
		assert.Equal(t, "123456", code)
	})

	t.Run("returns the token's embedded email/code when a token is given", func(t *testing.T) {
		token, err := h.issueOTPLinkToken(otpLinkPurposeResetPassword, "ada@example.com", "123456")
		require.NoError(t, err)

		email, code, err := h.resolveEmailAndCode(token, otpLinkPurposeResetPassword, "", "")
		require.NoError(t, err)
		assert.Equal(t, "ada@example.com", email)
		assert.Equal(t, "123456", code)
	})

	t.Run("errors when neither token nor email+code is given", func(t *testing.T) {
		_, _, err := h.resolveEmailAndCode("", otpLinkPurposeResetPassword, "", "")
		require.Error(t, err)
		assertStatus(t, err, 400)
	})
}
