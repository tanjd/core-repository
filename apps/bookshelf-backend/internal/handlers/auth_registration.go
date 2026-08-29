package handlers

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// registrationOTPRateLimitAttempts/Window cap send-otp requests during
// registration per identifier (email or phone) — see loginRateLimit's
// comment above for why this is identifier-keyed rather than IP-keyed.
const (
	registrationOTPRateLimitAttempts = 3
	registrationOTPRateLimitWindow   = 5 * time.Minute
)

// registrationOTPExpiry is how long a pre-registration email/phone OTP code
// stays valid.
const registrationOTPExpiry = 15 * time.Minute

// registrationVerificationTokenTTL is how long a verify-phone-otp success
// token remains valid.
//
// Tech debt: nothing consumes this token any more. /auth/register, its only
// consumer, is gone — verify-email-otp now creates the account itself, so
// there's no second call to hand a proof-of-verification to. The phone
// endpoints and this token are kept working (rather than deleted) against a
// future real SMS provider; see the phone-OTP note on sendRegisterPhoneOTP.
const registrationVerificationTokenTTL = 30 * time.Minute

// registrationPurposePhone is embedded as a claim in a verify-phone-otp
// token so it can't be replayed as some other kind of token.
const registrationPurposePhone = "phone_verify"

const (
	registrationChannelEmail = "email"
	registrationChannelPhone = "phone"
)

// --- Input / Output types ---

// sendRegisterEmailOTPInput carries the whole signup form, not just the
// address to verify: the account details move server-side here (parked on
// the registration_verifications row, same 15-minute single-use lifecycle as
// the code itself) so that verifying the code later — from any device,
// including one that never saw this form — is enough to create the account.
// The password is hashed before it's stored and never leaves the backend;
// see the note on issueOTPLinkToken's caller for why it isn't carried in the
// magic-link token instead.
type sendRegisterEmailOTPInput struct {
	Body struct {
		Name       string  `json:"name" required:"true" minLength:"1" doc:"Display name"`
		Email      string  `json:"email" required:"true" format:"email" doc:"Email address to verify"`
		Password   string  `json:"password" required:"true" minLength:"12" doc:"Password (min 12 chars)"`
		Phone      *string `json:"phone,omitempty" doc:"Contact phone number (optional — never verified, and never blocks registration; see verification_requires_phone, which gates borrowing rather than signup)"`
		InviteCode string  `json:"invite_code,omitempty" doc:"Invite code from a member's link — bypasses allow_registration and require_registration_approval when valid"`
	}
}

type sendRegisterEmailOTPOutput struct {
	Body struct {
		DebugCode       string `json:"debug_code,omitempty" doc:"Only present when ENV=dev: the code, so local development doesn't require SMTP"`
		DebugVerifyLink string `json:"debug_verify_link,omitempty" doc:"Only present when ENV=dev: the magic-link URL the email would contain — no more sensitive than debug_code, which alone is already sufficient to finish registration"`
	}
}

type verifyRegisterEmailOTPInput struct {
	Body struct {
		Email string `json:"email,omitempty" format:"email" doc:"Email address being verified. Required unless token is set."`
		Code  string `json:"code,omitempty" doc:"6-digit code. Required unless token is set."`
		Token string `json:"token,omitempty" doc:"Magic-link token from the verification email, as an alternative to submitting email+code."`
	}
}

// registrationResult is what a successful verify-email-otp returns: the
// account now exists either way, and Status says whether the caller is
// signed in or waiting on an admin.
type registrationResult struct {
	Status string      `json:"status" enum:"complete,pending_approval" doc:"\"complete\" — the account is created and Token is a live session; \"pending_approval\" — the account is created but held for an admin, and there is no token"`
	Token  string      `json:"token,omitempty" doc:"Session JWT. Present only when status is \"complete\"."`
	User   models.User `json:"user"`
}

type verifyRegisterEmailOTPOutput struct {
	Body registrationResult
}

type sendRegisterPhoneOTPInput struct {
	Body struct {
		Phone string `json:"phone" required:"true" doc:"Phone number to verify"`
	}
}

type sendRegisterPhoneOTPOutput struct {
	Body struct {
		MockCode string `json:"mock_code" doc:"Phone verification is mocked — no SMS provider is configured, so the code is returned directly instead of being texted"`
	}
}

