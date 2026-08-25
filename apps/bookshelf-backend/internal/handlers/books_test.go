package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
)

func newBookHandler() (*BookHandler, *repotest.BookRepository) {
	books := repotest.NewBookRepository()
	users := repotest.NewUserRepository()
	return NewBookHandler(books, users, "", nil), books
}

// bookHandlerFixture wires up the full set of fake repositories needed by
// the reading-activity code paths (books + copies + loan requests +
// waitlist) so tests can assert on borrow/waitlist counts flowing through
// toBookResponse / toBooksResponse. Constructed here rather than folded
// into newBookHandler so the existing createBook tests, which need none of
// these extras, stay untouched.
type bookHandlerFixture struct {
	handler  *BookHandler
	books    *repotest.BookRepository
	copies   *repotest.CopyRepository
	loans    *repotest.LoanRequestRepository
	waitlist *repotest.WaitlistRepository
}

func newBookHandlerWithReadingActivity() *bookHandlerFixture {
	books := repotest.NewBookRepository()
	copies := repotest.NewCopyRepository()
	notifs := repotest.NewNotificationRepository()
	users := repotest.NewUserRepository()
	loans := repotest.NewLoanRequestRepository(copies, notifs, users)
	waitlist := repotest.NewWaitlistRepository()
	books.SetCopies(copies)
	books.SetLoans(loans)
	books.SetWaitlist(waitlist)
	return &bookHandlerFixture{
		handler:  NewBookHandler(books, users, "", nil),
		books:    books,
		copies:   copies,
		loans:    loans,
		waitlist: waitlist,
	}
}

func createBookBody(title, olKey, googleBooksID, isbn string) *createBookInput {
	input := &createBookInput{}
	input.Body.Title = title
	input.Body.OLKey = olKey
	input.Body.GoogleBooksID = googleBooksID
	input.Body.ISBN = isbn
	return input
}

func TestCreateBook_DedupsByExistingOLKey(t *testing.T) {
	h, books := newBookHandler()

	first, err := h.createBook(fakeAuthedCtx(t, 1, "user"), createBookBody("T1", "OL1", "", ""))
	require.NoError(t, err)

	second, err := h.createBook(fakeAuthedCtx(t, 1, "user"), createBookBody("T1 Reprint", "OL1", "", ""))
	require.NoError(t, err)

	assert.Equal(t, first.Body.ID, second.Body.ID)
	assert.Equal(t, 1, books.Count())
}

func TestCreateBook_DedupsByExistingGoogleBooksID(t *testing.T) {
	h, books := newBookHandler()

	first, err := h.createBook(fakeAuthedCtx(t, 1, "user"), createBookBody("T1", "", "GB1", ""))
	require.NoError(t, err)

	second, err := h.createBook(fakeAuthedCtx(t, 1, "user"), createBookBody("T1 Reprint", "", "GB1", ""))
	require.NoError(t, err)

	assert.Equal(t, first.Body.ID, second.Body.ID)
	assert.Equal(t, 1, books.Count())
}

func TestCreateBook_DedupsByISBN_WhenNoOLKeyOrGoogleBooksID(t *testing.T) {
	h, books := newBookHandler()

	// Mirrors a BookBrainz-sourced metadata result, which never carries an
	// OLKey or GoogleBooksID.
	first, err := h.createBook(fakeAuthedCtx(t, 1, "user"), createBookBody("T1", "", "", "9780000000001"))
	require.NoError(t, err)

	second, err := h.createBook(fakeAuthedCtx(t, 1, "user"), createBookBody("T1 Reprint", "", "", "9780000000001"))
	require.NoError(t, err)

	assert.Equal(t, first.Body.ID, second.Body.ID)
	assert.Equal(t, 1, books.Count())
}

func TestCreateBook_DoesNotDedupByISBN_WhenOLKeyProvided(t *testing.T) {
	h, books := newBookHandler()

	_, err := h.createBook(fakeAuthedCtx(t, 1, "user"), createBookBody("Omnibus Edition", "", "", "9780000000001"))
	require.NoError(t, err)

	// A distinct edition carrying its own OLKey but sharing the omnibus's
	// ISBN must not be merged into the existing book.
	second, err := h.createBook(fakeAuthedCtx(t, 1, "user"), createBookBody("Single Volume Edition", "OL-single", "", "9780000000001"))
	require.NoError(t, err)

	assert.Equal(t, "Single Volume Edition", second.Body.Title)
	assert.Equal(t, 2, books.Count())
}

