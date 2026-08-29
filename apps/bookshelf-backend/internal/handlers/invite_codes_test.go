package handlers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
)

func newInviteCodeHandler() (*InviteCodeHandler, *repotest.InviteCodeRepository, *repotest.AdminRepository, *repotest.UserRepository) {
	users := repotest.NewUserRepository()
	admin := repotest.NewAdminRepository()
	codes := repotest.NewInviteCodeRepository(users)
	return NewInviteCodeHandler(codes, admin, users, noopEmail()), codes, admin, users
}

func seedVerifiedUser(t *testing.T, users *repotest.UserRepository) {
	t.Helper()
	require.NoError(t, users.Save(&models.User{ID: 1, Name: "Member", Email: "member@example.com", Verified: true}))
}

func TestGetInviteCode(t *testing.T) {
	t.Run("creates a code on first call", func(t *testing.T) {
		h, _, _, users := newInviteCodeHandler()
		seedVerifiedUser(t, users)

		out, err := h.getInviteCode(fakeAuthedCtx(t, 1, "user"), &struct{}{})
		require.NoError(t, err)
		assert.Len(t, out.Body.Code, inviteCodeLength)
		assert.Contains(t, out.Body.URL, out.Body.Code)
	})

	t.Run("returns the same code on a subsequent call", func(t *testing.T) {
		h, _, _, users := newInviteCodeHandler()
		seedVerifiedUser(t, users)

		first, err := h.getInviteCode(fakeAuthedCtx(t, 1, "user"), &struct{}{})
		require.NoError(t, err)
		second, err := h.getInviteCode(fakeAuthedCtx(t, 1, "user"), &struct{}{})
		require.NoError(t, err)
		assert.Equal(t, first.Body.Code, second.Body.Code)
	})

	t.Run("403 when disabled and no code exists yet", func(t *testing.T) {
		h, _, admin, users := newInviteCodeHandler()
		seedVerifiedUser(t, users)
		require.NoError(t, admin.UpsertSetting("allow_invite_codes", "false"))

		_, err := h.getInviteCode(fakeAuthedCtx(t, 1, "user"), &struct{}{})
		require.Error(t, err)
		assertStatus(t, err, 403)
	})

	t.Run("returns the existing code even when disabled", func(t *testing.T) {
		h, _, admin, users := newInviteCodeHandler()
		seedVerifiedUser(t, users)

		first, err := h.getInviteCode(fakeAuthedCtx(t, 1, "user"), &struct{}{})
		require.NoError(t, err)

		require.NoError(t, admin.UpsertSetting("allow_invite_codes", "false"))
		second, err := h.getInviteCode(fakeAuthedCtx(t, 1, "user"), &struct{}{})
		require.NoError(t, err)
		assert.Equal(t, first.Body.Code, second.Body.Code)
	})

	t.Run("403 for an unverified caller", func(t *testing.T) {
		h, _, _, users := newInviteCodeHandler()
		require.NoError(t, users.Save(&models.User{ID: 1, Name: "Unverified", Email: "u@example.com", Verified: false}))

		_, err := h.getInviteCode(fakeAuthedCtx(t, 1, "user"), &struct{}{})
		require.Error(t, err)
		assertStatus(t, err, 403)
	})

	t.Run("unauthenticated is unauthorized", func(t *testing.T) {
		h, _, _, _ := newInviteCodeHandler()
		_, err := h.getInviteCode(fakeAuthedCtxNone(), &struct{}{})
		assertStatus(t, err, 401)
	})
}

func TestRegenerateInviteCode(t *testing.T) {
	t.Run("returns a new code and invalidates the old one", func(t *testing.T) {
		h, codes, _, users := newInviteCodeHandler()
		seedVerifiedUser(t, users)

		first, err := h.getInviteCode(fakeAuthedCtx(t, 1, "user"), &struct{}{})
		require.NoError(t, err)

		second, err := h.regenerateInviteCode(fakeAuthedCtx(t, 1, "user"), &struct{}{})
		require.NoError(t, err)
		assert.NotEqual(t, first.Body.Code, second.Body.Code)

		_, findErr := codes.FindByCode(first.Body.Code)
		require.Error(t, findErr, "old code must no longer be findable")
	})

	t.Run("403 when disabled", func(t *testing.T) {
		h, _, admin, users := newInviteCodeHandler()
		seedVerifiedUser(t, users)
		require.NoError(t, admin.UpsertSetting("allow_invite_codes", "false"))

		_, err := h.regenerateInviteCode(fakeAuthedCtx(t, 1, "user"), &struct{}{})
		require.Error(t, err)
		assertStatus(t, err, 403)
	})
}

