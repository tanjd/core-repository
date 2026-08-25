package gorm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

func TestBookRepository_FindByISBN(t *testing.T) {
	db := openTestDB(t)
	books := NewBookRepository(db)

	require.NoError(t, books.Create(&models.Book{Title: "T1", Author: "A", ISBN: "9780000000001"}))

	t.Run("found", func(t *testing.T) {
		book, err := books.FindByISBN("9780000000001")
		require.NoError(t, err)
		assert.Equal(t, "T1", book.Title)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := books.FindByISBN("nonexistent")
		assert.ErrorIs(t, err, repository.ErrNotFound)
	})

	t.Run("ignores empty-string collisions", func(t *testing.T) {
		require.NoError(t, books.Create(&models.Book{Title: "T2", Author: "A"}))
		require.NoError(t, books.Create(&models.Book{Title: "T3", Author: "A"}))

		_, err := books.FindByISBN("")
		assert.ErrorIs(t, err, repository.ErrNotFound)
	})
}

func TestBookRepository_ListPaginated_ExcludesBooksWithNoCopies(t *testing.T) {
	db := openTestDB(t)
	books := NewBookRepository(db)
	copies := NewCopyRepository(db)

	owner := models.User{Name: "Owner", Email: "owner@example.com"}
	require.NoError(t, db.Create(&owner).Error)

	withCopy := models.Book{Title: "Has A Copy", Author: "A"}
	require.NoError(t, books.Create(&withCopy))
	require.NoError(t, copies.Create(&models.Copy{BookID: withCopy.ID, OwnerID: owner.ID, Condition: "good", Status: "available"}))

	// Simulates a book whose last copy was just removed by its owner.
	noCopy := models.Book{Title: "No Copies Left", Author: "A"}
	require.NoError(t, books.Create(&noCopy))

	result, err := books.ListPaginated("", "title", false, 1, 20)
	require.NoError(t, err)

	var titles []string
	for _, b := range result.Items {
		titles = append(titles, b.Title)
	}
	assert.Contains(t, titles, "Has A Copy")
	assert.NotContains(t, titles, "No Copies Left")
	assert.EqualValues(t, 1, result.Total)
}

func TestBookRepository_ListPaginated_RelevanceSort(t *testing.T) {
	db := openTestDB(t)
	books := NewBookRepository(db)
	copies := NewCopyRepository(db)

	owner := models.User{Name: "Owner", Email: "owner@example.com"}
	require.NoError(t, db.Create(&owner).Error)

	for _, title := range []string{"The Harried Reader", "Harry Potter", "A History of Time"} {
		book := models.Book{Title: title, Author: "A"}
		require.NoError(t, books.Create(&book))
		require.NoError(t, copies.Create(&models.Copy{BookID: book.ID, OwnerID: owner.ID, Condition: "good", Status: "available"}))
	}

	result, err := books.ListPaginated("harr", "relevance", false, 1, 20)
	require.NoError(t, err)

	var titles []string
	for _, b := range result.Items {
		titles = append(titles, b.Title)
	}
	// "Harry Potter" is a prefix match and should rank above the
	// substring-only match "The Harried Reader"; "A History of Time"
	// doesn't match "harr" at all and is excluded by the search filter.
	require.Equal(t, []string{"Harry Potter", "The Harried Reader"}, titles)
}

