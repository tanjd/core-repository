package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/services"
)

func newJobsHandler(botHealth *repotest.BotHealthChecker) *JobsHandler {
	books := repotest.NewBookRepository()
	admin := repotest.NewAdminRepository()
	scheduler := services.NewScheduler(books, admin, "", "24h")
	users := repotest.NewUserRepository()
	recommendations := repotest.NewRecommendationRepository(users)
	digest := services.NewDigestService(books, recommendations, users, admin, noopEmail(), repotest.NewTelegramNotifier())
	return NewJobsHandler(scheduler, digest, users, botHealth)
}

func TestTelegramBotStatus(t *testing.T) {
	t.Run("requires admin", func(t *testing.T) {
		h := newJobsHandler(repotest.NewBotHealthChecker())
		_, err := h.telegramBotStatus(fakeAuthedCtxNone(), &struct{}{})
		assertStatus(t, err, 401)
	})

	t.Run("non-admin is forbidden", func(t *testing.T) {
		h := newJobsHandler(repotest.NewBotHealthChecker())
		_, err := h.telegramBotStatus(fakeAuthedCtx(t, 1, "user"), &struct{}{})
		assertStatus(t, err, 403)
	})

	t.Run("reports online when configured and reachable", func(t *testing.T) {
		bh := repotest.NewBotHealthChecker()
		h := newJobsHandler(bh)

		out, err := h.telegramBotStatus(fakeAuthedCtx(t, 1, "admin"), &struct{}{})

		require.NoError(t, err)
		assert.True(t, out.Body.Configured)
		assert.True(t, out.Body.Online)
	})

	t.Run("reports offline when configured but unreachable", func(t *testing.T) {
		bh := repotest.NewBotHealthChecker()
		bh.IsOnline = false
		h := newJobsHandler(bh)

		out, err := h.telegramBotStatus(fakeAuthedCtx(t, 1, "admin"), &struct{}{})

		require.NoError(t, err)
		assert.True(t, out.Body.Configured)
		assert.False(t, out.Body.Online)
	})

	t.Run("reports not configured without attempting a check", func(t *testing.T) {
		bh := &repotest.BotHealthChecker{IsConfigured: false, IsOnline: true}
		h := newJobsHandler(bh)

		out, err := h.telegramBotStatus(fakeAuthedCtx(t, 1, "admin"), &struct{}{})

		require.NoError(t, err)
		assert.False(t, out.Body.Configured)
		assert.False(t, out.Body.Online, "online should read false when not configured, even though the fake's IsOnline is true")
	})
}
