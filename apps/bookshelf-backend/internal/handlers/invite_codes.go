package handlers

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/services"
)

// inviteCodeAlphabet/Length: 8-character lowercase alphanumeric, generated
// via crypto/rand. 36^8 ≈ 2.8 trillion combinations — effectively
// unguessable at community scale, short enough to look clean in a shared
// URL. See docs/invite-code-spec.md's "Code format".
const (
	inviteCodeAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	inviteCodeLength   = 8
)

// generateInviteCode returns a cryptographically random 8-character
// lowercase-alphanumeric invite code.
func generateInviteCode() (string, error) {
	b := make([]byte, inviteCodeLength)
	n := big.NewInt(int64(len(inviteCodeAlphabet)))
	for i := range b {
		idx, err := rand.Int(rand.Reader, n)
		if err != nil {
			return "", err
		}
		b[i] = inviteCodeAlphabet[idx.Int64()]
	}
	return string(b), nil
}

// InviteCodeHandler holds dependencies for the invite-code routes — member
// get/regenerate, public validation, and admin list/revoke. Kept in its own
// file rather than folded into auth.go or admin.go, both already at their
// cognitive-complexity ceiling. See docs/invite-code-spec.md.
type InviteCodeHandler struct {
	inviteCodes repository.InviteCodeRepository
	admin       repository.AdminRepository
	users       repository.UserRepository
	email       *services.EmailService
}

// NewInviteCodeHandler creates a new InviteCodeHandler.
func NewInviteCodeHandler(inviteCodes repository.InviteCodeRepository, admin repository.AdminRepository, users repository.UserRepository, email *services.EmailService) *InviteCodeHandler {
	return &InviteCodeHandler{inviteCodes: inviteCodes, admin: admin, users: users, email: email}
}

// --- Input / Output types ---

type inviteCodeOutput struct {
	Body struct {
		Code string `json:"code"`
		URL  string `json:"url"`
	}
}

type validateInviteCodeInput struct {
	Code string `path:"code" doc:"Invite code from a member's link"`
}

type validateInviteCodeOutput struct {
	Body struct {
		Valid       bool   `json:"valid"`
		InviterName string `json:"inviter_name"`
	}
}

// adminInviteCodeEntry is the wire shape for one row in the admin invite
// links table — narrowed to the inviter's name, not the full models.User,
// same reasoning as recommendationEntry in recommendations.go.
type adminInviteCodeEntry struct {
	ID          uint      `json:"id"`
	Code        string    `json:"code"`
	InviterID   uint      `json:"inviter_id"`
	InviterName string    `json:"inviter_name"`
	CreatedAt   time.Time `json:"created_at"`
}

type listInviteCodesOutput struct {
	Body []adminInviteCodeEntry
}

type adminInviteCodeIDInput struct {
	ID uint `path:"id" doc:"Invite code ID"`
}

// --- Route registration ---