type verifyRegisterPhoneOTPInput struct {
	Body struct {
		Phone string `json:"phone" required:"true" doc:"Phone number being verified"`
		Code  string `json:"code" required:"true" doc:"6-digit code"`
	}
}

type verifyRegisterPhoneOTPOutput struct {
	Body struct {
		VerificationToken string `json:"verification_token" doc:"Pass this as phone_verification_token to /auth/register"`
	}
}

// registrationVerificationClaims is the JWT payload minted by
// verify-phone-otp. Stateless by design — unlike the OTP code itself
// (DB-backed, see registrationOTPExpiry), this token needs no storage;
// validating it later is just a signature/claims check.
//
// Tech debt: no endpoint validates it any more — see
// registrationVerificationTokenTTL.
type registrationVerificationClaims struct {
	Purpose    string `json:"purpose"`
	Identifier string `json:"identifier"`
	jwt.RegisteredClaims
}

type setupStatusOutput struct {
	Body struct {
		NeedsSetup bool `json:"needs_setup"`
	}
}

type registrationRequirementsOutput struct {
	Body struct {
		RequirePhone bool `json:"require_phone" doc:"Whether this community requires a phone number on file before a member can borrow. Registration never blocks on it — the field is collected at signup so the requirement is already met by the time it's enforced."`
	}
}

type setupInput struct {
	Body struct {
		Name     string `json:"name" required:"true" minLength:"1" doc:"Admin display name"`
		Email    string `json:"email" required:"true" format:"email" doc:"Admin email address"`
		Password string `json:"password" required:"true" minLength:"12" doc:"Admin password (min 12 chars)"`
	}
}

// --- Handlers ---

// sendRegisterEmailOTP starts a registration: it validates the submitted
// account details, hashes the password, parks all of it on a
// registration_verifications row keyed by the (normalized) email, and emails
// a 6-digit code plus a magic link that carries the same code. Nothing is
// created yet — verifyRegisterEmailOTP turns the parked row into a real
// account once the address is proven. See EmailService.SendEmail for why the
// send no-ops (but still logs the code) when SMTP isn't configured.
//
// The password is hashed here rather than at account-creation time so the
// plaintext never has to be held anywhere between the two calls. That moves
// bcrypt (~150–300ms at cost 12) earlier in the flow, but this endpoint
// already rejects disabled registration, duplicate emails, weak passwords
// and over-rate callers before reaching it, so it isn't a new abuse surface.
func (h *AuthHandler) sendRegisterEmailOTP(ctx context.Context, input *sendRegisterEmailOTPInput) (*sendRegisterEmailOTPOutput, error) {
	pendingInviteCode, err := h.validatePendingInviteCode(input.Body.InviteCode)
	if err != nil {
		return nil, err
	}
	// An invite code is the vouch that lets this signup skip the gate —
	// see docs/invite-code-spec.md's "Registration flow changes". Without
	// one, behavior is unchanged from before this feature existed.
	if pendingInviteCode == "" {
		if val, _ := h.admin.GetSetting("allow_registration"); val == "false" {
			return nil, huma.Error403Forbidden("registration is currently disabled")
		}
	}
	email := normalizeEmail(input.Body.Email)
	if !h.emailOTPLimiter.Allow(email) {
		return nil, huma.Error429TooManyRequests("too many verification requests — try again later")
	}
	// Checked against the normalized address, matching the key the pending
	// row is about to be written under — and FindByEmail itself is
	// case-insensitive (see its doc comment), so a case variant of an
	// existing account is caught here rather than becoming a second account
	// for the same mailbox at finalize time.
	if _, err := h.users.FindByEmail(email); err == nil {
		return nil, huma.Error400BadRequest("email already registered")
	}
	if msg := validatePasswordComplexity(input.Body.Password, input.Body.Name, emailLocalPart(input.Body.Email)); msg != "" {
		return nil, huma.Error400BadRequest(msg)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Body.Password), 12)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not hash password")
	}

	code, err := generateOTPCode()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not generate OTP")
	}
	pending := models.PendingRegistrationData{
		PendingName: strings.TrimSpace(input.Body.Name),
		// Stored as typed, unlike the normalized identifier this row is keyed
		// by — see PendingRegistrationData.PendingEmail.
		PendingEmail:        strings.TrimSpace(input.Body.Email),
		PendingPasswordHash: string(hash),
		PendingPhone:        trimmedPhone(input.Body.Phone),
		PendingInviteCode:   pendingInviteCode,
	}
	if err := h.registrationVerifications.Upsert(
		registrationChannelEmail, email, code, time.Now().Add(registrationOTPExpiry), pending,
	); err != nil {
		return nil, huma.Error500InternalServerError("could not save verification code")
	}

	body := fmt.Sprintf(
		"<p>Your Bookshelf email verification code is: <strong>%s</strong></p><p>This code expires in 15 minutes.</p>",
		code,
	)
	verifyPath := ""
	// The link token stays as narrow as every other magic link here
	// ({purpose, identifier, code}) — deliberately *not* carrying the
	// name/password hash parked above. It's the one part of this flow that
	// transits email, URLs, browser history and link scanners, so it stays
	// minimal.
	if linkToken, tokenErr := h.issueOTPLinkToken(otpLinkPurposeRegisterEmail, email, code); tokenErr == nil {
		verifyPath = fmt.Sprintf("/register?verifyToken=%s", url.QueryEscape(linkToken))
		body += h.email.Button(verifyPath, "Verify email")
	}
	h.email.SendEmailAsync(ctx, input.Body.Email, "Verify your email for Bookshelf", body)

	out := &sendRegisterEmailOTPOutput{}
	if h.env == "dev" {
		out.Body.DebugCode = code
		if verifyPath != "" {
			out.Body.DebugVerifyLink = h.email.URL(verifyPath)
		}
	}
	return out, nil
}

