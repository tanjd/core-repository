package gorm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

func TestAdminRepository_ListUsersPaginated_PreloadsInvitedBy(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.AppSetting{}))
	admin := NewAdminRepository(db)

	inviter := models.User{Name: "Inviter", Email: "inviter@example.com"}
	require.NoError(t, db.Create(&inviter).Error)
	invitee := models.User{Name: "Invitee", Email: "invitee@example.com", InvitedByID: &inviter.ID}
	require.NoError(t, db.Create(&invitee).Error)
	noInvite := models.User{Name: "Direct signup", Email: "direct@example.com"}
	require.NoError(t, db.Create(&noInvite).Error)

	result, err := admin.ListUsersPaginated(1, 10, repository.UserListFilter{})
	require.NoError(t, err)
	require.Len(t, result.Items, 3)

	byEmail := map[string]models.User{}
	for _, u := range result.Items {
		byEmail[u.Email] = u
	}

	require.NotNil(t, byEmail["invitee@example.com"].InvitedByID)
	assert.Equal(t, inviter.ID, *byEmail["invitee@example.com"].InvitedByID)
	assert.Equal(t, "Inviter", byEmail["invitee@example.com"].InvitedBy.Name)

	assert.Nil(t, byEmail["direct@example.com"].InvitedByID)
}

func TestAdminRepository_ListUsersPaginated_Filters(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.AppSetting{}))
	admin := NewAdminRepository(db)

	alice := models.User{Name: "Alice Admin", Email: "alice@example.com", Role: "admin", Verified: true}
	require.NoError(t, db.Create(&alice).Error)
	bob := models.User{Name: "Bob Suspended", Email: "bob@example.com", Role: "user", Verified: true, Suspended: true}
	require.NoError(t, db.Create(&bob).Error)
	carol := models.User{Name: "Carol Pending", Email: "carol@example.com", Role: "user", Verified: false, PendingApproval: true}
	require.NoError(t, db.Create(&carol).Error)

	// Search matches name OR email, case-insensitively.
	result, err := admin.ListUsersPaginated(1, 10, repository.UserListFilter{Search: "ALICE"})
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "alice@example.com", result.Items[0].Email)

	result, err = admin.ListUsersPaginated(1, 10, repository.UserListFilter{Search: "example.com"})
	require.NoError(t, err)
	assert.Len(t, result.Items, 3)

	// Role.
	result, err = admin.ListUsersPaginated(1, 10, repository.UserListFilter{Role: "admin"})
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "alice@example.com", result.Items[0].Email)

	// Status.
	result, err = admin.ListUsersPaginated(1, 10, repository.UserListFilter{Status: "suspended"})
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "bob@example.com", result.Items[0].Email)

	result, err = admin.ListUsersPaginated(1, 10, repository.UserListFilter{Status: "pending_approval"})
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "carol@example.com", result.Items[0].Email)

	// Combined: role + search narrows further than either alone.
	result, err = admin.ListUsersPaginated(1, 10, repository.UserListFilter{Role: "user", Search: "bob"})
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "bob@example.com", result.Items[0].Email)
}
