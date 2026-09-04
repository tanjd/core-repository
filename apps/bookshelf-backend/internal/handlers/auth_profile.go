package handlers

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// --- Input / Output types ---

type meBody struct {
	models.User
	GoogleBooksKeyConfigured bool `json:"google_books_key_configured"`
	// TelegramLinked is computed from User.TelegramChatID, which itself
	// carries json:"-" (no reason to expose the raw chat ID to the
	// frontend) — same "expose a derived bool, not the underlying secret
	// field" shape as GoogleBooksKeyConfigured above.
	TelegramLinked bool `json:"telegram_linked"`
}

type meOutput struct{ Body meBody }

type updateMeOutput struct {
	Body struct {
		models.User
		GoogleBooksKeyConfigured bool   `json:"google_books_key_configured"`
		TelegramLinked           bool   `json:"telegram_linked"`
		PendingEmailDebugCode    string `json:"pending_email_debug_code,omitempty" doc:"Only present when ENV=dev: the code sent to confirm a pending email change, so local development doesn't require SMTP"`
	}
}

type updateMeInput struct {
	Body updateMeBody
}

type updateMeBody struct {
	Name                      *string `json:"name,omitempty" doc:"New display name"`
	Phone                     *string `json:"phone,omitempty" doc:"Contact phone number"`
	Email                     *string `json:"email,omitempty" format:"email" doc:"New email address"`
	GoogleBooksAPIKey         *string `json:"google_books_api_key,omitempty" doc:"Your Google Books API key. Set to empty string to remove."`
	EmailNotificationsEnabled *bool   `json:"email_notifications_enabled,omitempty" doc:"Whether to receive non-transactional notification emails (loan requests, wishlist matches). Account/security emails are unaffected."`
	MonthlyDigestEnabled      *bool   `json:"monthly_digest_enabled,omitempty" doc:"Whether to receive the monthly community digest email (new books, top recommended)."`
	// TelegramNotificationsEnabled is rejected with 400 unless the member
	// has already linked Telegram (see POST /profile/telegram/link-token) —
	// there's nothing to toggle before that. Unrelated to TelegramUsername
	// below.
	TelegramNotificationsEnabled *bool   `json:"telegram_notifications_enabled,omitempty" doc:"Whether to receive Telegram push notifications. Requires Telegram to already be linked."`
	TelegramUsername             *string `json:"telegram_username,omitempty" maxLength:"100" doc:"Telegram username, for other members to reach you. Set to empty string to remove."`
	WhatsAppUsername             *string `json:"whatsapp_username,omitempty" maxLength:"100" doc:"WhatsApp username, for other members to reach you. Set to empty string to remove."`
	ContactNote                  *string `json:"contact_note,omitempty" maxLength:"500" doc:"Free-text note on the best way/times to arrange pickup, shown to the other party once a loan request is accepted. Set to empty string to remove."`
}

type testGoogleBooksKeyInput struct {
	Body struct {
		Key string `json:"key,omitempty" doc:"Key to test. Omit to test the currently stored key."`
	}
}

type testGoogleBooksKeyOutput struct {
	Body struct {
		OK      bool   `json:"ok"`
		Message string `json:"message,omitempty"`
	}
}

type confirmEmailChangeInput struct {
	Body struct {
		Code  string `json:"code,omitempty" doc:"6-digit code sent to the new email address. Required unless token is set."`
		Token string `json:"token,omitempty" doc:"Magic-link token from the confirmation email, as an alternative to submitting code."`
	}
}

// --- Handlers ---

func (h *AuthHandler) me(ctx context.Context, _ *struct{}) (*meOutput, error) {
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

	if user.Suspended {
		return nil, huma.Error403Forbidden("this account has been suspended")
	}

	if user.PendingApproval {
		return nil, huma.Error403Forbidden("this account is pending admin approval")
	}

	return &meOutput{Body: meBody{
		User:                     *user,
		GoogleBooksKeyConfigured: user.GoogleBooksAPIKey != "",
		TelegramLinked:           user.TelegramChatID != nil,
	}}, nil
}

func (h *AuthHandler) updateMe(ctx context.Context, input *updateMeInput) (*updateMeOutput, error) {
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

	if user.Suspended {
		return nil, huma.Error403Forbidden("this account has been suspended")
	}

	if user.PendingApproval {
		return nil, huma.Error403Forbidden("this account is pending admin approval")
	}

	pendingEmailDebugCode, err := h.applyUpdateMeFields(ctx, user, input.Body)
	if err != nil {
		return nil, err
	}

	if err := h.users.Save(user); err != nil {
		return nil, huma.Error500InternalServerError("could not update user")
	}

	out := &updateMeOutput{}
	out.Body.User = *user
	out.Body.GoogleBooksKeyConfigured = user.GoogleBooksAPIKey != ""
	out.Body.TelegramLinked = user.TelegramChatID != nil
	out.Body.PendingEmailDebugCode = pendingEmailDebugCode
	return out, nil
}

