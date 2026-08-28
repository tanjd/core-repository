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
	// recommendations is optional (nil-safe), same reasoning as
	// wishlistWorkflow — a nil value degrades recommendation_count/
	// your_recommendation to their zero values rather than panicking.
	recommendations repository.RecommendationRepository
}

// NewBookHandler creates a new BookHandler.
func NewBookHandler(books repository.BookRepository, users repository.UserRepository, coversDir string, wishlistWorkflow *services.WishlistWorkflow, recommendations repository.RecommendationRepository) *BookHandler {
	return &BookHandler{books: books, users: users, coversDir: coversDir, wishlistWorkflow: wishlistWorkflow, recommendations: recommendations}
}

// bookResponse wraps a Book and adds the computed availability +
// community-reading-activity counts. BorrowCount is the number of
// LoanRequests against any copy of this book that reached "accepted" or
// "returned" (pending/rejected/cancelled don't count — same rule as
// admin_repo.go's MostBorrowedBooks). WaitlistCount is the live waitlist
// depth across every copy. See docs/community-reading-activity-spec.md.
// RecommendationCount and YourRecommendation are the book-recommendations
// feature's count + viewer-relative flag — see
// docs/book-recommendations-spec.md. YourRecommendation follows the same
// canSeeOwner-style degradation as getBook: false when there's no session.
type bookResponse struct {
	models.Book
	AvailableCopies     int64 `json:"available_copies"`
	BorrowCount         int64 `json:"borrow_count"`
	WaitlistCount       int64 `json:"waitlist_count"`
	RecommendationCount int64 `json:"recommendation_count"`
	YourRecommendation  bool  `json:"your_recommendation"`
}

// --- Input / Output types ---

