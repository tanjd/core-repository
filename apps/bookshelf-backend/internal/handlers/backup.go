package handlers

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/danielgtaylor/huma/v2"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/services"
)

// BackupHandler exposes admin endpoints for listing, downloading, and
// deleting backup snapshots. Creating a snapshot and configuring its
// schedule reuse the existing jobs ("backup") and settings
// ("backup_interval"/"backup_retention_count") endpoints rather than
// duplicating that surface here.
type BackupHandler struct {
	backups *services.BackupService
}

// NewBackupHandler creates a new BackupHandler.
func NewBackupHandler(backups *services.BackupService) *BackupHandler {
	return &BackupHandler{backups: backups}
}

// --- Input / Output types ---

type listBackupsOutput struct {
	Body []services.BackupInfo
}

type backupFilenameInput struct {
	Filename string `path:"filename" doc:"Backup archive filename"`
}

// --- Route registration ---

// RegisterRoutes registers the admin backups endpoints on the given API.
func (h *BackupHandler) RegisterRoutes(api huma.API) {
	security := []map[string][]string{{"bearer": {}}}

	huma.Register(api, huma.Operation{
		OperationID: "admin-list-backups",
		Method:      "GET",
		Path:        "/admin/backups",
		Tags:        []string{"admin"},
		Summary:     "List backup snapshots",
		Security:    security,
	}, h.listBackups)

	huma.Register(api, huma.Operation{
		OperationID:   "admin-delete-backup",
		Method:        "DELETE",
		Path:          "/admin/backups/{filename}",
		Tags:          []string{"admin"},
		Summary:       "Delete a backup snapshot",
		Security:      security,
		DefaultStatus: 204,
	}, h.deleteBackup)

	huma.Register(api, huma.Operation{
		OperationID: "admin-download-backup",
		Method:      "GET",
		Path:        "/admin/backups/{filename}/download",
		Tags:        []string{"admin"},
		Summary:     "Download a backup snapshot",
		Security:    security,
	}, h.downloadBackup)
}

// --- Handlers ---

func (h *BackupHandler) listBackups(ctx context.Context, _ *struct{}) (*listBackupsOutput, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, backupAdminError(err)
	}
	snapshots, err := h.backups.ListSnapshots()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not list backups")
	}
	return &listBackupsOutput{Body: snapshots}, nil
}

func (h *BackupHandler) deleteBackup(ctx context.Context, input *backupFilenameInput) (*struct{}, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, backupAdminError(err)
	}
	if err := h.backups.DeleteSnapshot(input.Filename); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("backup not found")
		}
		return nil, huma.Error500InternalServerError("could not delete backup")
	}
	return nil, nil
}

func (h *BackupHandler) downloadBackup(ctx context.Context, input *backupFilenameInput) (*huma.StreamResponse, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, backupAdminError(err)
	}
	path, err := h.backups.ResolvePath(input.Filename)
	if err != nil {
		return nil, huma.Error404NotFound("backup not found")
	}

	return &huma.StreamResponse{
		Body: func(sctx huma.Context) {
			f, openErr := os.Open(path) //nolint:gosec // path validated by BackupService.ResolvePath's filename allowlist
			if openErr != nil {
				sctx.SetStatus(500)
				return
			}
			defer f.Close() //nolint:errcheck

			sctx.SetHeader("Content-Type", "application/gzip")
			sctx.SetHeader("Content-Disposition", `attachment; filename="`+input.Filename+`"`)
			_, _ = io.Copy(sctx.BodyWriter(), f)
		},
	}, nil
}

func backupAdminError(err error) error {
	if errors.Is(err, middleware.ErrUnauthorized) {
		return huma.Error401Unauthorized("authentication required")
	}
	return huma.Error403Forbidden("admin access required")
}