func TestBookRepository_Delete(t *testing.T) {
	db := openTestDB(t)
	books := NewBookRepository(db)

	book := models.Book{Title: "T1", Author: "A"}
	require.NoError(t, books.Create(&book))

	require.NoError(t, books.Delete(&book))

	_, err := books.GetByIDWithCopies(book.ID)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

// seedBookWithAvailableCopy creates a Book with one available Copy owned by
// owner, and returns the created book plus the copy's ID — used by the
// community-reading-activity tests below.
func seedBookWithAvailableCopy(t *testing.T, books *BookRepository, copies *CopyRepository, owner models.User, title string) (models.Book, uint) {
	t.Helper()
	book := models.Book{Title: title, Author: "A"}
	require.NoError(t, books.Create(&book))
	copyRec := models.Copy{BookID: book.ID, OwnerID: owner.ID, Condition: "good", Status: "available"}
	require.NoError(t, copies.Create(&copyRec))
	return book, copyRec.ID
}

func TestBookRepository_CountBorrowsBatch(t *testing.T) {
	db := openTestDB(t)
	books := NewBookRepository(db)
	copies := NewCopyRepository(db)

	owner := models.User{Name: "Owner", Email: "owner@example.com"}
	require.NoError(t, db.Create(&owner).Error)
	borrower := models.User{Name: "Borrower", Email: "borrower@example.com"}
	require.NoError(t, db.Create(&borrower).Error)

	// A book with no completed loans should not appear in the map at all.
	zero, _ := seedBookWithAvailableCopy(t, books, copies, owner, "Zero Loans")

	// A book with a mix of statuses — only accepted/returned count, so the
	// pending + rejected + cancelled requests here must be ignored.
	one, oneCopy := seedBookWithAvailableCopy(t, books, copies, owner, "One Loan")
	require.NoError(t, db.Create(&models.LoanRequest{CopyID: oneCopy, BorrowerID: borrower.ID, Status: "accepted"}).Error)
	require.NoError(t, db.Create(&models.LoanRequest{CopyID: oneCopy, BorrowerID: borrower.ID, Status: "pending"}).Error)
	require.NoError(t, db.Create(&models.LoanRequest{CopyID: oneCopy, BorrowerID: borrower.ID, Status: "rejected"}).Error)
	require.NoError(t, db.Create(&models.LoanRequest{CopyID: oneCopy, BorrowerID: borrower.ID, Status: "cancelled"}).Error)

	// A book with copies from two owners — the count spans both copies.
	two, twoCopyA := seedBookWithAvailableCopy(t, books, copies, owner, "Two Loans")
	twoCopyB := models.Copy{BookID: two.ID, OwnerID: owner.ID, Condition: "good", Status: "available"}
	require.NoError(t, copies.Create(&twoCopyB))
	require.NoError(t, db.Create(&models.LoanRequest{CopyID: twoCopyA, BorrowerID: borrower.ID, Status: "returned"}).Error)
	require.NoError(t, db.Create(&models.LoanRequest{CopyID: twoCopyB.ID, BorrowerID: borrower.ID, Status: "accepted"}).Error)

	counts, err := books.CountBorrowsBatch([]uint{zero.ID, one.ID, two.ID})
	require.NoError(t, err)

	// zero is intentionally absent — callers must treat a missing key as 0,
	// same convention as CountAvailableCopiesBatch.
	assert.NotContains(t, counts, zero.ID)
	assert.EqualValues(t, 1, counts[one.ID])
	assert.EqualValues(t, 2, counts[two.ID])
}

func TestBookRepository_CountBorrowsBatch_EmptyInput(t *testing.T) {
	db := openTestDB(t)
	books := NewBookRepository(db)

	counts, err := books.CountBorrowsBatch(nil)
	require.NoError(t, err)
	assert.Empty(t, counts)
}

func TestBookRepository_CountWaitlistBatch(t *testing.T) {
	db := openTestDB(t)
	books := NewBookRepository(db)
	copies := NewCopyRepository(db)
	waitlist := NewWaitlistRepository(db)

	owner := models.User{Name: "Owner", Email: "owner@example.com"}
	require.NoError(t, db.Create(&owner).Error)
	u1 := models.User{Name: "U1", Email: "u1@example.com"}
	require.NoError(t, db.Create(&u1).Error)
	u2 := models.User{Name: "U2", Email: "u2@example.com"}
	require.NoError(t, db.Create(&u2).Error)

	zero, _ := seedBookWithAvailableCopy(t, books, copies, owner, "Zero Waiters")

	one, oneCopy := seedBookWithAvailableCopy(t, books, copies, owner, "One Waiter")
	require.NoError(t, waitlist.Add(oneCopy, u1.ID))

	// A book whose two copies each carry a waiter — the count spans both.
	two, twoCopyA := seedBookWithAvailableCopy(t, books, copies, owner, "Two Waiters")
	twoCopyB := models.Copy{BookID: two.ID, OwnerID: owner.ID, Condition: "good", Status: "available"}
	require.NoError(t, copies.Create(&twoCopyB))
	require.NoError(t, waitlist.Add(twoCopyA, u1.ID))
	require.NoError(t, waitlist.Add(twoCopyB.ID, u2.ID))

	counts, err := books.CountWaitlistBatch([]uint{zero.ID, one.ID, two.ID})
	require.NoError(t, err)

	assert.NotContains(t, counts, zero.ID)
	assert.EqualValues(t, 1, counts[one.ID])
	assert.EqualValues(t, 2, counts[two.ID])
}

func TestBookRepository_ListPaginated_PopularSort(t *testing.T) {
	db := openTestDB(t)
	books := NewBookRepository(db)
	copies := NewCopyRepository(db)

	owner := models.User{Name: "Owner", Email: "owner@example.com"}
	require.NoError(t, db.Create(&owner).Error)
	borrower := models.User{Name: "Borrower", Email: "borrower@example.com"}
	require.NoError(t, db.Create(&borrower).Error)

	// Ordering by title alone would put "Alpha" first — sort=popular should
	// override that with borrow count. Distinct-borrower isn't required for
	// the count; we're just measuring completed loan volume.
	alpha, alphaCopy := seedBookWithAvailableCopy(t, books, copies, owner, "Alpha (never borrowed)")
	beta, betaCopy := seedBookWithAvailableCopy(t, books, copies, owner, "Beta (borrowed twice)")
	gamma, gammaCopy := seedBookWithAvailableCopy(t, books, copies, owner, "Gamma (borrowed once)")

	require.NoError(t, db.Create(&models.LoanRequest{CopyID: betaCopy, BorrowerID: borrower.ID, Status: "accepted"}).Error)
	require.NoError(t, db.Create(&models.LoanRequest{CopyID: betaCopy, BorrowerID: borrower.ID, Status: "returned"}).Error)
	require.NoError(t, db.Create(&models.LoanRequest{CopyID: gammaCopy, BorrowerID: borrower.ID, Status: "accepted"}).Error)
	// A pending request must not tip the ranking.
	require.NoError(t, db.Create(&models.LoanRequest{CopyID: alphaCopy, BorrowerID: borrower.ID, Status: "pending"}).Error)

	result, err := books.ListPaginated("", "popular", false, 1, 20)
	require.NoError(t, err)

	titles := make([]string, len(result.Items))
	for i, b := range result.Items {
		titles[i] = b.Title
	}
	// Beta (2) > Gamma (1) > Alpha (0, title tiebreaker among zeroes).
	assert.Equal(t, []string{beta.Title, gamma.Title, alpha.Title}, titles)
}

func TestBookRepository_CountCopies(t *testing.T) {
	db := openTestDB(t)
	books := NewBookRepository(db)
	copies := NewCopyRepository(db)

	owner := models.User{Name: "Owner", Email: "owner@example.com"}
	require.NoError(t, db.Create(&owner).Error)

	zero := models.Book{Title: "Zero", Author: "A"}
	require.NoError(t, books.Create(&zero))

	one := models.Book{Title: "One", Author: "A"}
	require.NoError(t, books.Create(&one))
	require.NoError(t, copies.Create(&models.Copy{BookID: one.ID, OwnerID: owner.ID, Status: "available"}))

	two := models.Book{Title: "Two", Author: "A"}
	require.NoError(t, books.Create(&two))
	require.NoError(t, copies.Create(&models.Copy{BookID: two.ID, OwnerID: owner.ID, Status: "available"}))
	require.NoError(t, copies.Create(&models.Copy{BookID: two.ID, OwnerID: owner.ID, Status: "loaned"}))

	count, err := books.CountCopies(zero.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 0, count)

	count, err = books.CountCopies(one.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)

	count, err = books.CountCopies(two.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)
}
