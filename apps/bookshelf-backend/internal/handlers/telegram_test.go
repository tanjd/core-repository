package handlers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
)

const testTelegramInternalSecret = "test-internal-secret"

func newTelegramHandler() (*TelegramHandler, *repotest.UserRepository, *repotest.TelegramNotifier) {
	users := repotest.NewUserRepository()
	telegram := repotest.NewTelegramNotifier()
	return NewTelegramHandler(users, telegram, testTelegramInternalSecret, "testbookshelfbot"), users, telegram
}

func TestCreateLinkToken_RequiresAuth(t *testing.T) {
	h, _, _ := newTelegramHandler()
	_, err := h.createLinkToken(fakeAuthedCtxNone(), &struct{}{})
	assertStatus(t, err, 401)
}

func TestCreateLinkToken_MintsTokenForAuthedUser(t *testing.T) {
	h, users, _ := newTelegramHandler()
	user := &models.User{Name: "Ada", Email: "ada@example.com", Password: "x"}
	require.NoError(t, users.Create(user))

	out, err := h.createLinkToken(fakeAuthedCtx(t, user.ID, "user"), &struct{}{})

	require.NoError(t, err)
	assert.NotEmpty(t, out.Body.Token)
	assert.Equal(t, "testbookshelfbot", out.Body.BotUsername)

	userID, verifyErr := h.verifyLinkToken(out.Body.Token)
	require.NoError(t, verifyErr)
	assert.Equal(t, user.ID, userID)
}

func TestConfirmLink(t *testing.T) {
	t.Run("valid token links the chat and enables notifications", func(t *testing.T) {
		h, users, _ := newTelegramHandler()
		user := &models.User{Name: "Ada", Email: "ada@example.com", Password: "x"}
		require.NoError(t, users.Create(user))

		token, err := h.issueLinkToken(user.ID)
		require.NoError(t, err)

		input := &confirmLinkInput{}
		input.Body.Token = token
		input.Body.ChatID = 555
		out, err := h.confirmLink(context.Background(), input)

		require.NoError(t, err)
		assert.Equal(t, "Ada", out.Body.Name)

		reloaded, findErr := users.FindByID(user.ID)
		require.NoError(t, findErr)
		require.NotNil(t, reloaded.TelegramChatID)
		assert.Equal(t, int64(555), *reloaded.TelegramChatID)
		assert.True(t, reloaded.TelegramNotificationsEnabled)
		assert.NotNil(t, reloaded.TelegramLinkedAt)
	})

	t.Run("unknown token returns 400", func(t *testing.T) {
		h, _, _ := newTelegramHandler()

		input := &confirmLinkInput{}
		input.Body.Token = "not-a-real-token"
		input.Body.ChatID = 555
		_, err := h.confirmLink(context.Background(), input)

		assertStatus(t, err, 400)
	})

	t.Run("expired token returns 400", func(t *testing.T) {
		h, users, _ := newTelegramHandler()
		user := &models.User{Name: "Ada", Email: "ada@example.com", Password: "x"}
		require.NoError(t, users.Create(user))

		start := time.Now()
		h.now = func() time.Time { return start }
		token, err := h.issueLinkToken(user.ID)
		require.NoError(t, err)
		h.now = func() time.Time { return start.Add(telegramLinkTokenTTL + time.Second) }

		input := &confirmLinkInput{}
		input.Body.Token = token
		input.Body.ChatID = 555
		_, err = h.confirmLink(context.Background(), input)

		assertStatus(t, err, 400)
	})

	t.Run("a token can only be used once", func(t *testing.T) {
		h, users, _ := newTelegramHandler()
		user := &models.User{Name: "Ada", Email: "ada@example.com", Password: "x"}
		require.NoError(t, users.Create(user))
		token, err := h.issueLinkToken(user.ID)
		require.NoError(t, err)

		input := &confirmLinkInput{}
		input.Body.Token = token
		input.Body.ChatID = 555
		_, err = h.confirmLink(context.Background(), input)
		require.NoError(t, err)

		_, err = h.confirmLink(context.Background(), input)
		assertStatus(t, err, 400)
	})
}

