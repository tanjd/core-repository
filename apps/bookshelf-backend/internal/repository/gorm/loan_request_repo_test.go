package gorm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// openTestDB opens a fresh in-memory SQLite database and auto-migrates the
// models these integration tests need. A single shared connection is used
// (rather than a pool) since ":memory:" databases are connection-scoped —
// a second connection would see an empty database.
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gormsqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&models.User{}, &models.Book{}, &models.Copy{},
		&models.LoanRequest{}, &models.Notification{}, &models.WaitlistEntry{},
		&models.Announcement{}, &models.WishlistRequest{},
	))
	return db
}

func TestLoanRequestRepository_CreateAndMarkRequested(t *testing.T) {
	t.Run("succeeds and marks the copy requested when available", func(t *testing.T) {
		db := openTestDB(t)
		copies := NewCopyRepository(db)
		loanReqs := NewLoanRequestRepository(db)

		owner := models.User{Name: "Owner", Email: "owner@example.com"}
		require.NoError(t, db.Create(&owner).Error)
		book := models.Book{Title: "Some Book", Author: "Someone"}
		require.NoError(t, db.Create(&book).Error)
		bookCopy := models.Copy{BookID: book.ID, OwnerID: owner.ID, Status: "available"}
		require.NoError(t, copies.Create(&bookCopy))

		lr := &models.LoanRequest{CopyID: bookCopy.ID, BorrowerID: owner.ID, Status: "pending"}
		err := loanReqs.CreateAndMarkRequested(lr)

		require.NoError(t, err)
		assert.NotZero(t, lr.ID)

		updated, err := copies.GetByID(bookCopy.ID)
		require.NoError(t, err)
		assert.Equal(t, "requested", updated.Status)
	})

	t.Run("rejects with ErrConflict when the copy is no longer available", func(t *testing.T) {
		// This is the TOCTOU guard the transaction exists for: the copy's
		// status changed between an earlier availability check and this call.
		db := openTestDB(t)
		copies := NewCopyRepository(db)
		loanReqs := NewLoanRequestRepository(db)

		owner := models.User{Name: "Owner", Email: "owner2@example.com"}
		require.NoError(t, db.Create(&owner).Error)
		book := models.Book{Title: "Some Book", Author: "Someone"}
		require.NoError(t, db.Create(&book).Error)
		bookCopy := models.Copy{BookID: book.ID, OwnerID: owner.ID, Status: "requested"}
		require.NoError(t, copies.Create(&bookCopy))

		lr := &models.LoanRequest{CopyID: bookCopy.ID, BorrowerID: owner.ID, Status: "pending"}
		err := loanReqs.CreateAndMarkRequested(lr)

		require.ErrorIs(t, err, repository.ErrConflict)

		results, listErr := loanReqs.ListByCopyID(bookCopy.ID)
		require.NoError(t, listErr)
		assert.Empty(t, results, "no loan request should have been created")
	})
}

func TestLoanRequestRepository_RejectCompetingAndUpdateCopy(t *testing.T) {
	db := openTestDB(t)
	copies := NewCopyRepository(db)
	loanReqs := NewLoanRequestRepository(db)
	notifs := NewNotificationRepository(db)

	owner := models.User{Name: "Owner", Email: "owner@example.com"}
	require.NoError(t, db.Create(&owner).Error)
	borrowerA := models.User{Name: "A", Email: "a@example.com"}
	require.NoError(t, db.Create(&borrowerA).Error)
	borrowerB := models.User{Name: "B", Email: "b@example.com"}
	require.NoError(t, db.Create(&borrowerB).Error)
	book := models.Book{Title: "Some Book", Author: "Someone"}
	require.NoError(t, db.Create(&book).Error)
	bookCopy := models.Copy{BookID: book.ID, OwnerID: owner.ID, Status: "requested"}
	require.NoError(t, copies.Create(&bookCopy))

	accepted := models.LoanRequest{CopyID: bookCopy.ID, BorrowerID: borrowerA.ID, Status: "pending"}
	require.NoError(t, loanReqs.Create(&accepted))
	competing := models.LoanRequest{CopyID: bookCopy.ID, BorrowerID: borrowerB.ID, Status: "pending"}
	require.NoError(t, loanReqs.Create(&competing))
	alreadyRejected := models.LoanRequest{CopyID: bookCopy.ID, BorrowerID: borrowerB.ID, Status: "rejected"}
	require.NoError(t, loanReqs.Create(&alreadyRejected))

	err := loanReqs.RejectCompetingAndUpdateCopy(bookCopy.ID, accepted.ID)

	require.NoError(t, err)

	reloadedCompeting, err := loanReqs.GetByID(competing.ID)
	require.NoError(t, err)
	assert.Equal(t, "rejected", reloadedCompeting.Status)

	reloadedAccepted, err := loanReqs.GetByID(accepted.ID)
	require.NoError(t, err)
	assert.Equal(t, "pending", reloadedAccepted.Status, "the accepted request itself is not touched by this method")

	untouchedAlreadyRejected, err := loanReqs.GetByID(alreadyRejected.ID)
	require.NoError(t, err)
	assert.Equal(t, "rejected", untouchedAlreadyRejected.Status)

	updatedCopy, err := copies.GetByID(bookCopy.ID)
	require.NoError(t, err)
	assert.Equal(t, "loaned", updatedCopy.Status)

	notifications, err := notifs.FindByRecipient(borrowerB.ID, false)
	require.NoError(t, err)
	require.Len(t, notifications, 1, "only the newly-rejected competing request should generate a notification")
	assert.Equal(t, "request_rejected", notifications[0].Type)
}

