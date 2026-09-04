package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
)

func newDueReminderService() (*DueReminderService, *repotest.LoanRequestRepository, *repotest.CopyRepository, *repotest.UserRepository, *repotest.NotificationRepository, *repotest.TelegramNotifier) {
	copies := repotest.NewCopyRepository()
	notifs := repotest.NewNotificationRepository()
	users := repotest.NewUserRepository()
	loanReqs := repotest.NewLoanRequestRepository(copies, notifs, users)
	admin := repotest.NewAdminRepository()
	telegram := repotest.NewTelegramNotifier()
	email := NewEmailService("", "", "", "", "", "", "", "http://localhost:3000")
	return NewDueReminderService(loanReqs, notifs, admin, telegram, email), loanReqs, copies, users, notifs, telegram
}

// seedDueLoan creates a borrower/owner/copy/loan request due in 2 days
// (the default reminder window), returning the loan request.
func seedDueLoan(t *testing.T, loanReqs *repotest.LoanRequestRepository, copies *repotest.CopyRepository, users *repotest.UserRepository, borrower *models.User) *models.LoanRequest {
	t.Helper()
	require.NoError(t, users.Create(borrower))
	owner := &models.User{Name: "Owner", Email: "o@example.com"}
	require.NoError(t, users.Create(owner))
	bookCopy := &models.Copy{OwnerID: owner.ID, Owner: *owner, Book: models.Book{Title: "Some Book"}, Status: "loaned"}
	require.NoError(t, copies.Create(bookCopy))

	dueDate := time.Now().UTC().AddDate(0, 0, 2)
	lr := &models.LoanRequest{CopyID: bookCopy.ID, BorrowerID: borrower.ID, Status: "accepted", ExpectedReturnDate: dueDate}
	require.NoError(t, loanReqs.Create(lr))
	return lr
}

func TestDueReminderService_Run(t *testing.T) {
	t.Run("always creates an in-app notification, regardless of channel prefs", func(t *testing.T) {
		svc, loanReqs, copies, users, notifs, _ := newDueReminderService()
		borrower := &models.User{Name: "Borrower", Email: "b@example.com"}
		lr := seedDueLoan(t, loanReqs, copies, users, borrower)

		result := svc.Run(context.Background())

		assert.Contains(t, result, "reminded 1 of 1")
		require.Equal(t, 1, notifs.Count())
		items, err := notifs.FindByRecipient(borrower.ID, false)
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, "loan_due_soon", items[0].Type)
		require.NotNil(t, items[0].LoanRequestID)
		assert.Equal(t, lr.ID, *items[0].LoanRequestID)
	})

	t.Run("emails when the borrower has email notifications enabled", func(t *testing.T) {
		svc, loanReqs, copies, users, _, _ := newDueReminderService()
		borrower := &models.User{Name: "Borrower", Email: "b@example.com", EmailNotificationsEnabled: true}
		seedDueLoan(t, loanReqs, copies, users, borrower)

		// EmailService has no SMTP host configured, so SendEmail silently
		// no-ops (see EmailService.SendEmail) — this test only exercises
		// that the code path is reached without panicking/erroring, the
		// same way other workflow tests do for email (see loan_workflow_test.go).
		assert.NotPanics(t, func() {
			svc.Run(context.Background())
		})
	})

	t.Run("sends telegram when the borrower is linked and enabled, marks reminded", func(t *testing.T) {
		svc, loanReqs, copies, users, _, telegram := newDueReminderService()
		chatID := int64(42)
		borrower := &models.User{Name: "Borrower", Email: "b@example.com", TelegramChatID: &chatID, TelegramNotificationsEnabled: true}
		lr := seedDueLoan(t, loanReqs, copies, users, borrower)

		result := svc.Run(context.Background())

		assert.Contains(t, result, "reminded 1 of 1")
		require.Equal(t, 1, telegram.Count())
		assert.Equal(t, chatID, telegram.Messages()[0].ChatID)

		reloaded, err := loanReqs.GetByID(lr.ID)
		require.NoError(t, err)
		assert.NotNil(t, reloaded.DueReminderSentAt)
	})

	t.Run("does not remind twice", func(t *testing.T) {
		svc, loanReqs, copies, users, notifs, telegram := newDueReminderService()
		chatID := int64(42)
		borrower := &models.User{Name: "Borrower", Email: "b@example.com", TelegramChatID: &chatID, TelegramNotificationsEnabled: true}
		require.NoError(t, users.Create(borrower))
		owner := &models.User{Name: "Owner", Email: "o@example.com"}
		require.NoError(t, users.Create(owner))
		bookCopy := &models.Copy{OwnerID: owner.ID, Owner: *owner, Book: models.Book{Title: "Some Book"}, Status: "loaned"}
		require.NoError(t, copies.Create(bookCopy))

		dueDate := time.Now().UTC().AddDate(0, 0, 2)
		alreadySent := time.Now().Add(-time.Hour)
		lr := &models.LoanRequest{CopyID: bookCopy.ID, BorrowerID: borrower.ID, Status: "accepted", ExpectedReturnDate: dueDate, DueReminderSentAt: &alreadySent}
		require.NoError(t, loanReqs.Create(lr))

		result := svc.Run(context.Background())

		assert.Contains(t, result, "reminded 0 of 0")
		assert.Equal(t, 0, telegram.Count())
		assert.Equal(t, 0, notifs.Count())
	})

	t.Run("skips telegram send when borrower isn't linked, but still marks reminded", func(t *testing.T) {
		svc, loanReqs, copies, users, _, telegram := newDueReminderService()
		borrower := &models.User{Name: "Borrower", Email: "b@example.com"}
		lr := seedDueLoan(t, loanReqs, copies, users, borrower)

		svc.Run(context.Background())

		assert.Equal(t, 0, telegram.Count())
		reloaded, err := loanReqs.GetByID(lr.ID)
		require.NoError(t, err)
		assert.NotNil(t, reloaded.DueReminderSentAt)
	})
}
