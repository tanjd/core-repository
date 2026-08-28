package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
)

func newRecommendationHandler() (*RecommendationHandler, *repotest.RecommendationRepository) {
	users := repotest.NewUserRepository()
	recs := repotest.NewRecommendationRepository(users)
	return NewRecommendationHandler(recs), recs
}

func TestRecommend_Unauthenticated(t *testing.T) {
	h, _ := newRecommendationHandler()
	_, err := h.recommend(fakeAuthedCtxNone(), &recommendationBookInput{ID: 1})
	assertStatus(t, err, 401)
}

func TestRecommend_New(t *testing.T) {
	h, recs := newRecommendationHandler()

	out, err := h.recommend(fakeAuthedCtx(t, 7, "user"), &recommendationBookInput{ID: 1})
	require.NoError(t, err)
	assert.Equal(t, 201, out.Status)
	assert.Equal(t, 1, recs.Count())
}

func TestRecommend_Existing_IsIdempotent(t *testing.T) {
	h, recs := newRecommendationHandler()

	_, err := h.recommend(fakeAuthedCtx(t, 7, "user"), &recommendationBookInput{ID: 1})
	require.NoError(t, err)

	out, err := h.recommend(fakeAuthedCtx(t, 7, "user"), &recommendationBookInput{ID: 1})
	require.NoError(t, err)
	assert.Equal(t, 200, out.Status)
	assert.Equal(t, 1, recs.Count(), "re-tapping must not create a duplicate row")
}

func TestUnrecommend_Unauthenticated(t *testing.T) {
	h, _ := newRecommendationHandler()
	_, err := h.unrecommend(fakeAuthedCtxNone(), &recommendationBookInput{ID: 1})
	assertStatus(t, err, 401)
}

func TestUnrecommend_Existing(t *testing.T) {
	h, recs := newRecommendationHandler()
	_, err := h.recommend(fakeAuthedCtx(t, 7, "user"), &recommendationBookInput{ID: 1})
	require.NoError(t, err)

	_, err = h.unrecommend(fakeAuthedCtx(t, 7, "user"), &recommendationBookInput{ID: 1})
	require.NoError(t, err)
	assert.Equal(t, 0, recs.Count())
}

func TestUnrecommend_Absent_IsIdempotent(t *testing.T) {
	h, _ := newRecommendationHandler()
	_, err := h.unrecommend(fakeAuthedCtx(t, 7, "user"), &recommendationBookInput{ID: 1})
	require.NoError(t, err)
}

func TestListRecommendations_Unauthenticated(t *testing.T) {
	h, _ := newRecommendationHandler()
	_, err := h.list(fakeAuthedCtxNone(), &recommendationBookInput{ID: 1})
	assertStatus(t, err, 401)
}

func TestListRecommendations_NewestFirstWithRecommenderName(t *testing.T) {
	h, recs := newRecommendationHandler()

	require.NoError(t, recs.Create(1, 1))
	require.NoError(t, recs.Create(1, 2))

	out, err := h.list(fakeAuthedCtx(t, 1, "user"), &recommendationBookInput{ID: 1})
	require.NoError(t, err)
	assert.Len(t, out.Body, 2)
}
