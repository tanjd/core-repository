package services

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// backupFilenamePattern matches server-generated snapshot filenames — the
// sole validation gate before any filename-derived path touches the
// filesystem (list/download/delete all route through resolvePath).
var backupFilenamePattern = regexp.MustCompile(`^bookshelf-backup-\d{8}T\d{6}Z\.tar\.gz$`)

// defaultBackupRetention is used when backup_retention_count is absent/invalid.
const defaultBackupRetention = 7

// BackupInfo describes one stored backup snapshot.
type BackupInfo struct {
	Filename  string    `json:"filename"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

// BackupService creates, lists, and prunes backup snapshots — each a
// tar.gz bundle of a VACUUM INTO'd copy of the SQLite database plus the
// cover-image cache directory.
type BackupService struct {
	dbPath     string
	coversDir  string
	backupsDir string
	sqlDB      *sql.DB
	admin      repository.AdminRepository
}

// NewBackupService creates a BackupService. backupsDir must already exist
// (created at boot alongside coversDir, same as main.go does today).
func NewBackupService(sqlDB *sql.DB, admin repository.AdminRepository, dbPath, coversDir, backupsDir string) *BackupService {
	return &BackupService{
		dbPath:     dbPath,
		coversDir:  coversDir,
		backupsDir: backupsDir,
		sqlDB:      sqlDB,
		admin:      admin,
	}
}

// CreateSnapshot builds a new backup archive and prunes old ones. Suitable
// as a Scheduler job run function (matches func(ctx context.Context) string).
func (b *BackupService) CreateSnapshot(ctx context.Context) string {
	info, err := b.createSnapshot(ctx)
	if err != nil {
		log.Error().Err(err).Msg("backup: snapshot failed")
		return "failed: " + err.Error()
	}
	result := fmt.Sprintf("created %s (%s)", info.Filename, humanSize(info.SizeBytes))
	log.Info().Str("filename", info.Filename).Int64("size_bytes", info.SizeBytes).Msg("backup: snapshot created")
	return result
}

func (b *BackupService) createSnapshot(ctx context.Context) (BackupInfo, error) {
	filename := "bookshelf-backup-" + time.Now().UTC().Format("20060102T150405Z") + ".tar.gz"
	finalPath := filepath.Join(b.backupsDir, filename)

	tmpDB := filepath.Join(b.backupsDir, ".tmp-db-"+filename)
	if err := b.vacuumToFile(ctx, tmpDB); err != nil {
		return BackupInfo{}, fmt.Errorf("snapshot database: %w", err)
	}
	defer os.Remove(tmpDB) //nolint:errcheck

	tmpArchive := filepath.Join(b.backupsDir, ".tmp-"+filename)
	if err := b.writeArchive(tmpArchive, tmpDB); err != nil {
		_ = os.Remove(tmpArchive)
		return BackupInfo{}, fmt.Errorf("write archive: %w", err)
	}

	if err := os.Rename(tmpArchive, finalPath); err != nil {
		_ = os.Remove(tmpArchive)
		return BackupInfo{}, fmt.Errorf("finalize archive: %w", err)
	}

	if err := b.pruneOldSnapshots(); err != nil {
		log.Warn().Err(err).Msg("backup: pruning old snapshots failed")
	}

	stat, err := os.Stat(finalPath)
	if err != nil {
		return BackupInfo{}, err
	}
	return BackupInfo{Filename: filename, SizeBytes: stat.Size(), CreatedAt: stat.ModTime()}, nil
}

// vacuumToFile snapshots the live database to dest via SQLite's VACUUM INTO —
// atomic from the app's perspective and WAL-safe (no separate -wal/-shm
// handling needed, unlike a raw file copy).
func (b *BackupService) vacuumToFile(ctx context.Context, dest string) error {
	_, err := b.sqlDB.ExecContext(ctx, "VACUUM INTO ?", dest)
	return err
}

// writeArchive bundles the vacuumed database file and coversDir's contents
// into a gzip-compressed tar at destPath.
func (b *BackupService) writeArchive(destPath, dbFile string) error {
	f, err := os.Create(destPath) //nolint:gosec // destPath is server-generated (backupsDir + our own filename)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck

	gz := gzip.NewWriter(f)
	defer gz.Close() //nolint:errcheck

	tw := tar.NewWriter(gz)
	defer tw.Close() //nolint:errcheck

	if err := addFileToTar(tw, dbFile, "bookshelf.db"); err != nil {
		return err
	}
	if err := addDirToTar(tw, b.coversDir, "covers"); err != nil {
		return err
	}
	return nil
}

// addFileToTar writes a single file into tw under arcName.
func addFileToTar(tw *tar.Writer, srcPath, arcName string) error {
	stat, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	hdr, err := tar.FileInfoHeader(stat, "")
	if err != nil {
		return err
	}
	hdr.Name = arcName
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	f, err := os.Open(srcPath) //nolint:gosec // srcPath is our own vacuumed temp file
	if err != nil {
		return err
	}
	defer f.Close()         //nolint:errcheck
	_, err = io.Copy(tw, f) //nolint:gosec // tar writer, not an HTTP response
	return err
}

// addDirToTar recursively writes dirPath's contents into tw under arcPrefix.
// A missing dirPath is treated as empty (nothing to back up yet), not an error.
func addDirToTar(tw *tar.Writer, dirPath, arcPrefix string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return nil
	}
	return filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(dirPath, path)
		if relErr != nil {
			return relErr
		}
		return addFileToTar(tw, path, filepath.Join(arcPrefix, rel))
	})
}

// ListSnapshots returns stored backup archives, newest first.
func (b *BackupService) ListSnapshots() ([]BackupInfo, error) {
	entries, err := os.ReadDir(b.backupsDir)
	if err != nil {
		return nil, err
	}
	out := make([]BackupInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !backupFilenamePattern.MatchString(e.Name()) {
			continue
		}
		stat, statErr := e.Info()
		if statErr != nil {
			continue
		}
		out = append(out, BackupInfo{Filename: e.Name(), SizeBytes: stat.Size(), CreatedAt: stat.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// ResolvePath validates filename against the server-generated backup
// filename pattern and returns its full path, or repository.ErrNotFound if
// the name is invalid or the file doesn't exist. This is the single choke
// point every filename-taking operation (download, delete) routes through.
func (b *BackupService) ResolvePath(filename string) (string, error) {
	if !backupFilenamePattern.MatchString(filename) {
		return "", repository.ErrNotFound
	}
	path := filepath.Join(b.backupsDir, filename)
	if _, err := os.Stat(path); err != nil {
		return "", repository.ErrNotFound
	}
	return path, nil
}

// DeleteSnapshot removes the named backup archive.
func (b *BackupService) DeleteSnapshot(filename string) error {
	path, err := b.ResolvePath(filename)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// pruneOldSnapshots deletes all but the newest backup_retention_count
// snapshots (falling back to defaultBackupRetention when the setting is
// absent/invalid).
func (b *BackupService) pruneOldSnapshots() error {
	snapshots, err := b.ListSnapshots()
	if err != nil {
		return err
	}
	keep := b.retentionCount()
	if len(snapshots) <= keep {
		return nil
	}
	for _, s := range snapshots[keep:] {
		if delErr := b.DeleteSnapshot(s.Filename); delErr != nil {
			log.Warn().Err(delErr).Str("filename", s.Filename).Msg("backup: failed to prune snapshot")
		}
	}
	return nil
}

func (b *BackupService) retentionCount() int {
	if b.admin != nil {
		if val, err := b.admin.GetSetting("backup_retention_count"); err == nil && val != "" {
			if n, convErr := strconv.Atoi(val); convErr == nil && n > 0 {
				return n
			}
		}
	}
	return defaultBackupRetention
}

// humanSize formats n bytes as a short human-readable size (e.g. "12.3 MB").
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