// applyUpdateMeFields applies every field of an updateMe request to user
// (except the Suspended/PendingApproval guards, already checked by the
// caller). Split out from updateMe to keep its cognitive complexity under
// the repo's gocognit threshold — see apps/bookshelf-backend/CLAUDE.md's
// "Go tooling" section for the same pattern applied to createLoanRequest.
func (h *AuthHandler) applyUpdateMeFields(ctx context.Context, user *models.User, body updateMeBody) (pendingEmailDebugCode string, err error) {
	if body.Name != nil {
		user.Name = *body.Name
	}
	if body.Phone != nil {
		applyPhoneUpdate(user, *body.Phone)
	}
	if body.Email != nil {
		pendingEmailDebugCode, err = h.handleEmailUpdateRequest(ctx, user, *body.Email)
		if err != nil {
			return "", err
		}
	}
	if body.GoogleBooksAPIKey != nil {
		if err := h.applyGoogleBooksKeyUpdate(user, *body.GoogleBooksAPIKey); err != nil {
			return "", err
		}
	}
	if err := applyContactPrefsUpdate(user, body); err != nil {
		return "", err
	}
	return pendingEmailDebugCode, nil
}

// applyContactPrefsUpdate applies the notification-preference and
// messaging-username fields of an updateMe request. Split out from updateMe
// to keep its cognitive complexity under the repo's gocognit threshold.
// Returns an error only for TelegramNotificationsEnabled's link-required
// guard — every other field is unconditionally applicable.
func applyContactPrefsUpdate(user *models.User, body updateMeBody) error {
	if body.EmailNotificationsEnabled != nil {
		user.EmailNotificationsEnabled = *body.EmailNotificationsEnabled
	}
	if body.MonthlyDigestEnabled != nil {
		user.MonthlyDigestEnabled = *body.MonthlyDigestEnabled
	}
	if body.TelegramNotificationsEnabled != nil {
		if user.TelegramChatID == nil {
			return huma.Error400BadRequest("link Telegram before changing this setting")
		}
		user.TelegramNotificationsEnabled = *body.TelegramNotificationsEnabled
	}
	if body.TelegramUsername != nil {
		user.TelegramUsername = *body.TelegramUsername
	}
	if body.WhatsAppUsername != nil {
		user.WhatsAppUsername = *body.WhatsAppUsername
	}
	if body.ContactNote != nil {
		user.ContactNote = strings.TrimSpace(*body.ContactNote)
	}
	return nil
}

// applyPhoneUpdate sets the user's phone number, clearing PhoneVerified when
// it actually changes — a new number hasn't been verified even if the old
// one was, and leaving the flag set would let it lie about the new number.
func applyPhoneUpdate(user *models.User, newPhone string) {
	if newPhone == user.Phone {
		return
	}
	user.Phone = newPhone
	user.PhoneVerified = false
}

// handleEmailUpdateRequest routes an email field on updateMe's input to the
// right behavior: unchanged email with a pending change cancels it; a
// changed email either applies immediately or stages a pending
// confirmation, depending on the require_email_confirmation_on_change flag
// (defaults to "true" in db.Seed() — see the comment there for why). Returns
// the pending-change OTP code when ENV=dev (see requestEmailChange), or ""
// otherwise.
func (h *AuthHandler) handleEmailUpdateRequest(ctx context.Context, user *models.User, newEmail string) (string, error) {
	if newEmail == user.Email {
		if user.PendingEmail != "" {
			// User resubmitted their current (unchanged) email while a change
			// was pending — treat as cancelling the pending change rather than
			// leaving stale pending state.
			user.PendingEmail = ""
			user.PendingEmailOTPCode = ""
			user.PendingEmailOTPExpiry = nil
		}
		return "", nil
	}

	if val, _ := h.admin.GetSetting("require_email_confirmation_on_change"); val == "true" {
		return h.requestEmailChange(ctx, user, newEmail)
	}
	return "", h.applyEmailUpdate(user, newEmail)
}

// applyEmailUpdate changes the user's email, rejecting duplicates and
// resetting verification state since a new email must be re-verified.
func (h *AuthHandler) applyEmailUpdate(user *models.User, newEmail string) error {
	existing, findErr := h.users.FindByEmail(newEmail)
	if findErr == nil && existing.ID != user.ID {
		return huma.Error400BadRequest("email already in use")
	}
	user.Email = newEmail
	user.Verified = false
	user.OTPCode = ""
	user.OTPExpiry = nil
	return nil
}

