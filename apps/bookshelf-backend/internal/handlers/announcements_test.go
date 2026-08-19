package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
)

func newAnnouncementHandler() (*AnnouncementHandler, *repotest.AnnouncementRepository) {
	repo := repotest.NewAnnouncementRepository()
	return NewAnnouncementHandler(repo), repo
}

func boolPtr(b bool) *bool    { return &b }
func strPtr(s string) *string { return &s }

func TestListActiveAnnouncements(t *testing.T) {
	h, repo := newAnnouncementHandler()
	require.NoError(t, repo.Create(&models.Announcement{Title: "Active one", Body: "b", Type: "info", Active: true}))
	require.NoError(t, repo.Create(&models.Announcement{Title: "Inactive one", Body: "b", Type: "info", Active: false}))

	t.Run("unauthenticated is unauthorized", func(t *testing.T) {
		_, err := h.listActive(fakeAuthedCtxNone(), &struct{}{})
		require.Error(t, err)
		assertStatus(t, err, 401)
	})

	t.Run("returns only active announcements", func(t *testing.T) {
		out, err := h.listActive(fakeAuthedCtx(t, 1, "user"), &struct{}{})
		require.NoError(t, err)
		require.Len(t, out.Body, 1)
		assert.Equal(t, "Active one", out.Body[0].Title)
	})
}

func TestAdminAnnouncementRoutes_RequireAdmin(t *testing.T) {
	h, repo := newAnnouncementHandler()
	require.NoError(t, repo.Create(&models.Announcement{Title: "T", Body: "B", Type: "info", Active: true}))

	createInput := &createAnnouncementInput{}
	createInput.Body.Title = "New"
	createInput.Body.Body = "Body"
	createInput.Body.Type = "info"

	t.Run("adminList: non-admin forbidden, unauthenticated unauthorized, admin allowed", func(t *testing.T) {
		_, err := h.adminList(fakeAuthedCtx(t, 1, "user"), &adminListAnnouncementsInput{})
		assertStatus(t, err, 403)

		_, err = h.adminList(fakeAuthedCtxNone(), &adminListAnnouncementsInput{})
		assertStatus(t, err, 401)

		_, err = h.adminList(fakeAuthedCtx(t, 1, "admin"), &adminListAnnouncementsInput{})
		require.NoError(t, err)
	})

	t.Run("adminCreate: non-admin forbidden, unauthenticated unauthorized", func(t *testing.T) {
		_, err := h.adminCreate(fakeAuthedCtx(t, 1, "user"), createInput)
		assertStatus(t, err, 403)

		_, err = h.adminCreate(fakeAuthedCtxNone(), createInput)
		assertStatus(t, err, 401)
	})

	t.Run("adminUpdate: non-admin forbidden, unauthenticated unauthorized", func(t *testing.T) {
		input := &updateAnnouncementInput{ID: 1}
		input.Body.Active = boolPtr(false)

		_, err := h.adminUpdate(fakeAuthedCtx(t, 1, "user"), input)
		assertStatus(t, err, 403)

		_, err = h.adminUpdate(fakeAuthedCtxNone(), input)
		assertStatus(t, err, 401)
	})

	t.Run("adminDelete: non-admin forbidden, unauthenticated unauthorized", func(t *testing.T) {
		_, err := h.adminDelete(fakeAuthedCtx(t, 1, "user"), &announcementIDInput{ID: 1})
		assertStatus(t, err, 403)

		_, err = h.adminDelete(fakeAuthedCtxNone(), &announcementIDInput{ID: 1})
		assertStatus(t, err, 401)
	})
}

func TestAdminCreateAnnouncement(t *testing.T) {
	h, repo := newAnnouncementHandler()

	t.Run("rejects an invalid type", func(t *testing.T) {
		input := &createAnnouncementInput{}
		input.Body.Title = "T"
		input.Body.Body = "B"
		input.Body.Type = "bogus"

		_, err := h.adminCreate(fakeAuthedCtx(t, 1, "admin"), input)
		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("defaults active to true when omitted", func(t *testing.T) {
		input := &createAnnouncementInput{}
		input.Body.Title = "T"
		input.Body.Body = "B"
		input.Body.Type = "info"

		out, err := h.adminCreate(fakeAuthedCtx(t, 1, "admin"), input)
		require.NoError(t, err)
		assert.True(t, out.Body.Active)
	})

	t.Run("honors an explicit active value", func(t *testing.T) {
		input := &createAnnouncementInput{}
		input.Body.Title = "T"
		input.Body.Body = "B"
		input.Body.Type = "known_issue"
		input.Body.Active = boolPtr(false)

		out, err := h.adminCreate(fakeAuthedCtx(t, 1, "admin"), input)
		require.NoError(t, err)
		assert.False(t, out.Body.Active)

		// Regression: GORM's Create() treats a zero-valued bool field with a
		// "default" tag as unset and substitutes the DB default — verify the
		// false value actually persisted through the repository, not just in
		// the in-memory struct returned by the handler.
		stored, err := repo.GetByID(out.Body.ID)
		require.NoError(t, err)
		assert.False(t, stored.Active)
	})
}

func TestAdminUpdateAnnouncement(t *testing.T) {
	h, repo := newAnnouncementHandler()
	require.NoError(t, repo.Create(&models.Announcement{Title: "Original", Body: "OrigBody", Type: "info", Active: true}))

	t.Run("partial update leaves other fields untouched", func(t *testing.T) {
		input := &updateAnnouncementInput{ID: 1}
		input.Body.Active = boolPtr(false)

		out, err := h.adminUpdate(fakeAuthedCtx(t, 1, "admin"), input)
		require.NoError(t, err)
		assert.False(t, out.Body.Active)
		assert.Equal(t, "Original", out.Body.Title)
		assert.Equal(t, "OrigBody", out.Body.Body)
		assert.Equal(t, "info", out.Body.Type)
	})

	t.Run("rejects an invalid type", func(t *testing.T) {
		input := &updateAnnouncementInput{ID: 1}
		input.Body.Type = strPtr("bogus")

		_, err := h.adminUpdate(fakeAuthedCtx(t, 1, "admin"), input)
		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("404 for an unknown ID", func(t *testing.T) {
		input := &updateAnnouncementInput{ID: 999}
		input.Body.Active = boolPtr(true)

		_, err := h.adminUpdate(fakeAuthedCtx(t, 1, "admin"), input)
		require.Error(t, err)
		assertStatus(t, err, 404)
	})
}

func TestAdminDeleteAnnouncement(t *testing.T) {
	h, repo := newAnnouncementHandler()
	require.NoError(t, repo.Create(&models.Announcement{Title: "T", Body: "B", Type: "info", Active: true}))

	_, err := h.adminDelete(fakeAuthedCtx(t, 1, "admin"), &announcementIDInput{ID: 1})
	require.NoError(t, err)

	_, err = repo.GetByID(1)
	require.Error(t, err)

	t.Run("repeat delete does not error", func(t *testing.T) {
		_, err := h.adminDelete(fakeAuthedCtx(t, 1, "admin"), &announcementIDInput{ID: 1})
		require.NoError(t, err)
	})
}
