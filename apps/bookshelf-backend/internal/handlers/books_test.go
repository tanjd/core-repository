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