// requestEmailChange stages a pending email change and sends a confirmation
// OTP to the *new* address; user.Email is left unchanged until
// confirmEmailChange succeeds. Returns the code when ENV=dev, so local
// development doesn't require SMTP — same convention as the registration and
// password-reset OTP flows.
func (h *AuthHandler) requestEmailChange(ctx context.Context, user *models.User, newEmail string) (string, error) {
	existing, findErr := h.users.FindByEmail(newEmail)
	if findErr == nil && existing.ID != user.ID {
		return "", huma.Error400BadRequest("email already in use")
	}

	code, err := generateOTPCode()
	if err != nil {
		return "", huma.Error500InternalServerError("could not generate OTP")
	}
	expiry := time.Now().Add(15 * time.Minute)

	user.PendingEmail = newEmail
	user.PendingEmailOTPCode = code
	user.PendingEmailOTPExpiry = &expiry

	body := fmt.Sprintf(
		"<p>Hi %s,</p><p>You requested to change your Bookshelf account email to this address. Your confirmation code is: <strong>%s</strong></p><p>This code expires in 15 minutes. If you didn't request this change, you can safely ignore this email.</p>",
		html.EscapeString(user.Name), code,
	)
	if linkToken, tokenErr := h.issueOTPLinkToken(otpLinkPurposeEmailChange, fmt.Sprint(user.ID), code); tokenErr == nil {
		body += h.email.Button(fmt.Sprintf("/profile?confirmEmailToken=%s", url.QueryEscape(linkToken)), "Confirm email change")
	}
	h.email.SendEmailAsync(ctx, newEmail, "Confirm your new Bookshelf email address", body)

	if h.env == "dev" {
		return code, nil
	}
	return "", nil
}

// applyGoogleBooksKeyUpdate sets or clears the user's stored (encrypted) Google Books API key.
func (h *AuthHandler) applyGoogleBooksKeyUpdate(user *models.User, newKey string) error {
	if newKey == "" {
		user.GoogleBooksAPIKey = ""
		return nil
	}
	encrypted, err := encryptField(newKey, h.encryptionSecret)
	if err != nil {
		return huma.Error500InternalServerError("could not save API key")
	}
	user.GoogleBooksAPIKey = encrypted
	return nil
}

func (h *AuthHandler) testGoogleBooksKey(ctx context.Context, input *testGoogleBooksKeyInput) (*testGoogleBooksKeyOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	key := input.Body.Key
	if key == "" {
		user, err := h.users.FindByID(userID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, huma.Error404NotFound("user not found")
			}
			return nil, huma.Error500InternalServerError("could not fetch user")
		}
		if user.GoogleBooksAPIKey == "" {
			return nil, huma.Error400BadRequest("no Google Books API key configured")
		}
		decrypted, err := decryptField(user.GoogleBooksAPIKey, h.encryptionSecret)
		if err != nil {
			return nil, huma.Error500InternalServerError("could not read stored key")
		}
		key = decrypted
	}

	out := &testGoogleBooksKeyOutput{}
	if err := validateGoogleBooksAPIKey(ctx, key); err != nil {
		out.Body.OK = false
		out.Body.Message = err.Error()
	} else {
		out.Body.OK = true
	}
	return out, nil
}

func (h *AuthHandler) confirmEmailChange(ctx context.Context, input *confirmEmailChangeInput) (*meOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	user, err := h.users.FindByID(userID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch user")
	}

	if user.Suspended {
		return nil, huma.Error403Forbidden("this account has been suspended")
	}

	if user.PendingApproval {
		return nil, huma.Error403Forbidden("this account is pending admin approval")
	}

	code, err := h.resolveCode(input.Body.Token, otpLinkPurposeEmailChange, input.Body.Code)
	if err != nil {
		return nil, err
	}

	if user.PendingEmail == "" || user.PendingEmailOTPCode == "" || user.PendingEmailOTPExpiry == nil {
		return nil, huma.Error400BadRequest("no pending email change")
	}
	if time.Now().After(*user.PendingEmailOTPExpiry) {
		return nil, huma.Error400BadRequest("confirmation code has expired")
	}
	if subtle.ConstantTimeCompare([]byte(user.PendingEmailOTPCode), []byte(code)) != 1 {
		user.PendingEmailOTPCode = ""
		user.PendingEmailOTPExpiry = nil
		_ = h.users.Save(user) //nolint:errcheck
		return nil, huma.Error400BadRequest("invalid confirmation code — please request a new one")
	}

	// Re-check for a race: someone else may have claimed this email between
	// the original request and this confirmation.
	if existing, findErr := h.users.FindByEmail(user.PendingEmail); findErr == nil && existing.ID != user.ID {
		user.PendingEmail = ""
		user.PendingEmailOTPCode = ""
		user.PendingEmailOTPExpiry = nil
		_ = h.users.Save(user) //nolint:errcheck
		return nil, huma.Error400BadRequest("email already in use")
	}

	user.Email = user.PendingEmail
	user.Verified = true
	user.PendingEmail = ""
	user.PendingEmailOTPCode = ""
	user.PendingEmailOTPExpiry = nil
	// Clear any unrelated leftover signup-verification OTP state too, since
	// verification is now settled by this confirmation.
	user.OTPCode = ""
	user.OTPExpiry = nil
	if err := h.users.Save(user); err != nil {
		return nil, huma.Error500InternalServerError("could not update user")
	}

	return &meOutput{Body: meBody{
		User:                     *user,
		GoogleBooksKeyConfigured: user.GoogleBooksAPIKey != "",
		TelegramLinked:           user.TelegramChatID != nil,
	}}, nil
}