func TestValidateInviteCode(t *testing.T) {
	t.Run("valid code returns the inviter's name", func(t *testing.T) {
		h, _, _, users := newInviteCodeHandler()
		seedVerifiedUser(t, users)
		out, err := h.getInviteCode(fakeAuthedCtx(t, 1, "user"), &struct{}{})
		require.NoError(t, err)

		result, err := h.validateInviteCode(context.Background(), &validateInviteCodeInput{Code: out.Body.Code})
		require.NoError(t, err)
		assert.True(t, result.Body.Valid)
		assert.Equal(t, "Member", result.Body.InviterName)
	})

	t.Run("unknown code is invalid with no inviter name", func(t *testing.T) {
		h, _, _, _ := newInviteCodeHandler()
		result, err := h.validateInviteCode(context.Background(), &validateInviteCodeInput{Code: "nosuchcode"})
		require.NoError(t, err)
		assert.False(t, result.Body.Valid)
		assert.Empty(t, result.Body.InviterName)
	})

	t.Run("revoked code is invalid", func(t *testing.T) {
		h, codes, _, users := newInviteCodeHandler()
		seedVerifiedUser(t, users)
		out, err := h.getInviteCode(fakeAuthedCtx(t, 1, "user"), &struct{}{})
		require.NoError(t, err)
		require.NoError(t, codes.DeleteByInviter(1))

		result, err := h.validateInviteCode(context.Background(), &validateInviteCodeInput{Code: out.Body.Code})
		require.NoError(t, err)
		assert.False(t, result.Body.Valid)
	})
}

func TestListInviteCodes(t *testing.T) {
	t.Run("non-admin is forbidden", func(t *testing.T) {
		h, _, _, users := newInviteCodeHandler()
		seedVerifiedUser(t, users)
		_, err := h.listInviteCodes(fakeAuthedCtx(t, 1, "user"), &struct{}{})
		assertStatus(t, err, 403)
	})

	t.Run("admin sees the full list with inviter names", func(t *testing.T) {
		h, _, _, users := newInviteCodeHandler()
		seedVerifiedUser(t, users)
		require.NoError(t, users.Save(&models.User{ID: 2, Name: "Admin", Email: "admin@example.com", Verified: true, Role: "admin"}))
		_, err := h.getInviteCode(fakeAuthedCtx(t, 1, "user"), &struct{}{})
		require.NoError(t, err)

		out, err := h.listInviteCodes(fakeAuthedCtx(t, 2, "admin"), &struct{}{})
		require.NoError(t, err)
		require.Len(t, out.Body, 1)
		assert.Equal(t, "Member", out.Body[0].InviterName)
	})
}

func TestRevokeInviteCode(t *testing.T) {
	t.Run("non-admin is forbidden", func(t *testing.T) {
		h, _, _, users := newInviteCodeHandler()
		seedVerifiedUser(t, users)
		_, err := h.revokeInviteCode(fakeAuthedCtx(t, 1, "user"), &adminInviteCodeIDInput{ID: 1})
		assertStatus(t, err, 403)
	})

	t.Run("admin can revoke a code", func(t *testing.T) {
		h, codes, _, users := newInviteCodeHandler()
		seedVerifiedUser(t, users)
		out, err := h.getInviteCode(fakeAuthedCtx(t, 1, "user"), &struct{}{})
		require.NoError(t, err)
		ic, err := codes.FindByCode(out.Body.Code)
		require.NoError(t, err)

		_, err = h.revokeInviteCode(fakeAuthedCtx(t, 99, "admin"), &adminInviteCodeIDInput{ID: ic.ID})
		require.NoError(t, err)

		_, findErr := codes.FindByCode(out.Body.Code)
		require.Error(t, findErr)
	})

	t.Run("404 for an unknown id", func(t *testing.T) {
		h, _, _, _ := newInviteCodeHandler()
		_, err := h.revokeInviteCode(fakeAuthedCtx(t, 99, "admin"), &adminInviteCodeIDInput{ID: 999})
		assertStatus(t, err, 404)
	})
}
