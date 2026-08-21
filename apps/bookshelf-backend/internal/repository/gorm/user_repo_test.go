package gorm

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// An email address is one identity however it's capitalised. Registration
// leans on this directly: sendRegisterEmailOTP's duplicate check is the only
// thing standing between "Ada@example.com" and a second account for a
// mailbox that already has one.
func TestUserRepository_FindByEmailIsCaseInsensitive(t *testing.T) {
	db := openTestDB(t)
	repo := NewUserRepository(db)
	require.NoError(t, repo.Create(&models.User{Name: "Ada", Email: "Ada.Lovelace@Example.com", Password: "x"}))

	for _, query := range []string{
		"Ada.Lovelace@Example.com",
		"ada.lovelace@example.com",
		"ADA.LOVELACE@EXAMPLE.COM",
	} {
		found, err := repo.FindByEmail(query)
		require.NoError(t, err, "looking up %q", query)
		assert.Equal(t, "Ada.Lovelace@Example.com", found.Email, "the stored casing is preserved, not normalized away")
	}

	_, err := repo.FindByEmail("someone.else@example.com")
	require.ErrorIs(t, err, repository.ErrNotFound)
}

// The app-level duplicate check is racy on its own; idx_users_email_nocase is
// what actually makes two case-variant accounts impossible. Runs the shipped
// migration rather than a copy of it, so the assertion is about the SQL that
// really ships.
func TestUserRepository_MigrationRejectsCaseVariantDuplicate(t *testing.T) {
	db := openTestDB(t)
	migration, err := os.ReadFile("../../db/migrations/000011_users_email_nocase_index.up.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(migration)).Error)

	repo := NewUserRepository(db)
	require.NoError(t, repo.Create(&models.User{Name: "Ada", Email: "ada@example.com", Password: "x"}))

	err = repo.Create(&models.User{Name: "Impostor", Email: "Ada@Example.com", Password: "y"})
	require.Error(t, err, "a case variant of an existing address must not become a second account")
}
