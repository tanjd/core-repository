package handlers

import (
	"context"
	"errors"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/services"
)

// WishlistHandler holds dependencies for the wishlist board routes.
type WishlistHandler struct {
	requests repository.WishlistRequestRepository
	books    repository.BookRepository
	workflow *services.WishlistWorkflow
}

// NewWishlistHandler creates a new WishlistHandler.
func NewWishlistHandler(
	requests repository.WishlistRequestRepository,
	books repository.BookRepository,
	workflow *services.WishlistWorkflow,
) *WishlistHandler {
	return &WishlistHandler{requests: requests, books: books, workflow: workflow}
}

// --- Input / Output types ---

type createWishlistRequestInput struct {
	Body struct {
		Title         string `json:"title" required:"true" minLength:"1" doc:"Book title"`
		Author        string `json:"author" required:"true" minLength:"1" doc:"Author name"`
		ISBN          string `json:"isbn,omitempty" doc:"ISBN-13"`
		OLKey         string `json:"ol_key,omitempty" doc:"Open Library key"`
		GoogleBooksID string `json:"google_books_id,omitempty" doc:"Google Books volume ID"`
		CoverURL      string `json:"cover_url,omitempty" doc:"Cover image URL"`
		Notes         string `json:"notes,omitempty" doc:"Optional note, e.g. preferred edition"`
		IsAnonymous   bool   `json:"is_anonymous,omitempty" doc:"Hide the requester's identity from other members"`
	}
}

// wishlistResponse is the wire shape for a WishlistRequest — unlike the raw
// model (whose Requester is a full models.User), it narrows the requester to
// a safeUser and, when the request is anonymous, redacts it entirely for
// anyone but the requester or an admin. Same reasoning as loanRequestCopyResponse
// in loan_requests.go: never serialize models.User directly.
type wishlistResponse struct {
	ID              uint         `json:"id"`
	RequesterID     uint         `json:"requester_id"`
	Title           string       `json:"title"`
	Author          string       `json:"author"`
	ISBN            string       `json:"isbn"`
	OLKey           string       `json:"ol_key"`
	GoogleBooksID   string       `json:"google_books_id"`
	CoverURL        string       `json:"cover_url"`
	Notes           string       `json:"notes"`
	Status          string       `json:"status"`
	IsAnonymous     bool         `json:"is_anonymous"`
	FulfilledBookID *uint        `json:"fulfilled_book_id"`
	FulfilledAt     *time.Time   `json:"fulfilled_at"`
	CreatedAt       time.Time    `json:"created_at"`
	Requester       safeUser     `json:"requester"`
	FulfilledBook   *models.Book `json:"fulfilled_book,omitempty"`
}

// toWishlistResponse redacts the requester's name when the request is
// anonymous, unless the viewer is the requester themselves or an admin (who
// still need the real identity for moderation).
func toWishlistResponse(req models.WishlistRequest, viewerID uint, isAdmin bool) wishlistResponse {
	requester := safeUser{ID: req.Requester.ID, Name: req.Requester.Name}
	if req.IsAnonymous && req.RequesterID != viewerID && !isAdmin {
		requester = safeUser{Name: "Anonymous member"}
	}
	return wishlistResponse{
		ID:              req.ID,
		RequesterID:     req.RequesterID,
		Title:           req.Title,
		Author:          req.Author,
		ISBN:            req.ISBN,
		OLKey:           req.OLKey,
		GoogleBooksID:   req.GoogleBooksID,
		CoverURL:        req.CoverURL,
		Notes:           req.Notes,
		Status:          req.Status,
		IsAnonymous:     req.IsAnonymous,
		FulfilledBookID: req.FulfilledBookID,
		FulfilledAt:     req.FulfilledAt,
		CreatedAt:       req.CreatedAt,
		Requester:       requester,
		FulfilledBook:   req.FulfilledBook,
	}
}

type wishlistRequestOutput struct{ Body wishlistResponse }

type listWishlistInput struct {
	Q        string `query:"q" doc:"Search by title or author"`
	Page     int    `query:"page" minimum:"1" doc:"Page number (default 1)"`
	PageSize int    `query:"page_size" minimum:"1" maximum:"100" doc:"Items per page (default 20)"`
}

