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

	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// passwordResetOTPRateLimitAttempts/Window cap forgot-password requests per
// email address — same identifier-keyed rationale as loginRateLimit above,
// and the same send-cadence as the registration OTP limiters.
const (
	passwordResetOTPRateLimitAttempts = 3
	passwordResetOTPRateLimitWindow   = 5 * time.Minute
)

// passwordResetAttemptRateLimitAttempts/Window cap reset-password submissions
// (code + new password) per email address. A wrong code already invalidates
// itself (see resetPassword), so this mainly guards against a burst of
// requests exhausting the enumeration-safe "always 200" response on
// forgotPassword's sibling endpoint.
const (
	passwordResetAttemptRateLimitAttempts = 5
	passwordResetAttemptRateLimitWindow   = 15 * time.Minute
)

// passwordResetOTPExpiry is how long a forgot-password OTP code stays valid.
const passwordResetOTPExpiry = 15 * time.Minute

// minPasswordLength follows OWASP ASVS / NIST SP 800-63B guidance to favor
// length over arbitrary composition rules — 12 is the commonly cited floor
// for a system with no other compensating controls (rate limiting, MFA).
const minPasswordLength = 12

// maxPasswordLength guards against bcrypt's silent truncation at 72 bytes:
// golang.org/x/crypto/bcrypt ignores everything past byte 72, so without
// this check "the first 72 bytes of X" and "X" would hash identically for
// any longer X, silently weakening (not lengthening) the effective secret.
const maxPasswordLength = 72

// commonPasswords is a small denylist of passwords that appear at the top
// of nearly every breach corpus (SplashData/NordPass "worst passwords"
// lists) and dictionary/keyboard-walk patterns. It's not a substitute for a
// real breached-password database (e.g. HaveIBeenPwned's k-anonymity API),
// which would add an external network dependency this NAS-hosted app can't
// rely on being reachable — this catches the most trivially guessable
// passwords for free.
var commonPasswords = map[string]struct{}{
	"123456": {}, "123456789": {}, "12345678": {}, "1234567890": {}, "1234567": {},
	"1234": {}, "12345": {}, "111111": {}, "000000": {}, "123123": {},
	"password": {}, "password1": {}, "password123": {}, "passw0rd": {}, "iloveyou": {},
	"qwerty": {}, "qwerty123": {}, "qwertyuiop": {}, "qazwsx": {}, "azerty": {},
	"abc123": {}, "letmein": {}, "letmein1": {}, "welcome": {}, "welcome1": {},
	"monkey": {}, "dragon": {}, "master": {}, "shadow": {}, "superman": {},
	"batman": {}, "football": {}, "baseball": {}, "starwars": {}, "sunshine": {},
	"princess": {}, "flower": {}, "freedom": {}, "whatever": {}, "trustno1": {},
	"admin": {}, "admin123": {}, "administrator": {}, "root": {}, "toor": {},
	"guest": {}, "guest123": {}, "test": {}, "test123": {}, "temp123": {},
	"changeme": {}, "changeme123": {}, "hunter2": {}, "michael": {}, "ashley": {},
	"bailey": {}, "jennifer": {}, "jordan": {}, "michelle": {}, "mustang": {},
	"ninja": {}, "121212": {}, "123321": {}, "654321": {}, "1q2w3e4r": {},
	"zaq1zaq1": {}, "1qaz2wsx": {}, "aa123456": {}, "google": {}, "bookshelf": {},
	"bookshelf1": {}, "bookshelf123": {},
}

// validatePasswordComplexity checks that p meets minimum complexity
// requirements. disallowed is a set of user-identifying strings (e.g. name,
// email local part) that must not appear (case-insensitively) within p.
// Returns a human-readable error string, or "" if valid.
func validatePasswordComplexity(p string, disallowed ...string) string {
	if msg := validatePasswordLength(p); msg != "" {
		return msg
	}
	if msg := validatePasswordCharacterClasses(p); msg != "" {
		return msg
	}
	return validatePasswordNotGuessable(p, disallowed...)
}

func validatePasswordLength(p string) string {
	if len(p) < minPasswordLength {
		return fmt.Sprintf("password must be at least %d characters", minPasswordLength)
	}
	if len(p) > maxPasswordLength {
		return fmt.Sprintf("password must be at most %d characters", maxPasswordLength)
	}
	return ""
}

