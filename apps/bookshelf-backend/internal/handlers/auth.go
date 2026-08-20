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
	"net/url"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/ratelimit"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/services"
)

// loginRateLimit caps failed-and-successful /auth/login attempts per email
// address, so a known account's bcrypt password can't be brute-forced over
// the network. Keyed by email rather than client IP: this backend sits
// behind the frontend's server-side proxy (apps/bookshelf's
// src/app/api/[...path]/route.ts), so every request arrives from the same
// container IP regardless of which browser originated it — an IP-keyed
// limiter would either do nothing (limit never reached) or lock out every
// user at once (one shared bucket), whichever number was picked.
const (
	loginRateLimitAttempts = 5
	loginRateLimitWindow   = 15 * time.Minute
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

// registrationVerificationTokenTTL is how long a verify-email-otp/
// verify-phone-otp success token remains valid for the subsequent register
// call — long enough to fill out the rest of the signup form.
const registrationVerificationTokenTTL = 30 * time.Minute

// Verification token purposes — embedded as a claim so a token minted for
// one channel (or a session JWT) can't be replayed as the other.
const (
	registrationPurposeEmail = "email_verify"
	registrationPurposePhone = "phone_verify"
)

const (
	registrationChannelEmail = "email"
	registrationChannelPhone = "phone"
)

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

// emailLocalPart returns the portion of email before "@", for use as a
// disallowed substring in validatePasswordComplexity — checking the local
// part rather than the full address so "j.tan@..." still catches a password
// containing "j.tan".
func emailLocalPart(email string) string {
	if i := strings.Index(email, "@"); i > 0 {
		return email[:i]
	}
	return email
}

// normalizeEmail lowercases and trims an email address so send/verify/
// register all key OTP state and token claims off the same identifier
// regardless of the casing a user happened to type.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// generateOTPCode returns a cryptographically random 6-digit numeric code.
func generateOTPCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// AuthHandler holds dependencies for authentication routes.
type AuthHandler struct {
	users                     repository.UserRepository
	admin                     repository.AdminRepository
	copies                    repository.CopyRepository
	registrationVerifications repository.RegistrationVerificationRepository
	jwtSecret                 string
	encryptionSecret          string
	email                     *services.EmailService
	sms                       services.SMSService
	registration              *services.RegistrationWorkflow
	env                       string
	loginLimiter              *ratelimit.Limiter
	emailOTPLimiter           *ratelimit.Limiter
	phoneOTPLimiter           *ratelimit.Limiter
	registerLimiter           *middleware.RateLimiter
	otpLimiter                *middleware.RateLimiter
	forgotPasswordLimiter     *ratelimit.Limiter
	resetPasswordLimiter      *ratelimit.Limiter
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(
	users repository.UserRepository,
	admin repository.AdminRepository,
	copies repository.CopyRepository,
	registrationVerifications repository.RegistrationVerificationRepository,
	jwtSecret, encryptionSecret string,
	email *services.EmailService,
	sms services.SMSService,
	registration *services.RegistrationWorkflow,
	env string,
) *AuthHandler {
	return &AuthHandler{
		users:                     users,
		admin:                     admin,
		copies:                    copies,
		registrationVerifications: registrationVerifications,
		jwtSecret:                 jwtSecret,
		encryptionSecret:          encryptionSecret,
		email:                     email,
		sms:                       sms,
		registration:              registration,
		env:                       env,
		loginLimiter:              ratelimit.New(loginRateLimitAttempts, loginRateLimitWindow),
		emailOTPLimiter:           ratelimit.New(registrationOTPRateLimitAttempts, registrationOTPRateLimitWindow),
		phoneOTPLimiter:           ratelimit.New(registrationOTPRateLimitAttempts, registrationOTPRateLimitWindow),
		// 5 immediately, refilling one every 10min (~11/hr steady-state) —
		// enough headroom for a real user retrying a rejected password, tight
		// enough to block registration spam. Keyed by IP; see ClientIP's doc
		// comment for the caveat about the current deployment topology.
		registerLimiter: middleware.NewRateLimiter(rate.Every(10*time.Minute), 5),
		// 3 immediately, refilling one every 5min — OTP codes last 15min so
		// legitimate resends are rare; keyed by user ID since this endpoint is
		// already authenticated.
		otpLimiter:            middleware.NewRateLimiter(rate.Every(5*time.Minute), 3),
		forgotPasswordLimiter: ratelimit.New(passwordResetOTPRateLimitAttempts, passwordResetOTPRateLimitWindow),
		resetPasswordLimiter:  ratelimit.New(passwordResetAttemptRateLimitAttempts, passwordResetAttemptRateLimitWindow),
	}
}

// --- Input / Output types ---

type registerInput struct {
	Body struct {
		Name                   string  `json:"name" required:"true" minLength:"1" doc:"Display name"`
		Email                  string  `json:"email" required:"true" format:"email" doc:"Email address"`
		Password               string  `json:"password" required:"true" minLength:"12" doc:"Password (min 12 chars)"`
		Phone                  *string `json:"phone,omitempty" doc:"Contact phone number (optional)"`
		EmailVerificationToken string  `json:"email_verification_token" required:"true" doc:"Token returned by verify-email-otp, proving control of the email address"`
		PhoneVerificationToken *string `json:"phone_verification_token,omitempty" doc:"Token returned by verify-phone-otp; required when phone is set"`
	}
}

type sendRegisterEmailOTPInput struct {
	Body struct {
		Email string `json:"email" required:"true" format:"email" doc:"Email address to verify"`
	}
}

type sendRegisterEmailOTPOutput struct {
	Body struct {
		DebugCode string `json:"debug_code,omitempty" doc:"Only present when ENV=dev: the code, so local development doesn't require SMTP"`
	}
}

type verifyRegisterEmailOTPInput struct {
	Body struct {
		Email string `json:"email,omitempty" format:"email" doc:"Email address being verified. Required unless token is set."`
		Code  string `json:"code,omitempty" doc:"6-digit code. Required unless token is set."`
		Token string `json:"token,omitempty" doc:"Magic-link token from the verification email, as an alternative to submitting email+code."`
	}
}

type verifyRegisterEmailOTPOutput struct {
	Body struct {
		VerificationToken string `json:"verification_token" doc:"Pass this as email_verification_token to /auth/register"`
		Email             string `json:"email" doc:"The email address that was verified — echoed back so a magic-link (token-only) verification, which never submits an email itself, can still prefill the rest of the signup form"`
	}
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

// registrationVerificationClaims is the JWT payload minted by verify-email-otp
// and verify-phone-otp, and re-validated by register(). Stateless by design —
// unlike the OTP code itself (DB-backed, see registrationOTPExpiry), this
// token needs no storage; validating it later is just a signature/claims check.
type registrationVerificationClaims struct {
	Purpose    string `json:"purpose"`
	Identifier string `json:"identifier"`
	jwt.RegisteredClaims
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

type meBody struct {
	models.User
	GoogleBooksKeyConfigured bool `json:"google_books_key_configured"`
}

type meOutput struct{ Body meBody }

type updateMeOutput struct {
	Body struct {
		models.User
		GoogleBooksKeyConfigured bool   `json:"google_books_key_configured"`
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
	TelegramUsername          *string `json:"telegram_username,omitempty" doc:"Telegram username, for other members to reach you. Set to empty string to remove."`
	WhatsAppUsername          *string `json:"whatsapp_username,omitempty" doc:"WhatsApp username, for other members to reach you. Set to empty string to remove."`
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
		Password string `json:"password" required:"true" minLength:"12" doc:"Admin password (min 12 chars)"`
	}
}

type sendOTPInput struct{}

type sendOTPOutput struct {
	Body struct {
		DebugCode string `json:"debug_code,omitempty" doc:"Only present when ENV=dev: the code, so local development doesn't require SMTP"`
	}
}

type verifyOTPInput struct {
	Body struct {
		Code  string `json:"code,omitempty" doc:"6-digit OTP code. Required unless token is set."`
		Token string `json:"token,omitempty" doc:"Magic-link token from the verification email, as an alternative to submitting code."`
	}
}

type confirmEmailChangeInput struct {
	Body struct {
		Code  string `json:"code,omitempty" doc:"6-digit code sent to the new email address. Required unless token is set."`
		Token string `json:"token,omitempty" doc:"Magic-link token from the confirmation email, as an alternative to submitting code."`
	}
}

type changePasswordInput struct {
	Body struct {
		CurrentPassword string `json:"current_password" required:"true" minLength:"1" doc:"Current password"`
		NewPassword     string `json:"new_password" required:"true" minLength:"12" doc:"New password (min 12 chars, mixed case + digit)"`
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
		Middlewares:   huma.Middlewares{middleware.RateLimit(api, h.registerLimiter, middleware.ClientIP)},
	}, h.register)

	huma.Register(api, huma.Operation{
		OperationID: "send-register-email-otp",
		Method:      "POST",
		Path:        "/auth/register/send-email-otp",
		Tags:        []string{"auth"},
		Summary:     "Send a 6-digit OTP to an email address, to be verified before registration",
	}, h.sendRegisterEmailOTP)

	huma.Register(api, huma.Operation{
		OperationID: "verify-register-email-otp",
		Method:      "POST",
		Path:        "/auth/register/verify-email-otp",
		Tags:        []string{"auth"},
		Summary:     "Verify an email OTP and receive a token to pass to /auth/register",
	}, h.verifyRegisterEmailOTP)

	huma.Register(api, huma.Operation{
		OperationID: "send-register-phone-otp",
		Method:      "POST",
		Path:        "/auth/register/send-phone-otp",
		Tags:        []string{"auth"},
		Summary:     "Send a 6-digit OTP to a phone number, to be verified before registration. No SMS provider is configured yet — this is mocked and returns the code directly.",
	}, h.sendRegisterPhoneOTP)

	huma.Register(api, huma.Operation{
		OperationID: "verify-register-phone-otp",
		Method:      "POST",
		Path:        "/auth/register/verify-phone-otp",
		Tags:        []string{"auth"},
		Summary:     "Verify a phone OTP and receive a token to pass to /auth/register",
	}, h.verifyRegisterPhoneOTP)

	huma.Register(api, huma.Operation{
		OperationID: "login",
		Method:      "POST",
		Path:        "/auth/login",
		Tags:        []string{"auth"},
		Summary:     "Log in and receive a JWT",
	}, h.login)

	huma.Register(api, huma.Operation{
		OperationID: "forgot-password",
		Method:      "POST",
		Path:        "/auth/forgot-password",
		Tags:        []string{"auth"},
		Summary:     "Request a password reset code by email. Always responds successfully, whether or not the email is registered.",
	}, h.forgotPassword)

	huma.Register(api, huma.Operation{
		OperationID: "reset-password",
		Method:      "POST",
		Path:        "/auth/reset-password",
		Tags:        []string{"auth"},
		Summary:     "Reset a forgotten password using the code sent by /auth/forgot-password",
	}, h.resetPassword)

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
		Middlewares: huma.Middlewares{middleware.RateLimit(api, h.otpLimiter, middleware.UserOrIP)},
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

func (h *AuthHandler) register(ctx context.Context, input *registerInput) (*authOutput, error) {
	if val, _ := h.admin.GetSetting("allow_registration"); val == "false" {
		return nil, huma.Error403Forbidden("registration is currently disabled")
	}
	if input.Body.Name == "" || input.Body.Email == "" || input.Body.Password == "" {
		return nil, huma.Error400BadRequest("name, email and password are required")
	}
	if msg := validatePasswordComplexity(input.Body.Password, input.Body.Name, emailLocalPart(input.Body.Email)); msg != "" {
		return nil, huma.Error400BadRequest(msg)
	}

	if err := h.verifyRegistrationVerificationToken(
		input.Body.EmailVerificationToken, registrationPurposeEmail, normalizeEmail(input.Body.Email),
	); err != nil {
		return nil, huma.Error400BadRequest("email must be verified before registering: " + err.Error())
	}

	phone, phoneVerified, err := h.resolveVerifiedPhone(input)
	if err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Body.Password), 12)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not hash password")
	}

	user := models.User{
		Name: input.Body.Name,
		// Preserve the casing the user typed rather than the lowercased form
		// used to match the verification token above.
		Email:                     input.Body.Email,
		Phone:                     phone,
		Password:                  string(hash),
		Verified:                  true,
		PhoneVerified:             phoneVerified,
		EmailNotificationsEnabled: true,
	}
	if val, _ := h.admin.GetSetting("require_registration_approval"); val == "true" {
		user.PendingApproval = true
	}
	if err := h.users.Create(&user); err != nil {
		return nil, huma.Error400BadRequest("email already registered")
	}

	// An account awaiting admin approval gets no session — the frontend shows
	// a pending-approval message instead of logging the user straight in.
	if user.PendingApproval {
		h.registration.OnPendingApproval(ctx, &user)
		return &authOutput{Body: authResponse{User: user}}, nil
	}

	token, err := h.issueToken(user.ID, user.Role)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not issue token")
	}

	return &authOutput{Body: authResponse{Token: token, User: user}}, nil
}

