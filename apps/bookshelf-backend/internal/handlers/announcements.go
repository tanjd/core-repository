package handlers

import (
	"context"
	"errors"

	"github.com/danielgtaylor/huma/v2"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// allowedAnnouncementTypes is the fixed set of category/severity values the
// frontend maps to a badge color — kept in sync by hand with the
// AnnouncementType union in src/lib/types.ts, same trade-off as
// MIN_PASSWORD_LENGTH in src/lib/api.ts.
var allowedAnnouncementTypes = map[string]bool{
	"info":        true,
	"new_feature": true,
	"known_issue": true,
}

// AnnouncementHandler holds dependencies for both the public
// list-active-announcements route and the admin CRUD routes.
type AnnouncementHandler struct {
	announcements repository.AnnouncementRepository
}

// NewAnnouncementHandler creates a new AnnouncementHandler.
func NewAnnouncementHandler(announcements repository.AnnouncementRepository) *AnnouncementHandler {
	return &AnnouncementHandler{announcements: announcements}
}

// --- Input / Output types ---

type listActiveAnnouncementsOutput struct {
	Body []models.Announcement
}

type adminListAnnouncementsInput struct {
	Page     int `query:"page" minimum:"1" doc:"Page number (default 1)"`
	PageSize int `query:"page_size" minimum:"1" maximum:"100" doc:"Items per page (default 20)"`
}

type adminListAnnouncementsOutput struct {
	Body struct {
		Items      []models.Announcement `json:"items"`
		Total      int64                 `json:"total"`
		Page       int                   `json:"page"`
		PageSize   int                   `json:"page_size"`
		TotalPages int                   `json:"total_pages"`
	}
}

type createAnnouncementInput struct {
	Body struct {
		Title  string `json:"title" required:"true" minLength:"1" doc:"Announcement title"`
		Body   string `json:"body" required:"true" minLength:"1" doc:"Announcement body text"`
		Type   string `json:"type" required:"true" doc:"info | new_feature | known_issue"`
		Active *bool  `json:"active,omitempty" doc:"Whether the announcement is shown (default true)"`
	}
}

type announcementIDInput struct {
	ID uint `path:"id" doc:"Announcement ID"`
}

type updateAnnouncementInput struct {
	ID   uint `path:"id" doc:"Announcement ID"`
	Body struct {
		Title  *string `json:"title,omitempty" doc:"Announcement title"`
		Body   *string `json:"body,omitempty" doc:"Announcement body text"`
		Type   *string `json:"type,omitempty" doc:"info | new_feature | known_issue"`
		Active *bool   `json:"active,omitempty" doc:"Whether the announcement is shown"`
	}
}

type announcementOutput struct{ Body models.Announcement }

// --- Route registration ---

// RegisterRoutes registers the public and admin announcement routes on the given huma API.
func (h *AnnouncementHandler) RegisterRoutes(api huma.API) {
	security := []map[string][]string{{"bearer": {}}}

	huma.Register(api, huma.Operation{
		OperationID: "list-active-announcements",
		Method:      "GET",
		Path:        "/announcements",
		Tags:        []string{"announcements"},
		Summary:     "List active announcements",
		Security:    security,
	}, h.listActive)

	huma.Register(api, huma.Operation{
		OperationID: "admin-list-announcements",
		Method:      "GET",
		Path:        "/admin/announcements",
		Tags:        []string{"admin"},
		Summary:     "List all announcements",
		Security:    security,
	}, h.adminList)

	huma.Register(api, huma.Operation{
		OperationID: "admin-create-announcement",
		Method:      "POST",
		Path:        "/admin/announcements",
		Tags:        []string{"admin"},
		Summary:     "Create an announcement",
		Security:    security,
	}, h.adminCreate)

	huma.Register(api, huma.Operation{
		OperationID: "admin-update-announcement",
		Method:      "PATCH",
		Path:        "/admin/announcements/{id}",
		Tags:        []string{"admin"},
		Summary:     "Update an announcement",
		Security:    security,
	}, h.adminUpdate)

	huma.Register(api, huma.Operation{
		OperationID:   "admin-delete-announcement",
		Method:        "DELETE",
		Path:          "/admin/announcements/{id}",
		Tags:          []string{"admin"},
		Summary:       "Delete an announcement",
		Security:      security,
		DefaultStatus: 204,
	}, h.adminDelete)
}

// --- Handlers ---

func (h *AnnouncementHandler) listActive(ctx context.Context, _ *struct{}) (*listActiveAnnouncementsOutput, error) {
	if _, err := middleware.GetRequiredUserID(ctx); err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}
	items, err := h.announcements.ListActive()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch announcements")
	}
	return &listActiveAnnouncementsOutput{Body: items}, nil
}

func (h *AnnouncementHandler) adminList(ctx context.Context, input *adminListAnnouncementsInput) (*adminListAnnouncementsOutput, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, adminError(err)
	}
	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	result, err := h.announcements.ListPaginated(page, pageSize)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not list announcements")
	}
	var out adminListAnnouncementsOutput
	out.Body.Items = result.Items
	out.Body.Total = result.Total
	out.Body.Page = result.Page
	out.Body.PageSize = result.PageSize
	out.Body.TotalPages = result.TotalPages
	return &out, nil
}

func (h *AnnouncementHandler) adminCreate(ctx context.Context, input *createAnnouncementInput) (*announcementOutput, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, adminError(err)
	}
	if !allowedAnnouncementTypes[input.Body.Type] {
		return nil, huma.Error400BadRequest("type must be one of: info, new_feature, known_issue")
	}
	active := true
	if input.Body.Active != nil {
		active = *input.Body.Active
	}
	a := &models.Announcement{
		Title:  input.Body.Title,
		Body:   input.Body.Body,
		Type:   input.Body.Type,
		Active: active,
	}
	if err := h.announcements.Create(a); err != nil {
		return nil, huma.Error500InternalServerError("could not create announcement")
	}
	return &announcementOutput{Body: *a}, nil
}

func (h *AnnouncementHandler) adminUpdate(ctx context.Context, input *updateAnnouncementInput) (*announcementOutput, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, adminError(err)
	}
	a, err := h.announcements.GetByID(input.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("announcement not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch announcement")
	}
	if input.Body.Title != nil {
		a.Title = *input.Body.Title
	}
	if input.Body.Body != nil {
		a.Body = *input.Body.Body
	}
	if input.Body.Type != nil {
		if !allowedAnnouncementTypes[*input.Body.Type] {
			return nil, huma.Error400BadRequest("type must be one of: info, new_feature, known_issue")
		}
		a.Type = *input.Body.Type
	}
	if input.Body.Active != nil {
		a.Active = *input.Body.Active
	}
	if err := h.announcements.Save(a); err != nil {
		return nil, huma.Error500InternalServerError("could not update announcement")
	}
	return &announcementOutput{Body: *a}, nil
}

func (h *AnnouncementHandler) adminDelete(ctx context.Context, input *announcementIDInput) (*struct{}, error) {
	if err := middleware.RequireAdmin(ctx); err != nil {
		return nil, adminError(err)
	}
	if err := h.announcements.Delete(input.ID); err != nil {
		return nil, huma.Error500InternalServerError("could not delete announcement")
	}
	return nil, nil
}
