package handlers

import (
	"context"
	"errors"

	"github.com/danielgtaylor/huma/v2"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// ToReadHandler holds dependencies for "to read" routes — a member's
// personal bookmark on a catalog Book they intend to borrow later. See
// WaitlistHandler for the closest-shaped existing feature.
type ToReadHandler struct {
	books   repository.BookRepository
	toReads repository.ToReadRepository
	// bookResp builds the shared bookResponse shape (available_copies,
	// borrow_count, your_to_read, etc.) for the "mine" listing, so it stays
	// in lockstep with GET /books rather than duplicating that assembly.
	bookResp *BookHandler
}

// NewToReadHandler creates a new ToReadHandler.
func NewToReadHandler(books repository.BookRepository, toReads repository.ToReadRepository, bookResp *BookHandler) *ToReadHandler {
	return &ToReadHandler{books: books, toReads: toReads, bookResp: bookResp}
}

type toReadBookInput struct {
	ID uint `path:"id" doc:"Book ID"`
}

type listMyToReadInput struct {
	PaginationParams
}

type listMyToReadOutput struct {
	Body struct {
		Items      []bookResponse `json:"items"`
		Total      int64          `json:"total"`
		Page       int            `json:"page"`
		PageSize   int            `json:"page_size"`
		TotalPages int            `json:"total_pages"`
	}
}

// RegisterRoutes registers all "to read" routes on the given huma API.
func (h *ToReadHandler) RegisterRoutes(api huma.API) {
	// /to-read/mine is registered before the /books/{id}/to-read routes
	// only as a matter of convention (no wildcard collision here, since
	// the latter is nested under /books), mirroring wishlist.go's
	// /wishlist/mine-before-/wishlist/{id} ordering note.
	huma.Register(api, huma.Operation{
		OperationID: "list-my-to-read",
		Method:      "GET",
		Path:        "/to-read/mine",
		Tags:        []string{"to-read"},
		Summary:     "List the caller's saved-to-read books, paginated",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.listMine)

	huma.Register(api, huma.Operation{
		OperationID:   "add-to-read",
		Method:        "POST",
		Path:          "/books/{id}/to-read",
		Tags:          []string{"to-read"},
		Summary:       "Save a book to read later",
		Security:      []map[string][]string{{"bearer": {}}},
		DefaultStatus: 201,
	}, h.add)

	huma.Register(api, huma.Operation{
		OperationID:   "remove-to-read",
		Method:        "DELETE",
		Path:          "/books/{id}/to-read",
		Tags:          []string{"to-read"},
		Summary:       "Remove a book from the caller's to-read list",
		Security:      []map[string][]string{{"bearer": {}}},
		DefaultStatus: 204,
	}, h.remove)
}

func (h *ToReadHandler) listMine(ctx context.Context, input *listMyToReadInput) (*listMyToReadOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	page, pageSize := input.normalize(20)
	result, err := h.toReads.ListByUserPaginated(userID, page, pageSize)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch to-read list")
	}

	items, err := h.bookResp.toBooksResponse(result.Items, userID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch book counts")
	}

	var out listMyToReadOutput
	out.Body.Items = items
	out.Body.Total = result.Total
	out.Body.Page = result.Page
	out.Body.PageSize = result.PageSize
	out.Body.TotalPages = result.TotalPages
	return &out, nil
}

func (h *ToReadHandler) add(ctx context.Context, input *toReadBookInput) (*struct{}, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}
	if _, err := h.books.GetByIDWithCopies(input.ID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("book not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch book")
	}
	if err := h.toReads.Add(input.ID, userID); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, huma.Error409Conflict("book is already on your to-read list")
		}
		return nil, huma.Error500InternalServerError("could not save book")
	}
	return nil, nil
}

func (h *ToReadHandler) remove(ctx context.Context, input *toReadBookInput) (*struct{}, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}
	if err := h.toReads.Remove(input.ID, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("book is not on your to-read list")
		}
		return nil, huma.Error500InternalServerError("could not remove book")
	}
	return nil, nil
}