type listWishlistOutput struct {
	Body struct {
		Items      []wishlistResponse `json:"items"`
		Total      int64              `json:"total"`
		Page       int                `json:"page"`
		PageSize   int                `json:"page_size"`
		TotalPages int                `json:"total_pages"`
	}
}

type listMyWishlistOutput struct{ Body []wishlistResponse }

type checkWishlistInput struct {
	ISBN          string `query:"isbn" doc:"ISBN-13"`
	OLKey         string `query:"ol_key" doc:"Open Library key"`
	GoogleBooksID string `query:"google_books_id" doc:"Google Books volume ID"`
}

type checkWishlistOutput struct {
	Body struct {
		// Match is the earliest open request already sharing one of the
		// given keys, or null if none exists — lets the client warn a
		// member before they post a duplicate.
		Match *wishlistResponse `json:"match"`
	}
}

type wishlistIDInput struct {
	ID uint `path:"id" doc:"Wishlist request ID"`
}

type fulfillWishlistInput struct {
	ID   uint `path:"id" doc:"Wishlist request ID"`
	Body struct {
		BookID uint `json:"book_id" required:"true" doc:"Existing catalog Book this request is fulfilled by"`
	}
}

// --- Route registration ---

// RegisterRoutes registers the wishlist board routes on the given huma API.
func (h *WishlistHandler) RegisterRoutes(api huma.API) {
	security := []map[string][]string{{"bearer": {}}}

	huma.Register(api, huma.Operation{
		OperationID:   "create-wishlist-request",
		Method:        "POST",
		Path:          "/wishlist",
		Tags:          []string{"wishlist"},
		Summary:       "Post a request for a book not currently in the catalog",
		Security:      security,
		DefaultStatus: 201,
	}, h.create)

	huma.Register(api, huma.Operation{
		OperationID: "list-wishlist-requests",
		Method:      "GET",
		Path:        "/wishlist",
		Tags:        []string{"wishlist"},
		Summary:     "Browse open wishlist requests",
		Security:    security,
	}, h.list)

	// Registered before the /wishlist/{id} wildcard so this literal path
	// isn't swallowed by it — same ordering care as notifications.go.
	huma.Register(api, huma.Operation{
		OperationID: "list-my-wishlist-requests",
		Method:      "GET",
		Path:        "/wishlist/mine",
		Tags:        []string{"wishlist"},
		Summary:     "List the caller's own wishlist requests",
		Security:    security,
	}, h.listMine)

	// Registered before the /wishlist/{id} wildcard, same reasoning as
	// /wishlist/mine above.
	huma.Register(api, huma.Operation{
		OperationID: "check-wishlist-request",
		Method:      "GET",
		Path:        "/wishlist/check",
		Tags:        []string{"wishlist"},
		Summary:     "Check whether an open request already exists for a book, by external key",
		Security:    security,
	}, h.check)

	huma.Register(api, huma.Operation{
		OperationID: "get-wishlist-request",
		Method:      "GET",
		Path:        "/wishlist/{id}",
		Tags:        []string{"wishlist"},
		Summary:     "Get a wishlist request by ID",
		Security:    security,
	}, h.get)

	huma.Register(api, huma.Operation{
		OperationID:   "cancel-wishlist-request",
		Method:        "DELETE",
		Path:          "/wishlist/{id}",
		Tags:          []string{"wishlist"},
		Summary:       "Cancel an open wishlist request (requester or admin)",
		Security:      security,
		DefaultStatus: 204,
	}, h.cancel)

	huma.Register(api, huma.Operation{
		OperationID: "fulfill-wishlist-request",
		Method:      "POST",
		Path:        "/wishlist/{id}/fulfill",
		Tags:        []string{"wishlist"},
		Summary:     "Manually link an open wishlist request to an existing catalog book",
		Security:    security,
	}, h.fulfill)
}

// --- Handlers ---