// TestIssueLinkToken_FitsTelegramDeepLinkConstraint guards against
// regressing to a token shape (like the JWT this replaced) that Telegram's
// deep-link `start` parameter can't actually carry: max 64 characters,
// alphabet restricted to [A-Za-z0-9_-]. A token that violates this arrives
// at the bot with no payload at all — /start silently succeeds with nothing
// to link, which is much harder to notice than an explicit rejection.
func TestIssueLinkToken_FitsTelegramDeepLinkConstraint(t *testing.T) {
	h, _, _ := newTelegramHandler()
	token, err := h.issueLinkToken(1)
	require.NoError(t, err)

	assert.LessOrEqual(t, len(token), 64)
	assert.Regexp(t, `^[A-Za-z0-9_-]+$`, token)
}

func TestRequireInternalSecret(t *testing.T) {
	h, _, _ := newTelegramHandler()

	require.NoError(t, h.requireInternalSecret(testTelegramInternalSecret))
	require.Error(t, h.requireInternalSecret("wrong-secret"))
	require.Error(t, h.requireInternalSecret(""))
}

func TestUnlink(t *testing.T) {
	h, users, _ := newTelegramHandler()
	chatID := int64(555)
	linkedAt := time.Now()
	user := &models.User{
		Name: "Ada", Email: "ada@example.com", Password: "x",
		TelegramChatID: &chatID, TelegramLinkedAt: &linkedAt, TelegramNotificationsEnabled: false,
	}
	require.NoError(t, users.Create(user))

	_, err := h.unlink(fakeAuthedCtx(t, user.ID, "user"), &struct{}{})
	require.NoError(t, err)

	reloaded, findErr := users.FindByID(user.ID)
	require.NoError(t, findErr)
	assert.Nil(t, reloaded.TelegramChatID)
	assert.Nil(t, reloaded.TelegramLinkedAt)
	assert.True(t, reloaded.TelegramNotificationsEnabled, "should reset to the default so a future re-link starts clean")
}

func TestUnlink_RequiresAuth(t *testing.T) {
	h, _, _ := newTelegramHandler()
	_, err := h.unlink(fakeAuthedCtxNone(), &struct{}{})
	assertStatus(t, err, 401)
}

func TestSendTestNotification(t *testing.T) {
	t.Run("requires auth", func(t *testing.T) {
		h, _, _ := newTelegramHandler()
		_, err := h.sendTestNotification(fakeAuthedCtxNone(), &struct{}{})
		assertStatus(t, err, 401)
	})

	t.Run("requires Telegram to already be linked", func(t *testing.T) {
		h, users, telegram := newTelegramHandler()
		user := &models.User{Name: "Ada", Email: "ada@example.com", Password: "x"}
		require.NoError(t, users.Create(user))

		_, err := h.sendTestNotification(fakeAuthedCtx(t, user.ID, "user"), &struct{}{})

		assertStatus(t, err, 400)
		assert.Equal(t, 0, telegram.Count())
	})

	t.Run("sends to the member's linked chat", func(t *testing.T) {
		h, users, telegram := newTelegramHandler()
		chatID := int64(555)
		user := &models.User{
			Name: "Ada", Email: "ada@example.com", Password: "x",
			TelegramChatID: &chatID, TelegramNotificationsEnabled: true,
		}
		require.NoError(t, users.Create(user))

		_, err := h.sendTestNotification(fakeAuthedCtx(t, user.ID, "user"), &struct{}{})

		require.NoError(t, err)
		require.Equal(t, 1, telegram.Count())
		assert.Equal(t, chatID, telegram.Messages()[0].ChatID)
	})

	t.Run("surfaces a delivery failure as 502", func(t *testing.T) {
		h, users, telegram := newTelegramHandler()
		chatID := int64(555)
		user := &models.User{
			Name: "Ada", Email: "ada@example.com", Password: "x",
			TelegramChatID: &chatID, TelegramNotificationsEnabled: true,
		}
		require.NoError(t, users.Create(user))
		telegram.NotifyErr = errors.New("telegram: sendMessage returned 403")

		_, err := h.sendTestNotification(fakeAuthedCtx(t, user.ID, "user"), &struct{}{})

		assertStatus(t, err, 502)
	})
}