type listBooksInput struct {
	Q             string `query:"q" doc:"Search by title or author"`
	OLKey         string `query:"ol_key" doc:"Filter by exact Open Library key (returns single book)"`
	Sort          string `query:"sort" doc:"Sort order: title (default), author, newest, popular, recommended, relevance (best-match, only meaningful with q)"`
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
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.listBooks)

	huma.Register(api, huma.Operation{
		OperationID: "list-recent-books",
		Method:      "GET",
		Path:        "/books/recent",
		Tags:        []string{"books"},
		Summary:     "List recently added books (for the new arrivals shelf)",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.listRecentBooks)

	huma.Register(api, huma.Operation{
		OperationID: "get-book",
		Method:      "GET",
		Path:        "/books/{id}",
		Tags:        []string{"books"},
		Summary:     "Get a book by ID",
		Security:    []map[string][]string{{"bearer": {}}},
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

func (h *BookHandler) listBooks(ctx context.Context, input *listBooksInput) (*listBooksOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	if input.OLKey != "" {
		book, err := h.books.FindByOLKey(input.OLKey)
		if err != nil {
			return nil, huma.Error404NotFound("book not found")
		}
		var out listBooksOutput
		out.Body.Items = []bookResponse{h.toBookResponse(*book, userID)}
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

	items, err := h.toBooksResponse(result.Items, userID)
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

func (h *BookHandler) listRecentBooks(ctx context.Context, input *listRecentBooksInput) (*listRecentBooksOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	limit := input.Limit
	if limit < 1 {
		limit = 16
	}
	books, err := h.books.ListRecent(limit)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch recent books")
	}
	resp, err := h.toBooksResponse(books, userID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch book counts")
	}
	return &listRecentBooksOutput{Body: resp}, nil
}

func (h *BookHandler) getBook(ctx context.Context, input *getBookInput) (*getBookOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	book, err := h.books.GetByIDWithCopies(input.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("book not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch book")
	}

	// Owner names are only revealed to email-verified members — an authenticated
	// but unverified caller still gets them stripped. HideOwner (member-choice
	// anonymity, per Copy) takes precedence regardless.
	u, lookupErr := h.users.FindByID(userID)
	canSeeOwner := lookupErr == nil && u.Verified

	for i := range book.Copies {
		if book.Copies[i].HideOwner || !canSeeOwner {
			book.Copies[i].Owner = models.User{}
		} else {
			book.Copies[i].Owner = models.User{
				ID:   book.Copies[i].Owner.ID,
				Name: book.Copies[i].Owner.Name,
			}
		}
	}

	return &getBookOutput{Body: h.toBookResponse(*book, userID)}, nil
}

func (h *BookHandler) createBook(ctx context.Context, input *createBookInput) (*createBookOutput, error) {
	if _, err := middleware.GetRequiredUserID(ctx); err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	if existing, err := findExistingBook(h.books, input.Body.OLKey, input.Body.GoogleBooksID, input.Body.ISBN); err == nil {
		h.backfillCover(ctx, existing, input.Body.CoverURL)
		return &createBookOutput{Body: *existing}, nil
	}

	coverURL := input.Body.CoverURL
	if h.coversDir != "" && coverURL != "" {
		if local, err := services.DownloadCover(ctx, coverURL, h.coversDir); err != nil {
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

// backfillCover fills in existing.CoverURL when a book matched by
// findExistingBook predates a cover (e.g. it was first added before one was
// available, or its last copy was removed and it's now being re-added —
// keyed books survive that as an orphaned Book row, see
// maybeDeleteOrphanedBook in copies.go). Without this, the caller's freshly
// supplied CoverURL is silently dropped in favor of the empty fallback
// forever — refreshBookCover's scheduled job only re-caches an
// already-set CoverURL, it never populates one that's empty.
func (h *BookHandler) backfillCover(ctx context.Context, existing *models.Book, newCoverURL string) {
	if existing.CoverURL != "" || newCoverURL == "" {
		return
	}
	coverURL := newCoverURL
	if h.coversDir != "" {
		if local, err := services.DownloadCover(ctx, coverURL, h.coversDir); err != nil {
			zerolog.Ctx(ctx).Warn().Err(err).Msg("cover download failed, keeping external url")
		} else if local != "" {
			coverURL = local
		}
	}
	existing.CoverURL = coverURL
	if err := h.books.Save(existing); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("failed to backfill cover on existing book")
	}
}

// findExistingBook implements createBook's upsert precedence — also reused
// by CopyHandler's book-import path (copies_import.go) so both entry points
// dedup against the catalog identically. A strong external key (OL key or
// Google Books ID) is trusted first and, when present, an ISBN match is
// never even consulted — ISBN alone can be shared across distinct
// editions/omnibuses, so unconditionally matching on it risks merging books
// that a strong key would have kept separate. ISBN is used only as a
// fallback when neither strong key is present — exactly the case metadata
// search's BookBrainz source hits (it never sets OLKey/GoogleBooksID) and
// the case a raw scanned/manually-entered ISBN hits. This is a deliberate,
// narrower-scoped divergence from WishlistWorkflow.OnBookCreated's stricter
// OLKey/GoogleBooksID-only matching (see apps/bookshelf-backend/CLAUDE.md) —
// that auto-match silently notifies a requester their book arrived, a much
// higher-stakes mistake than this dedup only affecting whether a second Copy
// attaches to an existing Book.
func findExistingBook(books repository.BookRepository, olKey, googleBooksID, isbn string) (*models.Book, error) {
	if olKey != "" {
		if b, err := books.FindByOLKey(olKey); err == nil {
			return b, nil
		}
	}
	if googleBooksID != "" {
		if b, err := books.FindByGoogleBooksID(googleBooksID); err == nil {
			return b, nil
		}
	}
	if olKey == "" && googleBooksID == "" && isbn != "" {
		if b, err := books.FindByISBN(isbn); err == nil {
			return b, nil
		}
	}
	return nil, repository.ErrNotFound
}

// toBookResponse computes the available_copies + borrow + waitlist +
// recommendation counts for a single book, reusing the batch queries with a
// one-element slice. Prefer toBooksResponse for list operations to avoid
// N+1 queries. userID is 0 for an anonymous caller — see toBooksResponse.
func (h *BookHandler) toBookResponse(book models.Book, userID uint) bookResponse {
	resp, err := h.toBooksResponse([]models.Book{book}, userID)
	if err != nil || len(resp) == 0 {
		// Errors from the counters degrade to zeroes rather than failing
		// the request — same forgiving contract the previous
		// CountAvailableCopies call had (its error was silently discarded).
		return bookResponse{Book: book}
	}
	return resp[0]
}

// toBooksResponse fetches available copy + borrow + waitlist +
// recommendation counts for all books in batched queries and returns the
// assembled responses. userID is the requesting caller's ID (0 for
// anonymous) — YourRecommendation is always false for userID 0, and the
// HasRecommendedBatch query is skipped entirely in that case.
func (h *BookHandler) toBooksResponse(books []models.Book, userID uint) ([]bookResponse, error) {
	ids := make([]uint, len(books))
	for i, b := range books {
		ids[i] = b.ID
	}
	available, err := h.books.CountAvailableCopiesBatch(ids)
	if err != nil {
		return nil, err
	}
	borrows, err := h.books.CountBorrowsBatch(ids)
	if err != nil {
		return nil, err
	}
	waitlists, err := h.books.CountWaitlistBatch(ids)
	if err != nil {
		return nil, err
	}

	var recCounts map[uint]int64
	var yourRecs map[uint]bool
	if h.recommendations != nil {
		recCounts, err = h.recommendations.CountByBookBatch(ids)
		if err != nil {
			return nil, err
		}
		if userID != 0 {
			yourRecs, err = h.recommendations.HasRecommendedBatch(userID, ids)
			if err != nil {
				return nil, err
			}
		}
	}

	resp := make([]bookResponse, len(books))
	for i, b := range books {
		resp[i] = bookResponse{
			Book:                b,
			AvailableCopies:     available[b.ID],
			BorrowCount:         borrows[b.ID],
			WaitlistCount:       waitlists[b.ID],
			RecommendationCount: recCounts[b.ID],
			YourRecommendation:  yourRecs[b.ID],
		}
	}
	return resp, nil
}
