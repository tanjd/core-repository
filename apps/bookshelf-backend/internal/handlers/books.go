package handlers

import (
	"context"
	"errors"

	"github.com/danielgtaylor/huma/v2"

	"github.com/rs/zerolog"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/services"
)

// BookHandler holds dependencies for book routes.
type BookHandler struct {
	books     repository.BookRepository
	users     repository.UserRepository
	coversDir string
	// wishlistWorkflow is optional (nil-safe) — see createBook — so
	// existing tests that construct a BookHandler without one keep working.
	wishlistWorkflow *services.WishlistWorkflow
}

// NewBookHandler creates a new BookHandler.
func NewBookHandler(books repository.BookRepository, users repository.UserRepository, coversDir string, wishlistWorkflow *services.WishlistWorkflow) *BookHandler {
	return &BookHandler{books: books, users: users, coversDir: coversDir, wishlistWorkflow: wishlistWorkflow}
}

// bookResponse wraps a Book and adds the computed available_copies count.
type bookResponse struct {
	models.Book
	AvailableCopies int64 `json:"available_copies"`
}

// --- Input / Output types ---

type listBooksInput struct {
	Q             string `query:"q" doc:"Search by title or author"`
	OLKey         string `query:"ol_key" doc:"Filter by exact Open Library key (returns single book)"`
	Sort          string `query:"sort" doc:"Sort order: title (default), author, newest"`
	AvailableOnly bool   `query:"available_only" doc:"Only return books with at least one available copy"`
	Page          int    `query:"page" minimum:"1" doc:"Page number (default 1)"`
	PageSize      int    `query:"page_size" minimum:"1" maximum:"100" doc:"Items per page (default 20)"`
}

type listBooksOutput struct {
	Body struct {
		Items      []bookResponse `json:"items"`
		Total      int64          `json:"total"`
		Page       int            `json:"page"`
		PageSize   int            `json:"page_size"`
		TotalPages int            `json:"total_pages"`
	}
}

type listRecentBooksInput struct {
	Limit int `query:"limit" minimum:"1" maximum:"50" doc:"Max books to return (default 16)"`
}

type listRecentBooksOutput struct{ Body []bookResponse }

type getBookInput struct {
	ID uint `path:"id" doc:"Book ID"`
}

type getBookOutput struct{ Body bookResponse }

type createBookInput struct {
	Body struct {
		Title         string `json:"title" required:"true" minLength:"1" doc:"Book title"`
		Author        string `json:"author,omitempty" doc:"Author name"`
		ISBN          string `json:"isbn,omitempty" doc:"ISBN-13"`
		OLKey         string `json:"ol_key,omitempty" doc:"Open Library key for deduplication"`
		CoverURL      string `json:"cover_url,omitempty" doc:"Cover image URL"`
		Description   string `json:"description,omitempty" doc:"Book description"`
		Publisher     string `json:"publisher,omitempty" doc:"Publisher name"`
		PublishedDate string `json:"published_date,omitempty" doc:"Publication date"`
		PageCount     int    `json:"page_count,omitempty" doc:"Number of pages"`
		Language      string `json:"language,omitempty" doc:"Language code"`
		GoogleBooksID string `json:"google_books_id,omitempty" doc:"Google Books volume ID for deduplication"`
	}
}

type createBookOutput struct{ Body models.Book }

// --- Route registration ---

// RegisterRoutes registers all book routes on the given huma API.
func (h *BookHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-books",
		Method:      "GET",
		Path:        "/books",
		Tags:        []string{"books"},
		Summary:     "List books with optional search, sort, filter, and pagination",
	}, h.listBooks)

	huma.Register(api, huma.Operation{
		OperationID: "list-recent-books",
		Method:      "GET",
		Path:        "/books/recent",
		Tags:        []string{"books"},
		Summary:     "List recently added books (for the new arrivals shelf)",
	}, h.listRecentBooks)

	huma.Register(api, huma.Operation{
		OperationID: "get-book",
		Method:      "GET",
		Path:        "/books/{id}",
		Tags:        []string{"books"},
		Summary:     "Get a book by ID",
	}, h.getBook)

	huma.Register(api, huma.Operation{
		OperationID:   "create-book",
		Method:        "POST",
		Path:          "/books",
		Tags:          []string{"books"},
		Summary:       "Create or upsert a book by Open Library key",
		Security:      []map[string][]string{{"bearer": {}}},
		DefaultStatus: 201,
	}, h.createBook)
}

// --- Handlers ---

