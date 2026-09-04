package services

import (
	"context"
	"fmt"
	"html"

	"github.com/rs/zerolog"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// LoanWorkflow orchestrates side-effects (notifications, emails, copy-status
// updates) that occur at each stage of a loan request lifecycle.
type LoanWorkflow struct {
	copies    repository.CopyRepository
	loanReqs  repository.LoanRequestRepository
	notifs    repository.NotificationRepository
	users     repository.UserRepository
	waitlists repository.WaitlistRepository
	email     *EmailService
	telegram  TelegramNotifier
}

// NewLoanWorkflow creates a new LoanWorkflow.
func NewLoanWorkflow(
	copies repository.CopyRepository,
	loanReqs repository.LoanRequestRepository,
	notifs repository.NotificationRepository,
	users repository.UserRepository,
	waitlists repository.WaitlistRepository,
	email *EmailService,
	telegram TelegramNotifier,
) *LoanWorkflow {
	return &LoanWorkflow{
		copies:    copies,
		loanReqs:  loanReqs,
		notifs:    notifs,
		users:     users,
		waitlists: waitlists,
		email:     email,
		telegram:  telegram,
	}
}

// notifyTelegram sends text to recipient's linked Telegram chat, if any, and
// if they haven't turned Telegram notifications off — the same gate every
// call site below applies before pushing. Not to be confused with
// recipient.TelegramUsername, an unrelated free-text contact field.
func (w *LoanWorkflow) notifyTelegram(ctx context.Context, recipient models.User, text string) {
	if recipient.WantsTelegram() {
		w.telegram.NotifyAsync(ctx, *recipient.TelegramChatID, text)
	}
}

// OnRequested fires when a borrower creates a new loan request.
// It notifies the copy owner.
func (w *LoanWorkflow) OnRequested(ctx context.Context, lr *models.LoanRequest) error {
	bookCopy, err := w.copies.GetByIDWithAssociations(lr.CopyID)
	if err != nil {
		return fmt.Errorf("OnRequested: load copy: %w", err)
	}

	n := models.Notification{
		RecipientID:   bookCopy.OwnerID,
		Type:          "request_received",
		LoanRequestID: &lr.ID,
	}
	if err := w.notifs.Create(&n); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("OnRequested: create notification")
	}

	borrower, err := w.users.FindByID(lr.BorrowerID)
	if err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Uint("borrower_id", lr.BorrowerID).Msg("OnRequested: load borrower")
		return nil // email is best-effort; don't fail the request
	}

	subject := "Someone wants to borrow your book"
	body := fmt.Sprintf(
		"<p>Hi %s,</p><p><strong>%s</strong> has requested to borrow your copy of <em>%s</em>.</p>",
		html.EscapeString(bookCopy.Owner.Name), html.EscapeString(borrower.Name), html.EscapeString(bookCopy.Book.Title),
	) + w.email.Button(fmt.Sprintf("/my-books/%d/requests", bookCopy.ID), "View request")
	if bookCopy.Owner.EmailNotificationsEnabled {
		w.email.SendEmailAsync(ctx, bookCopy.Owner.Email, subject, body)
	}

	telegramText := fmt.Sprintf(
		"<b>%s</b> has requested to borrow your copy of <i>%s</i>.\n<a href=\"%s\">View request</a>",
		html.EscapeString(borrower.Name), html.EscapeString(bookCopy.Book.Title),
		w.email.URL(fmt.Sprintf("/my-books/%d/requests", bookCopy.ID)),
	)
	w.notifyTelegram(ctx, bookCopy.Owner, telegramText)
	return nil
}

