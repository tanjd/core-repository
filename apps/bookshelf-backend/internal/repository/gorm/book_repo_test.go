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
