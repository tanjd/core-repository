package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
)

func newAdminHandler() (*AdminHandler, *repotest.AdminRepository) {
	admin := repotest.NewAdminRepository()
	return NewAdminHandler(admin, ""), admin
}

func TestAdminHandler_RequiresAdmin(t *testing.T) {
	h, admin := newAdminHandler()
	require.NoError(t, admin.SaveUser(&models.User{ID: 1, Role: "user"}))

	t.Run("non-admin is forbidden", func(t *testing.T) {
		_, err := h.getDashboardStats(fakeAuthedCtx(t, 1, "user"), &struct{}{})
		require.Error(t, err)
		assertStatus(t, err, 403)
	})

	t.Run("unauthenticated is unauthorized", func(t *testing.T) {
		_, err := h.getDashboardStats(fakeAuthedCtxNone(), &struct{}{})
		require.Error(t, err)
		assertStatus(t, err, 401)
	})

	t.Run("admin is allowed", func(t *testing.T) {
		_, err := h.getDashboardStats(fakeAuthedCtx(t, 1, "admin"), &struct{}{})
		require.NoError(t, err)
	})
}

func TestUpdateUser_RoleChanges(t *testing.T) {
	newHandlerWithUsers := func(t *testing.T, users ...*models.User) (*AdminHandler, *repotest.AdminRepository) {
		h, admin := newAdminHandler()
		for _, u := range users {
			require.NoError(t, admin.SaveUser(u))
		}
		return h, admin
	}

	t.Run("admin cannot demote themselves", func(t *testing.T) {
		h, admin := newHandlerWithUsers(t,
			&models.User{ID: 1, Role: "admin"},
			&models.User{ID: 2, Role: "admin"},
		)
		input := &updateAdminUserInput{ID: 1}
		role := "user"
		input.Body.Role = &role

		_, err := h.updateUser(fakeAuthedCtx(t, 1, "admin"), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
		_ = admin
	})

	t.Run("cannot demote the last admin", func(t *testing.T) {
		h, _ := newHandlerWithUsers(t,
			&models.User{ID: 1, Role: "admin"},
			&models.User{ID: 2, Role: "user"},
		)
		input := &updateAdminUserInput{ID: 1}
		role := "user"
		input.Body.Role = &role

		_, err := h.updateUser(fakeAuthedCtx(t, 2, "admin"), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("can demote an admin when another admin remains", func(t *testing.T) {
		h, admin := newHandlerWithUsers(t,
			&models.User{ID: 1, Role: "admin"},
			&models.User{ID: 2, Role: "admin"},
		)
		input := &updateAdminUserInput{ID: 1}
		role := "user"
		input.Body.Role = &role

		out, err := h.updateUser(fakeAuthedCtx(t, 2, "admin"), input)

		require.NoError(t, err)
		assert.Equal(t, "user", out.Body.Role)

		count, countErr := admin.CountByRole("admin")
		require.NoError(t, countErr)
		assert.Equal(t, int64(1), count)
	})

	t.Run("rejects an invalid role value", func(t *testing.T) {
		h, _ := newHandlerWithUsers(t, &models.User{ID: 1, Role: "admin"}, &models.User{ID: 2, Role: "user"})
		input := &updateAdminUserInput{ID: 2}
		role := "superuser"
		input.Body.Role = &role

		_, err := h.updateUser(fakeAuthedCtx(t, 1, "admin"), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})
}

func TestUpdateUser_SuspensionChanges(t *testing.T) {
	h, admin := newAdminHandler()
	require.NoError(t, admin.SaveUser(&models.User{ID: 1, Role: "admin"}))
	require.NoError(t, admin.SaveUser(&models.User{ID: 2, Role: "user"}))

	t.Run("admin cannot suspend themselves", func(t *testing.T) {
		input := &updateAdminUserInput{ID: 1}
		suspended := true
		input.Body.Suspended = &suspended

		_, err := h.updateUser(fakeAuthedCtx(t, 1, "admin"), input)

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("admin can suspend another user", func(t *testing.T) {
		input := &updateAdminUserInput{ID: 2}
		suspended := true
		input.Body.Suspended = &suspended

		out, err := h.updateUser(fakeAuthedCtx(t, 1, "admin"), input)

		require.NoError(t, err)
		assert.True(t, out.Body.Suspended)
	})
}

func TestDeleteUser(t *testing.T) {
	t.Run("cannot delete yourself", func(t *testing.T) {
		h, admin := newAdminHandler()
		require.NoError(t, admin.SaveUser(&models.User{ID: 1, Role: "admin"}))

		_, err := h.deleteUser(fakeAuthedCtx(t, 1, "admin"), &adminUserIDInput{ID: 1})

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("cannot delete the last admin", func(t *testing.T) {
		h, admin := newAdminHandler()
		require.NoError(t, admin.SaveUser(&models.User{ID: 1, Role: "admin"}))
		// ID 2's stored role is "user"; the request context grants "admin" via
		// the auth token directly (as middleware.RequireAdmin checks the token
		// claim, not the DB record) so this exercises deleteUser's own
		// last-admin guard on the *target*, independent of caller identity.
		require.NoError(t, admin.SaveUser(&models.User{ID: 2, Role: "user"}))

		_, err := h.deleteUser(fakeAuthedCtx(t, 2, "admin"), &adminUserIDInput{ID: 1})

		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("can delete a non-last admin or regular user", func(t *testing.T) {
		h, admin := newAdminHandler()
		require.NoError(t, admin.SaveUser(&models.User{ID: 1, Role: "admin"}))
		require.NoError(t, admin.SaveUser(&models.User{ID: 2, Role: "user"}))

		_, err := h.deleteUser(fakeAuthedCtx(t, 1, "admin"), &adminUserIDInput{ID: 2})

		require.NoError(t, err)
		_, findErr := admin.FindUserByID(2)
		assert.Error(t, findErr)
	})
}

func TestUpdateSettings(t *testing.T) {
	h, admin := newAdminHandler()
	require.NoError(t, admin.SaveUser(&models.User{ID: 1, Role: "admin"}))

	input := &updateSettingsInput{}
	input.Body = []struct {
		Key   string `json:"key" required:"true" doc:"Setting key"`
		Value string `json:"value" required:"true" doc:"Setting value"`
	}{
		{Key: "max_active_loans", Value: "3"},
	}

	out, err := h.updateSettings(fakeAuthedCtx(t, 1, "admin"), input)

	require.NoError(t, err)
	require.Len(t, out.Body, 1)
	assert.Equal(t, "max_active_loans", out.Body[0].Key)
	assert.Equal(t, "3", out.Body[0].Value)
}
