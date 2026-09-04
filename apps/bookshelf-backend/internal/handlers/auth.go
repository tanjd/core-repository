// Package handlers contains the huma HTTP handler implementations.
package handlers

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"html"

	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/golang-jwt/jwt/v5"
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
	registerSendLimiter       *middleware.RateLimiter
	otpLimiter                *middleware.RateLimiter
	forgotPasswordLimiter     *ratelimit.Limiter
	resetPasswordLimiter      *ratelimit.Limiter
	// inviteCodes is nil-safe — tests that construct AuthHandler without
	// invite-code support keep working; when nil, any invite_code in the
	// body is silently ignored. See docs/invite-code-spec.md.
	inviteCodes repository.InviteCodeRepository
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
	registerRateLimitBurst int,
	registerSendRateLimitBurst int,
	loginRateLimitAttempts int,
	inviteCodes repository.InviteCodeRepository,
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
		// Throttles account creation (verify-email-otp). 20 immediately,
		// refilling one every 30s (~120/hr steady-state): enough to stop a
		// runaway signup script, far above any real community's rate.
		//
		// Sized for a *shared* bucket, not a per-client one. Keyed by IP, but
		// every request reaches this backend from the frontend container (see
		// ClientIP's doc comment), so in practice the whole community shares
		// one bucket. The previous 5-burst/1-per-10min sizing predates this
		// endpoint absorbing registration; at that rate a community signing
		// people up at an onboarding session would lock itself out, and it's
		// no more protective — reaching this endpoint at all requires a code
		// emailed to an inbox you control, itself capped at
		// registrationOTPRateLimitAttempts per address.
		registerLimiter: middleware.NewRateLimiter(rate.Every(30*time.Second), registerRateLimitBurst),
		// Throttles send-email-otp. emailOTPLimiter above already caps
		// requests per *address*, which is the right shape for "stop
		// hammering one inbox" but no defence at all against a caller that
		// varies the address: each such request costs a bcrypt at cost 12
		// (~150-300ms of CPU) and one outbound email to an arbitrary
		// recipient, neither of which is bounded by a per-address bucket. So
		// this endpoint needs a second, address-independent cap on total
		// spend — CPU and outbound mail alike.
		//
		// 30 immediately by default (env REGISTER_SEND_RATE_LIMIT_BURST
		// overrides), refilling one every 20s (~180/hr steady-state). Same
		// shared-bucket caveat as registerLimiter (see above); sized a
		// little looser than it because a signup can legitimately send more
		// than once (resends) but only ever verifies once.
		registerSendLimiter: middleware.NewRateLimiter(rate.Every(20*time.Second), registerSendRateLimitBurst),
		// 3 immediately, refilling one every 5min — OTP codes last 15min so
		// legitimate resends are rare; keyed by user ID since this endpoint is
		// already authenticated.
		otpLimiter:            middleware.NewRateLimiter(rate.Every(5*time.Minute), 3),
		forgotPasswordLimiter: ratelimit.New(passwordResetOTPRateLimitAttempts, passwordResetOTPRateLimitWindow),
		resetPasswordLimiter:  ratelimit.New(passwordResetAttemptRateLimitAttempts, passwordResetAttemptRateLimitWindow),
		inviteCodes:           inviteCodes,
	}
}

// --- Input / Output types ---

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

// --- Route registration ---

// RegisterRoutes registers all auth routes on the given huma API.
func (h *AuthHandler) RegisterRoutes(api huma.API) {
	// registerSendLimiter caps this endpoint's total bcrypt-and-outbound-mail
	// spend regardless of which address each request names; the per-address
	// emailOTPLimiter inside the handler is the other half of the pair.
	huma.Register(api, huma.Operation{
		OperationID: "send-register-email-otp",
		Method:      "POST",
		Path:        "/auth/register/send-email-otp",
		Tags:        []string{"auth"},
		Summary:     "Start registration: hold the submitted account details and email a 6-digit code (and a magic link) to confirm the address",
		Middlewares: huma.Middlewares{middleware.RateLimit(api, h.registerSendLimiter, middleware.ClientIP)},
	}, h.sendRegisterEmailOTP)

	// The IP-keyed registerLimiter moved here from the deleted
	// /auth/register: this is the endpoint that creates accounts now, so
	// it's what registration-spam throttling has to sit on.
	huma.Register(api, huma.Operation{
		OperationID:   "verify-register-email-otp",
		Method:        "POST",
		Path:          "/auth/register/verify-email-otp",
		Tags:          []string{"auth"},
		Summary:       "Finish registration: confirm the emailed code (or magic-link token) and create the account",
		DefaultStatus: 201,
		Middlewares:   huma.Middlewares{middleware.RateLimit(api, h.registerLimiter, middleware.ClientIP)},
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
		Summary:     "Verify a phone OTP. Unused by the app — see the tech-debt note on sendRegisterPhoneOTP.",
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
		OperationID: "registration-requirements",
		Method:      "GET",
		Path:        "/auth/registration-requirements",
		Tags:        []string{"auth"},
		Summary:     "Check which fields the registration form should highlight for this community",
	}, h.registrationRequirements)

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

	return &meOutput{Body: meBody{
		User:                     *user,
		GoogleBooksKeyConfigured: user.GoogleBooksAPIKey != "",
		TelegramLinked:           user.TelegramChatID != nil,
	}}, nil
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
