package gorm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
)

func TestCopyRepository_ListOwnedBookIDs(t *testing.T) {
	db := openTestDB(t)
	copies := NewCopyRepository(db)

	owner := models.User{Name: "Owner", Email: "owner@example.com"}
	other := models.User{Name: "Other", Email: "other@example.com"}
	require.NoError(t, db.Create(&owner).Error)
	require.NoError(t, db.Create(&other).Error)

	bookA := models.Book{Title: "A", Author: "A"}
	bookB := models.Book{Title: "B", Author: "B"}
	require.NoError(t, db.Create(&bookA).Error)
	require.NoError(t, db.Create(&bookB).Error)

	// Two copies of bookA (owner) should collapse to a single ID, one copy
	// of bookB (owner), and one copy of bookA (other) should be excluded.
	require.NoError(t, db.Create(&models.Copy{BookID: bookA.ID, OwnerID: owner.ID}).Error)
	require.NoError(t, db.Create(&models.Copy{BookID: bookA.ID, OwnerID: owner.ID}).Error)
	require.NoError(t, db.Create(&models.Copy{BookID: bookB.ID, OwnerID: owner.ID}).Error)
	require.NoError(t, db.Create(&models.Copy{BookID: bookA.ID, OwnerID: other.ID}).Error)

	ids, err := copies.ListOwnedBookIDs(owner.ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []uint{bookA.ID, bookB.ID}, ids)
}