func validatePasswordCharacterClasses(p string) string {
	var hasUpper, hasLower, hasDigit bool
	for _, c := range p {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		}
	}
	if !hasUpper {
		return "password must contain at least one uppercase letter"
	}
	if !hasLower {
		return "password must contain at least one lowercase letter"
	}
	if !hasDigit {
		return "password must contain at least one number"
	}
	return ""
}

// validatePasswordNotGuessable rejects passwords pulled straight from a
// common-password denylist, or that embed one of the caller-supplied
// user-identifying strings (name, email local part).
func validatePasswordNotGuessable(p string, disallowed ...string) string {
	lower := strings.ToLower(p)
	if _, common := commonPasswords[lower]; common {
		return "this password is too common — please choose a stronger one"
	}
	for _, d := range disallowed {
		d = strings.ToLower(strings.TrimSpace(d))
		if d != "" && len(d) >= 3 && strings.Contains(lower, d) {
			return "password must not contain your name or email"
		}
	}
	return ""
}

// --- Input / Output types ---

type forgotPasswordInput struct {
	Body struct {
		Email string `json:"email" required:"true" format:"email" doc:"Account email address"`
	}
}

type forgotPasswordOutput struct {
	Body struct {
		DebugCode      string `json:"debug_code,omitempty" doc:"Only present when ENV=dev: the code, so local development doesn't require SMTP"`
		DebugResetLink string `json:"debug_reset_link,omitempty" doc:"Only present when ENV=dev: the magic-link URL the email would contain — no more sensitive than debug_code, which alone is already sufficient to reset the password"`
	}
}

type resetPasswordInput struct {
	Body struct {
		Email           string `json:"email,omitempty" format:"email" doc:"Account email address. Required unless token is set."`
		Code            string `json:"code,omitempty" doc:"6-digit code sent to the account's email. Required unless token is set."`
		Token           string `json:"token,omitempty" doc:"Magic-link token from the reset email, as an alternative to submitting email+code."`
		NewPassword     string `json:"new_password" required:"true" minLength:"12" doc:"New password (min 12 chars, mixed case + digit)"`
		ConfirmPassword string `json:"confirm_password" required:"true" minLength:"1" doc:"Must match new_password"`
	}
}

type changePasswordInput struct {
	Body struct {
		CurrentPassword string `json:"current_password" required:"true" minLength:"1" doc:"Current password"`
		NewPassword     string `json:"new_password" required:"true" minLength:"12" doc:"New password (min 12 chars, mixed case + digit)"`
		ConfirmPassword string `json:"confirm_password" required:"true" minLength:"1" doc:"Must match new_password"`
	}
}

// --- Handlers ---

// forgotPassword generates and emails a 6-digit reset code for the given
// email address, if an account for it exists. It always returns a
// successful (non-error) response regardless of whether the account exists,
// so a caller can't use this endpoint to enumerate registered emails — the
// same reason login/register error messages are kept generic elsewhere in
// this file.
func (h *AuthHandler) forgotPassword(ctx context.Context, input *forgotPasswordInput) (*forgotPasswordOutput, error) {
	email := normalizeEmail(input.Body.Email)
	if !h.forgotPasswordLimiter.Allow(email) {
		return nil, huma.Error429TooManyRequests("too many requests — try again later")
	}

	out := &forgotPasswordOutput{}

	user, err := h.users.FindByEmail(input.Body.Email)
	if err != nil {
		return out, nil
	}

	code, err := generateOTPCode()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not generate reset code")
	}
	expiry := time.Now().Add(passwordResetOTPExpiry)

	user.ResetPasswordOTPCode = code
	user.ResetPasswordOTPExpiry = &expiry
	if err := h.users.Save(user); err != nil {
		return nil, huma.Error500InternalServerError("could not save reset code")
	}

	body := fmt.Sprintf(
		"<p>Hi %s,</p><p>Your Bookshelf password reset code is: <strong>%s</strong></p><p>This code expires in 15 minutes. If you didn't request this, you can safely ignore this email — your password won't change.</p>",
		html.EscapeString(user.Name), code,
	)
	resetPath := ""
	// user.Email, not the normalized `email` above: resetPassword's
	// token path feeds this identifier straight into FindByEmail, a
	// case-sensitive lookup — a lowercased identifier would fail to find
	// an account whose stored email contains uppercase characters.
	if linkToken, tokenErr := h.issueOTPLinkToken(otpLinkPurposeResetPassword, user.Email, code); tokenErr == nil {
		resetPath = fmt.Sprintf("/forgot-password?resetToken=%s", url.QueryEscape(linkToken))
		body += h.email.Button(resetPath, "Reset password")
	}
	h.email.SendEmailAsync(ctx, user.Email, "Reset your Bookshelf password", body)

	if h.env == "dev" {
		out.Body.DebugCode = code
		if resetPath != "" {
			out.Body.DebugResetLink = h.email.URL(resetPath)
		}
	}
	return out, nil
}

