package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
)

type wishlistWorkflowDeps struct {
	workflow *WishlistWorkflow
	requests *repotest.WishlistRequestRepository
	notifs   *repotest.NotificationRepository
	users    *repotest.UserRepository
}

func newWishlistWorkflow() *wishlistWorkflowDeps {
	requests := repotest.NewWishlistRequestRepository()
	notifs := repotest.NewNotificationRepository()
	users := repotest.NewUserRepository()
	email := NewEmailService("", "", "", "", "", "", "")
	return &wishlistWorkflowDeps{
		workflow: NewWishlistWorkflow(requests, notifs, users, email),
		requests: requests, notifs: notifs, users: users,
	}
}

func TestOnBookCreated_AutoMatchesByOLKey(t *testing.T) {
	d := newWishlistWorkflow()
	requester := &models.User{Name: "Requester", Email: "req@example.com"}
	require.NoError(t, d.users.Create(requester))
	req := &models.WishlistRequest{RequesterID: requester.ID, Title: "Wanted Book", Author: "A", OLKey: "OL123", Status: "open"}
	require.NoError(t, d.requests.Create(req))

	book := &models.Book{ID: 42, Title: "Wanted Book", Author: "A", OLKey: "OL123"}
	d.workflow.OnBookCreated(context.Background(), book)

	reloaded, err := d.requests.GetByID(req.ID)
	require.NoError(t, err)
	assert.Equal(t, "fulfilled", reloaded.Status)
	require.NotNil(t, reloaded.FulfilledBookID)
	assert.Equal(t, book.ID, *reloaded.FulfilledBookID)
	require.NotNil(t, reloaded.FulfilledAt)
	assert.Equal(t, 1, d.notifs.Count())
}

func TestOnBookCreated_AutoMatchesByGoogleBooksID(t *testing.T) {
	d := newWishlistWorkflow()
	requester := &models.User{Name: "Requester", Email: "req@example.com"}
	require.NoError(t, d.users.Create(requester))
	req := &models.WishlistRequest{RequesterID: requester.ID, Title: "Wanted Book", Author: "A", GoogleBooksID: "GB1", Status: "open"}
	require.NoError(t, d.requests.Create(req))

	book := &models.Book{ID: 7, Title: "Wanted Book", Author: "A", GoogleBooksID: "GB1"}
	d.workflow.OnBookCreated(context.Background(), book)

	reloaded, err := d.requests.GetByID(req.ID)
	require.NoError(t, err)
	assert.Equal(t, "fulfilled", reloaded.Status)
}

func TestOnBookCreated_NoMatch_NoSideEffects(t *testing.T) {
	d := newWishlistWorkflow()
	requester := &models.User{Name: "Requester", Email: "req@example.com"}
	require.NoError(t, d.users.Create(requester))
	req := &models.WishlistRequest{RequesterID: requester.ID, Title: "Wanted Book", Author: "A", OLKey: "OL123", Status: "open"}
	require.NoError(t, d.requests.Create(req))

	book := &models.Book{ID: 42, Title: "Unrelated Book", Author: "B", OLKey: "OL999"}
	d.workflow.OnBookCreated(context.Background(), book)

	reloaded, err := d.requests.GetByID(req.ID)
	require.NoError(t, err)
	assert.Equal(t, "open", reloaded.Status)
	assert.Equal(t, 0, d.notifs.Count())
}

func TestOnBookCreated_IgnoresNonOpenRequests(t *testing.T) {
	d := newWishlistWorkflow()
	requester := &models.User{Name: "Requester", Email: "req@example.com"}
	require.NoError(t, d.users.Create(requester))
	req := &models.WishlistRequest{RequesterID: requester.ID, Title: "Wanted Book", Author: "A", OLKey: "OL123", Status: "cancelled"}
	require.NoError(t, d.requests.Create(req))

	book := &models.Book{ID: 42, Title: "Wanted Book", Author: "A", OLKey: "OL123"}
	d.workflow.OnBookCreated(context.Background(), book)

	reloaded, err := d.requests.GetByID(req.ID)
	require.NoError(t, err)
	assert.Equal(t, "cancelled", reloaded.Status)
	assert.Equal(t, 0, d.notifs.Count())
}

func TestOnFulfilled_ManualLink(t *testing.T) {
	d := newWishlistWorkflow()
	requester := &models.User{Name: "Requester", Email: "req@example.com"}
	require.NoError(t, d.users.Create(requester))
	req := &models.WishlistRequest{RequesterID: requester.ID, Title: "Wanted Book", Author: "A", ISBN: "1234", Status: "open"}
	require.NoError(t, d.requests.Create(req))

	book := &models.Book{ID: 99, Title: "A Different Edition", Author: "A"}
	d.workflow.OnFulfilled(context.Background(), req, book)

	reloaded, err := d.requests.GetByID(req.ID)
	require.NoError(t, err)
	assert.Equal(t, "fulfilled", reloaded.Status)
	require.NotNil(t, reloaded.FulfilledBookID)
	assert.Equal(t, book.ID, *reloaded.FulfilledBookID)
	assert.Equal(t, 1, d.notifs.Count())
}
