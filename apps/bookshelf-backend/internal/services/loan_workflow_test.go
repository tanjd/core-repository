package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
)

type workflowDeps struct {
	workflow  *LoanWorkflow
	copies    *repotest.CopyRepository
	loanReqs  *repotest.LoanRequestRepository
	notifs    *repotest.NotificationRepository
	users     *repotest.UserRepository
	waitlists *repotest.WaitlistRepository
}

func newWorkflow() *workflowDeps {
	copies := repotest.NewCopyRepository()
	notifs := repotest.NewNotificationRepository()
	users := repotest.NewUserRepository()
	loanReqs := repotest.NewLoanRequestRepository(copies, notifs, users)
	waitlists := repotest.NewWaitlistRepository()
	email := NewEmailService("", "", "", "", "", "", "")
	return &workflowDeps{
		workflow: NewLoanWorkflow(copies, loanReqs, notifs, users, waitlists, email),
		copies:   copies, loanReqs: loanReqs, notifs: notifs, users: users, waitlists: waitlists,
	}
}

func TestOnRequested_NotifiesOwner(t *testing.T) {
	d := newWorkflow()
	owner := &models.User{Name: "Owner", Email: "owner@example.com"}
	require.NoError(t, d.users.Create(owner))
	borrower := &models.User{Name: "Borrower", Email: "borrower@example.com"}
	require.NoError(t, d.users.Create(borrower))
	bookCopy := &models.Copy{OwnerID: owner.ID, Owner: *owner, Book: models.Book{Title: "Some Book"}, Status: "requested"}
	require.NoError(t, d.copies.Create(bookCopy))
	lr := &models.LoanRequest{ID: 1, CopyID: bookCopy.ID, BorrowerID: borrower.ID}

	err := d.workflow.OnRequested(context.Background(), lr)

	require.NoError(t, err)
	assert.Equal(t, 1, d.notifs.Count())
}

func TestOnAccepted_RejectsCompetingRequestsAndMarksLoaned(t *testing.T) {
	d := newWorkflow()
	owner := &models.User{Name: "Owner", Email: "owner@example.com"}
	require.NoError(t, d.users.Create(owner))
	borrower := &models.User{Name: "Borrower", Email: "borrower@example.com"}
	require.NoError(t, d.users.Create(borrower))
	otherBorrower := &models.User{Name: "Other", Email: "other@example.com"}
	require.NoError(t, d.users.Create(otherBorrower))

	bookCopy := &models.Copy{OwnerID: owner.ID, Owner: *owner, Book: models.Book{Title: "Some Book"}, Status: "requested"}
	require.NoError(t, d.copies.Create(bookCopy))

	accepted := &models.LoanRequest{CopyID: bookCopy.ID, BorrowerID: borrower.ID, Status: "accepted"}
	require.NoError(t, d.loanReqs.Create(accepted))
	competing := &models.LoanRequest{CopyID: bookCopy.ID, BorrowerID: otherBorrower.ID, Status: "pending"}
	require.NoError(t, d.loanReqs.Create(competing))

	err := d.workflow.OnAccepted(context.Background(), accepted)

	require.NoError(t, err)

	reloadedCompeting, findErr := d.loanReqs.GetByID(competing.ID)
	require.NoError(t, findErr)
	assert.Equal(t, "rejected", reloadedCompeting.Status)

	updatedCopy, findErr := d.copies.GetByID(bookCopy.ID)
	require.NoError(t, findErr)
	assert.Equal(t, "loaned", updatedCopy.Status)

	// One rejection notification for the competing borrower, one acceptance
	// notification for the accepted borrower.
	assert.Equal(t, 2, d.notifs.Count())
}

func TestOnRejected_FreesCopyOnlyWhenNoOtherPendingRequests(t *testing.T) {
	t.Run("frees the copy when it was the only pending request", func(t *testing.T) {
		d := newWorkflow()
		bookCopy := &models.Copy{Status: "requested"}
		require.NoError(t, d.copies.Create(bookCopy))
		lr := &models.LoanRequest{CopyID: bookCopy.ID, BorrowerID: 1, Status: "rejected"}
		require.NoError(t, d.loanReqs.Create(lr))

		require.NoError(t, d.workflow.OnRejected(context.Background(), lr))

		updated, err := d.copies.GetByID(bookCopy.ID)
		require.NoError(t, err)
		assert.Equal(t, "available", updated.Status)
	})

	t.Run("leaves the copy alone when other pending requests remain", func(t *testing.T) {
		d := newWorkflow()
		bookCopy := &models.Copy{Status: "requested"}
		require.NoError(t, d.copies.Create(bookCopy))
		lr := &models.LoanRequest{CopyID: bookCopy.ID, BorrowerID: 1, Status: "rejected"}
		require.NoError(t, d.loanReqs.Create(lr))
		other := &models.LoanRequest{CopyID: bookCopy.ID, BorrowerID: 2, Status: "pending"}
		require.NoError(t, d.loanReqs.Create(other))

		require.NoError(t, d.workflow.OnRejected(context.Background(), lr))

		updated, err := d.copies.GetByID(bookCopy.ID)
		require.NoError(t, err)
		assert.Equal(t, "requested", updated.Status)
	})
}

