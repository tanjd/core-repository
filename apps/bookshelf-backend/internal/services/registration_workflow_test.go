package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
)

func newRegistrationWorkflow() (*RegistrationWorkflow, *repotest.AdminRepository, *repotest.NotificationRepository, *repotest.BookRepository, *repotest.TelegramNotifier) {
	admin := repotest.NewAdminRepository()
	notifs := repotest.NewNotificationRepository()
	books := repotest.NewBookRepository()
	email := NewEmailService("", "", "", "", "", "", "", "http://localhost:3000")
	telegram := repotest.NewTelegramNotifier()
	return NewRegistrationWorkflow(admin, notifs, books, email, telegram), admin, notifs, books, telegram
}

func TestOnPendingApproval_NotifiesEveryAdmin(t *testing.T) {
	workflow, admin, notifs, _, telegram := newRegistrationWorkflow()
	chatID := int64(111)
	admin1 := &models.User{ID: 1, Name: "Admin One", Email: "a1@example.com", Role: "admin", EmailNotificationsEnabled: true, TelegramChatID: &chatID, TelegramNotificationsEnabled: true}
	admin2 := &models.User{ID: 2, Name: "Admin Two", Email: "a2@example.com", Role: "admin", EmailNotificationsEnabled: false}
	notAdmin := &models.User{ID: 3, Name: "Regular User", Email: "u@example.com", Role: "user"}
	require.NoError(t, admin.SaveUser(admin1))
	require.NoError(t, admin.SaveUser(admin2))
	require.NoError(t, admin.SaveUser(notAdmin))

	user := &models.User{ID: 9, Name: "New Member", Email: "new@example.com"}
	workflow.OnPendingApproval(context.Background(), user)

	// One in-app notification per admin (2), regardless of channel prefs —
	// notAdmin (role "user") gets nothing.
	assert.Equal(t, 2, notifs.Count())

	// Only admin1 has Telegram linked and enabled.
	require.Equal(t, 1, telegram.Count())
	assert.Equal(t, chatID, telegram.Messages()[0].ChatID)
}

func TestOnPendingApproval_SkipsTelegram_WhenNotLinkedOrDisabled(t *testing.T) {
	workflow, admin, _, _, telegram := newRegistrationWorkflow()
	notLinked := &models.User{ID: 1, Name: "Admin One", Email: "a1@example.com", Role: "admin"}
	chatID := int64(222)
	linkedButDisabled := &models.User{ID: 2, Name: "Admin Two", Email: "a2@example.com", Role: "admin", TelegramChatID: &chatID, TelegramNotificationsEnabled: false}
	require.NoError(t, admin.SaveUser(notLinked))
	require.NoError(t, admin.SaveUser(linkedButDisabled))

	user := &models.User{ID: 9, Name: "New Member", Email: "new@example.com"}
	workflow.OnPendingApproval(context.Background(), user)

	assert.Equal(t, 0, telegram.Count())
}

func TestOnApproved_CreatesInAppNotification(t *testing.T) {
	workflow, _, notifs, _, _ := newRegistrationWorkflow()
	user := &models.User{ID: 5, Name: "New User", Email: "new@example.com"}

	workflow.OnApproved(context.Background(), user)

	assert.Equal(t, 1, notifs.Count())
	items, err := notifs.FindByRecipient(user.ID, false)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "user_approved", items[0].Type)
}

func TestOnRegistered_NoNotification(t *testing.T) {
	workflow, _, notifs, _, _ := newRegistrationWorkflow()
	user := &models.User{ID: 9, Name: "Self Signup", Email: "self@example.com", EmailNotificationsEnabled: true}

	workflow.OnRegistered(context.Background(), user)

	assert.Equal(t, 0, notifs.Count())
}

func TestOnRegistered_SkipsWhenEmailDisabled(t *testing.T) {
	workflow, _, _, _, _ := newRegistrationWorkflow()
	user := &models.User{ID: 10, Name: "Opted Out", Email: "optout@example.com", EmailNotificationsEnabled: false}

	assert.NotPanics(t, func() {
		workflow.OnRegistered(context.Background(), user)
	})
}

func TestCatalogPitch(t *testing.T) {
	t.Run("pluralizes for multiple books", func(t *testing.T) {
		workflow, _, _, books, _ := newRegistrationWorkflow()
		require.NoError(t, books.Create(&models.Book{Title: "A"}))
		require.NoError(t, books.Create(&models.Book{Title: "B"}))

		pitch := workflow.catalogPitch(context.Background())

		assert.Contains(t, pitch, "There are currently <strong>2 books</strong>")
	})

	t.Run("singular for exactly one book", func(t *testing.T) {
		workflow, _, _, books, _ := newRegistrationWorkflow()
		require.NoError(t, books.Create(&models.Book{Title: "A"}))

		pitch := workflow.catalogPitch(context.Background())

		assert.Contains(t, pitch, "There is currently <strong>1 book</strong>")
	})

	t.Run("zero books still renders a pitch", func(t *testing.T) {
		workflow, _, _, _, _ := newRegistrationWorkflow()

		pitch := workflow.catalogPitch(context.Background())

		assert.Contains(t, pitch, "There are currently <strong>0 books</strong>")
	})
}