// OnAccepted fires when the owner accepts a loan request.
// In a single transaction it:
//   - Rejects all other pending requests for the same copy.
//   - Creates rejection notifications for their borrowers.
//   - Updates the copy status to "loaned".
//
// Then it notifies the accepted borrower.
func (w *LoanWorkflow) OnAccepted(ctx context.Context, lr *models.LoanRequest) error {
	if err := w.loanReqs.RejectCompetingAndUpdateCopy(lr.CopyID, lr.ID); err != nil {
		return fmt.Errorf("OnAccepted: transaction: %w", err)
	}

	// Notify the borrower.
	n := models.Notification{
		RecipientID:   lr.BorrowerID,
		Type:          "request_accepted",
		LoanRequestID: &lr.ID,
	}
	if err := w.notifs.Create(&n); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("OnAccepted: create notification")
	}

	// Send email to borrower.
	borrower, err := w.users.FindByID(lr.BorrowerID)
	if err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Uint("borrower_id", lr.BorrowerID).Msg("OnAccepted: load borrower")
		return nil // email is best-effort
	}

	bookCopy, err := w.copies.GetByIDWithAssociations(lr.CopyID)
	if err != nil {
		return fmt.Errorf("OnAccepted: load copy: %w", err)
	}

	subject := "Your loan request was accepted"
	body := fmt.Sprintf(
		"<p>Hi %s,</p><p>Your request to borrow <em>%s</em> has been accepted by %s. "+
			"Please get in touch to arrange collection.</p>",
		html.EscapeString(borrower.Name), html.EscapeString(bookCopy.Book.Title), html.EscapeString(bookCopy.Owner.Name),
	) + ownerContactExtrasHTML(bookCopy.Owner) + w.email.Button("/my-requests", "View your loans")
	if borrower.EmailNotificationsEnabled {
		w.email.SendEmailAsync(ctx, borrower.Email, subject, body)
	}

	telegramText := fmt.Sprintf(
		"Your request to borrow <i>%s</i> has been accepted by %s. Please get in touch to arrange collection.\n<a href=\"%s\">View your loans</a>",
		html.EscapeString(bookCopy.Book.Title), html.EscapeString(bookCopy.Owner.Name), w.email.URL("/my-requests"),
	)
	w.notifyTelegram(ctx, *borrower, telegramText)
	return nil
}

// ownerContactExtrasHTML renders the owner's optional contact fields (beyond
// email/phone, which are surfaced separately in-app) as extra paragraphs for
// the acceptance email, omitting any that aren't set.
func ownerContactExtrasHTML(owner models.User) string {
	var extras string
	if owner.TelegramUsername != "" {
		extras += fmt.Sprintf("<p>Telegram: %s</p>", html.EscapeString(owner.TelegramUsername))
	}
	if owner.WhatsAppUsername != "" {
		extras += fmt.Sprintf("<p>WhatsApp: %s</p>", html.EscapeString(owner.WhatsAppUsername))
	}
	if owner.ContactNote != "" {
		extras += fmt.Sprintf("<p>Note: %s</p>", html.EscapeString(owner.ContactNote))
	}
	return extras
}

// OnRejected fires when the owner rejects a loan request.
// If no other pending requests exist for the copy, the copy is set back to
// "available".
func (w *LoanWorkflow) OnRejected(ctx context.Context, lr *models.LoanRequest) error {
	pendingCount, _ := w.loanReqs.CountPendingForCopyExcluding(lr.CopyID, lr.ID)
	if pendingCount == 0 {
		w.copies.UpdateStatus(lr.CopyID, "available") //nolint:errcheck,gosec
		w.notifyWaitlistAndClear(ctx, lr)
	}

	n := models.Notification{
		RecipientID:   lr.BorrowerID,
		Type:          "request_rejected",
		LoanRequestID: &lr.ID,
	}
	if err := w.notifs.Create(&n); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("OnRejected: create notification")
	}

	return nil
}

// OnCancelled fires when the borrower cancels a pending request.
// If no other pending requests exist for the copy, the copy is set back to
// "available".
func (w *LoanWorkflow) OnCancelled(ctx context.Context, lr *models.LoanRequest) error {
	pendingCount, _ := w.loanReqs.CountPendingForCopyExcluding(lr.CopyID, lr.ID)
	if pendingCount == 0 {
		w.copies.UpdateStatus(lr.CopyID, "available") //nolint:errcheck,gosec
		w.notifyWaitlistAndClear(ctx, lr)
	}

	return nil
}