func TestOnReturned_NotifiesWaitlistAndClearsIt(t *testing.T) {
	d := newWorkflow()
	owner := &models.User{Name: "Owner", Email: "owner@example.com"}
	require.NoError(t, d.users.Create(owner))
	borrower := &models.User{Name: "Borrower", Email: "borrower@example.com"}
	require.NoError(t, d.users.Create(borrower))
	waiter := &models.User{Name: "Waiter", Email: "waiter@example.com"}
	require.NoError(t, d.users.Create(waiter))

	bookCopy := &models.Copy{OwnerID: owner.ID, Owner: *owner, Book: models.Book{Title: "Some Book"}, Status: "loaned"}
	require.NoError(t, d.copies.Create(bookCopy))
	require.NoError(t, d.waitlists.Add(bookCopy.ID, waiter.ID))

	lr := &models.LoanRequest{CopyID: bookCopy.ID, BorrowerID: borrower.ID, Status: "returned", ReturnedBy: &owner.ID}
	require.NoError(t, d.loanReqs.Create(lr))

	err := d.workflow.OnReturned(context.Background(), lr)

	require.NoError(t, err)

	updatedCopy, findErr := d.copies.GetByID(bookCopy.ID)
	require.NoError(t, findErr)
	assert.Equal(t, "available", updatedCopy.Status)

	entries, listErr := d.waitlists.ListByCopyID(bookCopy.ID)
	require.NoError(t, listErr)
	assert.Empty(t, entries, "waitlist should be cleared after notifying")

	// One "marked_returned" notification for the borrower, one
	// "waitlist_available" notification for the waiter.
	assert.Equal(t, 2, d.notifs.Count())
}

func TestOnReturned_NotifiesWhicheverPartyDidNotAct(t *testing.T) {
	t.Run("owner-initiated return notifies the borrower", func(t *testing.T) {
		d := newWorkflow()
		owner := &models.User{Name: "Owner", Email: "owner@example.com"}
		require.NoError(t, d.users.Create(owner))
		borrower := &models.User{Name: "Borrower", Email: "borrower@example.com"}
		require.NoError(t, d.users.Create(borrower))
		bookCopy := &models.Copy{OwnerID: owner.ID, Owner: *owner, Book: models.Book{Title: "Some Book"}, Status: "loaned"}
		require.NoError(t, d.copies.Create(bookCopy))
		lr := &models.LoanRequest{CopyID: bookCopy.ID, BorrowerID: borrower.ID, Status: "returned", ReturnedBy: &owner.ID}
		require.NoError(t, d.loanReqs.Create(lr))

		require.NoError(t, d.workflow.OnReturned(context.Background(), lr))

		notifs := mustFindByRecipient(t, d.notifs, borrower.ID)
		require.Len(t, notifs, 1)
		assert.Equal(t, "marked_returned", notifs[0].Type)
		assert.Empty(t, mustFindByRecipient(t, d.notifs, owner.ID), "owner shouldn't be notified about their own action")
	})

	t.Run("borrower-initiated return notifies the owner", func(t *testing.T) {
		d := newWorkflow()
		owner := &models.User{Name: "Owner", Email: "owner@example.com"}
		require.NoError(t, d.users.Create(owner))
		borrower := &models.User{Name: "Borrower", Email: "borrower@example.com"}
		require.NoError(t, d.users.Create(borrower))
		bookCopy := &models.Copy{OwnerID: owner.ID, Owner: *owner, Book: models.Book{Title: "Some Book"}, Status: "loaned"}
		require.NoError(t, d.copies.Create(bookCopy))
		lr := &models.LoanRequest{CopyID: bookCopy.ID, BorrowerID: borrower.ID, Status: "returned", ReturnedBy: &borrower.ID}
		require.NoError(t, d.loanReqs.Create(lr))

		require.NoError(t, d.workflow.OnReturned(context.Background(), lr))

		notifs := mustFindByRecipient(t, d.notifs, owner.ID)
		require.Len(t, notifs, 1)
		assert.Equal(t, "marked_returned", notifs[0].Type)
		assert.Empty(t, mustFindByRecipient(t, d.notifs, borrower.ID), "borrower shouldn't be notified about their own action")
	})
}

