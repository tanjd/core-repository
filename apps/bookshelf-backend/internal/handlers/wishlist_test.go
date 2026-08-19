package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/services"
)

func newWishlistHandler() (*WishlistHandler, *repotest.WishlistRequestRepository, *repotest.NotificationRepository, *repotest.UserRepository, *repotest.BookRepository) {
	requests := repotest.NewWishlistRequestRepository()
	notifs := repotest.NewNotificationRepository()
	users := repotest.NewUserRepository()
	books := repotest.NewBookRepository()
	email := noopEmail()
	workflow := services.NewWishlistWorkflow(requests, notifs, users, email)
	return NewWishlistHandler(requests, books, workflow), requests, notifs, users, books
}

func validCreateInput() *createWishlistRequestInput {
	input := &createWishlistRequestInput{}
	input.Body.Title = "Wanted Book"
	input.Body.Author = "Some Author"
	input.Body.OLKey = "OL123"
	return input
}

func TestCreateWishlistRequest(t *testing.T) {
	h, repo, _, _, _ := newWishlistHandler()

	t.Run("unauthenticated is unauthorized", func(t *testing.T) {
		_, err := h.create(fakeAuthedCtxNone(), validCreateInput())
		assertStatus(t, err, 401)
	})

	t.Run("requires at least one external key", func(t *testing.T) {
		input := &createWishlistRequestInput{}
		input.Body.Title = "T"
		input.Body.Author = "A"
		_, err := h.create(fakeAuthedCtx(t, 1, "user"), input)
		assertStatus(t, err, 400)
	})

	t.Run("creates an open request owned by the caller", func(t *testing.T) {
		out, err := h.create(fakeAuthedCtx(t, 5, "user"), validCreateInput())
		require.NoError(t, err)
		assert.Equal(t, "open", out.Body.Status)
		assert.Equal(t, uint(5), out.Body.RequesterID)

		stored, err := repo.GetByID(out.Body.ID)
		require.NoError(t, err)
		assert.Equal(t, "Wanted Book", stored.Title)
	})
}

func TestListWishlistRequests(t *testing.T) {
	h, repo, _, _, _ := newWishlistHandler()
	require.NoError(t, repo.Create(&models.WishlistRequest{RequesterID: 1, Title: "Open One", Author: "A", OLKey: "OL1", Status: "open"}))
	require.NoError(t, repo.Create(&models.WishlistRequest{RequesterID: 1, Title: "Fulfilled One", Author: "A", OLKey: "OL2", Status: "fulfilled"}))

	t.Run("unauthenticated is unauthorized", func(t *testing.T) {
		_, err := h.list(fakeAuthedCtxNone(), &listWishlistInput{})
		assertStatus(t, err, 401)
	})

	t.Run("only lists open requests", func(t *testing.T) {
		out, err := h.list(fakeAuthedCtx(t, 1, "user"), &listWishlistInput{})
		require.NoError(t, err)
		require.Len(t, out.Body.Items, 1)
		assert.Equal(t, "Open One", out.Body.Items[0].Title)
	})
}

func TestCheckWishlistRequest(t *testing.T) {
	h, repo, _, _, _ := newWishlistHandler()

	t.Run("unauthenticated is unauthorized", func(t *testing.T) {
		_, err := h.check(fakeAuthedCtxNone(), &checkWishlistInput{OLKey: "OL1"})
		assertStatus(t, err, 401)
	})

	t.Run("no match returns a null match", func(t *testing.T) {
		out, err := h.check(fakeAuthedCtx(t, 1, "user"), &checkWishlistInput{OLKey: "nonexistent"})
		require.NoError(t, err)
		assert.Nil(t, out.Body.Match)
	})

	t.Run("returns the matching open request by external key", func(t *testing.T) {
		require.NoError(t, repo.Create(&models.WishlistRequest{RequesterID: 5, Title: "Wanted Book", Author: "A", OLKey: "OL1", Status: "open"}))

		out, err := h.check(fakeAuthedCtx(t, 1, "user"), &checkWishlistInput{OLKey: "OL1"})
		require.NoError(t, err)
		require.NotNil(t, out.Body.Match)
		assert.Equal(t, "Wanted Book", out.Body.Match.Title)
	})
}

func TestCancelWishlistRequest(t *testing.T) {
	h, repo, _, _, _ := newWishlistHandler()

	t.Run("requester can cancel their own open request", func(t *testing.T) {
		req := &models.WishlistRequest{RequesterID: 5, Title: "T", Author: "A", OLKey: "OL1", Status: "open"}
		require.NoError(t, repo.Create(req))

		_, err := h.cancel(fakeAuthedCtx(t, 5, "user"), &wishlistIDInput{ID: req.ID})
		require.NoError(t, err)

		reloaded, err := repo.GetByID(req.ID)
		require.NoError(t, err)
		assert.Equal(t, "cancelled", reloaded.Status)
	})

	t.Run("a different non-admin user is forbidden", func(t *testing.T) {
		req := &models.WishlistRequest{RequesterID: 5, Title: "T", Author: "A", OLKey: "OL1", Status: "open"}
		require.NoError(t, repo.Create(req))

		_, err := h.cancel(fakeAuthedCtx(t, 6, "user"), &wishlistIDInput{ID: req.ID})
		assertStatus(t, err, 403)
	})

	t.Run("an admin can cancel someone else's request", func(t *testing.T) {
		req := &models.WishlistRequest{RequesterID: 5, Title: "T", Author: "A", OLKey: "OL1", Status: "open"}
		require.NoError(t, repo.Create(req))

		_, err := h.cancel(fakeAuthedCtx(t, 99, "admin"), &wishlistIDInput{ID: req.ID})
		require.NoError(t, err)
	})

	t.Run("cannot cancel an already-fulfilled request", func(t *testing.T) {
		req := &models.WishlistRequest{RequesterID: 5, Title: "T", Author: "A", OLKey: "OL1", Status: "fulfilled"}
		require.NoError(t, repo.Create(req))

		_, err := h.cancel(fakeAuthedCtx(t, 5, "user"), &wishlistIDInput{ID: req.ID})
		assertStatus(t, err, 400)
	})

	t.Run("unknown ID is 404", func(t *testing.T) {
		_, err := h.cancel(fakeAuthedCtx(t, 5, "user"), &wishlistIDInput{ID: 999})
		assertStatus(t, err, 404)
	})
}

