// Package handlers contains the huma HTTP handler implementations.
package handlers

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"html"

	"math/big"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/services"
)

// validatePasswordComplexity checks that p meets minimum complexity requirements.
// Returns a human-readable error string, or "" if valid.
func validatePasswordComplexity(p string) string {
	if len(p) < 8 {
		return "password must be at least 8 characters"
	}
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

// AuthHandler holds dependencies for authentication routes.
type AuthHandler struct {
	users            repository.UserRepository
	admin            repository.AdminRepository
	copies           repository.CopyRepository
	jwtSecret        string
	encryptionSecret string
	email            *services.EmailService
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(users repository.UserRepository, admin repository.AdminRepository, copies repository.CopyRepository, jwtSecret, encryptionSecret string, email *services.EmailService) *AuthHandler {
	return &AuthHandler{users: users, admin: admin, copies: copies, jwtSecret: jwtSecret, encryptionSecret: encryptionSecret, email: email}
}

// --- Input / Output types ---

type registerInput struct {
	Body struct {
		Name     string `json:"name" required:"true" minLength:"1" doc:"Display name"`
		Email    string `json:"email" required:"true" format:"email" doc:"Email address"`
		Password string `json:"password" required:"true" minLength:"8" doc:"Password (min 8 chars)"`
	}
}

type loginInput struct {
	Body struct {
		Email    string `json:"email" required:"true"`
		Password string `json:"password" required:"true"`
	}
}

type authResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

type authOutput struct{ Body authResponse }

type meBody struct {
	models.User
	GoogleBooksKeyConfigured bool `json:"google_books_key_configured"`
}

type meOutput struct{ Body meBody }

type updateMeInput struct {
	Body struct {
		Name              *string `json:"name,omitempty" doc:"New display name"`
		Phone             *string `json:"phone,omitempty" doc:"Contact phone number"`
		Email             *string `json:"email,omitempty" format:"email" doc:"New email address"`
		GoogleBooksAPIKey *string `json:"google_books_api_key,omitempty" doc:"Your Google Books API key. Set to empty string to remove."`
	}
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

type setupStatusOutput struct {
	Body struct {
		NeedsSetup bool `json:"needs_setup"`
	}
}

type setupInput struct {
	Body struct {
		Name     string `json:"name" required:"true" minLength:"1" doc:"Admin display name"`
		Email    string `json:"email" required:"true" format:"email" doc:"Admin email address"`
		Password string `json:"password" required:"true" minLength:"8" doc:"Admin password (min 8 chars)"`
	}
}

type sendOTPInput struct{}

type verifyOTPInput struct {
	Body struct {
		Code string `json:"code" required:"true" doc:"6-digit OTP code"`
	}
}

type confirmEmailChangeInput struct {
	Body struct {
		Code string `json:"code" required:"true" doc:"6-digit code sent to the new email address"`
	}
}

type changePasswordInput struct {
	Body struct {
		CurrentPassword string `json:"current_password" required:"true" minLength:"1" doc:"Current password"`
		NewPassword     string `json:"new_password" required:"true" minLength:"8" doc:"New password (min 8 chars, mixed case + digit)"`
		ConfirmPassword string `json:"confirm_password" required:"true" minLength:"1" doc:"Must match new_password"`
	}
}

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

// --- Route registration ---

// RegisterRoutes registers all auth routes on the given huma API.
func (h *AuthHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "register",
		Method:        "POST",
		Path:          "/auth/register",
		Tags:          []string{"auth"},
		Summary:       "Register a new user",
		DefaultStatus: 201,
	}, h.register)

	huma.Register(api, huma.Operation{
		OperationID: "login",
		Method:      "POST",
		Path:        "/auth/login",
		Tags:        []string{"auth"},
		Summary:     "Log in and receive a JWT",
	}, h.login)

	huma.Register(api, huma.Operation{
		OperationID: "get-me",
		Method:      "GET",
		Path:        "/auth/me",
		Tags:        []string{"auth"},
		Summary:     "Get the authenticated user's profile",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.me)

	huma.Register(api, huma.Operation{
		OperationID: "update-me",
		Method:      "PATCH",
		Path:        "/auth/me",
		Tags:        []string{"auth"},
		Summary:     "Update the authenticated user's profile",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.updateMe)

	huma.Register(api, huma.Operation{
		OperationID: "setup-status",
		Method:      "GET",
		Path:        "/auth/setup-status",
		Tags:        []string{"auth"},
		Summary:     "Check whether initial admin setup is required",
	}, h.setupStatus)

	huma.Register(api, huma.Operation{
		OperationID:   "setup",
		Method:        "POST",
		Path:          "/auth/setup",
		Tags:          []string{"auth"},
		Summary:       "Create the initial admin account (one-time, fails if admin already exists)",
		DefaultStatus: 201,
	}, h.setup)

	huma.Register(api, huma.Operation{
		OperationID: "send-otp",
		Method:      "POST",
		Path:        "/auth/send-otp",
		Tags:        []string{"auth"},
		Summary:     "Send a 6-digit OTP to the authenticated user's email for verification",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.sendOTP)

	huma.Register(api, huma.Operation{
		OperationID: "verify-otp",
		Method:      "POST",
		Path:        "/auth/verify-otp",
		Tags:        []string{"auth"},
		Summary:     "Verify the OTP and mark the user as verified",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.verifyOTP)

	huma.Register(api, huma.Operation{
		OperationID: "confirm-email-change",
		Method:      "POST",
		Path:        "/auth/confirm-email-change",
		Tags:        []string{"auth"},
		Summary:     "Confirm a pending email change using the code sent to the new address",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.confirmEmailChange)

	huma.Register(api, huma.Operation{
		OperationID: "change-password",
		Method:      "POST",
		Path:        "/auth/me/password",
		Tags:        []string{"auth"},
		Summary:     "Change the authenticated user's password",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.changePassword)

	huma.Register(api, huma.Operation{
		OperationID: "test-google-books-key",
		Method:      "POST",
		Path:        "/auth/me/google-books-key/test",
		Tags:        []string{"auth"},
		Summary:     "Test a Google Books API key. Pass a key in the body to test it directly, or omit to test the currently stored key.",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.testGoogleBooksKey)

	huma.Register(api, huma.Operation{
		OperationID: "get-verification-status",
		Method:      "GET",
		Path:        "/auth/me/verification-status",
		Tags:        []string{"auth"},
		Summary:     "Get the authenticated user's verification status against the current admin-configured requirements",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.verificationStatus)
}

// --- Handlers ---

func (h *AuthHandler) register(_ context.Context, input *registerInput) (*authOutput, error) {
	if val, _ := h.admin.GetSetting("allow_registration"); val == "false" {
		return nil, huma.Error403Forbidden("registration is currently disabled")
	}
	if input.Body.Name == "" || input.Body.Email == "" || input.Body.Password == "" {
		return nil, huma.Error400BadRequest("name, email and password are required")
	}
	if msg := validatePasswordComplexity(input.Body.Password); msg != "" {
		return nil, huma.Error400BadRequest(msg)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Body.Password), 12)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not hash password")
	}

	user := models.User{
		Name:     input.Body.Name,
		Email:    input.Body.Email,
		Password: string(hash),
	}
	if err := h.users.Create(&user); err != nil {
		return nil, huma.Error400BadRequest("email already registered")
	}

	token, err := h.issueToken(user.ID, user.Role)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not issue token")
	}

	return &authOutput{Body: authResponse{Token: token, User: user}}, nil
}

func (h *AuthHandler) login(_ context.Context, input *loginInput) (*authOutput, error) {
	user, err := h.users.FindByEmail(input.Body.Email)
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Body.Password)); err != nil {
		return nil, huma.Error401Unauthorized("invalid credentials")
	}

	if user.Suspended {
		return nil, huma.Error403Forbidden("this account has been suspended")
	}

	token, err := h.issueToken(user.ID, user.Role)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not issue token")
	}

	return &authOutput{Body: authResponse{Token: token, User: *user}}, nil
}

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

	return &meOutput{Body: meBody{User: *user, GoogleBooksKeyConfigured: user.GoogleBooksAPIKey != ""}}, nil
}

func (h *AuthHandler) updateMe(ctx context.Context, input *updateMeInput) (*meOutput, error) {
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

	if input.Body.Name != nil {
		user.Name = *input.Body.Name
	}
	if input.Body.Phone != nil {
		user.Phone = *input.Body.Phone
	}
	if input.Body.Email != nil {
		if err := h.handleEmailUpdateRequest(ctx, user, *input.Body.Email); err != nil {
			return nil, err
		}
	}
	if input.Body.GoogleBooksAPIKey != nil {
		if err := h.applyGoogleBooksKeyUpdate(user, *input.Body.GoogleBooksAPIKey); err != nil {
			return nil, err
		}
	}

	if err := h.users.Save(user); err != nil {
		return nil, huma.Error500InternalServerError("could not update user")
	}

	return &meOutput{Body: meBody{User: *user, GoogleBooksKeyConfigured: user.GoogleBooksAPIKey != ""}}, nil
}

// handleEmailUpdateRequest routes an email field on updateMe's input to the
// right behavior: unchanged email with a pending change cancels it; a
// changed email either applies immediately or stages a pending
// confirmation, depending on the require_email_confirmation_on_change flag.
//
// TODO: the pending-confirmation branch (requestEmailChange) only runs when
// that flag is "true". It defaults to "false" in db.Seed() because SMTP
// delivery isn't guaranteed configured in every deployment — flip the
// default once it is.
func (h *AuthHandler) handleEmailUpdateRequest(ctx context.Context, user *models.User, newEmail string) error {
	if newEmail == user.Email {
		if user.PendingEmail != "" {
			// User resubmitted their current (unchanged) email while a change
			// was pending — treat as cancelling the pending change rather than
			// leaving stale pending state.
			user.PendingEmail = ""
			user.PendingEmailOTPCode = ""
			user.PendingEmailOTPExpiry = nil
		}
		return nil
	}

	if val, _ := h.admin.GetSetting("require_email_confirmation_on_change"); val == "true" {
		return h.requestEmailChange(ctx, user, newEmail)
	}
	return h.applyEmailUpdate(user, newEmail)
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
// confirmEmailChange succeeds.
func (h *AuthHandler) requestEmailChange(ctx context.Context, user *models.User, newEmail string) error {
	existing, findErr := h.users.FindByEmail(newEmail)
	if findErr == nil && existing.ID != user.ID {
		return huma.Error400BadRequest("email already in use")
	}

	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return huma.Error500InternalServerError("could not generate OTP")
	}
	code := fmt.Sprintf("%06d", n.Int64())
	expiry := time.Now().Add(15 * time.Minute)

	user.PendingEmail = newEmail
	user.PendingEmailOTPCode = code
	user.PendingEmailOTPExpiry = &expiry

	body := fmt.Sprintf(
		"<p>Hi %s,</p><p>You requested to change your Bookshelf account email to this address. Your confirmation code is: <strong>%s</strong></p><p>This code expires in 15 minutes. If you didn't request this change, you can safely ignore this email.</p>",
		html.EscapeString(user.Name), code,
	)
	h.email.SendEmailAsync(ctx, newEmail, "Confirm your new Bookshelf email address", body)
	return nil
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
	if err := validateGoogleBooksAPIKey(key); err != nil {
		out.Body.OK = false
		out.Body.Message = err.Error()
	} else {
		out.Body.OK = true
	}
	return out, nil
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

func (h *AuthHandler) setup(_ context.Context, input *setupInput) (*authOutput, error) {
	hasAdmin, err := h.users.HasAdmin()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not check setup status")
	}
	if hasAdmin {
		return nil, huma.Error403Forbidden("setup already complete")
	}
	if msg := validatePasswordComplexity(input.Body.Password); msg != "" {
		return nil, huma.Error400BadRequest(msg)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Body.Password), 12)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not hash password")
	}

	user := models.User{
		Name:     input.Body.Name,
		Email:    input.Body.Email,
		Password: string(hash),
		Role:     "admin",
		Verified: true,
	}
	if err := h.users.Create(&user); err != nil {
		return nil, huma.Error400BadRequest("email already registered")
	}

	token, err := h.issueToken(user.ID, user.Role)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not issue token")
	}

	return &authOutput{Body: authResponse{Token: token, User: user}}, nil
}