func (h *BookHandler) listBooks(_ context.Context, input *listBooksInput) (*listBooksOutput, error) {
	if input.OLKey != "" {
		book, err := h.books.FindByOLKey(input.OLKey)
		if err != nil {
			return nil, huma.Error404NotFound("book not found")
		}
		var out listBooksOutput
		out.Body.Items = []bookResponse{h.toBookResponse(*book)}
		out.Body.Total = 1
		out.Body.Page = 1
		out.Body.PageSize = 1
		out.Body.TotalPages = 1
		return &out, nil
	}

	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	result, err := h.books.ListPaginated(input.Q, input.Sort, input.AvailableOnly, page, pageSize)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch books")
	}

	items, err := h.toBooksResponse(result.Items)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch book counts")
	}

	var out listBooksOutput
	out.Body.Items = items
	out.Body.Total = result.Total
	out.Body.Page = result.Page
	out.Body.PageSize = result.PageSize
	out.Body.TotalPages = result.TotalPages
	return &out, nil
}

func (h *BookHandler) listRecentBooks(_ context.Context, input *listRecentBooksInput) (*listRecentBooksOutput, error) {
	limit := input.Limit
	if limit < 1 {
		limit = 16
	}
	books, err := h.books.ListRecent(limit)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch recent books")
	}
	resp, err := h.toBooksResponse(books)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch book counts")
	}
	return &listRecentBooksOutput{Body: resp}, nil
}

func (h *BookHandler) getBook(ctx context.Context, input *getBookInput) (*getBookOutput, error) {
	book, err := h.books.GetByIDWithCopies(input.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("book not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch book")
	}

	// Determine whether the requesting user can see owner names.
	// Requires: authenticated + email-verified.
	canSeeOwner := false
	if userID := middleware.GetUserID(ctx); userID != 0 {
		if u, lookupErr := h.users.FindByID(userID); lookupErr == nil && u.Verified {
			canSeeOwner = true
		}
	}

	for i := range book.Copies {
		if book.Copies[i].HideOwner || !canSeeOwner {
			// Strip owner entirely — frontend uses hide_owner flag to distinguish
			// "anonymous by choice" from "sign in and verify required".
			book.Copies[i].Owner = models.User{}
		} else {
			// Redact sensitive fields — expose name only.
			book.Copies[i].Owner = models.User{
				ID:   book.Copies[i].Owner.ID,
				Name: book.Copies[i].Owner.Name,
			}
		}
	}

	return &getBookOutput{Body: h.toBookResponse(*book)}, nil
}

func (h *BookHandler) createBook(ctx context.Context, input *createBookInput) (*createBookOutput, error) {
	if _, err := middleware.GetRequiredUserID(ctx); err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	// Upsert by ol_key or google_books_id: return existing record when known.
	if input.Body.OLKey != "" {
		if existing, err := h.books.FindByOLKey(input.Body.OLKey); err == nil {
			return &createBookOutput{Body: *existing}, nil
		}
	}
	if input.Body.GoogleBooksID != "" {
		if existing, err := h.books.FindByGoogleBooksID(input.Body.GoogleBooksID); err == nil {
			return &createBookOutput{Body: *existing}, nil
		}
	}

	coverURL := input.Body.CoverURL
	if h.coversDir != "" && coverURL != "" {
		if local, err := downloadCover(ctx, coverURL, h.coversDir); err != nil {
			zerolog.Ctx(ctx).Warn().Err(err).Msg("cover download failed, keeping external url")
		} else if local != "" {
			coverURL = local
		}
	}

	book := models.Book{
		Title:         input.Body.Title,
		Author:        input.Body.Author,
		ISBN:          input.Body.ISBN,
		OLKey:         input.Body.OLKey,
		CoverURL:      coverURL,
		Description:   input.Body.Description,
		Publisher:     input.Body.Publisher,
		PublishedDate: input.Body.PublishedDate,
		PageCount:     input.Body.PageCount,
		Language:      input.Body.Language,
		GoogleBooksID: input.Body.GoogleBooksID,
	}
	if err := h.books.Create(&book); err != nil {
		return nil, huma.Error500InternalServerError("could not create book")
	}

	if h.wishlistWorkflow != nil {
		h.wishlistWorkflow.OnBookCreated(ctx, &book) // log-and-continue; never blocks book creation
	}

	return &createBookOutput{Body: book}, nil
}

// toBookResponse computes the available_copies count for a single book.
// Prefer toBooksResponse for list operations to avoid N+1 queries.
func (h *BookHandler) toBookResponse(book models.Book) bookResponse {
	count, _ := h.books.CountAvailableCopies(book.ID)
	return bookResponse{Book: book, AvailableCopies: count}
}

// toBooksResponse fetches available copy counts for all books in a single
// batch query and returns the assembled responses.
func (h *BookHandler) toBooksResponse(books []models.Book) ([]bookResponse, error) {
	ids := make([]uint, len(books))
	for i, b := range books {
		ids[i] = b.ID
	}
	counts, err := h.books.CountAvailableCopiesBatch(ids)
	if err != nil {
		return nil, err
	}
	resp := make([]bookResponse, len(books))
	for i, b := range books {
		resp[i] = bookResponse{Book: b, AvailableCopies: counts[b.ID]}
	}
	return resp, nil
}