func TestWishlistAnonymity(t *testing.T) {
	h, repo, _, _, _ := newWishlistHandler()
	req := &models.WishlistRequest{
		RequesterID: 5,
		Title:       "T",
		Author:      "A",
		OLKey:       "OL1",
		Status:      "open",
		IsAnonymous: true,
		Requester:   models.User{ID: 5, Name: "Real Name"},
	}
	require.NoError(t, repo.Create(req))

	t.Run("list redacts the requester's name for other members", func(t *testing.T) {
		out, err := h.list(fakeAuthedCtx(t, 1, "user"), &listWishlistInput{})
		require.NoError(t, err)
		require.Len(t, out.Body.Items, 1)
		assert.Equal(t, "Anonymous member", out.Body.Items[0].Requester.Name)
		assert.Zero(t, out.Body.Items[0].Requester.ID)
		// requester_id itself stays visible — it's needed for the
		// requester's own "can I manage this" check on the frontend.
		assert.Equal(t, uint(5), out.Body.Items[0].RequesterID)
	})

	t.Run("get reveals the real name to the requester themselves", func(t *testing.T) {
		out, err := h.get(fakeAuthedCtx(t, 5, "user"), &wishlistIDInput{ID: req.ID})
		require.NoError(t, err)
		assert.Equal(t, "Real Name", out.Body.Requester.Name)
	})

	t.Run("get reveals the real name to an admin", func(t *testing.T) {
		out, err := h.get(fakeAuthedCtx(t, 99, "admin"), &wishlistIDInput{ID: req.ID})
		require.NoError(t, err)
		assert.Equal(t, "Real Name", out.Body.Requester.Name)
	})

	t.Run("get redacts the real name from another member", func(t *testing.T) {
		out, err := h.get(fakeAuthedCtx(t, 6, "user"), &wishlistIDInput{ID: req.ID})
		require.NoError(t, err)
		assert.Equal(t, "Anonymous member", out.Body.Requester.Name)
	})

	t.Run("non-anonymous requests are never redacted", func(t *testing.T) {
		public := &models.WishlistRequest{
			RequesterID: 7,
			Title:       "T2",
			Author:      "A",
			OLKey:       "OL2",
			Status:      "open",
			Requester:   models.User{ID: 7, Name: "Public Name"},
		}
		require.NoError(t, repo.Create(public))

		out, err := h.get(fakeAuthedCtx(t, 6, "user"), &wishlistIDInput{ID: public.ID})
		require.NoError(t, err)
		assert.Equal(t, "Public Name", out.Body.Requester.Name)
	})
}

func TestFulfillWishlistRequest(t *testing.T) {
	h, repo, notifs, users, books := newWishlistHandler()
	require.NoError(t, users.Create(&models.User{Name: "Requester", Email: "req@example.com"}))

	t.Run("any authenticated user can link a book to an open request", func(t *testing.T) {
		req := &models.WishlistRequest{RequesterID: 1, Title: "T", Author: "A", ISBN: "123", Status: "open"}
		require.NoError(t, repo.Create(req))
		book := &models.Book{Title: "Matched Book", Author: "A"}
		require.NoError(t, books.Create(book))

		input := &fulfillWishlistInput{ID: req.ID}
		input.Body.BookID = book.ID
		out, err := h.fulfill(fakeAuthedCtx(t, 2, "user"), input)
		require.NoError(t, err)
		assert.Equal(t, "fulfilled", out.Body.Status)
		assert.Equal(t, 1, notifs.Count())
	})

	t.Run("cannot fulfill an already-fulfilled request", func(t *testing.T) {
		req := &models.WishlistRequest{RequesterID: 1, Title: "T", Author: "A", ISBN: "123", Status: "fulfilled"}
		require.NoError(t, repo.Create(req))
		book := &models.Book{Title: "Book", Author: "A"}
		require.NoError(t, books.Create(book))

		input := &fulfillWishlistInput{ID: req.ID}
		input.Body.BookID = book.ID
		_, err := h.fulfill(fakeAuthedCtx(t, 2, "user"), input)
		assertStatus(t, err, 400)
	})

	t.Run("unknown book is 404", func(t *testing.T) {
		req := &models.WishlistRequest{RequesterID: 1, Title: "T", Author: "A", ISBN: "123", Status: "open"}
		require.NoError(t, repo.Create(req))

		input := &fulfillWishlistInput{ID: req.ID}
		input.Body.BookID = 9999
		_, err := h.fulfill(fakeAuthedCtx(t, 2, "user"), input)
		assertStatus(t, err, 404)
	})
}
