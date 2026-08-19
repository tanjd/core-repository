package handlers

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/services"
)

func newBackupHandler(t *testing.T) *BackupHandler {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() }) //nolint:errcheck

	coversDir := t.TempDir()
	backupsDir := t.TempDir()
	admin := repotest.NewAdminRepository()
	backupSvc := services.NewBackupService(sqlDB, admin, dbPath, coversDir, backupsDir)
	return NewBackupHandler(backupSvc)
}

func TestBackupHandler_RequiresAdmin(t *testing.T) {
	h := newBackupHandler(t)

	t.Run("non-admin is forbidden", func(t *testing.T) {
		_, err := h.listBackups(fakeAuthedCtx(t, 1, "user"), &struct{}{})
		require.Error(t, err)
		assertStatus(t, err, 403)
	})

	t.Run("unauthenticated is unauthorized", func(t *testing.T) {
		_, err := h.listBackups(fakeAuthedCtxNone(), &struct{}{})
		require.Error(t, err)
		assertStatus(t, err, 401)
	})

	t.Run("admin is allowed", func(t *testing.T) {
		_, err := h.listBackups(fakeAuthedCtx(t, 1, "admin"), &struct{}{})
		require.NoError(t, err)
	})
}

func TestBackupHandler_ListCreateDownloadDelete(t *testing.T) {
	h := newBackupHandler(t)
	ctx := fakeAuthedCtx(t, 1, "admin")

	out, err := h.listBackups(ctx, &struct{}{})
	require.NoError(t, err)
	require.Empty(t, out.Body)

	h.backups.CreateSnapshot(ctx)

	out, err = h.listBackups(ctx, &struct{}{})
	require.NoError(t, err)
	require.Len(t, out.Body, 1)
	filename := out.Body[0].Filename

	t.Run("download streams the archive", func(t *testing.T) {
		resp, downloadErr := h.downloadBackup(ctx, &backupFilenameInput{Filename: filename})
		require.NoError(t, downloadErr)
		require.NotNil(t, resp.Body)
	})

	t.Run("download rejects an unknown filename", func(t *testing.T) {
		_, downloadErr := h.downloadBackup(ctx, &backupFilenameInput{Filename: "../../etc/passwd"})
		require.Error(t, downloadErr)
		assertStatus(t, downloadErr, 404)
	})

	t.Run("delete removes the snapshot", func(t *testing.T) {
		_, deleteErr := h.deleteBackup(ctx, &backupFilenameInput{Filename: filename})
		require.NoError(t, deleteErr)

		out, listErr := h.listBackups(ctx, &struct{}{})
		require.NoError(t, listErr)
		require.Empty(t, out.Body)
	})

	t.Run("delete a missing filename 404s", func(t *testing.T) {
		_, deleteErr := h.deleteBackup(ctx, &backupFilenameInput{Filename: filename})
		require.Error(t, deleteErr)
		assertStatus(t, deleteErr, 404)
	})
}