// resolveVerifiedPhone validates registerInput's optional phone and its
// verification token. Phone is optional — an absent or blank one returns
// ("", false, nil) rather than an error. A present phone must carry a token
// that verifyRegistrationVerificationToken accepts for it.
func (h *AuthHandler) resolveVerifiedPhone(input *registerInput) (phone string, verified bool, err error) {
	if input.Body.Phone == nil || strings.TrimSpace(*input.Body.Phone) == "" {
		return "", false, nil
	}
	phone = strings.TrimSpace(*input.Body.Phone)
	if input.Body.PhoneVerificationToken == nil {
		return "", false, huma.Error400BadRequest("phone must be verified before registering")
	}
	if err := h.verifyRegistrationVerificationToken(
		*input.Body.PhoneVerificationToken, registrationPurposePhone, phone,
	); err != nil {
		return "", false, huma.Error400BadRequest("phone must be verified before registering: " + err.Error())
	}
	return phone, true, nil
}

// sendRegisterEmailOTP sends a 6-digit code to an email address that hasn't
// registered yet, so it can be proven before /auth/register accepts it. See
// EmailService.SendEmail for why this no-ops (but still logs the code) when
// SMTP isn't configured.
func (h *AuthHandler) sendRegisterEmailOTP(ctx context.Context, input *sendRegisterEmailOTPInput) (*sendRegisterEmailOTPOutput, error) {
	email := normalizeEmail(input.Body.Email)
	if !h.emailOTPLimiter.Allow(email) {
		return nil, huma.Error429TooManyRequests("too many verification requests — try again later")
	}
	if _, err := h.users.FindByEmail(input.Body.Email); err == nil {
		return nil, huma.Error400BadRequest("email already registered")
	}

	code, err := generateOTPCode()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not generate OTP")
	}
	if err := h.registrationVerifications.Upsert(registrationChannelEmail, email, code, time.Now().Add(registrationOTPExpiry)); err != nil {
		return nil, huma.Error500InternalServerError("could not save verification code")
	}

	body := fmt.Sprintf(
		"<p>Your Bookshelf email verification code is: <strong>%s</strong></p><p>This code expires in 15 minutes.</p>",
		code,
	)
	if linkToken, tokenErr := h.issueOTPLinkToken(otpLinkPurposeRegisterEmail, email, code); tokenErr == nil {
		body += h.email.Button(fmt.Sprintf("/register?verifyToken=%s", url.QueryEscape(linkToken)), "Verify email")
	}
	h.email.SendEmailAsync(ctx, input.Body.Email, "Verify your email for Bookshelf", body)

	out := &sendRegisterEmailOTPOutput{}
	if h.env == "dev" {
		out.Body.DebugCode = code
	}
	return out, nil
}

