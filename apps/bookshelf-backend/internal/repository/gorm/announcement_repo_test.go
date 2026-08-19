package gorm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
)

// Regression: GORM's Create() treats a zero-valued field (false, for bool)
// with a struct "default" tag as unset and substitutes the DB default
// instead — Announcement.Active deliberately has no "default" GORM tag to
// avoid this. This test creates directly against a real GORM/SQLite
// connection (not the in-memory repotest fake, which has no notion of GORM
// tags and can't catch this class of bug).
func TestAnnouncementRepository_CreateHonorsExplicitFalseActive(t *testing.T) {
	db := openTestDB(t)
	announcements := NewAnnouncementRepository(db)

	a := &models.Announcement{Title: "T", Body: "B", Type: "info", Active: false}
	require.NoError(t, announcements.Create(a))

	stored, err := announcements.GetByID(a.ID)
	require.NoError(t, err)
	assert.False(t, stored.Active)
}

func TestAnnouncementRepository_ListActive(t *testing.T) {
	db := openTestDB(t)
	announcements := NewAnnouncementRepository(db)

	require.NoError(t, announcements.Create(&models.Announcement{Title: "Active", Body: "B", Type: "info", Active: true}))
	require.NoError(t, announcements.Create(&models.Announcement{Title: "Inactive", Body: "B", Type: "info", Active: false}))

	items, err := announcements.ListActive()
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Active", items[0].Title)
}
