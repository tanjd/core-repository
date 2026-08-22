package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
)

func newBookHandler() (*BookHandler, *repotest.BookRepository) {
	books := repotest.NewBookRepository()
	users := repotest.NewUserRepository()
	return NewBookHandler(books, users, "", nil), books
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