func (h *AuthHandler) verifyRegisterEmailOTP(_ context.Context, input *verifyRegisterEmailOTPInput) (*verifyRegisterEmailOTPOutput, error) {
	rawEmail, code, err := h.resolveEmailAndCode(input.Body.Token, otpLinkPurposeRegisterEmail, input.Body.Email, input.Body.Code)
	if err != nil {
		return nil, err
	}
	email := normalizeEmail(rawEmail)
	token, err := h.checkRegistrationOTP(registrationChannelEmail, email, code)
	if err != nil {
		return nil, err
	}
	out := &verifyRegisterEmailOTPOutput{}
	out.Body.VerificationToken = token
	out.Body.Email = email
	return out, nil
}

// sendRegisterPhoneOTP "sends" a 6-digit code to a phone number. No SMS
// provider is wired up yet (see services.MockSMSService), so the code is
// always returned directly in the response rather than actually texted —
// the frontend shows it inline with a note that phone verification is
// mocked, regardless of ENV (unlike email, which only exposes debug_code in
// dev, because email has a real delivery path in prod).
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
	if err := h.registrationVerifications.Upsert(registrationChannelPhone, phone, code, time.Now().Add(registrationOTPExpiry)); err != nil {
		return nil, huma.Error500InternalServerError("could not save verification code")
	}
	_ = h.sms.SendOTP(ctx, phone, code) //nolint:errcheck // best-effort; the code is also returned below

	out := &sendRegisterPhoneOTPOutput{}
	out.Body.MockCode = code
	return out, nil
}