func (h *WishlistHandler) create(ctx context.Context, input *createWishlistRequestInput) (*wishlistRequestOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	if input.Body.OLKey == "" && input.Body.GoogleBooksID == "" && input.Body.ISBN == "" {
		return nil, huma.Error400BadRequest("at least one of ol_key, google_books_id, or isbn is required")
	}

	req := &models.WishlistRequest{
		RequesterID:   userID,
		Title:         input.Body.Title,
		Author:        input.Body.Author,
		ISBN:          input.Body.ISBN,
		OLKey:         input.Body.OLKey,
		GoogleBooksID: input.Body.GoogleBooksID,
		CoverURL:      input.Body.CoverURL,
		Notes:         input.Body.Notes,
		Status:        "open",
		IsAnonymous:   input.Body.IsAnonymous,
	}
	if err := h.requests.Create(req); err != nil {
		return nil, huma.Error500InternalServerError("could not create wishlist request")
	}
	return &wishlistRequestOutput{Body: toWishlistResponse(*req, userID, false)}, nil
}

func (h *WishlistHandler) list(ctx context.Context, input *listWishlistInput) (*listWishlistOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}
	isAdmin := middleware.GetUserRole(ctx) == "admin"

	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	result, err := h.requests.ListOpenPaginated(input.Q, page, pageSize)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not list wishlist requests")
	}
	var out listWishlistOutput
	out.Body.Items = make([]wishlistResponse, len(result.Items))
	for i, item := range result.Items {
		out.Body.Items[i] = toWishlistResponse(item, userID, isAdmin)
	}
	out.Body.Total = result.Total
	out.Body.Page = result.Page
	out.Body.PageSize = result.PageSize
	out.Body.TotalPages = result.TotalPages
	return &out, nil
}

func (h *WishlistHandler) check(ctx context.Context, input *checkWishlistInput) (*checkWishlistOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}
	isAdmin := middleware.GetUserRole(ctx) == "admin"
	match, err := h.requests.FindOpenMatch(input.ISBN, input.OLKey, input.GoogleBooksID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not check for an existing request")
	}
	out := &checkWishlistOutput{}
	if match != nil {
		resp := toWishlistResponse(*match, userID, isAdmin)
		out.Body.Match = &resp
	}
	return out, nil
}

func (h *WishlistHandler) listMine(ctx context.Context, _ *struct{}) (*listMyWishlistOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}
	items, err := h.requests.ListByRequesterID(userID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not list your wishlist requests")
	}
	out := make([]wishlistResponse, len(items))
	for i, item := range items {
		out[i] = toWishlistResponse(item, userID, false)
	}
	return &listMyWishlistOutput{Body: out}, nil
}

func (h *WishlistHandler) get(ctx context.Context, input *wishlistIDInput) (*wishlistRequestOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}
	isAdmin := middleware.GetUserRole(ctx) == "admin"
	req, err := h.requests.GetByID(input.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("wishlist request not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch wishlist request")
	}
	return &wishlistRequestOutput{Body: toWishlistResponse(*req, userID, isAdmin)}, nil
}

func (h *WishlistHandler) cancel(ctx context.Context, input *wishlistIDInput) (*struct{}, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	req, err := h.requests.GetByID(input.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("wishlist request not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch wishlist request")
	}

	if req.RequesterID != userID {
		if adminErr := middleware.RequireAdmin(ctx); adminErr != nil {
			return nil, huma.Error403Forbidden("only the requester or an admin can cancel this request")
		}
	}

	if req.Status != "open" {
		return nil, huma.Error400BadRequest("only an open request can be cancelled")
	}

	req.Status = "cancelled"
	if err := h.requests.Save(req); err != nil {
		return nil, huma.Error500InternalServerError("could not cancel wishlist request")
	}
	return nil, nil
}

func (h *WishlistHandler) fulfill(ctx context.Context, input *fulfillWishlistInput) (*wishlistRequestOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}
	isAdmin := middleware.GetUserRole(ctx) == "admin"

	req, err := h.requests.GetByID(input.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("wishlist request not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch wishlist request")
	}
	if req.Status != "open" {
		return nil, huma.Error400BadRequest("only an open request can be fulfilled")
	}

	book, err := h.books.GetByIDWithCopies(input.Body.BookID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("book not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch book")
	}

	h.workflow.OnFulfilled(ctx, req, book)

	return &wishlistRequestOutput{Body: toWishlistResponse(*req, userID, isAdmin)}, nil
}
