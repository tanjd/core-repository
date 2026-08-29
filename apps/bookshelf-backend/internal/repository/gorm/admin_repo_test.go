package gorm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
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

	result, err := admin.ListUsersPaginated(1, 10)
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
