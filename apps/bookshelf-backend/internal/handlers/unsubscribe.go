package handlers

import (
	"context"
	"errors"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/golang-jwt/jwt/v5"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// UnsubscribeHandler handles the one-click monthly digest unsubscribe flow.
// The endpoint is public (no auth required) — the signed token is the only
// proof of identity, which is sufficient because the only action it takes is
// disabling a preference the member can re-enable at any time.
type UnsubscribeHandler struct {
	users     repository.UserRepository
	jwtSecret string
	env       string
}

// NewUnsubscribeHandler creates a new UnsubscribeHandler.
func NewUnsubscribeHandler(users repository.UserRepository, jwtSecret, env string) *UnsubscribeHandler {
	return &UnsubscribeHandler{users: users, jwtSecret: jwtSecret, env: env}
}

// issueUnsubscribeToken mints a long-lived signed token for userID.
// Called by the digest service at send time; tested directly here so Slice 2
// can be verified end-to-end before the digest job exists.
func (h *UnsubscribeHandler) issueUnsubscribeToken(userID uint) (string, error) {
	claims := unsubscribeClaims{
		Purpose: unsubscribePurpose,
		UserID:  userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(unsubscribeTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}

// verifyUnsubscribeToken checks the token's signature, expiry, and purpose,
// returning the user ID it was minted for.
func (h *UnsubscribeHandler) verifyUnsubscribeToken(tokenStr string) (uint, error) {
	var claims unsubscribeClaims
	token, err := jwt.ParseWithClaims(tokenStr, &claims, func(*jwt.Token) (any, error) {
		return []byte(h.jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return 0, errors.New("invalid or expired link")
	}
	if claims.Purpose != unsubscribePurpose {
		return 0, errors.New("link does not match")
	}
	return claims.UserID, nil
}

// --- Input / Output types ---

type unsubscribeDigestInput struct {
	Body struct {
		Token string `json:"token" doc:"Signed unsubscribe token from the digest email footer link."`
	}
}

type unsubscribeDigestOutput struct {
	Body struct {
		Email string `json:"email" doc:"Email address of the unsubscribed member, for confirmation copy."`
	}
}

// --- Handlers ---

func (h *UnsubscribeHandler) unsubscribeDigest(_ context.Context, input *unsubscribeDigestInput) (*unsubscribeDigestOutput, error) {
	userID, err := h.verifyUnsubscribeToken(input.Body.Token)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}

	user, err := h.users.FindByID(userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("member not found")
		}
		return nil, huma.Error500InternalServerError("could not load member")
	}

	// Idempotent: flip only if still enabled; always return the email so
	// the confirmation page can show it.
	if user.MonthlyDigestEnabled {
		user.MonthlyDigestEnabled = false
		if err := h.users.Save(user); err != nil {
			return nil, huma.Error500InternalServerError("could not save preference")
		}
	}

	out := &unsubscribeDigestOutput{}
	out.Body.Email = user.Email
	return out, nil
}

// --- Dev-only: token minting for E2E tests ---

type debugUnsubscribeTokenInput struct {
	UserID uint `query:"user_id" doc:"ID of the member to mint an unsubscribe token for."`
}

type debugUnsubscribeTokenOutput struct {
	Body struct {
		Token string `json:"token"`
	}
}

func (h *UnsubscribeHandler) debugMintToken(_ context.Context, input *debugUnsubscribeTokenInput) (*debugUnsubscribeTokenOutput, error) {
	token, err := h.issueUnsubscribeToken(input.UserID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not mint token")
	}
	out := &debugUnsubscribeTokenOutput{}
	out.Body.Token = token
	return out, nil
}

// --- Route registration ---

// RegisterRoutes registers the unsubscribe endpoint on the given huma API.
// In dev mode an additional debug endpoint is registered so E2E tests can
// mint tokens without SMTP — matches the pattern used by auth debug_code
// fields throughout this app.
func (h *UnsubscribeHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "unsubscribe-digest",
		Method:      "POST",
		Path:        "/unsubscribe/digest",
		Summary:     "Unsubscribe from the monthly digest",
		Description: "Verifies a signed token from a digest email footer link and disables the monthly digest for that member. No authentication required.",
		Tags:        []string{"auth"},
	}, h.unsubscribeDigest)

	if h.env == "dev" {
		huma.Register(api, huma.Operation{
			OperationID: "debug-unsubscribe-token",
			Method:      "GET",
			Path:        "/unsubscribe/digest/debug-token",
			Summary:     "Mint an unsubscribe token (dev only)",
			Description: "Returns a signed unsubscribe token for the given user ID. Only available when ENV=dev.",
			Tags:        []string{"auth"},
		}, h.debugMintToken)
	}
}
