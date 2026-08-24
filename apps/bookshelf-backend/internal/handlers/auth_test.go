package handlers

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/services"
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
	registration := services.NewRegistrationWorkflow(admin, repotest.NewNotificationRepository(), repotest.NewBookRepository(), noopEmail())
	h := NewAuthHandler(users, admin, copies, regVerifications, testJWTSecret, "encryption-secret", noopEmail(), noopSMS(), registration, "dev", 20, 5)
	return h, users, admin, regVerifications
}

// sendRegistration submits the registration form (the details step) and
// returns the emailed code, for tests that then verify it.
func sendRegistration(t *testing.T, h *AuthHandler, name, email, password string, phone *string) string {
	t.Helper()
	in := &sendRegisterEmailOTPInput{}
	in.Body.Name = name
	in.Body.Email = email
	in.Body.Password = password
	in.Body.Phone = phone
	out, err := h.sendRegisterEmailOTP(context.Background(), in)
	require.NoError(t, err)
	require.NotEmpty(t, out.Body.DebugCode, "ENV=dev should surface the code so tests/dev don't need SMTP")
	return out.Body.DebugCode
}

// verifyRegistrationCode completes a registration by typing the code, the
// same-tab path through verify-email-otp.
func verifyRegistrationCode(h *AuthHandler, email, code string) (*verifyRegisterEmailOTPOutput, error) {
	in := &verifyRegisterEmailOTPInput{}
	in.Body.Email = email
	in.Body.Code = code
	return h.verifyRegisterEmailOTP(context.Background(), in)
}

