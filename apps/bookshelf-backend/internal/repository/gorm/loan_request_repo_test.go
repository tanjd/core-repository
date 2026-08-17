package gorm

import (
	"testing"

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
