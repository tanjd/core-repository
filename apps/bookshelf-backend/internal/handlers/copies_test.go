package handlers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
)

func newCopyHandler(coversDir string) (*CopyHandler, *repotest.CopyRepository, *repotest.BookRepository, *repotest.WishlistRequestRepository) {
	copies := repotest.NewCopyRepository()
	users := repotest.NewUserRepository()
	notifs := repotest.NewNotificationRepository()
	waitlists := repotest.NewWaitlistRepository()
	admin := repotest.NewAdminRepository()
	books := repotest.NewBookRepository()
	books.SetCopies(copies)
	wishlists := repotest.NewWishlistRequestRepository()
	return NewCopyHandler(copies, users, notifs, waitlists, admin, books, wishlists, coversDir, nil), copies, books, wishlists
}

func TestDeleteCopy_OrphanedKeylessBookCleanup(t *testing.T) {
	t.Run("deletes the book when its last copy was keyless", func(t *testing.T) {
		h, copies, books, _ := newCopyHandler("")

		book := models.Book{Title: "Handwritten Notes", Author: "A"}
		require.NoError(t, books.Create(&book))
		bookCopy := models.Copy{BookID: book.ID, OwnerID: 1, Status: "available"}
		require.NoError(t, copies.Create(&bookCopy))

		_, err := h.deleteCopy(fakeAuthedCtx(t, 1, "user"), &deleteCopyInput{ID: bookCopy.ID})
		require.NoError(t, err)

		_, err = books.GetByIDWithCopies(book.ID)
		require.Error(t, err, "keyless book should be deleted once its last copy is gone")
	})

	t.Run("keeps the book when it carries an OLKey", func(t *testing.T) {
		h, copies, books, _ := newCopyHandler("")

		book := models.Book{Title: "Cataloged Book", Author: "A", OLKey: "OL123"}
		require.NoError(t, books.Create(&book))
		bookCopy := models.Copy{BookID: book.ID, OwnerID: 1, Status: "available"}
		require.NoError(t, copies.Create(&bookCopy))

		_, err := h.deleteCopy(fakeAuthedCtx(t, 1, "user"), &deleteCopyInput{ID: bookCopy.ID})
		require.NoError(t, err)

		got, err := books.GetByIDWithCopies(book.ID)
		require.NoError(t, err, "book with an external key must survive its last copy being deleted")
		assert.Equal(t, "Cataloged Book", got.Title)
	})

	t.Run("keeps a keyless book that still has another copy", func(t *testing.T) {
		h, copies, books, _ := newCopyHandler("")

		book := models.Book{Title: "Shared By Two", Author: "A"}
		require.NoError(t, books.Create(&book))
		first := models.Copy{BookID: book.ID, OwnerID: 1, Status: "available"}
		require.NoError(t, copies.Create(&first))
		second := models.Copy{BookID: book.ID, OwnerID: 1, Status: "available"}
		require.NoError(t, copies.Create(&second))

		_, err := h.deleteCopy(fakeAuthedCtx(t, 1, "user"), &deleteCopyInput{ID: first.ID})
		require.NoError(t, err)

		got, err := books.GetByIDWithCopies(book.ID)
		require.NoError(t, err, "book should not be deleted while a copy remains")
		assert.Equal(t, "Shared By Two", got.Title)
	})

	t.Run("nulls a wishlist request's fulfilled_book_id rather than leaving a dangling reference", func(t *testing.T) {
		h, copies, books, wishlists := newCopyHandler("")

		book := models.Book{Title: "Manually Linked", Author: "A"}
		require.NoError(t, books.Create(&book))
		bookCopy := models.Copy{BookID: book.ID, OwnerID: 1, Status: "available"}
		require.NoError(t, copies.Create(&bookCopy))

		req := models.WishlistRequest{RequesterID: 2, Title: "Manually Linked", Author: "A", Status: "fulfilled", FulfilledBookID: &book.ID}
		require.NoError(t, wishlists.Create(&req))

		_, err := h.deleteCopy(fakeAuthedCtx(t, 1, "user"), &deleteCopyInput{ID: bookCopy.ID})
		require.NoError(t, err)

		_, err = books.GetByIDWithCopies(book.ID)
		require.Error(t, err, "book should still be deleted")

		got, err := wishlists.GetByID(req.ID)
		require.NoError(t, err, "wishlist request row must survive, just with the dangling pointer cleared")
		assert.Nil(t, got.FulfilledBookID)
	})

	t.Run("removes the cached cover file for a deleted orphaned book", func(t *testing.T) {
		coversDir := t.TempDir()
		h, copies, books, _ := newCopyHandler(coversDir)

		coverPath := filepath.Join(coversDir, "abc123.jpg")
		require.NoError(t, os.WriteFile(coverPath, []byte("fake image"), 0o600))

		book := models.Book{Title: "Has A Cover", Author: "A", CoverURL: "/api/covers/abc123.jpg"}
		require.NoError(t, books.Create(&book))
		bookCopy := models.Copy{BookID: book.ID, OwnerID: 1, Status: "available"}
		require.NoError(t, copies.Create(&bookCopy))

		_, err := h.deleteCopy(fakeAuthedCtx(t, 1, "user"), &deleteCopyInput{ID: bookCopy.ID})
		require.NoError(t, err)

		_, statErr := os.Stat(coverPath)
		assert.True(t, os.IsNotExist(statErr), "cached cover file should have been removed")
	})
}