// validatePendingInviteCode checks a raw invite code submitted at
// send-email-otp time, returning it unchanged (ready to park in
// PendingInviteCode) if valid. Returns "" for an absent code — h.inviteCodes
// being nil is treated the same as no code, so tests that construct
// AuthHandler without invite-code support keep working. A present but
// invalid/revoked code is a hard 400, not a silent fall-through to the
// no-code path — see docs/invite-code-spec.md.
func (h *AuthHandler) validatePendingInviteCode(code string) (string, error) {
	if code == "" || h.inviteCodes == nil {
		return "", nil
	}
	if _, err := h.inviteCodes.FindByCode(code); err != nil {
		return "", huma.Error400BadRequest("invite code is invalid or has already been revoked")
	}
	return code, nil
}

// trimmedPhone flattens an optional phone field to a trimmed string —
// absent, blank and whitespace-only all collapse to "".
func trimmedPhone(phone *string) string {
	if phone == nil {
		return ""
	}
	return strings.TrimSpace(*phone)
}

// verifyRegisterEmailOTP finishes a registration. Both entry points — the
// 6-digit code typed into the wizard, and the magic link clicked from the
// email on any device — land here and produce the same outcome: the parked
// account details become a real account, and the caller is either signed in
// or told they're awaiting approval. There is no follow-up call.
func (h *AuthHandler) verifyRegisterEmailOTP(ctx context.Context, input *verifyRegisterEmailOTPInput) (*verifyRegisterEmailOTPOutput, error) {
	rawEmail, code, err := h.resolveEmailAndCode(input.Body.Token, otpLinkPurposeRegisterEmail, input.Body.Email, input.Body.Code)
	if err != nil {
		return nil, err
	}
	email := normalizeEmail(rawEmail)
	pending, err := h.consumeRegistrationOTP(registrationChannelEmail, email, code)
	if err != nil {
		return nil, err
	}
	// Belt-and-braces: every row this channel writes carries pending details
	// (see sendRegisterEmailOTP), so an empty hash means a row from before
	// this flow existed, or one written by hand. Nothing sane to finalize.
	if pending.PendingPasswordHash == "" {
		return nil, huma.Error400BadRequest("registration details are no longer available — please start again")
	}

	result, err := h.finalizeRegistration(ctx, *pending)
	if err != nil {
		return nil, err
	}
	return &verifyRegisterEmailOTPOutput{Body: *result}, nil
}

