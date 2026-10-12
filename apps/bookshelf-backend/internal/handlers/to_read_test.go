package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
)

func newToReadHandler() (*ToReadHandler, *repotest.BookRepository, *repotest.ToReadRepository) {
	fixture := newBookHandlerWithReadingActivity()
	return NewToReadHandler(fixture.books, fixture.toReads, fixture.handler), fixture.books, fixture.toReads
}

func TestAddToRead_Unauthenticated(t *testing.T) {
	h, _, _ := newToReadHandler()
	_, err := h.add(fakeAuthedCtxNone(), &toReadBookInput{ID: 1})
	assertStatus(t, err, 401)
}

func TestAddToRead_BookNotFound(t *testing.T) {
	h, _, _ := newToReadHandler()
	_, err := h.add(fakeAuthedCtx(t, 7, "user"), &toReadBookInput{ID: 999})
	assertStatus(t, err, 404)
}

func TestAddToRead_New(t *testing.T) {
	h, books, toReads := newToReadHandler()
	require.NoError(t, books.Create(&models.Book{Title: "Dune"}))

	_, err := h.add(fakeAuthedCtx(t, 7, "user"), &toReadBookInput{ID: 1})
	require.NoError(t, err)
	saved, err := toReads.IsToRead(1, 7)
	require.NoError(t, err)
	assert.True(t, saved)
}

func TestAddToRead_Duplicate_Conflicts(t *testing.T) {
	h, books, _ := newToReadHandler()
	require.NoError(t, books.Create(&models.Book{Title: "Dune"}))

	_, err := h.add(fakeAuthedCtx(t, 7, "user"), &toReadBookInput{ID: 1})
	require.NoError(t, err)

	_, err = h.add(fakeAuthedCtx(t, 7, "user"), &toReadBookInput{ID: 1})
	assertStatus(t, err, 409)
}

func TestRemoveToRead_Unauthenticated(t *testing.T) {
	h, _, _ := newToReadHandler()
	_, err := h.remove(fakeAuthedCtxNone(), &toReadBookInput{ID: 1})
	assertStatus(t, err, 401)
}

func TestRemoveToRead_NotSaved(t *testing.T) {
	h, _, _ := newToReadHandler()
	_, err := h.remove(fakeAuthedCtx(t, 7, "user"), &toReadBookInput{ID: 1})
	assertStatus(t, err, 404)
}

func TestRemoveToRead_Existing(t *testing.T) {
	h, books, toReads := newToReadHandler()
	require.NoError(t, books.Create(&models.Book{Title: "Dune"}))
	require.NoError(t, toReads.Add(1, 7))

	_, err := h.remove(fakeAuthedCtx(t, 7, "user"), &toReadBookInput{ID: 1})
	require.NoError(t, err)
	saved, err := toReads.IsToRead(1, 7)
	require.NoError(t, err)
	assert.False(t, saved)
}

func TestListMyToRead_Unauthenticated(t *testing.T) {
	h, _, _ := newToReadHandler()
	_, err := h.listMine(fakeAuthedCtxNone(), &listMyToReadInput{})
	assertStatus(t, err, 401)
}

func TestListMyToRead_ReturnsSavedBooksWithYourToReadFlag(t *testing.T) {
	h, books, toReads := newToReadHandler()
	require.NoError(t, books.Create(&models.Book{Title: "Dune"}))
	require.NoError(t, books.Create(&models.Book{Title: "Foundation"}))
	require.NoError(t, toReads.Add(1, 7))

	out, err := h.listMine(fakeAuthedCtx(t, 7, "user"), &listMyToReadInput{})
	require.NoError(t, err)
	require.Len(t, out.Body.Items, 1)
	assert.Equal(t, "Dune", out.Body.Items[0].Title)
	assert.True(t, out.Body.Items[0].YourToRead)
}

func TestToBooksResponse_YourToReadReflectsSavedState(t *testing.T) {
	fixture := newBookHandlerWithReadingActivity()
	require.NoError(t, fixture.books.Create(&models.Book{Title: "Dune"}))
	require.NoError(t, fixture.toReads.Add(1, 7))

	resp, err := fixture.handler.toBooksResponse([]models.Book{{ID: 1, Title: "Dune"}}, 7)
	require.NoError(t, err)
	require.Len(t, resp, 1)
	assert.True(t, resp[0].YourToRead)
}