// resetPassword completes a forgot-password flow: it checks the emailed code
// (constant-time compare, invalidate-on-wrong-attempt — same anti-enumeration
// shape as verifyOTP/checkRegistrationOTP) and, on success, sets the new
// password. Errors are deliberately identical for "no such account", "no
// code was requested", "code expired", and "code doesn't match", so a caller
// can't distinguish an unregistered email from a wrong code.
func (h *AuthHandler) resetPassword(ctx context.Context, input *resetPasswordInput) (*struct{}, error) {
	rawEmail, code, err := h.resolveEmailAndCode(input.Body.Token, otpLinkPurposeResetPassword, input.Body.Email, input.Body.Code)
	if err != nil {
		return nil, err
	}
	email := normalizeEmail(rawEmail)
	if !h.resetPasswordLimiter.Allow(email) {
		return nil, huma.Error429TooManyRequests("too many attempts — try again later")
	}

	invalidCodeErr := huma.Error400BadRequest("invalid or expired code")

	user, err := h.users.FindByEmail(rawEmail)
	if err != nil {
		return nil, invalidCodeErr
	}

	if user.ResetPasswordOTPCode == "" || user.ResetPasswordOTPExpiry == nil {
		return nil, invalidCodeErr
	}
	if time.Now().After(*user.ResetPasswordOTPExpiry) {
		user.ResetPasswordOTPCode = ""
		user.ResetPasswordOTPExpiry = nil
		_ = h.users.Save(user) //nolint:errcheck
		return nil, invalidCodeErr
	}
	if subtle.ConstantTimeCompare([]byte(user.ResetPasswordOTPCode), []byte(code)) != 1 {
		user.ResetPasswordOTPCode = ""
		user.ResetPasswordOTPExpiry = nil
		_ = h.users.Save(user) //nolint:errcheck
		return nil, invalidCodeErr
	}

	if input.Body.NewPassword != input.Body.ConfirmPassword {
		return nil, huma.Error400BadRequest("new passwords do not match")
	}
	if msg := validatePasswordComplexity(input.Body.NewPassword, user.Name, emailLocalPart(user.Email)); msg != "" {
		return nil, huma.Error400BadRequest(msg)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Body.NewPassword), 12)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not hash password")
	}

	user.Password = string(hash)
	user.ResetPasswordOTPCode = ""
	user.ResetPasswordOTPExpiry = nil
	if err := h.users.Save(user); err != nil {
		return nil, huma.Error500InternalServerError("could not update password")
	}

	zerolog.Ctx(ctx).Info().Uint("user_id", user.ID).Msg("password reset via forgot-password flow")
	return nil, nil
}

func (h *AuthHandler) changePassword(ctx context.Context, input *changePasswordInput) (*struct{}, error) {
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

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Body.CurrentPassword)); err != nil {
		return nil, huma.Error400BadRequest("current password is incorrect")
	}

	if input.Body.NewPassword != input.Body.ConfirmPassword {
		return nil, huma.Error400BadRequest("new passwords do not match")
	}

	if msg := validatePasswordComplexity(input.Body.NewPassword, user.Name, emailLocalPart(user.Email)); msg != "" {
		return nil, huma.Error400BadRequest(msg)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Body.NewPassword), 12)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not hash password")
	}

	user.Password = string(hash)
	if err := h.users.Save(user); err != nil {
		return nil, huma.Error500InternalServerError("could not update password")
	}

	zerolog.Ctx(ctx).Info().Uint("user_id", userID).Msg("password changed")
	return nil, nil
}
