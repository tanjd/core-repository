package services

import (
	"fmt"

	"github.com/rs/zerolog/log"

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
}

// NewLoanWorkflow creates a new LoanWorkflow.
func NewLoanWorkflow(
	copies repository.CopyRepository,
	loanReqs repository.LoanRequestRepository,
	notifs repository.NotificationRepository,
	users repository.UserRepository,
	waitlists repository.WaitlistRepository,
	email *EmailService,
) *LoanWorkflow {
	return &LoanWorkflow{
		copies:    copies,
		loanReqs:  loanReqs,
		notifs:    notifs,
		users:     users,
		waitlists: waitlists,
		email:     email,
	}
}

// OnRequested fires when a borrower creates a new loan request.
// It notifies the copy owner.
func (w *LoanWorkflow) OnRequested(lr *models.LoanRequest) error {
	bookCopy, err := w.copies.GetByIDWithOwner(lr.CopyID)
	if err != nil {
		return fmt.Errorf("OnRequested: load copy: %w", err)
	}

	n := models.Notification{
		RecipientID:   bookCopy.OwnerID,
		Type:          "request_received",
		LoanRequestID: &lr.ID,
	}
	if err := w.notifs.Create(&n); err != nil {
		log.Warn().Err(err).Msg("OnRequested: create notification")
	}

	borrower, err := w.users.FindByID(lr.BorrowerID)
	if err != nil {
		log.Warn().Err(err).Uint("borrower_id", lr.BorrowerID).Msg("OnRequested: load borrower")
		return nil // email is best-effort; don't fail the request
	}

	subject := "Someone wants to borrow your book"
	html := fmt.Sprintf(
		"<p>Hi %s,</p><p><strong>%s</strong> has requested to borrow your copy of <em>%s</em>.</p>",
		bookCopy.Owner.Name, borrower.Name, bookCopy.Book.Title,
	)
	return w.email.SendEmail(bookCopy.Owner.Email, subject, html)
}

// OnAccepted fires when the owner accepts a loan request.
// In a single transaction it:
//   - Rejects all other pending requests for the same copy.
//   - Creates rejection notifications for their borrowers.
//   - Updates the copy status to "loaned".
//
// Then it notifies the accepted borrower.
func (w *LoanWorkflow) OnAccepted(lr *models.LoanRequest) error {
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
		log.Warn().Err(err).Msg("OnAccepted: create notification")
	}

	// Send email to borrower.
	borrower, err := w.users.FindByID(lr.BorrowerID)
	if err != nil {
		log.Warn().Err(err).Uint("borrower_id", lr.BorrowerID).Msg("OnAccepted: load borrower")
		return nil // email is best-effort
	}

	bookCopy, err := w.copies.GetByIDWithAssociations(lr.CopyID)
	if err != nil {
		return fmt.Errorf("OnAccepted: load copy: %w", err)
	}

	subject := "Your loan request was accepted"
	html := fmt.Sprintf(
		"<p>Hi %s,</p><p>Your request to borrow <em>%s</em> has been accepted by %s. "+
			"Please get in touch to arrange collection.</p>",
		borrower.Name, bookCopy.Book.Title, bookCopy.Owner.Name,
	)
	return w.email.SendEmail(borrower.Email, subject, html)
}

// OnRejected fires when the owner rejects a loan request.
// If no other pending requests exist for the copy, the copy is set back to
// "available".
func (w *LoanWorkflow) OnRejected(lr *models.LoanRequest) error {
	pendingCount, _ := w.loanReqs.CountPendingForCopyExcluding(lr.CopyID, lr.ID)
	if pendingCount == 0 {
		w.copies.UpdateStatus(lr.CopyID, "available") //nolint:errcheck,gosec
	}

	n := models.Notification{
		RecipientID:   lr.BorrowerID,
		Type:          "request_rejected",
		LoanRequestID: &lr.ID,
	}
	if err := w.notifs.Create(&n); err != nil {
		log.Warn().Err(err).Msg("OnRejected: create notification")
	}

	return nil
}

// OnCancelled fires when the borrower cancels a pending request.
// If no other pending requests exist for the copy, the copy is set back to
// "available".
func (w *LoanWorkflow) OnCancelled(lr *models.LoanRequest) error {
	pendingCount, _ := w.loanReqs.CountPendingForCopyExcluding(lr.CopyID, lr.ID)
	if pendingCount == 0 {
		w.copies.UpdateStatus(lr.CopyID, "available") //nolint:errcheck,gosec
	}

	return nil
}

// OnReturned fires when the owner marks a loan as returned.
// The copy is set back to "available", the borrower is notified, and any
// waitlisted users are notified that the copy is now available.
func (w *LoanWorkflow) OnReturned(lr *models.LoanRequest) error {
	w.copies.UpdateStatus(lr.CopyID, "available") //nolint:errcheck,gosec

	n := models.Notification{
		RecipientID:   lr.BorrowerID,
		Type:          "marked_returned",
		LoanRequestID: &lr.ID,
	}
	if err := w.notifs.Create(&n); err != nil {
		log.Warn().Err(err).Msg("OnReturned: create notification")
	}

	borrower, err := w.users.FindByID(lr.BorrowerID)
	if err != nil {
		log.Warn().Err(err).Uint("borrower_id", lr.BorrowerID).Msg("OnReturned: load borrower")
		return nil // email is best-effort
	}

	bookCopy, err := w.copies.GetByIDWithAssociations(lr.CopyID)
	if err != nil {
		return fmt.Errorf("OnReturned: load copy: %w", err)
	}

	// Notify waitlisted users and clear the waitlist.
	if w.waitlists != nil {
		entries, wErr := w.waitlists.ListByCopyID(lr.CopyID)
		if wErr == nil && len(entries) > 0 {
			for _, entry := range entries {
				wn := models.Notification{
					RecipientID:   entry.UserID,
					Type:          "waitlist_available",
					LoanRequestID: &lr.ID,
				}
				if nErr := w.notifs.Create(&wn); nErr != nil {
					log.Warn().Err(nErr).Msg("OnReturned: waitlist notification")
				}
			}
			w.waitlists.DeleteByCopyID(lr.CopyID) //nolint:errcheck,gosec
		}
	}

	subject := "Your loan has been marked as returned"
	html := fmt.Sprintf(
		"<p>Hi %s,</p><p>Your loan of <em>%s</em> has been marked as returned. Thank you!</p>",
		borrower.Name, bookCopy.Book.Title,
	)
	return w.email.SendEmail(borrower.Email, subject, html)
}
