package gorm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
)

func TestWishlistRequestRepository_ListOpenPaginated(t *testing.T) {
	db := openTestDB(t)
	requester := models.User{Name: "Requester", Email: "req@example.com"}
	require.NoError(t, db.Create(&requester).Error)
	requests := NewWishlistRequestRepository(db)

	require.NoError(t, requests.Create(&models.WishlistRequest{RequesterID: requester.ID, Title: "Open Book", Author: "A", OLKey: "OL1", Status: "open"}))
	require.NoError(t, requests.Create(&models.WishlistRequest{RequesterID: requester.ID, Title: "Fulfilled Book", Author: "A", OLKey: "OL2", Status: "fulfilled"}))
	require.NoError(t, requests.Create(&models.WishlistRequest{RequesterID: requester.ID, Title: "Cancelled Book", Author: "A", OLKey: "OL3", Status: "cancelled"}))

	t.Run("only returns open requests", func(t *testing.T) {
		result, err := requests.ListOpenPaginated("", 1, 20)
		require.NoError(t, err)
		require.Len(t, result.Items, 1)
		assert.Equal(t, "Open Book", result.Items[0].Title)
	})

	t.Run("search filters by title or author", func(t *testing.T) {
		result, err := requests.ListOpenPaginated("Nonexistent", 1, 20)
		require.NoError(t, err)
		assert.Empty(t, result.Items)
	})
}

func TestWishlistRequestRepository_FindOpenByExternalKey(t *testing.T) {
	db := openTestDB(t)
	requester := models.User{Name: "Requester", Email: "req2@example.com"}
	require.NoError(t, db.Create(&requester).Error)
	requests := NewWishlistRequestRepository(db)

	require.NoError(t, requests.Create(&models.WishlistRequest{RequesterID: requester.ID, Title: "T1", Author: "A", OLKey: "OL123", Status: "open"}))
	require.NoError(t, requests.Create(&models.WishlistRequest{RequesterID: requester.ID, Title: "T2", Author: "A", OLKey: "OL123", Status: "fulfilled"}))
	require.NoError(t, requests.Create(&models.WishlistRequest{RequesterID: requester.ID, Title: "T3", Author: "A", GoogleBooksID: "GB1", Status: "open"}))

	t.Run("FindOpenByOLKey only returns open matches", func(t *testing.T) {
		matches, err := requests.FindOpenByOLKey("OL123")
		require.NoError(t, err)
		require.Len(t, matches, 1)
		assert.Equal(t, "T1", matches[0].Title)
	})

	t.Run("FindOpenByGoogleBooksID matches", func(t *testing.T) {
		matches, err := requests.FindOpenByGoogleBooksID("GB1")
		require.NoError(t, err)
		require.Len(t, matches, 1)
		assert.Equal(t, "T3", matches[0].Title)
	})

	t.Run("no match returns empty, not error", func(t *testing.T) {
		matches, err := requests.FindOpenByOLKey("nonexistent")
		require.NoError(t, err)
		assert.Empty(t, matches)
	})
}

func TestWishlistRequestRepository_FindOpenMatch(t *testing.T) {
	db := openTestDB(t)
	requester := models.User{Name: "Requester", Email: "req3@example.com"}
	require.NoError(t, db.Create(&requester).Error)
	requests := NewWishlistRequestRepository(db)

	require.NoError(t, requests.Create(&models.WishlistRequest{RequesterID: requester.ID, Title: "Earlier", Author: "A", OLKey: "OL1", Status: "open"}))
	require.NoError(t, requests.Create(&models.WishlistRequest{RequesterID: requester.ID, Title: "Later", Author: "A", OLKey: "OL1", Status: "open"}))
	require.NoError(t, requests.Create(&models.WishlistRequest{RequesterID: requester.ID, Title: "Fulfilled", Author: "A", ISBN: "ISBN1", Status: "fulfilled"}))
	require.NoError(t, requests.Create(&models.WishlistRequest{RequesterID: requester.ID, Title: "By GB", Author: "A", GoogleBooksID: "GB1", Status: "open"}))

	t.Run("returns the earliest open match by OL key, with requester preloaded", func(t *testing.T) {
		match, err := requests.FindOpenMatch("", "OL1", "")
		require.NoError(t, err)
		require.NotNil(t, match)
		assert.Equal(t, "Earlier", match.Title)
		assert.Equal(t, "Requester", match.Requester.Name)
	})

	t.Run("matches by google books id", func(t *testing.T) {
		match, err := requests.FindOpenMatch("", "", "GB1")
		require.NoError(t, err)
		require.NotNil(t, match)
		assert.Equal(t, "By GB", match.Title)
	})

	t.Run("does not match a fulfilled request", func(t *testing.T) {
		match, err := requests.FindOpenMatch("ISBN1", "", "")
		require.NoError(t, err)
		assert.Nil(t, match)
	})

	t.Run("no keys given returns nil, not an error", func(t *testing.T) {
		match, err := requests.FindOpenMatch("", "", "")
		require.NoError(t, err)
		assert.Nil(t, match)
	})

	t.Run("no match returns nil, not an error", func(t *testing.T) {
		match, err := requests.FindOpenMatch("", "nonexistent", "")
		require.NoError(t, err)
		assert.Nil(t, match)
	})
}

func TestWishlistRequestRepository_ClearFulfilledBookID(t *testing.T) {
	db := openTestDB(t)
	requester := models.User{Name: "Requester", Email: "req4@example.com"}
	require.NoError(t, db.Create(&requester).Error)
	requests := NewWishlistRequestRepository(db)

	book := models.Book{Title: "T1", Author: "A"}
	require.NoError(t, db.Create(&book).Error)
	otherBook := models.Book{Title: "T2", Author: "A"}
	require.NoError(t, db.Create(&otherBook).Error)

	linked := models.WishlistRequest{RequesterID: requester.ID, Title: "Linked", Author: "A", Status: "fulfilled", FulfilledBookID: &book.ID}
	require.NoError(t, requests.Create(&linked))
	linkedOther := models.WishlistRequest{RequesterID: requester.ID, Title: "Linked Other", Author: "A", Status: "fulfilled", FulfilledBookID: &otherBook.ID}
	require.NoError(t, requests.Create(&linkedOther))

	require.NoError(t, requests.ClearFulfilledBookID(book.ID))

	got, err := requests.GetByID(linked.ID)
	require.NoError(t, err)
	assert.Nil(t, got.FulfilledBookID)

	gotOther, err := requests.GetByID(linkedOther.ID)
	require.NoError(t, err)
	require.NotNil(t, gotOther.FulfilledBookID)
	assert.Equal(t, otherBook.ID, *gotOther.FulfilledBookID)
}