func (h *AuthHandler) verifyRegisterPhoneOTP(_ context.Context, input *verifyRegisterPhoneOTPInput) (*verifyRegisterPhoneOTPOutput, error) {
	phone := strings.TrimSpace(input.Body.Phone)
	token, err := h.checkRegistrationOTP(registrationChannelPhone, phone, input.Body.Code)
	if err != nil {
		return nil, err
	}
	out := &verifyRegisterPhoneOTPOutput{}
	out.Body.VerificationToken = token
	return out, nil
}

// checkRegistrationOTP validates a submitted code against the stored
// registration_verifications row for (channel, identifier) — constant-time
// compare, invalidate-on-wrong-attempt, same anti-enumeration shape as
// verifyOTP — and on success mints the hand-off token for register().
func (h *AuthHandler) checkRegistrationOTP(channel, identifier, code string) (string, error) {
	v, err := h.registrationVerifications.Find(channel, identifier)
	if err != nil {
		return "", huma.Error400BadRequest("no verification code was sent — please request one first")
	}
	if time.Now().After(v.ExpiresAt) {
		_ = h.registrationVerifications.Delete(channel, identifier) //nolint:errcheck
		return "", huma.Error400BadRequest("verification code has expired — please request a new one")
	}
	if subtle.ConstantTimeCompare([]byte(v.Code), []byte(code)) != 1 {
		_ = h.registrationVerifications.Delete(channel, identifier) //nolint:errcheck
		return "", huma.Error400BadRequest("invalid verification code — please request a new one")
	}
	_ = h.registrationVerifications.Delete(channel, identifier) //nolint:errcheck

	purpose := registrationPurposeEmail
	if channel == registrationChannelPhone {
		purpose = registrationPurposePhone
	}
	token, err := h.issueRegistrationVerificationToken(purpose, identifier)
	if err != nil {
		return "", huma.Error500InternalServerError("could not issue verification token")
	}
	return token, nil
}

