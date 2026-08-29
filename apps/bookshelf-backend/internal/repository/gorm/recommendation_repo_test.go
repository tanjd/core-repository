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

func TestRecommendationRepository_ListTopBooks(t *testing.T) {
	db := openTestDB(t)
	recs := NewRecommendationRepository(db)

	bookA := models.Book{Title: "Apple", Author: "A"}
	bookB := models.Book{Title: "Banana", Author: "B"}
	bookC := models.Book{Title: "Cherry", Author: "C"}
	bookD := models.Book{Title: "Date", Author: "D"}
	for _, b := range []*models.Book{&bookA, &bookB, &bookC, &bookD} {
		require.NoError(t, db.Create(b).Error)
	}

	u1 := models.User{Name: "U1", Email: "u1-ltb@example.com"}
	u2 := models.User{Name: "U2", Email: "u2-ltb@example.com"}
	u3 := models.User{Name: "U3", Email: "u3-ltb@example.com"}
	for _, u := range []*models.User{&u1, &u2, &u3} {
		require.NoError(t, db.Create(u).Error)
	}

	// bookB: 3 recs, bookA: 2 recs, bookC: 1 rec, bookD: 0 recs
	require.NoError(t, recs.Create(bookB.ID, u1.ID))
	require.NoError(t, recs.Create(bookB.ID, u2.ID))
	require.NoError(t, recs.Create(bookB.ID, u3.ID))
	require.NoError(t, recs.Create(bookA.ID, u1.ID))
	require.NoError(t, recs.Create(bookA.ID, u2.ID))
	require.NoError(t, recs.Create(bookC.ID, u1.ID))

	t.Run("ordered by count desc, title asc; excludes zero-count", func(t *testing.T) {
		results, err := recs.ListTopBooks(10)
		require.NoError(t, err)
		require.Len(t, results, 3, "bookD (zero recs) excluded")
		assert.Equal(t, bookB.ID, results[0].Book.ID)
		assert.EqualValues(t, 3, results[0].Count)
		assert.Equal(t, bookA.ID, results[1].Book.ID)
		assert.EqualValues(t, 2, results[1].Count)
		assert.Equal(t, bookC.ID, results[2].Book.ID)
		assert.EqualValues(t, 1, results[2].Count)
	})

	t.Run("respects limit", func(t *testing.T) {
		results, err := recs.ListTopBooks(2)
		require.NoError(t, err)
		require.Len(t, results, 2)
		assert.Equal(t, bookB.ID, results[0].Book.ID)
	})

	t.Run("title asc tie-break when counts are equal", func(t *testing.T) {
		// bookA and bookC both have 1 rec in a separate DB where bookA also has 1 and
		// alphabetically Apple < Cherry — create a fresh db with two books tied at 1.
		db2 := openTestDB(t)
		recs2 := NewRecommendationRepository(db2)
		cherry := models.Book{Title: "Cherry", Author: "C"}
		apple := models.Book{Title: "Apple", Author: "A"}
		require.NoError(t, db2.Create(&cherry).Error)
		require.NoError(t, db2.Create(&apple).Error)
		user := models.User{Name: "U", Email: "u-tiebreak@example.com"}
		require.NoError(t, db2.Create(&user).Error)
		require.NoError(t, recs2.Create(cherry.ID, user.ID))
		require.NoError(t, recs2.Create(apple.ID, user.ID))

		results, err := recs2.ListTopBooks(10)
		require.NoError(t, err)
		require.Len(t, results, 2)
		assert.Equal(t, apple.ID, results[0].Book.ID, "Apple before Cherry alphabetically")
	})

	t.Run("empty when no recommendations exist", func(t *testing.T) {
		db3 := openTestDB(t)
		recs3 := NewRecommendationRepository(db3)
		results, err := recs3.ListTopBooks(10)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}
