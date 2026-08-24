package services

import (
	"context"
	"fmt"
	"html"

	"github.com/rs/zerolog"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// RegistrationWorkflow orchestrates the side-effects (notification, email)
// of a new registration being held for admin approval.
type RegistrationWorkflow struct {
	admin  repository.AdminRepository
	notifs repository.NotificationRepository
	books  repository.BookRepository
	email  *EmailService
}

// NewRegistrationWorkflow creates a new RegistrationWorkflow.
func NewRegistrationWorkflow(
	admin repository.AdminRepository,
	notifs repository.NotificationRepository,
	books repository.BookRepository,
	email *EmailService,
) *RegistrationWorkflow {
	return &RegistrationWorkflow{admin: admin, notifs: notifs, books: books, email: email}
}

// OnPendingApproval fires when a new registration is held for admin
// approval (User.PendingApproval == true). It notifies every admin, both
// in-app and by best-effort email — never fails the caller, since this is a
// side-effect of a registration that has already been persisted.
func (w *RegistrationWorkflow) OnPendingApproval(ctx context.Context, user *models.User) {
	admins, err := w.admin.ListByRole("admin")
	if err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("OnPendingApproval: list admins")
		return
	}

	subject := "New user awaiting approval"
	body := fmt.Sprintf(
		"<p>A new registration is waiting for your approval.</p><p><strong>%s</strong> (%s) signed up and needs an admin to approve their account before they can sign in.</p>",
		html.EscapeString(user.Name), html.EscapeString(user.Email),
	) + w.email.Button("/admin/users", "Review pending users")

	for _, admin := range admins {
		n := models.Notification{
			RecipientID:   admin.ID,
			Type:          "user_pending_approval",
			PendingUserID: &user.ID,
		}
		if err := w.notifs.Create(&n); err != nil {
			zerolog.Ctx(ctx).Warn().Err(err).Uint("admin_id", admin.ID).Msg("OnPendingApproval: create notification")
		}

		if admin.EmailNotificationsEnabled {
			w.email.SendEmailAsync(ctx, admin.Email, subject, body)
		}
	}
}

// OnApproved fires when an admin clears a user's PendingApproval flag. It
// notifies the approved user, both in-app and by best-effort email — never
// fails the caller, since this is a side-effect of an update that has
// already been persisted.
func (w *RegistrationWorkflow) OnApproved(ctx context.Context, user *models.User) {
	n := models.Notification{
		RecipientID: user.ID,
		Type:        "user_approved",
	}
	if err := w.notifs.Create(&n); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Uint("user_id", user.ID).Msg("OnApproved: create notification")
	}

	if user.EmailNotificationsEnabled {
		intro := fmt.Sprintf(
			"<p>Good news, <strong>%s</strong> — an admin has approved your account. You can now sign in.</p>",
			html.EscapeString(user.Name),
		)
		w.sendWelcomeEmail(ctx, user, "Your account has been approved", intro, "/login", "Sign in")
	}
}

// OnRegistered fires when a new registration completes without needing
// admin approval (User.PendingApproval == false at creation, i.e.
// `require_registration_approval` is off) — the account is live and the
// caller is signed in immediately, so this is a welcome email rather than
// an approval notice. No in-app notification: unlike OnApproved, there's no
// "admin acted on your account" event to surface in the bell, and the user
// is about to land on the app themselves anyway.
func (w *RegistrationWorkflow) OnRegistered(ctx context.Context, user *models.User) {
	if !user.EmailNotificationsEnabled {
		return
	}
	intro := fmt.Sprintf(
		"<p>Welcome, <strong>%s</strong> — your account is ready to go.</p>",
		html.EscapeString(user.Name),
	)
	w.sendWelcomeEmail(ctx, user, "Welcome to Bookshelf", intro, "/catalog", "Browse the catalog")
}

// sendWelcomeEmail is the shared body builder behind OnApproved and
// OnRegistered: an intro line, the catalog-engagement pitch, then a single
// CTA button — the two callers differ only in subject/intro/CTA.
func (w *RegistrationWorkflow) sendWelcomeEmail(ctx context.Context, user *models.User, subject, intro, ctaPath, ctaLabel string) {
	body := intro + w.catalogPitch(ctx) + w.email.Button(ctaPath, ctaLabel)
	w.email.SendEmailAsync(ctx, user.Email, subject, body)
}

// catalogPitch renders a short engagement nudge for the approval/welcome
// email —
// the current catalog size plus a call to browse or add a book. Falls back
// to a count-free pitch if the book count can't be fetched, since this is
// cosmetic and shouldn't block the approval email over it.
func (w *RegistrationWorkflow) catalogPitch(ctx context.Context) string {
	count, err := w.books.CountAll()
	if err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("catalogPitch: count books")
		return "<p>Take a look at what the community is already sharing, or add the next book to the shelf.</p>"
	}

	book := "books"
	if count == 1 {
		book = "book"
	}
	return fmt.Sprintf(
		"<p>There %s currently <strong>%d %s</strong> shared by the community — take a look at what's on offer, or add the next one to the shelf.</p>",
		pluralVerb(count), count, book,
	)
}

func pluralVerb(count int64) string {
	if count == 1 {
		return "is"
	}
	return "are"
}
