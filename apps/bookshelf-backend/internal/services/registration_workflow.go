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
	email  *EmailService
}

// NewRegistrationWorkflow creates a new RegistrationWorkflow.
func NewRegistrationWorkflow(
	admin repository.AdminRepository,
	notifs repository.NotificationRepository,
	email *EmailService,
) *RegistrationWorkflow {
	return &RegistrationWorkflow{admin: admin, notifs: notifs, email: email}
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