// finalizeRegistration creates the account behind a verified email and
// returns either a live session or a pending-approval result. Split out of
// the old /auth/register handler, minus the phone-verification branch: phone
// is now a plain optional field, so PhoneVerified is always false here.
func (h *AuthHandler) finalizeRegistration(ctx context.Context, pending models.PendingRegistrationData) (*registrationResult, error) {
	// Re-validated, not just checked in sendRegisterEmailOTP: the inviter may
	// have regenerated or been suspended during the 15-minute OTP window —
	// see docs/invite-code-spec.md's "Link lifecycle" step 6. A code parked
	// at send time that's gone by now is a hard 400, same as never having
	// had one plus allow_registration=false.
	var invite *models.InviteCode
	if pending.PendingInviteCode != "" && h.inviteCodes != nil {
		var err error
		invite, err = h.inviteCodes.FindByCode(pending.PendingInviteCode)
		if err != nil {
			return nil, huma.Error400BadRequest("invite link was revoked — please ask for a new one")
		}
	}

	// Both allow_registration and require_registration_approval are
	// bypassed when a valid invite is present at account-creation time — the
	// invite is the vouch. Re-checked here, not just in sendRegisterEmailOTP:
	// an admin can disable registration during the 15 minutes a code is
	// live, and this is the call that actually creates the account.
	if invite == nil {
		if val, _ := h.admin.GetSetting("allow_registration"); val == "false" {
			return nil, huma.Error403Forbidden("registration is currently disabled")
		}
	}

	user := models.User{
		Name:                      pending.PendingName,
		Email:                     pending.PendingEmail,
		Phone:                     pending.PendingPhone,
		Password:                  pending.PendingPasswordHash,
		Verified:                  true,
		EmailNotificationsEnabled: true,
		MonthlyDigestEnabled:      true,
	}
	if invite == nil {
		if val, _ := h.admin.GetSetting("require_registration_approval"); val == "true" {
			user.PendingApproval = true
		}
	}
	if err := h.users.Create(&user); err != nil {
		return nil, huma.Error400BadRequest("email already registered")
	}

	if invite != nil {
		user.InvitedByID = &invite.InviterID
		// Best-effort — the account is live and correctly unapproved either
		// way; a failure here only costs the "Invited by" attribution.
		_ = h.users.Save(&user) //nolint:errcheck
	}

	// An account awaiting admin approval gets no session — the frontend shows
	// a pending-approval message instead of logging the user straight in.
	if user.PendingApproval {
		h.registration.OnPendingApproval(ctx, &user)
		return &registrationResult{Status: "pending_approval", User: user}, nil
	}

	h.registration.OnRegistered(ctx, &user)

	token, err := h.issueToken(user.ID, user.Role)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not issue token")
	}
	return &registrationResult{Status: "complete", Token: token, User: user}, nil
}

// sendRegisterPhoneOTP "sends" a 6-digit code to a phone number. No SMS
// provider is wired up yet (see services.MockSMSService), so the code is
// always returned directly in the response rather than actually texted.
//
// Tech debt — this and verifyRegisterPhoneOTP are dead from the app's point
// of view: registration is a single email step now, and nothing calls
// either endpoint. They're kept registered, working, and covered by tests
// on the bet that a real SMS provider makes phone verification worth having
// again; at that point verifyRegisterPhoneOTP needs a consumer for the
// token it mints (nothing validates it today — see
// registrationVerificationTokenTTL) and User.PhoneVerified needs to start
// being written again. If that bet doesn't pay off, the clean removal is
// these two handlers, their routes, phoneOTPLimiter, registrationChannelPhone,
// registrationVerificationClaims + issueRegistrationVerificationToken,
// services.SMSService/MockSMSService, and — as a separate migration
// decision — User.PhoneVerified itself.
func (h *AuthHandler) sendRegisterPhoneOTP(ctx context.Context, input *sendRegisterPhoneOTPInput) (*sendRegisterPhoneOTPOutput, error) {
	phone := strings.TrimSpace(input.Body.Phone)
	if phone == "" {
		return nil, huma.Error400BadRequest("phone number is required")
	}
	if !h.phoneOTPLimiter.Allow(phone) {
		return nil, huma.Error429TooManyRequests("too many verification requests — try again later")
	}

	code, err := generateOTPCode()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not generate OTP")
	}
	if err := h.registrationVerifications.Upsert(
		registrationChannelPhone, phone, code, time.Now().Add(registrationOTPExpiry),
		models.PendingRegistrationData{},
	); err != nil {
		return nil, huma.Error500InternalServerError("could not save verification code")
	}
	_ = h.sms.SendOTP(ctx, phone, code) //nolint:errcheck // best-effort; the code is also returned below

	out := &sendRegisterPhoneOTPOutput{}
	out.Body.MockCode = code
	return out, nil
}

