package gorm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

func TestInviteCodeRepository_FindOrCreateByInviter(t *testing.T) {
	db := openTestDB(t)
	user := models.User{Name: "U1", Email: "u1-ic@example.com"}
	require.NoError(t, db.Create(&user).Error)
	codes := NewInviteCodeRepository(db)

	t.Run("creates on first call", func(t *testing.T) {
		ic, err := codes.FindOrCreateByInviter(user.ID, "abc12345")
		require.NoError(t, err)
		assert.Equal(t, "abc12345", ic.Code)
		assert.Equal(t, user.ID, ic.InviterID)
	})

	t.Run("returns the same row on a subsequent call, ignoring the passed code", func(t *testing.T) {
		ic, err := codes.FindOrCreateByInviter(user.ID, "zzzzzzzz")
		require.NoError(t, err)
		assert.Equal(t, "abc12345", ic.Code)
	})
}

func TestInviteCodeRepository_FindByCode(t *testing.T) {
	db := openTestDB(t)
	user := models.User{Name: "U1", Email: "u1-fbc@example.com"}
	require.NoError(t, db.Create(&user).Error)
	codes := NewInviteCodeRepository(db)
	_, err := codes.FindOrCreateByInviter(user.ID, "findme01")
	require.NoError(t, err)

	t.Run("returns the row when found", func(t *testing.T) {
		ic, err := codes.FindByCode("findme01")
		require.NoError(t, err)
		assert.Equal(t, user.ID, ic.InviterID)
	})

	t.Run("returns ErrNotFound when absent", func(t *testing.T) {
		_, err := codes.FindByCode("nosuchcode")
		require.ErrorIs(t, err, repository.ErrNotFound)
	})
}

func TestInviteCodeRepository_Regenerate(t *testing.T) {
	db := openTestDB(t)
	user := models.User{Name: "U1", Email: "u1-regen@example.com"}
	require.NoError(t, db.Create(&user).Error)
	codes := NewInviteCodeRepository(db)
	_, err := codes.FindOrCreateByInviter(user.ID, "oldcode1")
	require.NoError(t, err)

	newIC, err := codes.Regenerate(user.ID, "newcode1")
	require.NoError(t, err)
	assert.Equal(t, "newcode1", newIC.Code)

	_, err = codes.FindByCode("oldcode1")
	require.ErrorIs(t, err, repository.ErrNotFound, "old code is no longer findable")

	got, err := codes.FindByCode("newcode1")
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.InviterID)
}

func TestInviteCodeRepository_DeleteByInviter(t *testing.T) {
	db := openTestDB(t)
	user := models.User{Name: "U1", Email: "u1-dbi@example.com"}
	require.NoError(t, db.Create(&user).Error)
	codes := NewInviteCodeRepository(db)
	_, err := codes.FindOrCreateByInviter(user.ID, "delcode1")
	require.NoError(t, err)

	t.Run("removes the row", func(t *testing.T) {
		require.NoError(t, codes.DeleteByInviter(user.ID))
		_, err := codes.FindByCode("delcode1")
		assert.ErrorIs(t, err, repository.ErrNotFound)
	})

	t.Run("is a no-op if no code exists", func(t *testing.T) {
		require.NoError(t, codes.DeleteByInviter(user.ID))
	})
}

func TestInviteCodeRepository_DeleteByID(t *testing.T) {
	db := openTestDB(t)
	user := models.User{Name: "U1", Email: "u1-dbid@example.com"}
	require.NoError(t, db.Create(&user).Error)
	codes := NewInviteCodeRepository(db)
	ic, err := codes.FindOrCreateByInviter(user.ID, "byidcode")
	require.NoError(t, err)

	t.Run("removes by primary key", func(t *testing.T) {
		require.NoError(t, codes.DeleteByID(ic.ID))
		_, err := codes.FindByCode("byidcode")
		assert.ErrorIs(t, err, repository.ErrNotFound)
	})

	t.Run("returns ErrNotFound for an unknown id", func(t *testing.T) {
		err := codes.DeleteByID(999999)
		assert.ErrorIs(t, err, repository.ErrNotFound)
	})
}

func TestInviteCodeRepository_ListAll(t *testing.T) {
	db := openTestDB(t)
	u1 := models.User{Name: "U1", Email: "u1-list@example.com"}
	require.NoError(t, db.Create(&u1).Error)
	u2 := models.User{Name: "U2", Email: "u2-list@example.com"}
	require.NoError(t, db.Create(&u2).Error)
	codes := NewInviteCodeRepository(db)

	_, err := codes.FindOrCreateByInviter(u1.ID, "listcode1")
	require.NoError(t, err)
	_, err = codes.FindOrCreateByInviter(u2.ID, "listcode2")
	require.NoError(t, err)

	list, err := codes.ListAll()
	require.NoError(t, err)
	require.Len(t, list, 2)
	// newest first
	assert.Equal(t, "listcode2", list[0].Code)
	assert.Equal(t, "U2", list[0].Inviter.Name)
	assert.Equal(t, "listcode1", list[1].Code)
	assert.Equal(t, "U1", list[1].Inviter.Name)
}

func TestInviteCodeRepository_FindOrCreateByInviter_ConcurrentFirstAccess(t *testing.T) {
	db := openTestDB(t)
	user := models.User{Name: "U1", Email: "u1-race@example.com"}
	require.NoError(t, db.Create(&user).Error)
	codes := NewInviteCodeRepository(db)

	// Simulate the losing side of a race: another row is already inserted
	// for this inviter by the time this call's Create hits the unique
	// index, so it must fall back to the existing row instead of erroring.
	require.NoError(t, db.Create(&models.InviteCode{Code: "raceexist", InviterID: user.ID}).Error)

	ic, err := codes.FindOrCreateByInviter(user.ID, "shouldnotwin")
	require.NoError(t, err)
	assert.Equal(t, "raceexist", ic.Code)
}