func (h *AuthHandler) sendOTP(ctx context.Context, _ *sendOTPInput) (*struct{}, error) {
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

	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return nil, huma.Error500InternalServerError("could not generate OTP")
	}
	code := fmt.Sprintf("%06d", n.Int64())
	expiry := time.Now().Add(15 * time.Minute)

	user.OTPCode = code
	user.OTPExpiry = &expiry
	if err := h.users.Save(user); err != nil {
		return nil, huma.Error500InternalServerError("could not save OTP")
	}

	body := fmt.Sprintf(
		"<p>Hi %s,</p><p>Your Bookshelf verification code is: <strong>%s</strong></p><p>This code expires in 15 minutes.</p>",
		html.EscapeString(user.Name), code,
	)
	h.email.SendEmailAsync(ctx, user.Email, "Your Bookshelf verification code", body)

	return nil, nil
}

func (h *AuthHandler) verifyOTP(ctx context.Context, input *verifyOTPInput) (*meOutput, error) {
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

	if user.OTPCode == "" || user.OTPExpiry == nil {
		return nil, huma.Error400BadRequest("no OTP has been sent")
	}
	if time.Now().After(*user.OTPExpiry) {
		return nil, huma.Error400BadRequest("OTP has expired")
	}
	// Use constant-time comparison to prevent timing attacks, and invalidate
	// the OTP on any wrong attempt to prevent brute-force enumeration.
	if subtle.ConstantTimeCompare([]byte(user.OTPCode), []byte(input.Body.Code)) != 1 {
		user.OTPCode = ""
		user.OTPExpiry = nil
		_ = h.users.Save(user) //nolint:errcheck
		return nil, huma.Error400BadRequest("invalid OTP code — please request a new one")
	}

	user.Verified = true
	user.OTPCode = ""
	user.OTPExpiry = nil
	if err := h.users.Save(user); err != nil {
		return nil, huma.Error500InternalServerError("could not update user")
	}

	return &meOutput{Body: meBody{User: *user, GoogleBooksKeyConfigured: user.GoogleBooksAPIKey != ""}}, nil
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

	if user.PendingEmail == "" || user.PendingEmailOTPCode == "" || user.PendingEmailOTPExpiry == nil {
		return nil, huma.Error400BadRequest("no pending email change")
	}
	if time.Now().After(*user.PendingEmailOTPExpiry) {
		return nil, huma.Error400BadRequest("confirmation code has expired")
	}
	if subtle.ConstantTimeCompare([]byte(user.PendingEmailOTPCode), []byte(input.Body.Code)) != 1 {
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

	return &meOutput{Body: meBody{User: *user, GoogleBooksKeyConfigured: user.GoogleBooksAPIKey != ""}}, nil
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

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Body.CurrentPassword)); err != nil {
		return nil, huma.Error400BadRequest("current password is incorrect")
	}

	if input.Body.NewPassword != input.Body.ConfirmPassword {
		return nil, huma.Error400BadRequest("new passwords do not match")
	}

	if msg := validatePasswordComplexity(input.Body.NewPassword); msg != "" {
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

// phoneOnFileFactor returns the "phone on file" factor if that requirement is
// enabled, or nil if the setting is off.
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

// issueToken creates a signed HS256 JWT for the given user with a 24-hour expiry.
func (h *AuthHandler) issueToken(userID uint, role string) (string, error) {
	claims := middleware.JWTClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}