func TestLoanRequestRepository_ListByBorrowerIDPaginated_FiltersByStatus(t *testing.T) {
	db := openTestDB(t)
	copies := NewCopyRepository(db)
	loanReqs := NewLoanRequestRepository(db)

	owner := models.User{Name: "Owner", Email: "owner-lbip@example.com"}
	require.NoError(t, db.Create(&owner).Error)
	borrower := models.User{Name: "Borrower", Email: "borrower-lbip@example.com"}
	require.NoError(t, db.Create(&borrower).Error)
	book := models.Book{Title: "Some Book", Author: "Someone"}
	require.NoError(t, db.Create(&book).Error)

	statuses := []string{"pending", "accepted", "rejected", "cancelled", "returned"}
	for _, status := range statuses {
		bookCopy := models.Copy{BookID: book.ID, OwnerID: owner.ID, Status: "available"}
		require.NoError(t, copies.Create(&bookCopy))
		require.NoError(t, loanReqs.Create(&models.LoanRequest{CopyID: bookCopy.ID, BorrowerID: borrower.ID, Status: status}))
	}

	t.Run("nil statuses returns every status", func(t *testing.T) {
		result, err := loanReqs.ListByBorrowerIDPaginated(borrower.ID, nil, 1, 20)
		require.NoError(t, err)
		assert.Equal(t, int64(5), result.Total)
		assert.Len(t, result.Items, 5)
	})

	t.Run("current view returns only pending and accepted", func(t *testing.T) {
		result, err := loanReqs.ListByBorrowerIDPaginated(borrower.ID, []string{"pending", "accepted"}, 1, 20)
		require.NoError(t, err)
		assert.Equal(t, int64(2), result.Total)
		assert.Equal(t, 1, result.TotalPages)
		for _, item := range result.Items {
			assert.Contains(t, []string{"pending", "accepted"}, item.Status)
		}
	})

	t.Run("history view returns only returned, rejected, and cancelled", func(t *testing.T) {
		result, err := loanReqs.ListByBorrowerIDPaginated(borrower.ID, []string{"returned", "rejected", "cancelled"}, 1, 20)
		require.NoError(t, err)
		assert.Equal(t, int64(3), result.Total)
		for _, item := range result.Items {
			assert.Contains(t, []string{"returned", "rejected", "cancelled"}, item.Status)
		}
	})
}

func TestLoanRequestRepository_ListActiveByBorrowerID(t *testing.T) {
	db := openTestDB(t)
	copies := NewCopyRepository(db)
	loanReqs := NewLoanRequestRepository(db)

	owner := models.User{Name: "Owner", Email: "owner-active@example.com"}
	require.NoError(t, db.Create(&owner).Error)
	borrower := models.User{Name: "Borrower", Email: "borrower-active@example.com"}
	require.NoError(t, db.Create(&borrower).Error)
	book := models.Book{Title: "Some Book", Author: "Someone"}
	require.NoError(t, db.Create(&book).Error)

	newCopy := func() models.Copy {
		c := models.Copy{BookID: book.ID, OwnerID: owner.ID, Status: "loaned"}
		require.NoError(t, copies.Create(&c))
		return c
	}

	dueSoon := time.Now().Add(3 * 24 * time.Hour)
	withDueDate := newCopy()
	require.NoError(t, loanReqs.Create(&models.LoanRequest{
		CopyID: withDueDate.ID, BorrowerID: borrower.ID, Status: "accepted", ExpectedReturnDate: &dueSoon,
	}))

	noDueDate := newCopy()
	require.NoError(t, loanReqs.Create(&models.LoanRequest{
		CopyID: noDueDate.ID, BorrowerID: borrower.ID, Status: "accepted",
	}))

	pendingCopy := newCopy()
	require.NoError(t, loanReqs.Create(&models.LoanRequest{CopyID: pendingCopy.ID, BorrowerID: borrower.ID, Status: "pending"}))

	returnedCopy := newCopy()
	require.NoError(t, loanReqs.Create(&models.LoanRequest{CopyID: returnedCopy.ID, BorrowerID: borrower.ID, Status: "returned"}))

	active, err := loanReqs.ListActiveByBorrowerID(borrower.ID)
	require.NoError(t, err)
	require.Len(t, active, 2, "only accepted loans are active")
	assert.Equal(t, withDueDate.ID, active[0].CopyID, "the loan with a due date sorts before the one with no due date")
	assert.Equal(t, noDueDate.ID, active[1].CopyID)
}
