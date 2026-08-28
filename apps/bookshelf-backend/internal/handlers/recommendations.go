package handlers

import (
	"context"
	"errors"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// RecommendationHandler holds dependencies for the book-recommendation
// ("highly recommend this" thumbs-up) routes. See
// docs/book-recommendations-spec.md.
type RecommendationHandler struct {
	recommendations repository.RecommendationRepository
}

// NewRecommendationHandler creates a new RecommendationHandler.
func NewRecommendationHandler(recommendations repository.RecommendationRepository) *RecommendationHandler {
	return &RecommendationHandler{recommendations: recommendations}
}

// --- Input / Output types ---

type recommendationBookInput struct {
	ID uint `path:"id" doc:"Book ID"`
}

// recommendOutput carries a dynamic Status — huma treats a field literally
// named "Status" as the response status code, overriding DefaultStatus —
// since recommend must answer 201 on a new thumbs-up and 200 when the
// caller already recommended the book (idempotent re-tap).
type recommendOutput struct {
	Status int
}

// recommendationEntry is the wire shape for one recommender on a book —
// narrowed to a name, not the full models.User, same reasoning as
// wishlistResponse in wishlist.go.
type recommendationEntry struct {
	RecommenderName string    `json:"recommender_name"`
	CreatedAt       time.Time `json:"created_at"`
}

type listRecommendationsOutput struct {
	Body []recommendationEntry
}

// --- Route registration ---

// RegisterRoutes registers all recommendation routes on the given huma API.
func (h *RecommendationHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "recommend-book",
		Method:        "POST",
		Path:          "/books/{id}/recommendations",
		Tags:          []string{"books"},
		Summary:       "Add the caller's thumbs-up recommendation on a book",
		Security:      []map[string][]string{{"bearer": {}}},
		DefaultStatus: 201,
	}, h.recommend)

	huma.Register(api, huma.Operation{
		OperationID:   "unrecommend-book",
		Method:        "DELETE",
		Path:          "/books/{id}/recommendations",
		Tags:          []string{"books"},
		Summary:       "Remove the caller's thumbs-up recommendation on a book",
		Security:      []map[string][]string{{"bearer": {}}},
		DefaultStatus: 204,
	}, h.unrecommend)

	huma.Register(api, huma.Operation{
		OperationID: "list-book-recommendations",
		Method:      "GET",
		Path:        "/books/{id}/recommendations",
		Tags:        []string{"books"},
		Summary:     "List members who recommend a book, newest first",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.list)
}

// --- Handlers ---

func (h *RecommendationHandler) recommend(ctx context.Context, input *recommendationBookInput) (*recommendOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	if err := h.recommendations.Create(input.ID, userID); err != nil {
		// Two rapid taps racing on the unique (book_id, recommender_id)
		// constraint land here on the loser — that's the idempotent-success
		// path, not a failure. See docs/book-recommendations-plan.md § 3.
		if errors.Is(err, repository.ErrConflict) {
			return &recommendOutput{Status: 200}, nil
		}
		return nil, huma.Error500InternalServerError("could not add recommendation")
	}
	return &recommendOutput{Status: 201}, nil
}

func (h *RecommendationHandler) unrecommend(ctx context.Context, input *recommendationBookInput) (*struct{}, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	if err := h.recommendations.Delete(input.ID, userID); err != nil {
		return nil, huma.Error500InternalServerError("could not remove recommendation")
	}
	return nil, nil
}

func (h *RecommendationHandler) list(ctx context.Context, input *recommendationBookInput) (*listRecommendationsOutput, error) {
	if _, err := middleware.GetRequiredUserID(ctx); err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	recs, err := h.recommendations.ListByBookID(input.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch recommendations")
	}
	entries := make([]recommendationEntry, len(recs))
	for i, rec := range recs {
		entries[i] = recommendationEntry{RecommenderName: rec.Recommender.Name, CreatedAt: rec.CreatedAt}
	}
	return &listRecommendationsOutput{Body: entries}, nil
}
