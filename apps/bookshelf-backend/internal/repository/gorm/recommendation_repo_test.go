package gorm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

func TestRecommendationRepository_Create(t *testing.T) {
	db := openTestDB(t)
	book := models.Book{Title: "T1", Author: "A"}
	require.NoError(t, db.Create(&book).Error)
	user := models.User{Name: "U1", Email: "u1@example.com"}
	require.NoError(t, db.Create(&user).Error)
	recs := NewRecommendationRepository(db)

	t.Run("succeeds for a new (book, recommender) pair", func(t *testing.T) {
		require.NoError(t, recs.Create(book.ID, user.ID))
		got, err := recs.FindByBookAndRecommender(book.ID, user.ID)
		require.NoError(t, err)
		assert.Equal(t, book.ID, got.BookID)
		assert.Equal(t, user.ID, got.RecommenderID)
	})

	t.Run("returns ErrConflict for the same pair twice", func(t *testing.T) {
		err := recs.Create(book.ID, user.ID)
		require.ErrorIs(t, err, repository.ErrConflict)
	})
}

func TestRecommendationRepository_Delete(t *testing.T) {
	db := openTestDB(t)
	book := models.Book{Title: "T1", Author: "A"}
	require.NoError(t, db.Create(&book).Error)
	user := models.User{Name: "U1", Email: "u1@example.com"}
	require.NoError(t, db.Create(&user).Error)
	recs := NewRecommendationRepository(db)
	require.NoError(t, recs.Create(book.ID, user.ID))

	t.Run("succeeds for an existing row", func(t *testing.T) {
		require.NoError(t, recs.Delete(book.ID, user.ID))
		_, err := recs.FindByBookAndRecommender(book.ID, user.ID)
		require.ErrorIs(t, err, repository.ErrNotFound)
	})

	t.Run("is idempotent for an absent row", func(t *testing.T) {
		require.NoError(t, recs.Delete(book.ID, user.ID))
	})
}

func TestRecommendationRepository_FindByBookAndRecommender_NotFound(t *testing.T) {
	db := openTestDB(t)
	recs := NewRecommendationRepository(db)
	_, err := recs.FindByBookAndRecommender(999, 999)
	require.ErrorIs(t, err, repository.ErrNotFound)
}

func TestRecommendationRepository_ListByBookID(t *testing.T) {
	db := openTestDB(t)
	book := models.Book{Title: "T1", Author: "A"}
	require.NoError(t, db.Create(&book).Error)
	other := models.Book{Title: "T2", Author: "A"}
	require.NoError(t, db.Create(&other).Error)
	u1 := models.User{Name: "U1", Email: "u1@example.com"}
	require.NoError(t, db.Create(&u1).Error)
	u2 := models.User{Name: "U2", Email: "u2@example.com"}
	require.NoError(t, db.Create(&u2).Error)
	recs := NewRecommendationRepository(db)

	require.NoError(t, recs.Create(book.ID, u1.ID))
	require.NoError(t, recs.Create(book.ID, u2.ID))
	require.NoError(t, recs.Create(other.ID, u1.ID))

	list, err := recs.ListByBookID(book.ID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	// newest-first
	assert.Equal(t, u2.ID, list[0].RecommenderID)
	assert.Equal(t, "U2", list[0].Recommender.Name)
	assert.Equal(t, u1.ID, list[1].RecommenderID)
	assert.Equal(t, "U1", list[1].Recommender.Name)
}

func TestRecommendationRepository_CountByBookBatch(t *testing.T) {
	db := openTestDB(t)
	book1 := models.Book{Title: "T1", Author: "A"}
	require.NoError(t, db.Create(&book1).Error)
	book2 := models.Book{Title: "T2", Author: "A"}
	require.NoError(t, db.Create(&book2).Error)
	u1 := models.User{Name: "U1", Email: "u1@example.com"}
	require.NoError(t, db.Create(&u1).Error)
	u2 := models.User{Name: "U2", Email: "u2@example.com"}
	require.NoError(t, db.Create(&u2).Error)
	recs := NewRecommendationRepository(db)
	require.NoError(t, recs.Create(book1.ID, u1.ID))
	require.NoError(t, recs.Create(book1.ID, u2.ID))

	counts, err := recs.CountByBookBatch([]uint{book1.ID, book2.ID})
	require.NoError(t, err)
	assert.EqualValues(t, 2, counts[book1.ID])
	assert.EqualValues(t, 0, counts[book2.ID], "books with no rows are absent from the map, which a missing-key lookup defaults to 0")
}

func TestRecommendationRepository_HasRecommendedBatch(t *testing.T) {
	db := openTestDB(t)
	book1 := models.Book{Title: "T1", Author: "A"}
	require.NoError(t, db.Create(&book1).Error)
	book2 := models.Book{Title: "T2", Author: "A"}
	require.NoError(t, db.Create(&book2).Error)
	u1 := models.User{Name: "U1", Email: "u1@example.com"}
	require.NoError(t, db.Create(&u1).Error)
	recs := NewRecommendationRepository(db)
	require.NoError(t, recs.Create(book1.ID, u1.ID))

	got, err := recs.HasRecommendedBatch(u1.ID, []uint{book1.ID, book2.ID})
	require.NoError(t, err)
	assert.True(t, got[book1.ID])
	assert.False(t, got[book2.ID])
}

func TestRecommendationRepository_DeleteByBookID(t *testing.T) {
	db := openTestDB(t)
	book := models.Book{Title: "T1", Author: "A"}
	require.NoError(t, db.Create(&book).Error)
	u1 := models.User{Name: "U1", Email: "u1@example.com"}
	require.NoError(t, db.Create(&u1).Error)
	recs := NewRecommendationRepository(db)
	require.NoError(t, recs.Create(book.ID, u1.ID))

	require.NoError(t, recs.DeleteByBookID(book.ID))
	list, err := recs.ListByBookID(book.ID)
	require.NoError(t, err)
	assert.Empty(t, list)

	// idempotent for a book with no rows
	require.NoError(t, recs.DeleteByBookID(book.ID))
}

func TestRecommendationRepository_DeleteByRecommenderID(t *testing.T) {
	db := openTestDB(t)
	book := models.Book{Title: "T1", Author: "A"}
	require.NoError(t, db.Create(&book).Error)
	u1 := models.User{Name: "U1", Email: "u1@example.com"}
	require.NoError(t, db.Create(&u1).Error)
	recs := NewRecommendationRepository(db)
	require.NoError(t, recs.Create(book.ID, u1.ID))

	require.NoError(t, recs.DeleteByRecommenderID(u1.ID))
	_, err := recs.FindByBookAndRecommender(book.ID, u1.ID)
	require.ErrorIs(t, err, repository.ErrNotFound)

	// idempotent for a user with no rows
	require.NoError(t, recs.DeleteByRecommenderID(u1.ID))
}
