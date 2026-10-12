package gorm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

func TestToReadRepository_Add(t *testing.T) {
	db := openTestDB(t)
	book := models.Book{Title: "T1", Author: "A"}
	require.NoError(t, db.Create(&book).Error)
	user := models.User{Name: "U1", Email: "u1@example.com"}
	require.NoError(t, db.Create(&user).Error)
	toReads := NewToReadRepository(db)

	t.Run("succeeds for a new (book, user) pair", func(t *testing.T) {
		require.NoError(t, toReads.Add(book.ID, user.ID))
		saved, err := toReads.IsToRead(book.ID, user.ID)
		require.NoError(t, err)
		assert.True(t, saved)
	})

	t.Run("returns ErrConflict for the same pair twice", func(t *testing.T) {
		err := toReads.Add(book.ID, user.ID)
		require.ErrorIs(t, err, repository.ErrConflict)
	})
}

func TestToReadRepository_Remove(t *testing.T) {
	db := openTestDB(t)
	book := models.Book{Title: "T1", Author: "A"}
	require.NoError(t, db.Create(&book).Error)
	user := models.User{Name: "U1", Email: "u1@example.com"}
	require.NoError(t, db.Create(&user).Error)
	toReads := NewToReadRepository(db)
	require.NoError(t, toReads.Add(book.ID, user.ID))

	t.Run("succeeds for an existing row", func(t *testing.T) {
		require.NoError(t, toReads.Remove(book.ID, user.ID))
		saved, err := toReads.IsToRead(book.ID, user.ID)
		require.NoError(t, err)
		assert.False(t, saved)
	})

	t.Run("returns ErrNotFound for an absent row", func(t *testing.T) {
		err := toReads.Remove(book.ID, user.ID)
		require.ErrorIs(t, err, repository.ErrNotFound)
	})
}

func TestToReadRepository_HasToReadBatch(t *testing.T) {
	db := openTestDB(t)
	book1 := models.Book{Title: "T1", Author: "A"}
	require.NoError(t, db.Create(&book1).Error)
	book2 := models.Book{Title: "T2", Author: "A"}
	require.NoError(t, db.Create(&book2).Error)
	u1 := models.User{Name: "U1", Email: "u1@example.com"}
	require.NoError(t, db.Create(&u1).Error)
	toReads := NewToReadRepository(db)
	require.NoError(t, toReads.Add(book1.ID, u1.ID))

	got, err := toReads.HasToReadBatch(u1.ID, []uint{book1.ID, book2.ID})
	require.NoError(t, err)
	assert.True(t, got[book1.ID])
	assert.False(t, got[book2.ID])
}

func TestToReadRepository_ListByUserPaginated(t *testing.T) {
	db := openTestDB(t)
	bookA := models.Book{Title: "Apple", Author: "A"}
	bookB := models.Book{Title: "Banana", Author: "B"}
	require.NoError(t, db.Create(&bookA).Error)
	require.NoError(t, db.Create(&bookB).Error)
	user := models.User{Name: "U1", Email: "u1@example.com"}
	other := models.User{Name: "U2", Email: "u2@example.com"}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&other).Error)
	toReads := NewToReadRepository(db)

	require.NoError(t, toReads.Add(bookA.ID, user.ID))
	require.NoError(t, toReads.Add(bookB.ID, user.ID))
	require.NoError(t, toReads.Add(bookA.ID, other.ID))

	result, err := toReads.ListByUserPaginated(user.ID, 1, 20)
	require.NoError(t, err)
	assert.EqualValues(t, 2, result.Total)
	require.Len(t, result.Items, 2)
	// newest-saved first
	assert.Equal(t, bookB.ID, result.Items[0].ID)
	assert.Equal(t, bookA.ID, result.Items[1].ID)
}
