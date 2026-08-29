package handlers

import (
	"context"
	"errors"

	"github.com/danielgtaylor/huma/v2"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/services"
)

// JobsHandler exposes admin endpoints for inspecting and triggering background jobs.
type JobsHandler struct {
	scheduler *services.Scheduler
	digest    *services.DigestService
	users     repository.UserRepository
}

// NewJobsHandler creates a new JobsHandler.
func NewJobsHandler(scheduler *services.Scheduler, digest *services.DigestService, users repository.UserRepository) *JobsHandler {
	return &JobsHandler{scheduler: scheduler, digest: digest, users: users}
}

// --- Input / Output types ---

type listJobsOutput struct {
	Body []services.JobStatus
}

type runJobInput struct {
	Job string `path:"job" doc:"Job name (e.g. cover-refresh, backup)"`
}

type digestTestEmailOutput struct {
	Body struct {
		Sent      bool   `json:"sent"`
		Recipient string `json:"recipient"`
	}
}

// --- Route registration ---

// RegisterRoutes registers the admin jobs endpoints on the given API.
func (h *JobsHandler) RegisterRoutes(api huma.API) {
	security := []map[string][]string{{"bearer": {}}}

	huma.Register(api, huma.Operation{
		OperationID: "admin-list-jobs",
		Method:      "GET",
		Path:        "/admin/jobs",
		Tags:        []string{"admin"},
		Summary:     "List background job statuses",
		Security:    security,
	}, h.listJobs)

	huma.Register(api, huma.Operation{
		OperationID:   "admin-run-job",
		Method:        "POST",
		Path:          "/admin/jobs/{job}/run",
		Tags:          []string{"admin"},
		Summary:       "Trigger a background job immediately",
		Security:      security,
		DefaultStatus: 202,
	}, h.runJob)

	huma.Register(api, huma.Operation{
		OperationID: "admin-digest-test-email",
		Method:      "POST",
		Path:        "/admin/jobs/monthly-digest/test-email",
		Tags:        []string{"admin"},
		Summary:     "Send a preview monthly digest to the calling admin's email address",
		Security:    security,
	}, h.digestTestEmail)
}

// --- Handlers ---

func (h *JobsHandler) listJobs(ctx context.Context, _ *struct{}) (*listJobsOutput, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, jobsAdminError(err)
	}
	return &listJobsOutput{Body: h.scheduler.Status()}, nil
}

func (h *JobsHandler) runJob(ctx context.Context, input *runJobInput) (*struct{}, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, jobsAdminError(err)
	}
	if !h.scheduler.TriggerNow(input.Job) {
		return nil, huma.Error404NotFound("unknown job: " + input.Job)
	}
	return nil, nil
}

func (h *JobsHandler) digestTestEmail(ctx context.Context, _ *struct{}) (*digestTestEmailOutput, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, jobsAdminError(err)
	}
	adminID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}
	adminUser, err := h.users.FindByID(adminID)
	if err != nil {
		return nil, huma.Error404NotFound("admin user not found")
	}
	recipient, err := h.digest.SendTestEmail(ctx, *adminUser)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to send test email: " + err.Error())
	}
	out := &digestTestEmailOutput{}
	out.Body.Sent = true
	out.Body.Recipient = recipient
	return out, nil
}

func jobsAdminError(err error) error {
	if errors.Is(err, middleware.ErrUnauthorized) {
		return huma.Error401Unauthorized("authentication required")
	}
	return huma.Error403Forbidden("admin access required")
}
