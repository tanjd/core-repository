package handlers

import (
	"errors"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/golang-jwt/jwt/v5"
)

// otpLinkTokenTTL is how long a magic-link email token stays valid — matches
// every OTP flow's own code expiry (registrationOTPExpiry,
// passwordResetOTPExpiry, and the 15-minute windows used by
// requestEmailChange/sendOTP), so the link never outlives the code it
// carries.
const otpLinkTokenTTL = 15 * time.Minute

// unsubscribeTokenTTL is how long a digest unsubscribe link stays valid.
// Longer than OTP links because a digest email may sit unread for weeks;
// a 1-year window covers the realistic "received it last month, clicking now"
// case without requiring token storage or revocation machinery.
const unsubscribeTokenTTL = 365 * 24 * time.Hour

// Magic-link token purposes — embedded as a claim so a token minted for one
// flow can't be replayed against another's verify endpoint.
const (
	otpLinkPurposeRegisterEmail = "register_email_otp"
	otpLinkPurposeResetPassword = "reset_password"
	otpLinkPurposeEmailChange   = "email_change"
	otpLinkPurposeOTPVerify     = "otp_verify"
)

// unsubscribePurpose is the purpose claim embedded in digest unsubscribe
// tokens — kept separate from otpLinkPurpose* constants because it uses a
// different claims shape (UserID instead of Identifier+Code) and a
// different TTL.
const unsubscribePurpose = "unsubscribe_digest"

// unsubscribeClaims is the JWT payload for a one-click digest unsubscribe
// link. Carries only the member's identity — no code, no email — since
// the only action is "this link is genuine, flip the flag."
type unsubscribeClaims struct {
	Purpose string `json:"purpose"`
	UserID  uint   `json:"user_id"`
	jwt.RegisteredClaims
}

// otpLinkClaims is the JWT payload embedded in a magic-link email URL.
// Unlike registrationVerificationClaims (minted *after* a code is verified,
// as a hand-off to register()), this token is minted *alongside* the code
// itself so a click can skip manual entry — the corresponding verify
// endpoint accepts it as an alternative to submitting email+code directly.
type otpLinkClaims struct {
	Purpose    string `json:"purpose"`
	Identifier string `json:"identifier"`
	Code       string `json:"code"`
	jwt.RegisteredClaims
}

// issueOTPLinkToken mints a short-lived, stateless JWT carrying identifier
// and code for purpose, to embed in a magic-link email URL.
func (h *AuthHandler) issueOTPLinkToken(purpose, identifier, code string) (string, error) {
	claims := otpLinkClaims{
		Purpose:    purpose,
		Identifier: identifier,
		Code:       code,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(otpLinkTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}

// verifyOTPLinkToken checks tokenStr's signature, expiry, and purpose,
// returning the identifier and code it was minted with.
func (h *AuthHandler) verifyOTPLinkToken(tokenStr, expectedPurpose string) (identifier, code string, err error) {
	var claims otpLinkClaims
	token, parseErr := jwt.ParseWithClaims(tokenStr, &claims, func(*jwt.Token) (any, error) {
		return []byte(h.jwtSecret), nil
	})
	if parseErr != nil || !token.Valid {
		return "", "", errors.New("invalid or expired link")
	}
	if claims.Purpose != expectedPurpose {
		return "", "", errors.New("link does not match")
	}
	return claims.Identifier, claims.Code, nil
}

// resolveCode returns bodyCode when tokenStr is empty, otherwise decodes
// tokenStr (which must match expectedPurpose) and returns its embedded code.
// Used by the two auth-required flows (email-change confirm, generic
// verify-otp) where the account is already known from the session — only
// the code needs resolving.
func (h *AuthHandler) resolveCode(tokenStr, expectedPurpose, bodyCode string) (string, error) {
	if tokenStr != "" {
		_, code, err := h.verifyOTPLinkToken(tokenStr, expectedPurpose)
		if err != nil {
			return "", huma.Error400BadRequest(err.Error())
		}
		return code, nil
	}
	if bodyCode == "" {
		return "", huma.Error400BadRequest("code or token is required")
	}
	return bodyCode, nil
}

// resolveEmailAndCode returns bodyEmail/bodyCode when tokenStr is empty,
// otherwise decodes tokenStr (which must match expectedPurpose) and returns
// its embedded identifier (email) and code. Used by the two pre-auth flows
// (register email verify, password reset) where the token is the only
// identity proof available before the caller has a session.
func (h *AuthHandler) resolveEmailAndCode(tokenStr, expectedPurpose, bodyEmail, bodyCode string) (email, code string, err error) {
	if tokenStr != "" {
		email, code, err = h.verifyOTPLinkToken(tokenStr, expectedPurpose)
		if err != nil {
			return "", "", huma.Error400BadRequest(err.Error())
		}
		return email, code, nil
	}
	if bodyEmail == "" || bodyCode == "" {
		return "", "", huma.Error400BadRequest("email and code, or token, are required")
	}
	return bodyEmail, bodyCode, nil
}