// OnReturned fires when either the borrower or the copy owner marks a loan as
// returned. The copy is set back to "available", whichever party didn't
// perform the return is notified, and any waitlisted users are notified that
// the copy is now available.
//
// Known limitation: if this loan's return is later undone (OnReturnUndone),
// the waitlist notified/cleared below cannot be restored. In practice this is
// bounded — undo requires the copy to still be "available" (see undoReturn),
// so it's only possible before anyone has acted on their waitlist
// notification; once someone does, the copy moves off "available" and undo
// is blocked anyway.
func (w *LoanWorkflow) OnReturned(ctx context.Context, lr *models.LoanRequest) error {
	w.copies.UpdateStatus(lr.CopyID, "available") //nolint:errcheck,gosec

	bookCopy, err := w.copies.GetByIDWithAssociations(lr.CopyID)
	if err != nil {
		return fmt.Errorf("OnReturned: load copy: %w", err)
	}

	// Notify whichever party didn't perform the return.
	recipientID := lr.BorrowerID
	if lr.ReturnedBy != nil && *lr.ReturnedBy == lr.BorrowerID {
		recipientID = bookCopy.OwnerID
	}

	n := models.Notification{
		RecipientID:   recipientID,
		Type:          "marked_returned",
		LoanRequestID: &lr.ID,
	}
	if err := w.notifs.Create(&n); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("OnReturned: create notification")
	}

	w.notifyWaitlistAndClear(ctx, lr)
	w.sendReturnedEmail(ctx, recipientID, lr, bookCopy)
	return nil
}

// notifyWaitlistAndClear notifies every user waitlisted for lr's copy that it
// is now available, then clears the waitlist — the copy has just become
// available (a return, or the last pending/requested claim on it going away
// via reject/cancel), so it can't stay a promise for anyone still on the
// list.
func (w *LoanWorkflow) notifyWaitlistAndClear(ctx context.Context, lr *models.LoanRequest) {
	if w.waitlists == nil {
		return
	}
	entries, wErr := w.waitlists.ListByCopyID(lr.CopyID)
	if wErr != nil || len(entries) == 0 {
		return
	}
	for _, entry := range entries {
		wn := models.Notification{
			RecipientID:   entry.UserID,
			Type:          "waitlist_available",
			LoanRequestID: &lr.ID,
		}
		if nErr := w.notifs.Create(&wn); nErr != nil {
			zerolog.Ctx(ctx).Warn().Err(nErr).Msg("notifyWaitlistAndClear: create notification")
		}
	}
	w.waitlists.DeleteByCopyID(lr.CopyID) //nolint:errcheck,gosec
}

// sendReturnedEmail best-effort emails whichever party (identified by
// recipientID) didn't perform the return, with copy tailored to the
// direction: the borrower being thanked, or the owner being told their book
// came back.
func (w *LoanWorkflow) sendReturnedEmail(ctx context.Context, recipientID uint, lr *models.LoanRequest, bookCopy *models.Copy) {
	borrower, err := w.users.FindByID(lr.BorrowerID)
	if err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Uint("borrower_id", lr.BorrowerID).Msg("OnReturned: load borrower")
		return
	}

	if recipientID == lr.BorrowerID {
		subject := "Your loan has been marked as returned"
		body := fmt.Sprintf(
			"<p>Hi %s,</p><p>Your loan of <em>%s</em> has been marked as returned. Thank you!</p>",
			html.EscapeString(borrower.Name), html.EscapeString(bookCopy.Book.Title),
		) + w.email.Button("/my-requests", "View your loans")
		if borrower.EmailNotificationsEnabled {
			w.email.SendEmailAsync(ctx, borrower.Email, subject, body)
		}
		telegramText := fmt.Sprintf(
			"Your loan of <i>%s</i> has been marked as returned. Thank you!\n<a href=\"%s\">View your loans</a>",
			html.EscapeString(bookCopy.Book.Title), w.email.URL("/my-requests"),
		)
		w.notifyTelegram(ctx, *borrower, telegramText)
		return
	}

	owner := bookCopy.Owner
	subject := "A loan of your book was marked as returned"
	body := fmt.Sprintf(
		"<p>Hi %s,</p><p>%s marked your copy of <em>%s</em> as returned.</p>",
		html.EscapeString(owner.Name), html.EscapeString(borrower.Name), html.EscapeString(bookCopy.Book.Title),
	) + w.email.Button("/my-books", "View your books")
	if owner.EmailNotificationsEnabled {
		w.email.SendEmailAsync(ctx, owner.Email, subject, body)
	}
	telegramText := fmt.Sprintf(
		"%s marked your copy of <i>%s</i> as returned.\n<a href=\"%s\">View your books</a>",
		html.EscapeString(borrower.Name), html.EscapeString(bookCopy.Book.Title), w.email.URL("/my-books"),
	)
	w.notifyTelegram(ctx, owner, telegramText)
}

