package gorm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// The pending account details behind an in-flight registration ride on this
// row as an embedded struct (models.PendingRegistrationData). Exercised
// against real GORM/SQLite rather than the repotest fake, which has no
// notion of GORM tags and so can't catch a mis-mapped `gorm:"embedded"` or
// column name.
func TestRegistrationVerificationRepository_UpsertRoundTripsPendingData(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.RegistrationVerification{}))
	repo := NewRegistrationVerificationRepository(db)

	pending := models.PendingRegistrationData{
		PendingName:  "Ada Lovelace",
		PendingEmail: "Ada.Lovelace@Example.com",
		// Not a real hash — this test only cares that the column round-trips.
		PendingPasswordHash: "hash",
		PendingPhone:        "+65 9123 4567",
	}
	require.NoError(t, repo.Upsert("email", "ada.lovelace@example.com", "123456", time.Now().Add(time.Minute), pending))

	stored, err := repo.Find("email", "ada.lovelace@example.com")
	require.NoError(t, err)
	assert.Equal(t, pending, stored.PendingRegistrationData)
}

func TestRegistrationVerificationRepository_UpsertReplacesPendingData(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.RegistrationVerification{}))
	repo := NewRegistrationVerificationRepository(db)

	require.NoError(t, repo.Upsert("email", "ada@example.com", "111111", time.Now().Add(time.Minute),
		models.PendingRegistrationData{PendingName: "First", PendingEmail: "ada@example.com", PendingPasswordHash: "h1", PendingPhone: "+65 1"}))
	// A resend re-submits the whole form: nothing from the superseded
	// attempt may survive, including fields the second attempt left blank.
	require.NoError(t, repo.Upsert("email", "ada@example.com", "222222", time.Now().Add(time.Minute),
		models.PendingRegistrationData{PendingName: "Second", PendingEmail: "ada@example.com", PendingPasswordHash: "h2"}))

	stored, err := repo.Find("email", "ada@example.com")
	require.NoError(t, err)
	assert.Equal(t, "222222", stored.Code)
	assert.Equal(t, "Second", stored.PendingName)
	assert.Equal(t, "h2", stored.PendingPasswordHash)
	assert.Empty(t, stored.PendingPhone, "a phone dropped on the resend must not linger from the earlier attempt")
}

// Abandoned signups are the only rows that ever reach expiry — a submitted
// code deletes its row whether it was right or wrong — and for the email
// channel they hold a bcrypt hash of a password the person very likely
// reuses. The sweep has to take those and leave live ones alone.
func TestRegistrationVerificationRepository_DeleteExpired(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.RegistrationVerification{}))
	repo := NewRegistrationVerificationRepository(db)

	now := time.Now()
	require.NoError(t, repo.Upsert("email", "abandoned@example.com", "111111", now.Add(-time.Minute),
		models.PendingRegistrationData{PendingName: "Abandoned", PendingPasswordHash: "hash"}))
	require.NoError(t, repo.Upsert("email", "live@example.com", "222222", now.Add(time.Minute),
		models.PendingRegistrationData{PendingName: "Live", PendingPasswordHash: "hash"}))
	require.NoError(t, repo.Upsert("phone", "+6591234567", "333333", now.Add(-time.Hour),
		models.PendingRegistrationData{}))

	n, err := repo.DeleteExpired(now)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n, "both expired rows go, across channels")

	_, err = repo.Find("email", "abandoned@example.com")
	require.ErrorIs(t, err, repository.ErrNotFound, "the expired row's password hash must not survive the sweep")
	_, err = repo.Find("phone", "+6591234567")
	require.ErrorIs(t, err, repository.ErrNotFound)

	stored, err := repo.Find("email", "live@example.com")
	require.NoError(t, err)
	assert.Equal(t, "222222", stored.Code, "an unexpired signup in progress is untouched")
}
