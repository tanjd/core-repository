package services

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	ctx := context.Background()
	_, err = db.ExecContext(ctx, "CREATE TABLE books (id INTEGER PRIMARY KEY, title TEXT)")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "INSERT INTO books (title) VALUES ('Test Book')")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() }) //nolint:errcheck
	return db
}

func newTestBackupService(t *testing.T) (*BackupService, string) {
	t.Helper()
	coversDir := filepath.Join(t.TempDir(), "covers")
	require.NoError(t, os.MkdirAll(coversDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(coversDir, "cover1.jpg"), []byte("fake-jpeg"), 0o600))

	backupsDir := t.TempDir()
	svc := NewBackupService(openTestDB(t), stubAdminRepo{}, "", coversDir, backupsDir)
	return svc, backupsDir
}

func archiveNames(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path) //nolint:gosec // test fixture path
	require.NoError(t, err)
	defer f.Close() //nolint:errcheck

	gz, err := gzip.NewReader(f)
	require.NoError(t, err)
	defer gz.Close() //nolint:errcheck

	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		names = append(names, hdr.Name)
	}
	return names
}

func TestCreateSnapshot_ContainsDBAndCovers(t *testing.T) {
	svc, backupsDir := newTestBackupService(t)

	result := svc.CreateSnapshot(context.Background())
	assert.Contains(t, result, "created bookshelf-backup-")

	snapshots, err := svc.ListSnapshots()
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	assert.Positive(t, snapshots[0].SizeBytes)

	names := archiveNames(t, filepath.Join(backupsDir, snapshots[0].Filename))
	assert.Contains(t, names, "bookshelf.db")
	assert.Contains(t, names, "covers/cover1.jpg")
}

func TestListSnapshots_IgnoresUnrelatedFiles(t *testing.T) {
	svc, backupsDir := newTestBackupService(t)
	require.NoError(t, os.WriteFile(filepath.Join(backupsDir, "not-a-backup.txt"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(backupsDir, ".tmp-bookshelf-backup-20260101T000000Z.tar.gz"), []byte("x"), 0o600))

	svc.CreateSnapshot(context.Background())

	snapshots, err := svc.ListSnapshots()
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
}

func TestResolvePath_RejectsPathTraversal(t *testing.T) {
	svc, _ := newTestBackupService(t)
	svc.CreateSnapshot(context.Background())

	_, err := svc.ResolvePath("../../etc/passwd")
	require.Error(t, err)

	_, err = svc.ResolvePath("not-a-real-backup.tar.gz")
	require.Error(t, err)
}

func TestDeleteSnapshot(t *testing.T) {
	svc, _ := newTestBackupService(t)
	svc.CreateSnapshot(context.Background())

	snapshots, err := svc.ListSnapshots()
	require.NoError(t, err)
	require.Len(t, snapshots, 1)

	require.NoError(t, svc.DeleteSnapshot(snapshots[0].Filename))

	snapshots, err = svc.ListSnapshots()
	require.NoError(t, err)
	assert.Empty(t, snapshots)
}

func TestPruneOldSnapshots_KeepsRetentionCount(t *testing.T) {
	svc, backupsDir := newTestBackupService(t)
	admin := stubSettingAdminRepo{settings: map[string]string{"backup_retention_count": "2"}}
	svc.admin = admin

	// Create 4 fake pre-existing snapshots with distinct, backdated mtimes so
	// ordering (and thus which 2 survive) is deterministic.
	base := time.Now().Add(-time.Hour)
	for i := range 4 {
		name := fmt.Sprintf("bookshelf-backup-2026010%dT000000Z.tar.gz", i+1)
		path := filepath.Join(backupsDir, name)
		require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))
		mtime := base.Add(time.Duration(i) * time.Minute)
		require.NoError(t, os.Chtimes(path, mtime, mtime))
	}

	require.NoError(t, svc.pruneOldSnapshots())

	snapshots, err := svc.ListSnapshots()
	require.NoError(t, err)
	require.Len(t, snapshots, 2)
	// Newest-first: the two most recently modified fakes should survive.
	assert.Equal(t, "bookshelf-backup-20260104T000000Z.tar.gz", snapshots[0].Filename)
	assert.Equal(t, "bookshelf-backup-20260103T000000Z.tar.gz", snapshots[1].Filename)
}

// stubSettingAdminRepo is a minimal AdminRepository fake exposing a fixed
// settings map, for tests that need GetSetting to return a specific value
// (stubAdminRepo above always returns "").
type stubSettingAdminRepo struct {
	stubAdminRepo
	settings map[string]string
}

func (r stubSettingAdminRepo) GetSetting(key string) (string, error) {
	return r.settings[key], nil
}