// verifyRegisterPhoneOTP checks a phone code and mints a verification token
// for it. See sendRegisterPhoneOTP's tech-debt note: nothing calls this, and
// nothing consumes the token it returns.
func (h *AuthHandler) verifyRegisterPhoneOTP(_ context.Context, input *verifyRegisterPhoneOTPInput) (*verifyRegisterPhoneOTPOutput, error) {
	phone := strings.TrimSpace(input.Body.Phone)
	if _, err := h.consumeRegistrationOTP(registrationChannelPhone, phone, input.Body.Code); err != nil {
		return nil, err
	}
	token, err := h.issueRegistrationVerificationToken(registrationPurposePhone, phone)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not issue verification token")
	}
	out := &verifyRegisterPhoneOTPOutput{}
	out.Body.VerificationToken = token
	return out, nil
}

// consumeRegistrationOTP validates a submitted code against the stored
// registration_verifications row for (channel, identifier) — constant-time
// compare, invalidate-on-wrong-attempt, same anti-enumeration shape as
// verifyOTP — and deletes the row either way, so a code is single-use
// whether it was right or wrong. On success it returns the pending
// registration details that were parked alongside the code (zero-valued for
// the phone channel, which parks none).
func (h *AuthHandler) consumeRegistrationOTP(channel, identifier, code string) (*models.PendingRegistrationData, error) {
	v, err := h.registrationVerifications.Find(channel, identifier)
	if err != nil {
		return nil, huma.Error400BadRequest("no verification code was sent — please request one first")
	}
	if time.Now().After(v.ExpiresAt) {
		_ = h.registrationVerifications.Delete(channel, identifier) //nolint:errcheck
		return nil, huma.Error400BadRequest("verification code has expired — please request a new one")
	}
	if subtle.ConstantTimeCompare([]byte(v.Code), []byte(code)) != 1 {
		_ = h.registrationVerifications.Delete(channel, identifier) //nolint:errcheck
		return nil, huma.Error400BadRequest("invalid verification code — please request a new one")
	}
	_ = h.registrationVerifications.Delete(channel, identifier) //nolint:errcheck
	return &v.PendingRegistrationData, nil
}

// issueRegistrationVerificationToken mints a short-lived, stateless JWT
// proving identifier was verified for purpose.
//
// Tech debt: only verify-phone-otp still mints one, and nothing validates it
// — see registrationVerificationTokenTTL.
func (h *AuthHandler) issueRegistrationVerificationToken(purpose, identifier string) (string, error) {
	claims := registrationVerificationClaims{
		Purpose:    purpose,
		Identifier: identifier,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(registrationVerificationTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}

func (h *AuthHandler) setupStatus(_ context.Context, _ *struct{}) (*setupStatusOutput, error) {
	hasAdmin, err := h.users.HasAdmin()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not check setup status")
	}
	out := &setupStatusOutput{}
	out.Body.NeedsSetup = !hasAdmin
	return out, nil
}

func (h *AuthHandler) registrationRequirements(_ context.Context, _ *struct{}) (*registrationRequirementsOutput, error) {
	val, _ := h.admin.GetSetting("verification_requires_phone")
	out := &registrationRequirementsOutput{}
	out.Body.RequirePhone = val == "true"
	return out, nil
}

func (h *AuthHandler) setup(_ context.Context, input *setupInput) (*authOutput, error) {
	hasAdmin, err := h.users.HasAdmin()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not check setup status")
	}
	if hasAdmin {
		return nil, huma.Error403Forbidden("setup already complete")
	}
	if msg := validatePasswordComplexity(input.Body.Password, input.Body.Name, emailLocalPart(input.Body.Email)); msg != "" {
		return nil, huma.Error400BadRequest(msg)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Body.Password), 12)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not hash password")
	}

	user := models.User{
		Name:                      input.Body.Name,
		Email:                     input.Body.Email,
		Password:                  string(hash),
		Role:                      "admin",
		Verified:                  true,
		EmailNotificationsEnabled: true,
		MonthlyDigestEnabled:      true,
	}
	if err := h.users.CreateAdminIfNoneExists(&user); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, huma.Error403Forbidden("setup already complete")
		}
		return nil, huma.Error400BadRequest("email already registered")
	}

	token, err := h.issueToken(user.ID, user.Role)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not issue token")
	}

	return &authOutput{Body: authResponse{Token: token, User: user}}, nil
}
