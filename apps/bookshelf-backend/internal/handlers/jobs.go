package handlers

import (
	"context"
	"errors"

	"github.com/danielgtaylor/huma/v2"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/services"
)

// JobsHandler exposes admin endpoints for inspecting and triggering background jobs.
type JobsHandler struct {
	scheduler *services.Scheduler
}

// NewJobsHandler creates a new JobsHandler.
func NewJobsHandler(scheduler *services.Scheduler) *JobsHandler {
	return &JobsHandler{scheduler: scheduler}
}

// --- Input / Output types ---

type listJobsOutput struct {
	Body []services.JobStatus
}

type runJobInput struct {
	Job string `path:"job" doc:"Job name (e.g. cover-refresh)"`
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
}

// --- Handlers ---

func (h *JobsHandler) listJobs(ctx context.Context, _ *struct{}) (*listJobsOutput, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, jobsAdminError(err)
	}
	return &listJobsOutput{Body: []services.JobStatus{h.scheduler.Status()}}, nil
}

func (h *JobsHandler) runJob(ctx context.Context, input *runJobInput) (*struct{}, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, jobsAdminError(err)
	}
	switch input.Job {
	case "cover-refresh":
		h.scheduler.TriggerNow()
	default:
		return nil, huma.Error404NotFound("unknown job: " + input.Job)
	}
	return nil, nil
}

func jobsAdminError(err error) error {
	if errors.Is(err, middleware.ErrUnauthorized) {
		return huma.Error401Unauthorized("authentication required")
	}
	return huma.Error403Forbidden("admin access required")
}
