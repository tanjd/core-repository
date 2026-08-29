package handlers

import (
	"context"
	"errors"
	"fmt"

	"github.com/danielgtaylor/huma/v2"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// --- Input / Output types ---

type verificationFactor struct {
	Key       string `json:"key" doc:"Factor identifier: email, phone, or min_books_shared"`
	Label     string `json:"label" doc:"Human-readable description"`
	Required  bool   `json:"required"`
	Satisfied bool   `json:"satisfied"`
	Target    *int64 `json:"target,omitempty" doc:"Required count (min_books_shared only)"`
	Current   *int64 `json:"current,omitempty" doc:"User's current count (min_books_shared only)"`
}

type verificationStatusOutput struct {
	Body struct {
		Eligible bool                 `json:"eligible" doc:"True when all required factors are satisfied"`
		Factors  []verificationFactor `json:"factors" doc:"Status of each configured verification requirement"`
	}
}

// --- Handlers ---

func (h *AuthHandler) verificationStatus(ctx context.Context, _ *struct{}) (*verificationStatusOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	user, err := h.users.FindByID(userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("user not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch user")
	}

	factors := make([]verificationFactor, 0)
	eligible := true

	for _, f := range []*verificationFactor{
		h.emailVerifiedFactor(user),
		h.phoneOnFileFactor(user),
		h.minBooksSharedFactor(userID),
	} {
		if f == nil {
			continue
		}
		factors = append(factors, *f)
		if !f.Satisfied {
			eligible = false
		}
	}

	out := &verificationStatusOutput{}
	out.Body.Eligible = eligible
	out.Body.Factors = factors
	return out, nil
}

// emailVerifiedFactor returns the "verified email" factor if that requirement
// is enabled, or nil if the setting is off.
func (h *AuthHandler) emailVerifiedFactor(user *models.User) *verificationFactor {
	if val, _ := h.admin.GetSetting("require_verified_to_borrow"); val != "true" {
		return nil
	}
	return &verificationFactor{
		Key:       "email",
		Label:     "Verified email address",
		Required:  true,
		Satisfied: user.Verified,
	}
}

// phoneOnFileFactor returns the "phone on file" factor if that requirement
// is enabled, or nil if the setting is off. Checks Phone != "" rather than
// PhoneVerified, matching what checkPhoneRequirement (loan_requests.go)
// has always actually enforced at borrow time. Nothing sets PhoneVerified
// true any more — registration collects a phone without verifying it — so
// checking it here would leave this permanently unsatisfiable.
func (h *AuthHandler) phoneOnFileFactor(user *models.User) *verificationFactor {
	if val, _ := h.admin.GetSetting("verification_requires_phone"); val != "true" {
		return nil
	}
	return &verificationFactor{
		Key:       "phone",
		Label:     "Phone number on file",
		Required:  true,
		Satisfied: user.Phone != "",
	}
}

// minBooksSharedFactor returns the "minimum books shared" factor if a positive
// threshold is configured, or nil if it's unset/zero.
func (h *AuthHandler) minBooksSharedFactor(userID uint) *verificationFactor {
	minStr, _ := h.admin.GetSetting("verification_min_books_shared")
	if minStr == "" || minStr == "0" {
		return nil
	}
	var target int64
	if _, scanErr := fmt.Sscanf(minStr, "%d", &target); scanErr != nil || target <= 0 {
		return nil
	}
	current, _ := h.copies.CountByOwnerID(userID)
	return &verificationFactor{
		Key:       "min_books_shared",
		Label:     fmt.Sprintf("Share at least %d book(s)", target),
		Required:  true,
		Satisfied: current >= target,
		Target:    &target,
		Current:   &current,
	}
}