// OnReturnUndone fires when the owner reverses a "returned" loan back to
// "accepted" because the return wasn't genuine. The copy goes back to
// "loaned" and the borrower is notified. See OnReturned's doc comment for the
// waitlist-loss limitation this doesn't attempt to fix.
func (w *LoanWorkflow) OnReturnUndone(ctx context.Context, lr *models.LoanRequest) error {
	if err := w.copies.UpdateStatus(lr.CopyID, "loaned"); err != nil {
		return fmt.Errorf("OnReturnUndone: update copy status: %w", err)
	}

	n := models.Notification{
		RecipientID:   lr.BorrowerID,
		Type:          "return_undone",
		LoanRequestID: &lr.ID,
	}
	if err := w.notifs.Create(&n); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("OnReturnUndone: create notification")
	}

	borrower, err := w.users.FindByID(lr.BorrowerID)
	if err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Uint("borrower_id", lr.BorrowerID).Msg("OnReturnUndone: load borrower")
		return nil // email is best-effort
	}

	bookCopy, err := w.copies.GetByIDWithAssociations(lr.CopyID)
	if err != nil {
		return fmt.Errorf("OnReturnUndone: load copy: %w", err)
	}

	subject := "Your return was undone"
	body := fmt.Sprintf(
		"<p>Hi %s,</p><p>%s undid the return of <em>%s</em> — your loan is active again.</p>",
		html.EscapeString(borrower.Name), html.EscapeString(bookCopy.Owner.Name), html.EscapeString(bookCopy.Book.Title),
	) + w.email.Button("/my-requests", "View your loans")
	if borrower.EmailNotificationsEnabled {
		w.email.SendEmailAsync(ctx, borrower.Email, subject, body)
	}

	telegramText := fmt.Sprintf(
		"%s undid the return of <i>%s</i> — your loan is active again.\n<a href=\"%s\">View your loans</a>",
		html.EscapeString(bookCopy.Owner.Name), html.EscapeString(bookCopy.Book.Title), w.email.URL("/my-requests"),
	)
	w.notifyTelegram(ctx, *borrower, telegramText)
	return nil
}

// OnExpectedReturnDateChanged fires when either party amends the agreed
// return date on an accepted loan (see updateExpectedReturnDate). It notifies
// whichever party did *not* make the change — a bell-only nudge, no email,
// since this is a lightweight field edit rather than a lifecycle event.
func (w *LoanWorkflow) OnExpectedReturnDateChanged(ctx context.Context, lr *models.LoanRequest, changedBy uint) error {
	recipientID := lr.BorrowerID
	if changedBy == lr.BorrowerID {
		recipientID = lr.Copy.OwnerID
	}

	n := models.Notification{
		RecipientID:   recipientID,
		Type:          "expected_return_date_changed",
		LoanRequestID: &lr.ID,
	}
	if err := w.notifs.Create(&n); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("OnExpectedReturnDateChanged: create notification")
	}
	return nil
}
