package services

import (
	"context"
	"fmt"
	"html"
	"time"

	"github.com/rs/zerolog"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// WishlistWorkflow orchestrates the side-effects (notification, email) of
// fulfilling a WishlistRequest, whether triggered automatically (a newly
// added Book matches an open request's external key) or manually (a member
// links an existing Book to a near-match request).
type WishlistWorkflow struct {
	requests repository.WishlistRequestRepository
	notifs   repository.NotificationRepository
	users    repository.UserRepository
	email    *EmailService
	telegram TelegramNotifier
}

// NewWishlistWorkflow creates a new WishlistWorkflow.
func NewWishlistWorkflow(
	requests repository.WishlistRequestRepository,
	notifs repository.NotificationRepository,
	users repository.UserRepository,
	email *EmailService,
	telegram TelegramNotifier,
) *WishlistWorkflow {
	return &WishlistWorkflow{requests: requests, notifs: notifs, users: users, email: email, telegram: telegram}
}

// OnBookCreated auto-matches a newly created Book against open
// WishlistRequests by external key, fulfilling each match. Called from
// BookHandler.createBook only after a genuinely new Book row is inserted —
// never on the upsert-return-existing path.
func (w *WishlistWorkflow) OnBookCreated(ctx context.Context, book *models.Book) {
	var matches []models.WishlistRequest
	if book.OLKey != "" {
		if m, err := w.requests.FindOpenByOLKey(book.OLKey); err == nil {
			matches = append(matches, m...)
		} else {
			zerolog.Ctx(ctx).Warn().Err(err).Msg("OnBookCreated: find by ol_key")
		}
	}
	if book.GoogleBooksID != "" {
		if m, err := w.requests.FindOpenByGoogleBooksID(book.GoogleBooksID); err == nil {
			matches = append(matches, m...)
		} else {
			zerolog.Ctx(ctx).Warn().Err(err).Msg("OnBookCreated: find by google_books_id")
		}
	}
	for i := range matches {
		w.fulfill(ctx, &matches[i], book)
	}
}

// OnFulfilled handles the manual-link path: a member or admin ties an open
// request to an existing Book that wasn't an automatic key match (e.g. a
// different edition/ISBN). Shares the same notify side effect as auto-match.
func (w *WishlistWorkflow) OnFulfilled(ctx context.Context, req *models.WishlistRequest, book *models.Book) {
	w.fulfill(ctx, req, book)
}

// fulfill is the single write site for a request transitioning to
// "fulfilled" — marks it, notifies the requester in-app, and best-effort
// emails them. Never called concurrently for the same request in practice
// (each request can only match once per external key per book-creation
// call), so no additional locking is needed here.
func (w *WishlistWorkflow) fulfill(ctx context.Context, req *models.WishlistRequest, book *models.Book) {
	now := time.Now()
	req.Status = "fulfilled"
	req.FulfilledBookID = &book.ID
	req.FulfilledAt = &now
	if err := w.requests.Save(req); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Uint("wishlist_id", req.ID).Msg("fulfill: save request")
		return
	}

	n := models.Notification{
		RecipientID:       req.RequesterID,
		Type:              "wishlist_fulfilled",
		WishlistRequestID: &req.ID,
	}
	if err := w.notifs.Create(&n); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("fulfill: create notification")
	}

	requester, err := w.users.FindByID(req.RequesterID)
	if err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Uint("requester_id", req.RequesterID).Msg("fulfill: load requester")
		return // email is best-effort; don't fail the fulfillment
	}

	subject := "A book you were looking for is now in the catalog"
	body := fmt.Sprintf(
		"<p>Hi %s,</p><p><strong>%s</strong> by %s, which you were looking for, has been added to the catalog. "+
			"Visit the book's page to request to borrow it.</p>",
		html.EscapeString(requester.Name), html.EscapeString(book.Title), html.EscapeString(book.Author),
	) + w.email.Button(fmt.Sprintf("/catalog/%d", book.ID), "View book")
	if requester.EmailNotificationsEnabled {
		w.email.SendEmailAsync(ctx, requester.Email, subject, body)
	}

	if requester.TelegramChatID != nil && requester.TelegramNotificationsEnabled {
		telegramText := fmt.Sprintf(
			"<b>%s</b> by %s, which you were looking for, has been added to the catalog.\n<a href=\"%s\">View book</a>",
			html.EscapeString(book.Title), html.EscapeString(book.Author), w.email.URL(fmt.Sprintf("/catalog/%d", book.ID)),
		)
		w.telegram.NotifyAsync(ctx, *requester.TelegramChatID, telegramText)
	}
}
