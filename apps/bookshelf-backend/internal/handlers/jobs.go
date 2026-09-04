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
	botHealth services.BotHealthChecker
}

// NewJobsHandler creates a new JobsHandler.
func NewJobsHandler(scheduler *services.Scheduler, digest *services.DigestService, users repository.UserRepository, botHealth services.BotHealthChecker) *JobsHandler {
	return &JobsHandler{scheduler: scheduler, digest: digest, users: users, botHealth: botHealth}
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

type telegramBotStatusOutput struct {
	Body struct {
		Configured bool `json:"configured" doc:"Whether TELEGRAM_BOT_HEALTH_URL is set on the backend."`
		Online     bool `json:"online" doc:"Whether the bot's own /health endpoint responded OK just now. Always false when configured is false."`
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

	huma.Register(api, huma.Operation{
		OperationID: "admin-digest-test-telegram",
		Method:      "POST",
		Path:        "/admin/jobs/monthly-digest/test-telegram",
		Tags:        []string{"admin"},
		Summary:     "Send a preview monthly digest to the calling admin's linked Telegram chat",
		Security:    security,
	}, h.digestTestTelegram)

	huma.Register(api, huma.Operation{
		OperationID: "admin-telegram-bot-status",
		Method:      "GET",
		Path:        "/admin/telegram-bot/status",
		Tags:        []string{"admin"},
		Summary:     "Check whether apps/bookshelf-bot's own process is online",
		Security:    security,
	}, h.telegramBotStatus)
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

func (h *JobsHandler) digestTestTelegram(ctx context.Context, _ *struct{}) (*struct{}, error) {
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
	if err := h.digest.SendTestTelegram(ctx, *adminUser); err != nil {
		if errors.Is(err, services.ErrTelegramNotLinked) {
			return nil, huma.Error400BadRequest("link Telegram in your profile before sending a test message")
		}
		return nil, huma.Error502BadGateway("could not reach Telegram — check the bot is still linked and try again")
	}
	return nil, nil
}

func (h *JobsHandler) telegramBotStatus(ctx context.Context, _ *struct{}) (*telegramBotStatusOutput, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, jobsAdminError(err)
	}

	out := &telegramBotStatusOutput{}
	out.Body.Configured = h.botHealth.Configured()
	if out.Body.Configured {
		out.Body.Online = h.botHealth.Online(ctx)
	}
	return out, nil
}

func jobsAdminError(err error) error {
	if errors.Is(err, middleware.ErrUnauthorized) {
		return huma.Error401Unauthorized("authentication required")
	}
	return huma.Error403Forbidden("admin access required")
}