func TestOnReturnUndone_RestoresLoanedStatusAndNotifiesBorrower(t *testing.T) {
	d := newWorkflow()
	owner := &models.User{Name: "Owner", Email: "owner@example.com"}
	require.NoError(t, d.users.Create(owner))
	borrower := &models.User{Name: "Borrower", Email: "borrower@example.com"}
	require.NoError(t, d.users.Create(borrower))

	bookCopy := &models.Copy{OwnerID: owner.ID, Owner: *owner, Book: models.Book{Title: "Some Book"}, Status: "available"}
	require.NoError(t, d.copies.Create(bookCopy))
	lr := &models.LoanRequest{CopyID: bookCopy.ID, BorrowerID: borrower.ID, Status: "accepted"}
	require.NoError(t, d.loanReqs.Create(lr))

	err := d.workflow.OnReturnUndone(context.Background(), lr)

	require.NoError(t, err)

	updatedCopy, findErr := d.copies.GetByID(bookCopy.ID)
	require.NoError(t, findErr)
	assert.Equal(t, "loaned", updatedCopy.Status)

	notifs := mustFindByRecipient(t, d.notifs, borrower.ID)
	require.Len(t, notifs, 1)
	assert.Equal(t, "return_undone", notifs[0].Type)
}

// TestNotificationEmailGating_DoesNotBlockNotificationRow verifies that
// EmailNotificationsEnabled only gates the best-effort email send, never the
// in-app Notification row, at each site that checks it.
func TestNotificationEmailGating_DoesNotBlockNotificationRow(t *testing.T) {
	t.Run("OnRequested notifies owner regardless of the owner's email preference", func(t *testing.T) {
		d := newWorkflow()
		owner := &models.User{Name: "Owner", Email: "owner@example.com", EmailNotificationsEnabled: false}
		require.NoError(t, d.users.Create(owner))
		borrower := &models.User{Name: "Borrower", Email: "borrower@example.com"}
		require.NoError(t, d.users.Create(borrower))
		bookCopy := &models.Copy{OwnerID: owner.ID, Owner: *owner, Book: models.Book{Title: "Some Book"}, Status: "requested"}
		require.NoError(t, d.copies.Create(bookCopy))
		lr := &models.LoanRequest{ID: 1, CopyID: bookCopy.ID, BorrowerID: borrower.ID}

		err := d.workflow.OnRequested(context.Background(), lr)

		require.NoError(t, err)
		assert.Equal(t, 1, d.notifs.Count())
	})

	t.Run("OnAccepted notifies borrower regardless of the borrower's email preference", func(t *testing.T) {
		d := newWorkflow()
		owner := &models.User{Name: "Owner", Email: "owner@example.com"}
		require.NoError(t, d.users.Create(owner))
		borrower := &models.User{Name: "Borrower", Email: "borrower@example.com", EmailNotificationsEnabled: false}
		require.NoError(t, d.users.Create(borrower))
		bookCopy := &models.Copy{OwnerID: owner.ID, Owner: *owner, Book: models.Book{Title: "Some Book"}, Status: "requested"}
		require.NoError(t, d.copies.Create(bookCopy))
		accepted := &models.LoanRequest{CopyID: bookCopy.ID, BorrowerID: borrower.ID, Status: "accepted"}
		require.NoError(t, d.loanReqs.Create(accepted))

		err := d.workflow.OnAccepted(context.Background(), accepted)

		require.NoError(t, err)
		assert.Equal(t, 1, d.notifs.Count())
	})

	t.Run("OnReturned notifies borrower regardless of the borrower's email preference", func(t *testing.T) {
		d := newWorkflow()
		owner := &models.User{Name: "Owner", Email: "owner@example.com"}
		require.NoError(t, d.users.Create(owner))
		borrower := &models.User{Name: "Borrower", Email: "borrower@example.com", EmailNotificationsEnabled: false}
		require.NoError(t, d.users.Create(borrower))
		bookCopy := &models.Copy{OwnerID: owner.ID, Owner: *owner, Book: models.Book{Title: "Some Book"}, Status: "loaned"}
		require.NoError(t, d.copies.Create(bookCopy))
		lr := &models.LoanRequest{CopyID: bookCopy.ID, BorrowerID: borrower.ID, Status: "returned"}
		require.NoError(t, d.loanReqs.Create(lr))

		err := d.workflow.OnReturned(context.Background(), lr)

		require.NoError(t, err)
		assert.Equal(t, 1, d.notifs.Count())
	})
}

func mustFindByRecipient(t *testing.T, notifs *repotest.NotificationRepository, recipientID uint) []models.Notification {
	t.Helper()
	result, err := notifs.FindByRecipient(recipientID, false)
	require.NoError(t, err)
	return result
}