// issueRegistrationVerificationToken mints a short-lived, stateless JWT
// proving identifier was verified over channel's purpose. register() checks
// it via verifyRegistrationVerificationToken instead of a DB lookup.
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

// verifyRegistrationVerificationToken checks tokenStr's signature, expiry,
// purpose, and that it was minted for exactly expectedIdentifier.
func (h *AuthHandler) verifyRegistrationVerificationToken(tokenStr, expectedPurpose, expectedIdentifier string) error {
	var claims registrationVerificationClaims
	token, err := jwt.ParseWithClaims(tokenStr, &claims, func(*jwt.Token) (any, error) {
		return []byte(h.jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return errors.New("invalid or expired verification token")
	}
	if claims.Purpose != expectedPurpose || claims.Identifier != expectedIdentifier {
		return errors.New("verification token does not match")
	}
	return nil
}

func (h *AuthHandler) login(_ context.Context, input *loginInput) (*authOutput, error) {
	limiterKey := strings.ToLower(strings.TrimSpace(input.Body.Email))
	if !h.loginLimiter.Allow(limiterKey) {
		return nil, huma.Error429TooManyRequests("too many login attempts — try again later")
	}

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

	if user.PendingApproval {
		return nil, huma.Error403Forbidden("this account is pending admin approval")
	}

	token, err := h.issueToken(user.ID, user.Role)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not issue token")
	}

	return &authOutput{Body: authResponse{Token: token, User: *user}}, nil
}

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
	if linkToken, tokenErr := h.issueOTPLinkToken(otpLinkPurposeResetPassword, email, code); tokenErr == nil {
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

	return &meOutput{Body: meBody{User: *user, GoogleBooksKeyConfigured: user.GoogleBooksAPIKey != ""}}, nil
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

	if input.Body.Name != nil {
		user.Name = *input.Body.Name
	}
	if input.Body.Phone != nil {
		applyPhoneUpdate(user, *input.Body.Phone)
	}
	var pendingEmailDebugCode string
	if input.Body.Email != nil {
		code, err := h.handleEmailUpdateRequest(ctx, user, *input.Body.Email)
		if err != nil {
			return nil, err
		}
		pendingEmailDebugCode = code
	}
	if input.Body.GoogleBooksAPIKey != nil {
		if err := h.applyGoogleBooksKeyUpdate(user, *input.Body.GoogleBooksAPIKey); err != nil {
			return nil, err
		}
	}
	applyContactPrefsUpdate(user, input.Body)

	if err := h.users.Save(user); err != nil {
		return nil, huma.Error500InternalServerError("could not update user")
	}

	out := &updateMeOutput{}
	out.Body.User = *user
	out.Body.GoogleBooksKeyConfigured = user.GoogleBooksAPIKey != ""
	out.Body.PendingEmailDebugCode = pendingEmailDebugCode
	return out, nil
}

// applyContactPrefsUpdate applies the notification-preference and
// messaging-username fields of an updateMe request. Split out from updateMe
// to keep its cognitive complexity under the repo's gocognit threshold.
func applyContactPrefsUpdate(user *models.User, body updateMeBody) {
	if body.EmailNotificationsEnabled != nil {
		user.EmailNotificationsEnabled = *body.EmailNotificationsEnabled
	}
	if body.TelegramUsername != nil {
		user.TelegramUsername = *body.TelegramUsername
	}
	if body.WhatsAppUsername != nil {
		user.WhatsAppUsername = *body.WhatsAppUsername
	}
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

func (h *AuthHandler) sendOTP(ctx context.Context, _ *sendOTPInput) (*sendOTPOutput, error) {
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

	code, err := generateOTPCode()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not generate OTP")
	}
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
	if linkToken, tokenErr := h.issueOTPLinkToken(otpLinkPurposeOTPVerify, fmt.Sprint(user.ID), code); tokenErr == nil {
		body += h.email.Button(fmt.Sprintf("/profile?verifyOtpToken=%s", url.QueryEscape(linkToken)), "Verify")
	}
	h.email.SendEmailAsync(ctx, user.Email, "Your Bookshelf verification code", body)

	out := &sendOTPOutput{}
	if h.env == "dev" {
		out.Body.DebugCode = code
	}
	return out, nil
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

	if user.PendingApproval {
		return nil, huma.Error403Forbidden("this account is pending admin approval")
	}

	code, err := h.resolveCode(input.Body.Token, otpLinkPurposeOTPVerify, input.Body.Code)
	if err != nil {
		return nil, err
	}

	if user.OTPCode == "" || user.OTPExpiry == nil {
		return nil, huma.Error400BadRequest("no OTP has been sent")
	}
	if time.Now().After(*user.OTPExpiry) {
		return nil, huma.Error400BadRequest("OTP has expired")
	}
	// Use constant-time comparison to prevent timing attacks, and invalidate
	// the OTP on any wrong attempt to prevent brute-force enumeration.
	if subtle.ConstantTimeCompare([]byte(user.OTPCode), []byte(code)) != 1 {
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

// phoneOnFileFactor returns the "verified phone" factor if that requirement is
// enabled, or nil if the setting is off. Checks PhoneVerified rather than
// Phone != "" — a phone that was never verified (e.g. set via ProfileForm's
// unverified phone-edit field) doesn't satisfy this.
func (h *AuthHandler) phoneOnFileFactor(user *models.User) *verificationFactor {
	if val, _ := h.admin.GetSetting("verification_requires_phone"); val != "true" {
		return nil
	}
	return &verificationFactor{
		Key:       "phone",
		Label:     "Verified phone number",
		Required:  true,
		Satisfied: user.PhoneVerified,
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
