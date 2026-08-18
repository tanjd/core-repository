package handlers

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
)

func newAuthHandler() (*AuthHandler, *repotest.UserRepository, *repotest.AdminRepository) {
	h, users, admin, _ := newAuthHandlerWithVerifications()
	return h, users, admin
}

func newAuthHandlerWithVerifications() (*AuthHandler, *repotest.UserRepository, *repotest.AdminRepository, *repotest.RegistrationVerificationRepository) {
	users := repotest.NewUserRepository()
	admin := repotest.NewAdminRepository()
	copies := repotest.NewCopyRepository()
	regVerifications := repotest.NewRegistrationVerificationRepository()
	h := NewAuthHandler(users, admin, copies, regVerifications, testJWTSecret, "encryption-secret", noopEmail(), noopSMS(), "dev")
	return h, users, admin, regVerifications
}

// mustEmailVerificationToken mints a valid email-verification token the way
// verify-email-otp would, for tests that exercise register() directly
// without going through the send/verify OTP endpoints.
func mustEmailVerificationToken(t *testing.T, h *AuthHandler, email string) string {
	t.Helper()
	token, err := h.issueRegistrationVerificationToken(registrationPurposeEmail, normalizeEmail(email))
	require.NoError(t, err)
	return token
}

func TestRegister(t *testing.T) {
	t.Run("creates a user and issues a token", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		input := &registerInput{}
		input.Body.Name = "Ada Lovelace"
		input.Body.Email = "ada@example.com"
		input.Body.Password = "Passw0rd1234"
		input.Body.EmailVerificationToken = mustEmailVerificationToken(t, h, "ada@example.com")

		out, err := h.register(context.Background(), input)

		require.NoError(t, err)
		assert.NotEmpty(t, out.Body.Token)
		assert.Equal(t, "Ada Lovelace", out.Body.User.Name)
		assert.Equal(t, "user", out.Body.User.Role)
		assert.True(t, out.Body.User.Verified, "email is verified at registration time now")

		stored, err := users.FindByEmail("ada@example.com")
		require.NoError(t, err)
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored.Password), []byte("Passw0rd1234")))
	})

	t.Run("rejects registration without a valid email verification token", func(t *testing.T) {
		h, _, _ := newAuthHandler()
		input := &registerInput{}
		input.Body.Name = "Ada"
		input.Body.Email = "ada-noverify@example.com"
		input.Body.Password = "Passw0rd1234"
		input.Body.EmailVerificationToken = "not-a-real-token"

		_, err := h.register(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("rejects a token minted for a different email", func(t *testing.T) {
		h, _, _ := newAuthHandler()
		input := &registerInput{}
		input.Body.Name = "Ada"
		input.Body.Email = "ada-mismatch@example.com"
		input.Body.Password = "Passw0rd1234"
		input.Body.EmailVerificationToken = mustEmailVerificationToken(t, h, "someone-else@example.com")

		_, err := h.register(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("creates a phone-verified user when a phone and matching token are supplied", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		input := &registerInput{}
		input.Body.Name = "Ada"
		input.Body.Email = "ada-phone@example.com"
		input.Body.Password = "Passw0rd1234"
		input.Body.EmailVerificationToken = mustEmailVerificationToken(t, h, "ada-phone@example.com")
		phone := "+65 9123 4567"
		input.Body.Phone = &phone
		phoneToken, err := h.issueRegistrationVerificationToken(registrationPurposePhone, phone)
		require.NoError(t, err)
		input.Body.PhoneVerificationToken = &phoneToken

		out, err := h.register(context.Background(), input)

		require.NoError(t, err)
		assert.True(t, out.Body.User.PhoneVerified)
		assert.Equal(t, phone, out.Body.User.Phone)

		stored, err := users.FindByEmail("ada-phone@example.com")
		require.NoError(t, err)
		assert.True(t, stored.PhoneVerified)
	})

	t.Run("rejects a phone without a verification token", func(t *testing.T) {
		h, _, _ := newAuthHandler()
		input := &registerInput{}
		input.Body.Name = "Ada"
		input.Body.Email = "ada-phone-noverify@example.com"
		input.Body.Password = "Passw0rd1234"
		input.Body.EmailVerificationToken = mustEmailVerificationToken(t, h, "ada-phone-noverify@example.com")
		phone := "+65 9123 4567"
		input.Body.Phone = &phone

		_, err := h.register(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("rejects weak passwords", func(t *testing.T) {
		h, _, _ := newAuthHandler()
		input := &registerInput{}
		input.Body.Name = "Ada"
		input.Body.Email = "ada2@example.com"
		input.Body.Password = "weak"
		input.Body.EmailVerificationToken = mustEmailVerificationToken(t, h, "ada2@example.com")

		_, err := h.register(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("rejects duplicate email", func(t *testing.T) {
		h, _, _ := newAuthHandler()
		input := &registerInput{}
		input.Body.Name = "Ada"
		input.Body.Email = "dup@example.com"
		input.Body.Password = "Passw0rd1234"
		input.Body.EmailVerificationToken = mustEmailVerificationToken(t, h, "dup@example.com")
		_, err := h.register(context.Background(), input)
		require.NoError(t, err)

		input.Body.EmailVerificationToken = mustEmailVerificationToken(t, h, "dup@example.com")
		_, err = h.register(context.Background(), input)
		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("rejects registration when disabled by admin setting", func(t *testing.T) {
		h, _, admin := newAuthHandler()
		require.NoError(t, admin.UpsertSetting("allow_registration", "false"))

		input := &registerInput{}
		input.Body.Name = "Ada"
		input.Body.Email = "ada3@example.com"
		input.Body.Password = "Passw0rd1234"
		input.Body.EmailVerificationToken = mustEmailVerificationToken(t, h, "ada3@example.com")

		_, err := h.register(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 403)
	})

	t.Run("creates a pending, token-less account when approval is required", func(t *testing.T) {
		h, users, admin := newAuthHandler()
		require.NoError(t, admin.UpsertSetting("require_registration_approval", "true"))

		input := &registerInput{}
		input.Body.Name = "Ada"
		input.Body.Email = "ada4@example.com"
		input.Body.Password = "Passw0rd1234"
		input.Body.EmailVerificationToken = mustEmailVerificationToken(t, h, "ada4@example.com")

		out, err := h.register(context.Background(), input)

		require.NoError(t, err)
		assert.Empty(t, out.Body.Token)
		assert.True(t, out.Body.User.PendingApproval)

		stored, findErr := users.FindByEmail("ada4@example.com")
		require.NoError(t, findErr)
		assert.True(t, stored.PendingApproval)
	})
}

func TestLogin(t *testing.T) {
	h, users, _ := newAuthHandler()
	hash, err := bcrypt.GenerateFromPassword([]byte("Passw0rd1234"), 12)
	require.NoError(t, err)
	require.NoError(t, users.Create(&models.User{Name: "Ada", Email: "ada@example.com", Password: string(hash)}))

	t.Run("valid credentials issue a token", func(t *testing.T) {
		input := &loginInput{}
		input.Body.Email = "ada@example.com"
		input.Body.Password = "Passw0rd1234"

		out, err := h.login(context.Background(), input)

		require.NoError(t, err)
		assert.NotEmpty(t, out.Body.Token)
	})

	t.Run("wrong password is rejected", func(t *testing.T) {
		input := &loginInput{}
		input.Body.Email = "ada@example.com"
		input.Body.Password = "wrong"

		_, err := h.login(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 401)
	})

	t.Run("unknown email is rejected", func(t *testing.T) {
		input := &loginInput{}
		input.Body.Email = "nobody@example.com"
		input.Body.Password = "Passw0rd1234"

		_, err := h.login(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 401)
	})

	t.Run("suspended account is rejected", func(t *testing.T) {
		hash, err := bcrypt.GenerateFromPassword([]byte("Passw0rd1234"), 12)
		require.NoError(t, err)
		require.NoError(t, users.Create(&models.User{
			Name: "Suspended", Email: "suspended@example.com", Password: string(hash), Suspended: true,
		}))

		input := &loginInput{}
		input.Body.Email = "suspended@example.com"
		input.Body.Password = "Passw0rd1234"

		_, err = h.login(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 403)
	})

	t.Run("account pending approval is rejected", func(t *testing.T) {
		hash, err := bcrypt.GenerateFromPassword([]byte("Passw0rd1234"), 12)
		require.NoError(t, err)
		require.NoError(t, users.Create(&models.User{
			Name: "Pending", Email: "pending@example.com", Password: string(hash), PendingApproval: true,
		}))

		input := &loginInput{}
		input.Body.Email = "pending@example.com"
		input.Body.Password = "Passw0rd1234"

		_, err = h.login(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 403)
	})

	t.Run("rate limits repeated attempts for the same email", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		hash, err := bcrypt.GenerateFromPassword([]byte("Passw0rd1234"), 12)
		require.NoError(t, err)
		require.NoError(t, users.Create(&models.User{Name: "Rate", Email: "rate@example.com", Password: string(hash)}))

		input := &loginInput{}
		input.Body.Email = "rate@example.com"
		input.Body.Password = "wrong"

		for range loginRateLimitAttempts {
			_, err := h.login(context.Background(), input)
			require.Error(t, err)
			assertStatus(t, err, 401)
		}

		// The next attempt is throttled even with the correct password.
		input.Body.Password = "Passw0rd1234"
		_, err = h.login(context.Background(), input)
		require.Error(t, err)
		assertStatus(t, err, 429)
	})
}

func TestSetup(t *testing.T) {
	t.Run("creates the first admin", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		input := &setupInput{}
		input.Body.Name = "Root Admin"
		input.Body.Email = "admin@example.com"
		input.Body.Password = "Passw0rd1234"

		out, err := h.setup(context.Background(), input)

		require.NoError(t, err)
		assert.Equal(t, "admin", out.Body.User.Role)
		assert.True(t, out.Body.User.Verified)

		hasAdmin, err := users.HasAdmin()
		require.NoError(t, err)
		assert.True(t, hasAdmin)
	})

	t.Run("fails once an admin already exists", func(t *testing.T) {
		h, _, _ := newAuthHandler()
		input := &setupInput{}
		input.Body.Name = "Root Admin"
		input.Body.Email = "admin@example.com"
		input.Body.Password = "Passw0rd1234"
		_, err := h.setup(context.Background(), input)
		require.NoError(t, err)

		input2 := &setupInput{}
		input2.Body.Name = "Second Admin"
		input2.Body.Email = "admin2@example.com"
		input2.Body.Password = "Passw0rd1234"
		_, err = h.setup(context.Background(), input2)

		require.Error(t, err)
		assertStatus(t, err, 403)
	})
}

// TestSetup_ConcurrentRace exercises CreateAdminIfNoneExists under real
// goroutine contention, closing the TOCTOU window between the HasAdmin
// check and the admin Create — the sequential subtests in TestSetup call
// setup() one at a time and wouldn't catch a regression back to a plain
// check-then-act.
func TestSetup_ConcurrentRace(t *testing.T) {
	h, users, _ := newAuthHandler()

	var wg sync.WaitGroup
	errs := make([]error, 2)
	emails := []string{"admin-a@example.com", "admin-b@example.com"}
	for i := range 2 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			input := &setupInput{}
			input.Body.Name = "Root Admin"
			input.Body.Email = emails[i]
			input.Body.Password = "Passw0rd1234"
			_, errs[i] = h.setup(context.Background(), input)
		}(i)
	}
	wg.Wait()

	successes, failures := 0, 0
	for _, err := range errs {
		if err == nil {
			successes++
			continue
		}
		failures++
		assertStatus(t, err, 403)
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, 1, failures)

	hasAdmin, err := users.HasAdmin()
	require.NoError(t, err)
	assert.True(t, hasAdmin)
}

func TestSetupStatus(t *testing.T) {
	h, users, _ := newAuthHandler()

	out, err := h.setupStatus(context.Background(), &struct{}{})
	require.NoError(t, err)
	assert.True(t, out.Body.NeedsSetup)

	require.NoError(t, users.Create(&models.User{Name: "Admin", Email: "a@example.com", Password: "x", Role: "admin"}))

	out, err = h.setupStatus(context.Background(), &struct{}{})
	require.NoError(t, err)
	assert.False(t, out.Body.NeedsSetup)
}

func TestChangePassword(t *testing.T) {
	setup := func(t *testing.T) (*AuthHandler, *models.User) {
		h, users, _ := newAuthHandler()
		hash, err := bcrypt.GenerateFromPassword([]byte("OldPassw0rd1"), 12)
		require.NoError(t, err)
		user := &models.User{Name: "Ada", Email: "ada@example.com", Password: string(hash)}
		require.NoError(t, users.Create(user))
		return h, user
	}

	t.Run("changes the password with correct current password", func(t *testing.T) {
		h, user := setup(t)
		input := &changePasswordInput{}
		input.Body.CurrentPassword = "OldPassw0rd1"
		input.Body.NewPassword = "NewPassw0rd1"
		input.Body.ConfirmPassword = "NewPassw0rd1"

		_, err := h.changePassword(fakeAuthedCtx(t, user.ID, "user"), input)

		require.NoError(t, err)
	})

	t.Run("rejects wrong current password", func(t *testing.T) {
		h, user := setup(t)
		input := &changePasswordInput{}
		input.Body.CurrentPassword = "wrong"
		input.Body.NewPassword = "NewPassw0rd1"
		input.Body.ConfirmPassword = "NewPassw0rd1"

		_, err := h.changePassword(fakeAuthedCtx(t, user.ID, "user"), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("rejects mismatched confirmation", func(t *testing.T) {
		h, user := setup(t)
		input := &changePasswordInput{}
		input.Body.CurrentPassword = "OldPassw0rd1"
		input.Body.NewPassword = "NewPassw0rd1"
		input.Body.ConfirmPassword = "Different1"

		_, err := h.changePassword(fakeAuthedCtx(t, user.ID, "user"), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("requires authentication", func(t *testing.T) {
		h, _ := setup(t)
		input := &changePasswordInput{}
		input.Body.CurrentPassword = "OldPassw0rd1"
		input.Body.NewPassword = "NewPassw0rd1"
		input.Body.ConfirmPassword = "NewPassw0rd1"

		_, err := h.changePassword(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 401)
	})
}

func TestVerifyOTP(t *testing.T) {
	h, users, _ := newAuthHandler()
	expiry := time.Now().Add(15 * time.Minute)
	user := &models.User{Name: "Ada", Email: "ada@example.com", Password: "x", OTPCode: "123456", OTPExpiry: &expiry}
	require.NoError(t, users.Create(user))

	t.Run("correct code verifies the user", func(t *testing.T) {
		input := &verifyOTPInput{}
		input.Body.Code = "123456"

		out, err := h.verifyOTP(fakeAuthedCtx(t, user.ID, "user"), input)

		require.NoError(t, err)
		assert.True(t, out.Body.Verified)
	})

	t.Run("wrong code is rejected and invalidates the OTP", func(t *testing.T) {
		user2 := &models.User{Name: "Bob", Email: "bob@example.com", Password: "x", OTPCode: "654321", OTPExpiry: &expiry}
		require.NoError(t, users.Create(user2))

		input := &verifyOTPInput{}
		input.Body.Code = "000000"
		_, err := h.verifyOTP(fakeAuthedCtx(t, user2.ID, "user"), input)
		require.Error(t, err)
		assertStatus(t, err, 400)

		reloaded, findErr := users.FindByID(user2.ID)
		require.NoError(t, findErr)
		assert.Empty(t, reloaded.OTPCode, "a failed attempt should invalidate the OTP")
	})

	t.Run("no OTP sent is rejected", func(t *testing.T) {
		user3 := &models.User{Name: "Carl", Email: "carl@example.com", Password: "x"}
		require.NoError(t, users.Create(user3))

		input := &verifyOTPInput{}
		input.Body.Code = "123456"
		_, err := h.verifyOTP(fakeAuthedCtx(t, user3.ID, "user"), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})
}

func TestUpdateMeEmailChange(t *testing.T) {
	t.Run("immediately applies email change when flag is off", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		user := &models.User{Name: "Ada", Email: "ada@example.com", Password: "x"}
		require.NoError(t, users.Create(user))

		input := &updateMeInput{}
		newEmail := "ada-new@example.com"
		input.Body.Email = &newEmail

		out, err := h.updateMe(fakeAuthedCtx(t, user.ID, "user"), input)

		require.NoError(t, err)
		assert.Equal(t, "ada-new@example.com", out.Body.Email)
		assert.False(t, out.Body.Verified)
		assert.Empty(t, out.Body.PendingEmail)
	})

	t.Run("stages a pending change and does not touch Email when flag is on", func(t *testing.T) {
		h, users, admin := newAuthHandler()
		require.NoError(t, admin.UpsertSetting("require_email_confirmation_on_change", "true"))
		user := &models.User{Name: "Ada", Email: "ada@example.com", Password: "x", Verified: true}
		require.NoError(t, users.Create(user))

		input := &updateMeInput{}
		newEmail := "ada-new@example.com"
		input.Body.Email = &newEmail

		out, err := h.updateMe(fakeAuthedCtx(t, user.ID, "user"), input)

		require.NoError(t, err)
		assert.Equal(t, "ada@example.com", out.Body.Email, "email must not change until confirmed")
		assert.Equal(t, "ada-new@example.com", out.Body.PendingEmail)
		assert.True(t, out.Body.Verified, "existing verified status is untouched by the request step")

		reloaded, findErr := users.FindByID(user.ID)
		require.NoError(t, findErr)
		assert.NotEmpty(t, reloaded.PendingEmailOTPCode)
		assert.NotNil(t, reloaded.PendingEmailOTPExpiry)
	})

	t.Run("rejects a pending change to an email already in use", func(t *testing.T) {
		h, users, admin := newAuthHandler()
		require.NoError(t, admin.UpsertSetting("require_email_confirmation_on_change", "true"))
		require.NoError(t, users.Create(&models.User{Name: "Bob", Email: "taken@example.com", Password: "x"}))
		user := &models.User{Name: "Ada", Email: "ada@example.com", Password: "x"}
		require.NoError(t, users.Create(user))

		input := &updateMeInput{}
		taken := "taken@example.com"
		input.Body.Email = &taken

		_, err := h.updateMe(fakeAuthedCtx(t, user.ID, "user"), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})
}

func TestUpdateMePhoneChange(t *testing.T) {
	t.Run("resets PhoneVerified when phone number changes", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		user := &models.User{Name: "Ada", Email: "ada@example.com", Password: "x", Phone: "+15550100", PhoneVerified: true}
		require.NoError(t, users.Create(user))

		input := &updateMeInput{}
		newPhone := "+15550199"
		input.Body.Phone = &newPhone

		out, err := h.updateMe(fakeAuthedCtx(t, user.ID, "user"), input)

		require.NoError(t, err)
		assert.Equal(t, "+15550199", out.Body.Phone)
		assert.False(t, out.Body.PhoneVerified, "changing the phone number must clear the stale verified flag")
	})

	t.Run("leaves PhoneVerified untouched when phone is resubmitted unchanged", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		user := &models.User{Name: "Ada", Email: "ada@example.com", Password: "x", Phone: "+15550100", PhoneVerified: true}
		require.NoError(t, users.Create(user))

		input := &updateMeInput{}
		samePhone := "+15550100"
		input.Body.Phone = &samePhone

		out, err := h.updateMe(fakeAuthedCtx(t, user.ID, "user"), input)

		require.NoError(t, err)
		assert.True(t, out.Body.PhoneVerified)
	})
}

func TestConfirmEmailChange(t *testing.T) {
	setup := func(t *testing.T) (*AuthHandler, *repotest.UserRepository, *models.User) {
		h, users, admin := newAuthHandler()
		require.NoError(t, admin.UpsertSetting("require_email_confirmation_on_change", "true"))
		user := &models.User{Name: "Ada", Email: "ada@example.com", Password: "x"}
		require.NoError(t, users.Create(user))
		newEmail := "ada-new@example.com"
		in := &updateMeInput{}
		in.Body.Email = &newEmail
		_, err := h.updateMe(fakeAuthedCtx(t, user.ID, "user"), in)
		require.NoError(t, err)
		reloaded, err := users.FindByID(user.ID)
		require.NoError(t, err)
		return h, users, reloaded
	}

	t.Run("correct code applies the email and marks verified", func(t *testing.T) {
		h, users, user := setup(t)
		input := &confirmEmailChangeInput{}
		input.Body.Code = user.PendingEmailOTPCode

		out, err := h.confirmEmailChange(fakeAuthedCtx(t, user.ID, "user"), input)

		require.NoError(t, err)
		assert.Equal(t, "ada-new@example.com", out.Body.Email)
		assert.True(t, out.Body.Verified)
		assert.Empty(t, out.Body.PendingEmail)

		reloaded, findErr := users.FindByID(user.ID)
		require.NoError(t, findErr)
		assert.Empty(t, reloaded.PendingEmailOTPCode)
	})

	t.Run("wrong code is rejected and invalidates the pending OTP", func(t *testing.T) {
		h, users, user := setup(t)
		input := &confirmEmailChangeInput{}
		input.Body.Code = "000000"

		_, err := h.confirmEmailChange(fakeAuthedCtx(t, user.ID, "user"), input)

		require.Error(t, err)
		assertStatus(t, err, 400)

		reloaded, findErr := users.FindByID(user.ID)
		require.NoError(t, findErr)
		assert.Empty(t, reloaded.PendingEmailOTPCode)
		assert.NotEmpty(t, reloaded.PendingEmail, "pending target is kept so the user can resend")
	})

	t.Run("no pending change is rejected", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		user := &models.User{Name: "Carl", Email: "carl@example.com", Password: "x"}
		require.NoError(t, users.Create(user))

		input := &confirmEmailChangeInput{}
		input.Body.Code = "123456"
		_, err := h.confirmEmailChange(fakeAuthedCtx(t, user.ID, "user"), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})
}

func TestSendVerifyRegisterEmailOTP(t *testing.T) {
	t.Run("round trip: send, verify, use the token to register", func(t *testing.T) {
		h, _, _, _ := newAuthHandlerWithVerifications()

		sendIn := &sendRegisterEmailOTPInput{}
		sendIn.Body.Email = "New.User@Example.com"
		sendOut, err := h.sendRegisterEmailOTP(context.Background(), sendIn)
		require.NoError(t, err)
		require.NotEmpty(t, sendOut.Body.DebugCode, "ENV=dev should surface the code so tests/dev don't need SMTP")

		verifyIn := &verifyRegisterEmailOTPInput{}
		verifyIn.Body.Email = "New.User@Example.com"
		verifyIn.Body.Code = sendOut.Body.DebugCode
		verifyOut, err := h.verifyRegisterEmailOTP(context.Background(), verifyIn)
		require.NoError(t, err)
		require.NotEmpty(t, verifyOut.Body.VerificationToken)

		regIn := &registerInput{}
		regIn.Body.Name = "New User"
		regIn.Body.Email = "New.User@Example.com"
		regIn.Body.Password = "Passw0rd1234"
		regIn.Body.EmailVerificationToken = verifyOut.Body.VerificationToken
		out, err := h.register(context.Background(), regIn)
		require.NoError(t, err)
		assert.True(t, out.Body.User.Verified)
	})

	t.Run("debug_code is withheld outside dev", func(t *testing.T) {
		users := repotest.NewUserRepository()
		admin := repotest.NewAdminRepository()
		copies := repotest.NewCopyRepository()
		regVerifications := repotest.NewRegistrationVerificationRepository()
		h := NewAuthHandler(users, admin, copies, regVerifications, testJWTSecret, "encryption-secret", noopEmail(), noopSMS(), "prd")

		sendIn := &sendRegisterEmailOTPInput{}
		sendIn.Body.Email = "prod-user@example.com"
		sendOut, err := h.sendRegisterEmailOTP(context.Background(), sendIn)
		require.NoError(t, err)
		assert.Empty(t, sendOut.Body.DebugCode)
	})

	t.Run("rejects an already-registered email", func(t *testing.T) {
		h, users, _, _ := newAuthHandlerWithVerifications()
		require.NoError(t, users.Create(&models.User{Name: "Existing", Email: "existing@example.com", Password: "x"}))

		sendIn := &sendRegisterEmailOTPInput{}
		sendIn.Body.Email = "existing@example.com"
		_, err := h.sendRegisterEmailOTP(context.Background(), sendIn)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("wrong code is rejected and invalidates the OTP", func(t *testing.T) {
		h, _, _, regVerifications := newAuthHandlerWithVerifications()
		sendIn := &sendRegisterEmailOTPInput{}
		sendIn.Body.Email = "wrongcode@example.com"
		_, err := h.sendRegisterEmailOTP(context.Background(), sendIn)
		require.NoError(t, err)

		verifyIn := &verifyRegisterEmailOTPInput{}
		verifyIn.Body.Email = "wrongcode@example.com"
		verifyIn.Body.Code = "000000"
		_, err = h.verifyRegisterEmailOTP(context.Background(), verifyIn)
		require.Error(t, err)
		assertStatus(t, err, 400)

		_, findErr := regVerifications.Find(registrationChannelEmail, "wrongcode@example.com")
		assert.ErrorIs(t, findErr, repository.ErrNotFound, "a failed attempt should invalidate the code")
	})

	t.Run("expired code is rejected", func(t *testing.T) {
		h, _, _, regVerifications := newAuthHandlerWithVerifications()
		require.NoError(t, regVerifications.Upsert(registrationChannelEmail, "expired@example.com", "123456", time.Now().Add(-time.Minute)))

		verifyIn := &verifyRegisterEmailOTPInput{}
		verifyIn.Body.Email = "expired@example.com"
		verifyIn.Body.Code = "123456"
		_, err := h.verifyRegisterEmailOTP(context.Background(), verifyIn)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("rate limits repeated send requests for the same email", func(t *testing.T) {
		h, _, _, _ := newAuthHandlerWithVerifications()
		sendIn := &sendRegisterEmailOTPInput{}
		sendIn.Body.Email = "ratelimited@example.com"

		for range registrationOTPRateLimitAttempts {
			_, err := h.sendRegisterEmailOTP(context.Background(), sendIn)
			require.NoError(t, err)
		}

		_, err := h.sendRegisterEmailOTP(context.Background(), sendIn)
		require.Error(t, err)
		assertStatus(t, err, 429)
	})
}

func TestSendVerifyRegisterPhoneOTP(t *testing.T) {
	t.Run("round trip: send (always mocked), verify, use the token to register", func(t *testing.T) {
		h, _, _, _ := newAuthHandlerWithVerifications()

		sendIn := &sendRegisterPhoneOTPInput{}
		sendIn.Body.Phone = "+65 9123 4567"
		sendOut, err := h.sendRegisterPhoneOTP(context.Background(), sendIn)
		require.NoError(t, err)
		require.NotEmpty(t, sendOut.Body.MockCode, "phone verification has no real SMS provider — the code is always returned")

		verifyIn := &verifyRegisterPhoneOTPInput{}
		verifyIn.Body.Phone = "+65 9123 4567"
		verifyIn.Body.Code = sendOut.Body.MockCode
		verifyOut, err := h.verifyRegisterPhoneOTP(context.Background(), verifyIn)
		require.NoError(t, err)
		require.NotEmpty(t, verifyOut.Body.VerificationToken)

		regIn := &registerInput{}
		regIn.Body.Name = "Phone User"
		regIn.Body.Email = "phone-user@example.com"
		regIn.Body.Password = "Passw0rd1234"
		regIn.Body.EmailVerificationToken = mustEmailVerificationToken(t, h, "phone-user@example.com")
		phone := "+65 9123 4567"
		regIn.Body.Phone = &phone
		regIn.Body.PhoneVerificationToken = &verifyOut.Body.VerificationToken

		out, err := h.register(context.Background(), regIn)
		require.NoError(t, err)
		assert.True(t, out.Body.User.PhoneVerified)
	})

	t.Run("a phone token cannot be replayed as an email token", func(t *testing.T) {
		h, _, _, _ := newAuthHandlerWithVerifications()

		sendIn := &sendRegisterPhoneOTPInput{}
		sendIn.Body.Phone = "shared-identifier@example.com"
		sendOut, err := h.sendRegisterPhoneOTP(context.Background(), sendIn)
		require.NoError(t, err)

		verifyIn := &verifyRegisterPhoneOTPInput{}
		verifyIn.Body.Phone = "shared-identifier@example.com"
		verifyIn.Body.Code = sendOut.Body.MockCode
		verifyOut, err := h.verifyRegisterPhoneOTP(context.Background(), verifyIn)
		require.NoError(t, err)

		regIn := &registerInput{}
		regIn.Body.Name = "Confused User"
		regIn.Body.Email = "shared-identifier@example.com"
		regIn.Body.Password = "Passw0rd1234"
		// A phone-purpose token presented as the email token, for the same
		// identifier string, must still be rejected — purpose is checked
		// independently of identifier.
		regIn.Body.EmailVerificationToken = verifyOut.Body.VerificationToken

		_, err = h.register(context.Background(), regIn)
		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("rate limits repeated send requests for the same phone", func(t *testing.T) {
		h, _, _, _ := newAuthHandlerWithVerifications()
		sendIn := &sendRegisterPhoneOTPInput{}
		sendIn.Body.Phone = "+65 8888 8888"

		for range registrationOTPRateLimitAttempts {
			_, err := h.sendRegisterPhoneOTP(context.Background(), sendIn)
			require.NoError(t, err)
		}

		_, err := h.sendRegisterPhoneOTP(context.Background(), sendIn)
		require.Error(t, err)
		assertStatus(t, err, 429)
	})
}
