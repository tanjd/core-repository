package handlers

import (
	"context"
	"errors"

	"github.com/danielgtaylor/huma/v2"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// WaitlistHandler holds dependencies for waitlist routes.
type WaitlistHandler struct {
	copies    repository.CopyRepository
	waitlists repository.WaitlistRepository
}

// NewWaitlistHandler creates a new WaitlistHandler.
func NewWaitlistHandler(copies repository.CopyRepository, waitlists repository.WaitlistRepository) *WaitlistHandler {
	return &WaitlistHandler{copies: copies, waitlists: waitlists}
}

// --- Input / Output types ---

type waitlistCopyInput struct {
	ID uint `path:"id" doc:"Copy ID"`
}

type waitlistCountOutput struct {
	Body struct {
		Count      int64 `json:"count"`
		OnWaitlist bool  `json:"on_waitlist"`
	}
}

// --- Route registration ---

// RegisterRoutes registers all waitlist routes on the given huma API.
func (h *WaitlistHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-waitlist-count",
		Method:      "GET",
		Path:        "/copies/{id}/waitlist",
		Tags:        []string{"waitlist"},
		Summary:     "Get the waitlist count for a copy, and whether the caller is on it",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.getCount)

	huma.Register(api, huma.Operation{
		OperationID:   "join-waitlist",
		Method:        "POST",
		Path:          "/copies/{id}/waitlist",
		Tags:          []string{"waitlist"},
		Summary:       "Join the waitlist for a loaned copy",
		Security:      []map[string][]string{{"bearer": {}}},
		DefaultStatus: 201,
	}, h.join)

	huma.Register(api, huma.Operation{
		OperationID:   "leave-waitlist",
		Method:        "DELETE",
		Path:          "/copies/{id}/waitlist",
		Tags:          []string{"waitlist"},
		Summary:       "Leave the waitlist for a copy",
		Security:      []map[string][]string{{"bearer": {}}},
		DefaultStatus: 204,
	}, h.leave)
}

// --- Handlers ---

func (h *WaitlistHandler) getCount(ctx context.Context, input *waitlistCopyInput) (*waitlistCountOutput, error) {
	callerID, _ := middleware.GetRequiredUserID(ctx)

	count, err := h.waitlists.Count(input.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not get waitlist count")
	}

	onWaitlist := false
	if callerID > 0 {
		onWaitlist, _ = h.waitlists.IsOnWaitlist(input.ID, callerID)
	}

	var out waitlistCountOutput
	out.Body.Count = count
	out.Body.OnWaitlist = onWaitlist
	return &out, nil
}

func (h *WaitlistHandler) join(ctx context.Context, input *waitlistCopyInput) (*struct{}, error) {
	callerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	bookCopy, err := h.copies.GetByID(input.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("copy not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch copy")
	}
	if bookCopy.OwnerID == callerID {
		return nil, huma.Error400BadRequest("you cannot join the waitlist for your own copy")
	}
	if bookCopy.Status != "loaned" {
		return nil, huma.Error400BadRequest("can only join the waitlist for a loaned copy")
	}

	if err := h.waitlists.Add(input.ID, callerID); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, huma.Error409Conflict("you are already on the waitlist")
		}
		return nil, huma.Error500InternalServerError("could not join waitlist")
	}

	return nil, nil
}

func (h *WaitlistHandler) leave(ctx context.Context, input *waitlistCopyInput) (*struct{}, error) {
	callerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	if err := h.waitlists.Remove(input.ID, callerID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("you are not on the waitlist")
		}
		return nil, huma.Error500InternalServerError("could not leave waitlist")
	}

	return nil, nil
}