// TestRegisterViaEmailOTP covers the whole of registration: verify-email-otp
// creates the account outright, so there's no second call to test.
func TestRegisterViaEmailOTP(t *testing.T) {
	t.Run("creates a user and issues a token", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		code := sendRegistration(t, h, "Ada Lovelace", "ada@example.com", "Passw0rd1234", nil)

		out, err := verifyRegistrationCode(h, "ada@example.com", code)

		require.NoError(t, err)
		assert.Equal(t, "complete", out.Body.Status)
		assert.NotEmpty(t, out.Body.Token)
		assert.Equal(t, "Ada Lovelace", out.Body.User.Name)
		assert.Equal(t, "user", out.Body.User.Role)
		assert.True(t, out.Body.User.Verified, "email is verified at registration time now")
		assert.True(t, out.Body.User.EmailNotificationsEnabled, "email notifications default to on")

		stored, err := users.FindByEmail("ada@example.com")
		require.NoError(t, err)
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored.Password), []byte("Passw0rd1234")))
	})

	t.Run("no account exists until the code is verified", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		sendRegistration(t, h, "Ada", "ada-unverified@example.com", "Passw0rd1234", nil)

		_, err := users.FindByEmail("ada-unverified@example.com")
		assert.ErrorIs(t, err, repository.ErrNotFound)
	})

	t.Run("stores the account under the casing the user typed", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		// The row is keyed by the normalized address, but users.email is
		// looked up case-sensitively — creating the account lowercased would
		// lock the user out of their own first login.
		code := sendRegistration(t, h, "Ada", "Ada.Lovelace@Example.com", "Passw0rd1234", nil)

		out, err := verifyRegistrationCode(h, "Ada.Lovelace@Example.com", code)

		require.NoError(t, err)
		assert.Equal(t, "Ada.Lovelace@Example.com", out.Body.User.Email)
		_, findErr := users.FindByEmail("Ada.Lovelace@Example.com")
		assert.NoError(t, findErr)
	})

	t.Run("stores a supplied phone without marking it verified", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		phone := "+65 9123 4567"
		code := sendRegistration(t, h, "Ada", "ada-phone@example.com", "Passw0rd1234", &phone)

		out, err := verifyRegistrationCode(h, "ada-phone@example.com", code)

		require.NoError(t, err)
		assert.Equal(t, phone, out.Body.User.Phone)
		assert.False(t, out.Body.User.PhoneVerified, "no SMS provider is configured — a phone is on file, not verified")

		stored, err := users.FindByEmail("ada-phone@example.com")
		require.NoError(t, err)
		assert.Equal(t, phone, stored.Phone)
		assert.False(t, stored.PhoneVerified)
	})

	t.Run("a blank phone is stored as empty rather than whitespace", func(t *testing.T) {
		h, _, _ := newAuthHandler()
		phone := "   "
		code := sendRegistration(t, h, "Ada", "ada-blank-phone@example.com", "Passw0rd1234", &phone)

		out, err := verifyRegistrationCode(h, "ada-blank-phone@example.com", code)

		require.NoError(t, err)
		assert.Empty(t, out.Body.User.Phone)
	})

	t.Run("rejects weak passwords before sending a code", func(t *testing.T) {
		h, _, _ := newAuthHandler()
		in := &sendRegisterEmailOTPInput{}
		in.Body.Name = "Ada"
		in.Body.Email = "ada2@example.com"
		in.Body.Password = "weak"

		_, err := h.sendRegisterEmailOTP(context.Background(), in)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("rejects registration when disabled by admin setting", func(t *testing.T) {
		h, _, admin := newAuthHandler()
		require.NoError(t, admin.UpsertSetting("allow_registration", "false"))

		in := &sendRegisterEmailOTPInput{}
		in.Body.Name = "Ada"
		in.Body.Email = "ada3@example.com"
		in.Body.Password = "Passw0rd1234"

		_, err := h.sendRegisterEmailOTP(context.Background(), in)

		require.Error(t, err)
		assertStatus(t, err, 403)
	})

	t.Run("rejects a code verified after registration was disabled mid-flow", func(t *testing.T) {
		h, _, admin := newAuthHandler()
		code := sendRegistration(t, h, "Ada", "ada-disabled-midflow@example.com", "Passw0rd1234", nil)
		require.NoError(t, admin.UpsertSetting("allow_registration", "false"))

		_, err := verifyRegistrationCode(h, "ada-disabled-midflow@example.com", code)

		require.Error(t, err)
		assertStatus(t, err, 403)
	})

	t.Run("rejects an email that was claimed between send and verify", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		code := sendRegistration(t, h, "Ada", "ada-raced@example.com", "Passw0rd1234", nil)
		require.NoError(t, users.Create(&models.User{Name: "Faster", Email: "ada-raced@example.com", Password: "x"}))

		_, err := verifyRegistrationCode(h, "ada-raced@example.com", code)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("creates a pending, token-less account when approval is required", func(t *testing.T) {
		users := repotest.NewUserRepository()
		admin := repotest.NewAdminRepository()
		copies := repotest.NewCopyRepository()
		regVerifications := repotest.NewRegistrationVerificationRepository()
		notifs := repotest.NewNotificationRepository()
		registration := services.NewRegistrationWorkflow(admin, notifs, repotest.NewBookRepository(), noopEmail())
		h := NewAuthHandler(users, admin, copies, regVerifications, testJWTSecret, "encryption-secret", noopEmail(), noopSMS(), registration, "dev", 20, 5)
		require.NoError(t, admin.UpsertSetting("require_registration_approval", "true"))
		require.NoError(t, admin.SaveUser(&models.User{ID: 1, Name: "Site Admin", Email: "admin@example.com", Role: "admin"}))

		code := sendRegistration(t, h, "Ada", "ada4@example.com", "Passw0rd1234", nil)
		out, err := verifyRegistrationCode(h, "ada4@example.com", code)

		require.NoError(t, err)
		assert.Equal(t, "pending_approval", out.Body.Status)
		assert.Empty(t, out.Body.Token)
		assert.True(t, out.Body.User.PendingApproval)

		stored, findErr := users.FindByEmail("ada4@example.com")
		require.NoError(t, findErr)
		assert.True(t, stored.PendingApproval)

		adminNotifs, notifErr := notifs.FindByRecipient(1, false)
		require.NoError(t, notifErr)
		require.Len(t, adminNotifs, 1)
		assert.Equal(t, "user_pending_approval", adminNotifs[0].Type)
		require.NotNil(t, adminNotifs[0].PendingUserID)
		assert.Equal(t, stored.ID, *adminNotifs[0].PendingUserID)
	})
}

func TestVerificationStatusPhoneFactor(t *testing.T) {
	setup := func(t *testing.T, phone string, phoneVerified bool) (*AuthHandler, *models.User) {
		t.Helper()
		h, users, admin := newAuthHandler()
		require.NoError(t, admin.UpsertSetting("verification_requires_phone", "true"))
		user := &models.User{Name: "Ada", Email: "ada@example.com", Password: "x", Phone: phone, PhoneVerified: phoneVerified}
		require.NoError(t, users.Create(user))
		return h, user
	}

	phoneFactor := func(t *testing.T, out *verificationStatusOutput) verificationFactor {
		t.Helper()
		for _, f := range out.Body.Factors {
			if f.Key == "phone" {
				return f
			}
		}
		t.Fatal("expected a phone factor when verification_requires_phone is on")
		return verificationFactor{}
	}

	t.Run("a phone on file satisfies the factor even though it was never verified", func(t *testing.T) {
		// Nothing sets PhoneVerified true any more, and checkPhoneRequirement
		// (loan_requests.go) has only ever checked Phone != "" — so checking
		// PhoneVerified here would make this permanently unsatisfiable.
		h, user := setup(t, "+65 9123 4567", false)

		out, err := h.verificationStatus(fakeAuthedCtx(t, user.ID, "user"), nil)

		require.NoError(t, err)
		f := phoneFactor(t, out)
		assert.Equal(t, "Phone number on file", f.Label)
		assert.True(t, f.Satisfied)
		assert.True(t, out.Body.Eligible)
	})

	t.Run("no phone on file leaves the factor unsatisfied", func(t *testing.T) {
		h, user := setup(t, "", false)

		out, err := h.verificationStatus(fakeAuthedCtx(t, user.ID, "user"), nil)

		require.NoError(t, err)
		assert.False(t, phoneFactor(t, out).Satisfied)
		assert.False(t, out.Body.Eligible)
	})

	t.Run("the factor is absent when the community does not require a phone", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		user := &models.User{Name: "Ada", Email: "ada@example.com", Password: "x"}
		require.NoError(t, users.Create(user))

		out, err := h.verificationStatus(fakeAuthedCtx(t, user.ID, "user"), nil)

		require.NoError(t, err)
		for _, f := range out.Body.Factors {
			assert.NotEqual(t, "phone", f.Key)
		}
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

func TestRegistrationRequirements(t *testing.T) {
	h, _, admin := newAuthHandler()

	out, err := h.registrationRequirements(context.Background(), &struct{}{})
	require.NoError(t, err)
	assert.False(t, out.Body.RequirePhone)

	require.NoError(t, admin.UpsertSetting("verification_requires_phone", "true"))

	out, err = h.registrationRequirements(context.Background(), &struct{}{})
	require.NoError(t, err)
	assert.True(t, out.Body.RequirePhone)
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

func TestForgotPassword(t *testing.T) {
	t.Run("generates a reset code for a registered email", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		user := &models.User{Name: "Ada", Email: "ada@example.com", Password: "x"}
		require.NoError(t, users.Create(user))

		input := &forgotPasswordInput{}
		input.Body.Email = "ada@example.com"

		out, err := h.forgotPassword(context.Background(), input)

		require.NoError(t, err)
		assert.NotEmpty(t, out.Body.DebugCode, "ENV=dev should echo the code")

		reloaded, findErr := users.FindByEmail("ada@example.com")
		require.NoError(t, findErr)
		assert.Equal(t, out.Body.DebugCode, reloaded.ResetPasswordOTPCode)
		require.NotNil(t, reloaded.ResetPasswordOTPExpiry)
	})

	t.Run("responds successfully for an unregistered email, without a code", func(t *testing.T) {
		h, _, _ := newAuthHandler()
		input := &forgotPasswordInput{}
		input.Body.Email = "nobody@example.com"

		out, err := h.forgotPassword(context.Background(), input)

		require.NoError(t, err, "must not reveal whether the email is registered")
		assert.Empty(t, out.Body.DebugCode)
	})
}

func TestResetPassword(t *testing.T) {
	setup := func(t *testing.T) (*AuthHandler, *repotest.UserRepository, *models.User) {
		h, users, _ := newAuthHandler()
		expiry := time.Now().Add(15 * time.Minute)
		user := &models.User{
			Name: "Ada", Email: "ada@example.com", Password: "x",
			ResetPasswordOTPCode: "123456", ResetPasswordOTPExpiry: &expiry,
		}
		require.NoError(t, users.Create(user))
		return h, users, user
	}

	t.Run("resets the password with a correct code", func(t *testing.T) {
		h, users, user := setup(t)
		input := &resetPasswordInput{}
		input.Body.Email = "ada@example.com"
		input.Body.Code = "123456"
		input.Body.NewPassword = "NewPassw0rd1"
		input.Body.ConfirmPassword = "NewPassw0rd1"

		_, err := h.resetPassword(context.Background(), input)

		require.NoError(t, err)
		reloaded, findErr := users.FindByID(user.ID)
		require.NoError(t, findErr)
		assert.Empty(t, reloaded.ResetPasswordOTPCode, "the code must be consumed on success")
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(reloaded.Password), []byte("NewPassw0rd1")))
	})

	t.Run("wrong code is rejected and invalidates the reset code", func(t *testing.T) {
		h, users, user := setup(t)
		input := &resetPasswordInput{}
		input.Body.Email = "ada@example.com"
		input.Body.Code = "000000"
		input.Body.NewPassword = "NewPassw0rd1"
		input.Body.ConfirmPassword = "NewPassw0rd1"

		_, err := h.resetPassword(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
		reloaded, findErr := users.FindByID(user.ID)
		require.NoError(t, findErr)
		assert.Empty(t, reloaded.ResetPasswordOTPCode, "a failed attempt should invalidate the code")
	})

	t.Run("expired code is rejected", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		expiry := time.Now().Add(-time.Minute)
		user := &models.User{
			Name: "Bob", Email: "bob@example.com", Password: "x",
			ResetPasswordOTPCode: "123456", ResetPasswordOTPExpiry: &expiry,
		}
		require.NoError(t, users.Create(user))

		input := &resetPasswordInput{}
		input.Body.Email = "bob@example.com"
		input.Body.Code = "123456"
		input.Body.NewPassword = "NewPassw0rd1"
		input.Body.ConfirmPassword = "NewPassw0rd1"

		_, err := h.resetPassword(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("no code requested is rejected", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		user := &models.User{Name: "Carl", Email: "carl@example.com", Password: "x"}
		require.NoError(t, users.Create(user))

		input := &resetPasswordInput{}
		input.Body.Email = "carl@example.com"
		input.Body.Code = "123456"
		input.Body.NewPassword = "NewPassw0rd1"
		input.Body.ConfirmPassword = "NewPassw0rd1"

		_, err := h.resetPassword(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("unregistered email is rejected with the same error as a bad code", func(t *testing.T) {
		h, _, _ := newAuthHandler()
		input := &resetPasswordInput{}
		input.Body.Email = "nobody@example.com"
		input.Body.Code = "123456"
		input.Body.NewPassword = "NewPassw0rd1"
		input.Body.ConfirmPassword = "NewPassw0rd1"

		_, err := h.resetPassword(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("rejects mismatched confirmation", func(t *testing.T) {
		h, _, _ := setup(t)
		input := &resetPasswordInput{}
		input.Body.Email = "ada@example.com"
		input.Body.Code = "123456"
		input.Body.NewPassword = "NewPassw0rd1"
		input.Body.ConfirmPassword = "Different1"

		_, err := h.resetPassword(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("rejects a weak new password", func(t *testing.T) {
		h, _, _ := setup(t)
		input := &resetPasswordInput{}
		input.Body.Email = "ada@example.com"
		input.Body.Code = "123456"
		input.Body.NewPassword = "weak"
		input.Body.ConfirmPassword = "weak"

		_, err := h.resetPassword(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("resets the password via a magic-link token, without submitting email or code", func(t *testing.T) {
		h, users, user := setup(t)
		token, err := h.issueOTPLinkToken(otpLinkPurposeResetPassword, "ada@example.com", "123456")
		require.NoError(t, err)

		input := &resetPasswordInput{}
		input.Body.Token = token
		input.Body.NewPassword = "NewPassw0rd1"
		input.Body.ConfirmPassword = "NewPassw0rd1"

		_, err = h.resetPassword(context.Background(), input)

		require.NoError(t, err)
		reloaded, findErr := users.FindByID(user.ID)
		require.NoError(t, findErr)
		assert.Empty(t, reloaded.ResetPasswordOTPCode)
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(reloaded.Password), []byte("NewPassw0rd1")))
	})

	t.Run("a token minted for a different purpose is rejected", func(t *testing.T) {
		h, _, _ := setup(t)
		token, err := h.issueOTPLinkToken(otpLinkPurposeEmailChange, "ada@example.com", "123456")
		require.NoError(t, err)

		input := &resetPasswordInput{}
		input.Body.Token = token
		input.Body.NewPassword = "NewPassw0rd1"
		input.Body.ConfirmPassword = "NewPassw0rd1"

		_, err = h.resetPassword(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("neither token nor email/code is rejected", func(t *testing.T) {
		h, _, _ := setup(t)
		input := &resetPasswordInput{}
		input.Body.NewPassword = "NewPassw0rd1"
		input.Body.ConfirmPassword = "NewPassw0rd1"

		_, err := h.resetPassword(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
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

	t.Run("verifies via a magic-link token, without submitting a code", func(t *testing.T) {
		user4 := &models.User{Name: "Dana", Email: "dana@example.com", Password: "x", OTPCode: "111222", OTPExpiry: &expiry}
		require.NoError(t, users.Create(user4))

		token, err := h.issueOTPLinkToken(otpLinkPurposeOTPVerify, "anything", "111222")
		require.NoError(t, err)

		input := &verifyOTPInput{}
		input.Body.Token = token
		out, err := h.verifyOTP(fakeAuthedCtx(t, user4.ID, "user"), input)

		require.NoError(t, err)
		assert.True(t, out.Body.Verified)
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

func TestUpdateMeContactPrefs(t *testing.T) {
	t.Run("sets email notification preference and messaging usernames", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		user := &models.User{Name: "Ada", Email: "ada@example.com", Password: "x", EmailNotificationsEnabled: true}
		require.NoError(t, users.Create(user))

		input := &updateMeInput{}
		disabled := false
		telegram := "@ada"
		whatsapp := "+15550100"
		note := "prefer evenings"
		input.Body.EmailNotificationsEnabled = &disabled
		input.Body.TelegramUsername = &telegram
		input.Body.WhatsAppUsername = &whatsapp
		input.Body.ContactNote = &note

		out, err := h.updateMe(fakeAuthedCtx(t, user.ID, "user"), input)

		require.NoError(t, err)
		assert.False(t, out.Body.EmailNotificationsEnabled)
		assert.Equal(t, "@ada", out.Body.TelegramUsername)
		assert.Equal(t, "+15550100", out.Body.WhatsAppUsername)
		assert.Equal(t, "prefer evenings", out.Body.ContactNote)

		reloaded, findErr := users.FindByID(user.ID)
		require.NoError(t, findErr)
		assert.False(t, reloaded.EmailNotificationsEnabled)
		assert.Equal(t, "@ada", reloaded.TelegramUsername)
		assert.Equal(t, "+15550100", reloaded.WhatsAppUsername)
		assert.Equal(t, "prefer evenings", reloaded.ContactNote)
	})

	t.Run("empty string clears a previously set username", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		user := &models.User{Name: "Ada", Email: "ada2@example.com", Password: "x", TelegramUsername: "@ada"}
		require.NoError(t, users.Create(user))

		input := &updateMeInput{}
		empty := ""
		input.Body.TelegramUsername = &empty

		out, err := h.updateMe(fakeAuthedCtx(t, user.ID, "user"), input)

		require.NoError(t, err)
		assert.Empty(t, out.Body.TelegramUsername)
	})

	t.Run("empty string clears a previously set contact note", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		user := &models.User{Name: "Ada", Email: "ada4@example.com", Password: "x", ContactNote: "old note"}
		require.NoError(t, users.Create(user))

		input := &updateMeInput{}
		empty := ""
		input.Body.ContactNote = &empty

		out, err := h.updateMe(fakeAuthedCtx(t, user.ID, "user"), input)

		require.NoError(t, err)
		assert.Empty(t, out.Body.ContactNote)
	})

	t.Run("leaves fields untouched when omitted", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		user := &models.User{
			Name: "Ada", Email: "ada3@example.com", Password: "x",
			EmailNotificationsEnabled: true, TelegramUsername: "@ada",
		}
		require.NoError(t, users.Create(user))

		input := &updateMeInput{}
		newName := "Ada Lovelace"
		input.Body.Name = &newName

		out, err := h.updateMe(fakeAuthedCtx(t, user.ID, "user"), input)

		require.NoError(t, err)
		assert.True(t, out.Body.EmailNotificationsEnabled)
		assert.Equal(t, "@ada", out.Body.TelegramUsername)
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

	t.Run("confirms via a magic-link token, without submitting a code", func(t *testing.T) {
		h, _, user := setup(t)
		token, err := h.issueOTPLinkToken(otpLinkPurposeEmailChange, fmt.Sprint(user.ID), user.PendingEmailOTPCode)
		require.NoError(t, err)

		input := &confirmEmailChangeInput{}
		input.Body.Token = token
		out, err := h.confirmEmailChange(fakeAuthedCtx(t, user.ID, "user"), input)

		require.NoError(t, err)
		assert.Equal(t, "ada-new@example.com", out.Body.Email)
	})
}

func TestSendVerifyRegisterEmailOTP(t *testing.T) {
	t.Run("round trip: send, then verify the code to finish registration", func(t *testing.T) {
		h, _, _, _ := newAuthHandlerWithVerifications()
		code := sendRegistration(t, h, "New User", "New.User@Example.com", "Passw0rd1234", nil)

		out, err := verifyRegistrationCode(h, "New.User@Example.com", code)

		require.NoError(t, err)
		assert.Equal(t, "complete", out.Body.Status)
		assert.True(t, out.Body.User.Verified)
	})

	t.Run("the magic-link token alone finishes registration, with no form state", func(t *testing.T) {
		// The reported bug: the link is read on a device that never saw the
		// details step, so email+code aren't submitted and nothing but the
		// token identifies the pending signup.
		h, users, _, _ := newAuthHandlerWithVerifications()
		phone := "+65 9123 4567"
		code := sendRegistration(t, h, "Link User", "link.user@example.com", "Passw0rd1234", &phone)

		linkToken, err := h.issueOTPLinkToken(otpLinkPurposeRegisterEmail, normalizeEmail("link.user@example.com"), code)
		require.NoError(t, err)

		verifyIn := &verifyRegisterEmailOTPInput{}
		verifyIn.Body.Token = linkToken
		out, err := h.verifyRegisterEmailOTP(context.Background(), verifyIn)

		require.NoError(t, err)
		assert.Equal(t, "complete", out.Body.Status)
		assert.NotEmpty(t, out.Body.Token, "clicking the link signs the user in")
		assert.Equal(t, "Link User", out.Body.User.Name)
		assert.Equal(t, phone, out.Body.User.Phone)

		stored, findErr := users.FindByEmail("link.user@example.com")
		require.NoError(t, findErr)
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored.Password), []byte("Passw0rd1234")))
	})

	t.Run("a case variant of a registered address is rejected, not made a second account", func(t *testing.T) {
		// The pending row is keyed by the normalized address while the
		// account is created with the casing that was typed, so a
		// case-sensitive duplicate check here would let "Ada@Example.com"
		// through and hand one mailbox two accounts with two passwords.
		h, users, _, _ := newAuthHandlerWithVerifications()
		require.NoError(t, users.Create(&models.User{Name: "Ada", Email: "ada@example.com", Password: "hash"}))

		in := &sendRegisterEmailOTPInput{}
		in.Body.Name = "Impostor"
		in.Body.Email = "Ada@Example.com"
		in.Body.Password = "Passw0rd1234"
		_, err := h.sendRegisterEmailOTP(context.Background(), in)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("debug_code and debug_verify_link are withheld outside dev", func(t *testing.T) {
		users := repotest.NewUserRepository()
		admin := repotest.NewAdminRepository()
		copies := repotest.NewCopyRepository()
		regVerifications := repotest.NewRegistrationVerificationRepository()
		registration := services.NewRegistrationWorkflow(admin, repotest.NewNotificationRepository(), repotest.NewBookRepository(), noopEmail())
		h := NewAuthHandler(users, admin, copies, regVerifications, testJWTSecret, "encryption-secret", noopEmail(), noopSMS(), registration, "prd", 20, 5)

		sendIn := &sendRegisterEmailOTPInput{}
		sendIn.Body.Name = "Prod User"
		sendIn.Body.Email = "prod-user@example.com"
		sendIn.Body.Password = "Passw0rd1234"
		sendOut, err := h.sendRegisterEmailOTP(context.Background(), sendIn)
		require.NoError(t, err)
		assert.Empty(t, sendOut.Body.DebugCode)
		assert.Empty(t, sendOut.Body.DebugVerifyLink)
	})

	t.Run("debug_verify_link points at the frontend's /register?verifyToken= route", func(t *testing.T) {
		h, _, _, _ := newAuthHandlerWithVerifications()
		sendIn := &sendRegisterEmailOTPInput{}
		sendIn.Body.Name = "Dev User"
		sendIn.Body.Email = "dev-user@example.com"
		sendIn.Body.Password = "Passw0rd1234"

		sendOut, err := h.sendRegisterEmailOTP(context.Background(), sendIn)

		require.NoError(t, err)
		assert.Contains(t, sendOut.Body.DebugVerifyLink, "/register?verifyToken=")
	})

	t.Run("rejects an already-registered email", func(t *testing.T) {
		h, users, _, _ := newAuthHandlerWithVerifications()
		require.NoError(t, users.Create(&models.User{Name: "Existing", Email: "existing@example.com", Password: "x"}))

		sendIn := &sendRegisterEmailOTPInput{}
		sendIn.Body.Name = "Existing"
		sendIn.Body.Email = "existing@example.com"
		sendIn.Body.Password = "Passw0rd1234"
		_, err := h.sendRegisterEmailOTP(context.Background(), sendIn)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("wrong code is rejected and invalidates the OTP", func(t *testing.T) {
		h, _, _, regVerifications := newAuthHandlerWithVerifications()
		sendRegistration(t, h, "Wrong Code", "wrongcode@example.com", "Passw0rd1234", nil)

		_, err := verifyRegistrationCode(h, "wrongcode@example.com", "000000")
		require.Error(t, err)
		assertStatus(t, err, 400)

		_, findErr := regVerifications.Find(registrationChannelEmail, "wrongcode@example.com")
		assert.ErrorIs(t, findErr, repository.ErrNotFound, "a failed attempt should invalidate the code")
	})

	t.Run("a code is single-use — replaying it cannot create a second account", func(t *testing.T) {
		h, _, _, _ := newAuthHandlerWithVerifications()
		code := sendRegistration(t, h, "Replay", "replay@example.com", "Passw0rd1234", nil)
		_, err := verifyRegistrationCode(h, "replay@example.com", code)
		require.NoError(t, err)

		_, err = verifyRegistrationCode(h, "replay@example.com", code)
		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("expired code is rejected", func(t *testing.T) {
		h, _, _, regVerifications := newAuthHandlerWithVerifications()
		require.NoError(t, regVerifications.Upsert(
			registrationChannelEmail, "expired@example.com", "123456", time.Now().Add(-time.Minute),
			models.PendingRegistrationData{PendingName: "Expired", PendingEmail: "expired@example.com", PendingPasswordHash: "hash"},
		))

		_, err := verifyRegistrationCode(h, "expired@example.com", "123456")

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("editing the details and resending replaces the pending account", func(t *testing.T) {
		h, users, _, _ := newAuthHandlerWithVerifications()
		sendRegistration(t, h, "First Draft", "resend@example.com", "Passw0rd1234", nil)
		freshCode := sendRegistration(t, h, "Second Draft", "resend@example.com", "Passw0rd5678", nil)

		out, err := verifyRegistrationCode(h, "resend@example.com", freshCode)

		require.NoError(t, err)
		assert.Equal(t, "Second Draft", out.Body.User.Name, "the resend supersedes the earlier draft rather than accumulating")
		stored, findErr := users.FindByEmail("resend@example.com")
		require.NoError(t, findErr)
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored.Password), []byte("Passw0rd5678")))
	})

	t.Run("rate limits repeated send requests for the same email", func(t *testing.T) {
		h, _, _, _ := newAuthHandlerWithVerifications()
		sendIn := &sendRegisterEmailOTPInput{}
		sendIn.Body.Name = "Rate Limited"
		sendIn.Body.Email = "ratelimited@example.com"
		sendIn.Body.Password = "Passw0rd1234"

		for range registrationOTPRateLimitAttempts {
			_, err := h.sendRegisterEmailOTP(context.Background(), sendIn)
			require.NoError(t, err)
		}

		_, err := h.sendRegisterEmailOTP(context.Background(), sendIn)
		require.Error(t, err)
		assertStatus(t, err, 429)
	})
}

// TestSendVerifyRegisterPhoneOTP covers endpoints the app itself no longer
// calls — registration is a single email step now. They're kept working for
// a future real SMS provider (see sendRegisterPhoneOTP's tech-debt note), so
// they're kept tested too; the token they mint has no consumer, hence no
// assertion here about anything downstream accepting it.
func TestSendVerifyRegisterPhoneOTP(t *testing.T) {
	t.Run("round trip: send (always mocked), then verify", func(t *testing.T) {
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
		assert.NotEmpty(t, verifyOut.Body.VerificationToken)
	})

	t.Run("wrong code is rejected and invalidates the OTP", func(t *testing.T) {
		h, _, _, regVerifications := newAuthHandlerWithVerifications()
		sendIn := &sendRegisterPhoneOTPInput{}
		sendIn.Body.Phone = "+65 9000 0001"
		_, err := h.sendRegisterPhoneOTP(context.Background(), sendIn)
		require.NoError(t, err)

		verifyIn := &verifyRegisterPhoneOTPInput{}
		verifyIn.Body.Phone = "+65 9000 0001"
		verifyIn.Body.Code = "000000"
		_, err = h.verifyRegisterPhoneOTP(context.Background(), verifyIn)
		require.Error(t, err)
		assertStatus(t, err, 400)

		_, findErr := regVerifications.Find(registrationChannelPhone, "+65 9000 0001")
		assert.ErrorIs(t, findErr, repository.ErrNotFound, "a failed attempt should invalidate the code")
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
