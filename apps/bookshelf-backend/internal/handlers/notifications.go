package handlers

import (
	"context"

	"github.com/danielgtaylor/huma/v2"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// NotificationHandler holds dependencies for notification routes.
type NotificationHandler struct {
	notifs repository.NotificationRepository
}

// NewNotificationHandler creates a new NotificationHandler.
func NewNotificationHandler(notifs repository.NotificationRepository) *NotificationHandler {
	return &NotificationHandler{notifs: notifs}
}

// --- Input / Output types ---

type listNotificationsInput struct {
	Unread   bool `query:"unread" doc:"When true, return only unread notifications"`
	Page     int  `query:"page" minimum:"1" doc:"Page number (default 1)"`
	PageSize int  `query:"page_size" minimum:"1" maximum:"100" doc:"Items per page (default 20)"`
}

type listNotificationsOutput struct {
	Body struct {
		Items      []models.Notification `json:"items"`
		Total      int64                 `json:"total"`
		Page       int                   `json:"page"`
		PageSize   int                   `json:"page_size"`
		TotalPages int                   `json:"total_pages"`
	}
}

type markReadInput struct {
	ID uint `path:"id" doc:"Notification ID"`
}

type markReadOutput struct{ Body models.Notification }

type markAllReadOutput struct {
	Body struct {
		Message string `json:"message"`
	}
}

// --- Route registration ---

// RegisterRoutes registers all notification routes on the given huma API.
func (h *NotificationHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-notifications",
		Method:      "GET",
		Path:        "/notifications",
		Tags:        []string{"notifications"},
		Summary:     "List notifications for the authenticated user",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.listNotifications)

	// /notifications/read-all must be registered before /notifications/{id}/read
	// so the literal segment takes precedence over the wildcard.
	huma.Register(api, huma.Operation{
		OperationID: "mark-all-read",
		Method:      "PATCH",
		Path:        "/notifications/read-all",
		Tags:        []string{"notifications"},
		Summary:     "Mark all notifications as read",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.markAllRead)

	huma.Register(api, huma.Operation{
		OperationID: "mark-notification-read",
		Method:      "PATCH",
		Path:        "/notifications/{id}/read",
		Tags:        []string{"notifications"},
		Summary:     "Mark a single notification as read",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.markRead)
}

// --- Handlers ---

func (h *NotificationHandler) listNotifications(ctx context.Context, input *listNotificationsInput) (*listNotificationsOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	result, err := h.notifs.FindByRecipientPaginated(userID, input.Unread, page, pageSize)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch notifications")
	}

	var out listNotificationsOutput
	out.Body.Items = result.Items
	out.Body.Total = result.Total
	out.Body.Page = result.Page
	out.Body.PageSize = result.PageSize
	out.Body.TotalPages = result.TotalPages
	return &out, nil
}

func (h *NotificationHandler) markRead(ctx context.Context, input *markReadInput) (*markReadOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	n, err := h.notifs.GetByID(input.ID)
	if err != nil {
		return nil, huma.Error404NotFound("notification not found")
	}
	if n.RecipientID != userID {
		return nil, huma.Error403Forbidden("access denied")
	}

	n.Read = true
	if err := h.notifs.Save(n); err != nil {
		return nil, huma.Error500InternalServerError("could not update notification")
	}

	return &markReadOutput{Body: *n}, nil
}

func (h *NotificationHandler) markAllRead(ctx context.Context, _ *struct{}) (*markAllReadOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	if err := h.notifs.MarkAllReadForRecipient(userID); err != nil {
		return nil, huma.Error500InternalServerError("could not update notifications")
	}

	out := &markAllReadOutput{}
	out.Body.Message = "all notifications marked as read"
	return out, nil
}