func TestCreateBook_BackfillsCover_WhenExistingRowHasNone(t *testing.T) {
	h, books := newBookHandler()

	// Book first added with no cover available (e.g. metadata search found
	// none) — matches the fallback case described in the bug report.
	first, err := h.createBook(fakeAuthedCtx(t, 1, "user"), createBookBody("T1", "OL1", "", ""))
	require.NoError(t, err)
	require.Empty(t, first.Body.CoverURL)

	// Re-adding the same book (e.g. after its last copy was removed, which
	// leaves the keyed Book row in place — see maybeDeleteOrphanedBook) now
	// carries a real cover. The existing row must pick it up, not keep
	// serving the empty fallback forever.
	body := createBookBody("T1", "OL1", "", "")
	body.Body.CoverURL = "https://covers.openlibrary.org/b/id/12345-L.jpg"
	second, err := h.createBook(fakeAuthedCtx(t, 1, "user"), body)
	require.NoError(t, err)

	assert.Equal(t, first.Body.ID, second.Body.ID)
	assert.Equal(t, 1, books.Count())
	assert.Equal(t, "https://covers.openlibrary.org/b/id/12345-L.jpg", second.Body.CoverURL)
}

func TestCreateBook_KeepsExistingCover_WhenAlreadySet(t *testing.T) {
	h, books := newBookHandler()

	first := createBookBody("T1", "OL1", "", "")
	first.Body.CoverURL = "https://covers.openlibrary.org/b/id/11111-L.jpg"
	_, err := h.createBook(fakeAuthedCtx(t, 1, "user"), first)
	require.NoError(t, err)

	second := createBookBody("T1 Reprint", "OL1", "", "")
	second.Body.CoverURL = "https://covers.openlibrary.org/b/id/99999-L.jpg"
	out, err := h.createBook(fakeAuthedCtx(t, 1, "user"), second)
	require.NoError(t, err)

	assert.Equal(t, 1, books.Count())
	assert.Equal(t, "https://covers.openlibrary.org/b/id/11111-L.jpg", out.Body.CoverURL)
}

func TestCreateBook_CreatesNewBook_WhenNoMatch(t *testing.T) {
	h, books := newBookHandler()

	out, err := h.createBook(fakeAuthedCtx(t, 1, "user"), createBookBody("T1", "OL1", "", ""))
	require.NoError(t, err)
	assert.Equal(t, "T1", out.Body.Title)
	assert.Equal(t, 1, books.Count())
}

func TestCreateBook_Unauthenticated(t *testing.T) {
	h, _ := newBookHandler()

	_, err := h.createBook(fakeAuthedCtxNone(), createBookBody("T1", "OL1", "", ""))
	assertStatus(t, err, 401)
}

func TestToBooksResponse_PopulatesBorrowAndWaitlistCounts(t *testing.T) {
	f := newBookHandlerWithReadingActivity()

	borrowed := models.Book{Title: "Borrowed", Author: "A"}
	require.NoError(t, f.books.Create(&borrowed))
	borrowedCopy := models.Copy{BookID: borrowed.ID, OwnerID: 1, Status: "loaned"}
	require.NoError(t, f.copies.Create(&borrowedCopy))
	require.NoError(t, f.loans.Create(&models.LoanRequest{CopyID: borrowedCopy.ID, BorrowerID: 2, Status: "accepted"}))
	require.NoError(t, f.loans.Create(&models.LoanRequest{CopyID: borrowedCopy.ID, BorrowerID: 3, Status: "returned"}))
	// Pending shouldn't count — see the "completed loan" definition in the spec.
	require.NoError(t, f.loans.Create(&models.LoanRequest{CopyID: borrowedCopy.ID, BorrowerID: 4, Status: "pending"}))
	require.NoError(t, f.waitlist.Add(borrowedCopy.ID, 5))

	untouched := models.Book{Title: "Untouched", Author: "A"}
	require.NoError(t, f.books.Create(&untouched))
	untouchedCopy := models.Copy{BookID: untouched.ID, OwnerID: 1, Status: "available"}
	require.NoError(t, f.copies.Create(&untouchedCopy))

	resp, err := f.handler.toBooksResponse([]models.Book{borrowed, untouched})
	require.NoError(t, err)
	require.Len(t, resp, 2)

	byTitle := map[string]bookResponse{}
	for _, r := range resp {
		byTitle[r.Title] = r
	}
	assert.EqualValues(t, 2, byTitle["Borrowed"].BorrowCount)
	assert.EqualValues(t, 1, byTitle["Borrowed"].WaitlistCount)
	assert.EqualValues(t, 0, byTitle["Untouched"].BorrowCount)
	assert.EqualValues(t, 0, byTitle["Untouched"].WaitlistCount)
}

func TestToBookResponse_PopulatesBorrowAndWaitlistCounts(t *testing.T) {
	f := newBookHandlerWithReadingActivity()

	book := models.Book{Title: "Solo", Author: "A"}
	require.NoError(t, f.books.Create(&book))
	copyRec := models.Copy{BookID: book.ID, OwnerID: 1, Status: "loaned"}
	require.NoError(t, f.copies.Create(&copyRec))
	require.NoError(t, f.loans.Create(&models.LoanRequest{CopyID: copyRec.ID, BorrowerID: 2, Status: "returned"}))
	require.NoError(t, f.waitlist.Add(copyRec.ID, 3))
	require.NoError(t, f.waitlist.Add(copyRec.ID, 4))

	resp := f.handler.toBookResponse(book)
	assert.EqualValues(t, 1, resp.BorrowCount)
	assert.EqualValues(t, 2, resp.WaitlistCount)
}