// RegisterRoutes registers all invite-code routes on the given huma API.
func (h *InviteCodeHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-invite-code",
		Method:      "GET",
		Path:        "/invite-code",
		Tags:        []string{"invite-codes"},
		Summary:     "Get the caller's invite link, creating it on first access",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.getInviteCode)

	huma.Register(api, huma.Operation{
		OperationID: "regenerate-invite-code",
		Method:      "POST",
		Path:        "/invite-code/regenerate",
		Tags:        []string{"invite-codes"},
		Summary:     "Revoke the caller's current invite link and issue a new one",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.regenerateInviteCode)

	huma.Register(api, huma.Operation{
		OperationID: "validate-invite-code",
		Method:      "GET",
		Path:        "/auth/invite/{code}",
		Tags:        []string{"invite-codes"},
		Summary:     "Check whether an invite code is valid — public, no auth",
	}, h.validateInviteCode)

	huma.Register(api, huma.Operation{
		OperationID: "admin-list-invite-codes",
		Method:      "GET",
		Path:        "/admin/invite-codes",
		Tags:        []string{"admin"},
		Summary:     "List every member's invite link",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.listInviteCodes)

	huma.Register(api, huma.Operation{
		OperationID:   "admin-revoke-invite-code",
		Method:        "DELETE",
		Path:          "/admin/invite-codes/{id}",
		Tags:          []string{"admin"},
		Summary:       "Revoke a member's invite link by ID",
		Security:      []map[string][]string{{"bearer": {}}},
		DefaultStatus: 204,
	}, h.revokeInviteCode)
}

// --- Handlers ---

func (h *InviteCodeHandler) getInviteCode(ctx context.Context, _ *struct{}) (*inviteCodeOutput, error) {
	user, err := h.requireEligibleInviter(ctx)
	if err != nil {
		return nil, err
	}

	existing, err := h.inviteCodes.FindByInviter(user.ID)
	if err == nil {
		return h.inviteCodeResponse(existing), nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, huma.Error500InternalServerError("could not fetch invite code")
	}

	// allow_invite_codes gates creation, not use — see the setting's doc
	// comment in db.Seed. Reached only when the caller has no code yet.
	if val, _ := h.admin.GetSetting("allow_invite_codes"); val == "false" {
		return nil, huma.Error403Forbidden("invite links are currently disabled")
	}

	ic, err := h.createInviteCode(user.ID)
	if err != nil {
		return nil, err
	}
	return h.inviteCodeResponse(ic), nil
}

func (h *InviteCodeHandler) regenerateInviteCode(ctx context.Context, _ *struct{}) (*inviteCodeOutput, error) {
	user, err := h.requireEligibleInviter(ctx)
	if err != nil {
		return nil, err
	}
	if val, _ := h.admin.GetSetting("allow_invite_codes"); val == "false" {
		return nil, huma.Error403Forbidden("invite links are currently disabled")
	}

	code, err := generateInviteCode()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not generate invite code")
	}
	ic, err := h.inviteCodes.Regenerate(user.ID, code)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not regenerate invite code")
	}
	return h.inviteCodeResponse(ic), nil
}

func (h *InviteCodeHandler) validateInviteCode(_ context.Context, input *validateInviteCodeInput) (*validateInviteCodeOutput, error) {
	out := &validateInviteCodeOutput{}
	ic, err := h.inviteCodes.FindByCode(input.Code)
	if err != nil {
		return out, nil
	}
	inviter, err := h.users.FindByID(ic.InviterID)
	if err != nil {
		return out, nil
	}
	out.Body.Valid = true
	out.Body.InviterName = inviter.Name
	return out, nil
}

func (h *InviteCodeHandler) listInviteCodes(ctx context.Context, _ *struct{}) (*listInviteCodesOutput, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, adminError(err)
	}

	codes, err := h.inviteCodes.ListAll()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not list invite codes")
	}
	entries := make([]adminInviteCodeEntry, len(codes))
	for i, ic := range codes {
		entries[i] = adminInviteCodeEntry{
			ID:          ic.ID,
			Code:        ic.Code,
			InviterID:   ic.InviterID,
			InviterName: ic.Inviter.Name,
			CreatedAt:   ic.CreatedAt,
		}
	}
	return &listInviteCodesOutput{Body: entries}, nil
}

func (h *InviteCodeHandler) revokeInviteCode(ctx context.Context, input *adminInviteCodeIDInput) (*struct{}, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, adminError(err)
	}

	if err := h.inviteCodes.DeleteByID(input.ID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("invite code not found")
		}
		return nil, huma.Error500InternalServerError("could not revoke invite code")
	}
	return nil, nil
}

// requireEligibleInviter loads the authenticated caller and checks they're
// eligible to hold an invite link: verified. Suspended and pending-approval
// callers never reach here — RequireActiveUser middleware
// (internal/middleware/auth.go) already rejects them with 403 before any
// handler runs, so this doesn't re-check those two.
func (h *InviteCodeHandler) requireEligibleInviter(ctx context.Context) (*models.User, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}
	user, err := h.users.FindByID(userID)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}
	if !user.Verified {
		return nil, huma.Error403Forbidden("only verified members can create invite links")
	}
	return user, nil
}

// createInviteCode generates a fresh code and persists it for inviterID.
func (h *InviteCodeHandler) createInviteCode(inviterID uint) (*models.InviteCode, error) {
	code, err := generateInviteCode()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not generate invite code")
	}
	ic, err := h.inviteCodes.FindOrCreateByInviter(inviterID, code)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not create invite code")
	}
	return ic, nil
}

func (h *InviteCodeHandler) inviteCodeResponse(ic *models.InviteCode) *inviteCodeOutput {
	out := &inviteCodeOutput{}
	out.Body.Code = ic.Code
	out.Body.URL = h.email.URL("/register?invite=" + ic.Code)
	return out
}
