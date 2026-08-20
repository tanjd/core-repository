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
