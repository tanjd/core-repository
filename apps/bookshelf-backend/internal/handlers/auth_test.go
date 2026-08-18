package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
)

func newAuthHandler() (*AuthHandler, *repotest.UserRepository, *repotest.AdminRepository) {
	users := repotest.NewUserRepository()
	admin := repotest.NewAdminRepository()
	copies := repotest.NewCopyRepository()
	h := NewAuthHandler(users, admin, copies, testJWTSecret, "encryption-secret", noopEmail())
	return h, users, admin
}

func TestRegister(t *testing.T) {
	t.Run("creates a user and issues a token", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		input := &registerInput{}
		input.Body.Name = "Ada Lovelace"
		input.Body.Email = "ada@example.com"
		input.Body.Password = "Passw0rd"

		out, err := h.register(context.Background(), input)

		require.NoError(t, err)
		assert.NotEmpty(t, out.Body.Token)
		assert.Equal(t, "Ada Lovelace", out.Body.User.Name)
		assert.Equal(t, "user", out.Body.User.Role)

		stored, err := users.FindByEmail("ada@example.com")
		require.NoError(t, err)
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored.Password), []byte("Passw0rd")))
	})

	t.Run("rejects weak passwords", func(t *testing.T) {
		h, _, _ := newAuthHandler()
		input := &registerInput{}
		input.Body.Name = "Ada"
		input.Body.Email = "ada2@example.com"
		input.Body.Password = "weak"

		_, err := h.register(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("rejects duplicate email", func(t *testing.T) {
		h, _, _ := newAuthHandler()
		input := &registerInput{}
		input.Body.Name = "Ada"
		input.Body.Email = "dup@example.com"
		input.Body.Password = "Passw0rd"
		_, err := h.register(context.Background(), input)
		require.NoError(t, err)

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
		input.Body.Password = "Passw0rd"

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
		input.Body.Password = "Passw0rd"

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
	hash, err := bcrypt.GenerateFromPassword([]byte("Passw0rd"), 12)
	require.NoError(t, err)
	require.NoError(t, users.Create(&models.User{Name: "Ada", Email: "ada@example.com", Password: string(hash)}))

	t.Run("valid credentials issue a token", func(t *testing.T) {
		input := &loginInput{}
		input.Body.Email = "ada@example.com"
		input.Body.Password = "Passw0rd"

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
		input.Body.Password = "Passw0rd"

		_, err := h.login(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 401)
	})

	t.Run("suspended account is rejected", func(t *testing.T) {
		hash, err := bcrypt.GenerateFromPassword([]byte("Passw0rd"), 12)
		require.NoError(t, err)
		require.NoError(t, users.Create(&models.User{
			Name: "Suspended", Email: "suspended@example.com", Password: string(hash), Suspended: true,
		}))

		input := &loginInput{}
		input.Body.Email = "suspended@example.com"
		input.Body.Password = "Passw0rd"

		_, err = h.login(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 403)
	})

	t.Run("account pending approval is rejected", func(t *testing.T) {
		hash, err := bcrypt.GenerateFromPassword([]byte("Passw0rd"), 12)
		require.NoError(t, err)
		require.NoError(t, users.Create(&models.User{
			Name: "Pending", Email: "pending@example.com", Password: string(hash), PendingApproval: true,
		}))

		input := &loginInput{}
		input.Body.Email = "pending@example.com"
		input.Body.Password = "Passw0rd"

		_, err = h.login(context.Background(), input)

		require.Error(t, err)
		assertStatus(t, err, 403)
	})
}

func TestSetup(t *testing.T) {
	t.Run("creates the first admin", func(t *testing.T) {
		h, users, _ := newAuthHandler()
		input := &setupInput{}
		input.Body.Name = "Root Admin"
		input.Body.Email = "admin@example.com"
		input.Body.Password = "Passw0rd"

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
		input.Body.Password = "Passw0rd"
		_, err := h.setup(context.Background(), input)
		require.NoError(t, err)

		input2 := &setupInput{}
		input2.Body.Name = "Second Admin"
		input2.Body.Email = "admin2@example.com"
		input2.Body.Password = "Passw0rd"
		_, err = h.setup(context.Background(), input2)

		require.Error(t, err)
		assertStatus(t, err, 403)
	})
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
		hash, err := bcrypt.GenerateFromPassword([]byte("OldPassw0rd"), 12)
		require.NoError(t, err)
		user := &models.User{Name: "Ada", Email: "ada@example.com", Password: string(hash)}
		require.NoError(t, users.Create(user))
		return h, user
	}

	t.Run("changes the password with correct current password", func(t *testing.T) {
		h, user := setup(t)
		input := &changePasswordInput{}
		input.Body.CurrentPassword = "OldPassw0rd"
		input.Body.NewPassword = "NewPassw0rd"
		input.Body.ConfirmPassword = "NewPassw0rd"

		_, err := h.changePassword(fakeAuthedCtx(t, user.ID, "user"), input)

		require.NoError(t, err)
	})

	t.Run("rejects wrong current password", func(t *testing.T) {
		h, user := setup(t)
		input := &changePasswordInput{}
		input.Body.CurrentPassword = "wrong"
		input.Body.NewPassword = "NewPassw0rd"
		input.Body.ConfirmPassword = "NewPassw0rd"

		_, err := h.changePassword(fakeAuthedCtx(t, user.ID, "user"), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("rejects mismatched confirmation", func(t *testing.T) {
		h, user := setup(t)
		input := &changePasswordInput{}
		input.Body.CurrentPassword = "OldPassw0rd"
		input.Body.NewPassword = "NewPassw0rd"
		input.Body.ConfirmPassword = "Different1"

		_, err := h.changePassword(fakeAuthedCtx(t, user.ID, "user"), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("requires authentication", func(t *testing.T) {
		h, _ := setup(t)
		input := &changePasswordInput{}
		input.Body.CurrentPassword = "OldPassw0rd"
		input.Body.NewPassword = "NewPassw0rd"
		input.Body.ConfirmPassword = "NewPassw0rd"

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
	t.Run("immediately applies email change when flag is off (default)", func(t *testing.T) {
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
