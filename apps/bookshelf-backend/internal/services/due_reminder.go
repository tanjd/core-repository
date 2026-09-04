package services

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// dueDateReminderDaysBeforeKey is the AdminRepository setting controlling how
// many days before expected_return_date a reminder fires. Read fresh on
// every run so an admin change takes effect on the job's next tick, same
// approach Scheduler.interval takes for its own settings.
const dueDateReminderDaysBeforeKey = "due_date_reminder_days_before"

// defaultDueDateReminderDaysBefore is used when the setting above is absent
// or invalid.
const defaultDueDateReminderDaysBefore = 2

// DueReminderService reminds borrowers whose accepted loan is due back
// soon — in-app always, plus email/Telegram per the borrower's own channel
// preferences, the same three-way fan-out every other event in this app
// uses (see loan_workflow.go). Originally shipped Telegram-only, since it
// was new capability rather than backfilling an existing channel — brought
// in line with the rest once that inconsistency stood out.
type DueReminderService struct {
	loanReqs repository.LoanRequestRepository
	notifs   repository.NotificationRepository
	admin    repository.AdminRepository
	telegram TelegramNotifier
	email    *EmailService
}

// NewDueReminderService creates a DueReminderService.
func NewDueReminderService(
	loanReqs repository.LoanRequestRepository,
	notifs repository.NotificationRepository,
	admin repository.AdminRepository,
	telegram TelegramNotifier,
	email *EmailService,
) *DueReminderService {
	return &DueReminderService{loanReqs: loanReqs, notifs: notifs, admin: admin, telegram: telegram, email: email}
}

// Run finds loans due back daysBefore() days from now that haven't already
// had a reminder sent, notifies each borrower (in-app always; email/Telegram
// per their own preferences), and marks them reminded. Returns a
// human-readable summary for JobStatus.LastResult, matching the signature
// RegisterJob expects.
func (s *DueReminderService) Run(ctx context.Context) string {
	dueDate := time.Now().UTC().AddDate(0, 0, s.daysBefore())

	loans, err := s.loanReqs.ListDueForReminder(dueDate)
	if err != nil {
		log.Error().Err(err).Msg("due-date-reminder: failed to list loans")
		return "failed: " + err.Error()
	}

	var reminded int
	for i := range loans {
		if s.remindOne(ctx, &loans[i]) {
			reminded++
		}
	}

	result := fmt.Sprintf("reminded %d of %d due loan(s)", reminded, len(loans))
	log.Info().Int("reminded", reminded).Int("total", len(loans)).Msg("due-date-reminder: run complete")
	return result
}

func (s *DueReminderService) remindOne(ctx context.Context, lr *models.LoanRequest) bool {
	borrower := lr.Borrower

	// Persist the reminded marker before sending: ListDueForReminder's next
	// run only re-selects loans where this is still nil, so marking first
	// guarantees a Save failure can't leave a loan re-eligible for a
	// duplicate reminder after notifications already went out.
	now := time.Now()
	lr.DueReminderSentAt = &now
	if err := s.loanReqs.Save(lr); err != nil {
		log.Warn().Err(err).Uint("loan_request_id", lr.ID).Msg("due-date-reminder: failed to mark reminded")
		return false
	}

	n := models.Notification{
		RecipientID:   borrower.ID,
		Type:          "loan_due_soon",
		LoanRequestID: &lr.ID,
	}
	if err := s.notifs.Create(&n); err != nil {
		log.Warn().Err(err).Uint("loan_request_id", lr.ID).Msg("due-date-reminder: create notification")
	}

	if borrower.EmailNotificationsEnabled {
		subject := "Your loan is due back soon"
		body := fmt.Sprintf(
			"<p>Hi %s,</p><p>Your loan of <em>%s</em> is due back %s — don't forget!</p>",
			html.EscapeString(borrower.Name), html.EscapeString(lr.Copy.Book.Title), lr.ExpectedReturnDate.Format("Jan 2"),
		) + s.email.Button("/my-requests", "View your loans")
		s.email.SendEmailAsync(ctx, borrower.Email, subject, body)
	}

	if borrower.WantsTelegram() {
		text := fmt.Sprintf(
			"<i>%s</i> is due back %s — don't forget!\n<a href=\"%s\">View your loans</a>",
			html.EscapeString(lr.Copy.Book.Title), lr.ExpectedReturnDate.Format("Jan 2"), s.email.URL("/my-requests"),
		)
		s.telegram.NotifyAsync(ctx, *borrower.TelegramChatID, text)
	}

	return true
}

// daysBefore reads dueDateReminderDaysBeforeKey from admin settings, falling
// back to defaultDueDateReminderDaysBefore.
func (s *DueReminderService) daysBefore() int {
	if s.admin != nil {
		if val, err := s.admin.GetSetting(dueDateReminderDaysBeforeKey); err == nil && val != "" {
			if d, err := strconv.Atoi(val); err == nil && d >= 0 {
				return d
			}
		}
	}
	return defaultDueDateReminderDaysBefore
}
