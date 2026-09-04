package services

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// digestEmailer is the subset of EmailService the digest uses, allowing
// injection of a fake in tests without depending on EmailService directly.
type digestEmailer interface {
	SendEmail(ctx context.Context, recipient, subject, html string) error
	URL(path string) string
	Button(path, label string) string
	UnsubscribeLink(userID uint) string
}

// DigestContent holds the assembled content for one monthly digest.
// Both slices may be empty — the service treats an all-empty digest as
// "nothing to send" and marks the month handled without emailing anyone.
type DigestContent struct {
	NewBooks       []models.Book
	TopRecommended []repository.TopRecommendedBook
	PreviousMonth  string // human label, e.g. "April 2025"
	PeriodStart    time.Time
	PeriodEnd      time.Time
	TotalBooks     int64 // total unique Book rows in the catalog right now
}

// DigestService sends the monthly member digest on the configured day.
type DigestService struct {
	books           repository.BookRepository
	recommendations repository.RecommendationRepository
	users           repository.UserRepository
	admin           repository.AdminRepository
	email           digestEmailer
	telegram        TelegramNotifier
	clock           func() time.Time // injectable for tests; defaults to time.Now
}

// NewDigestService creates a DigestService wired to real repositories.
func NewDigestService(
	books repository.BookRepository,
	recommendations repository.RecommendationRepository,
	users repository.UserRepository,
	admin repository.AdminRepository,
	email *EmailService,
	telegram TelegramNotifier,
) *DigestService {
	return &DigestService{
		books:           books,
		recommendations: recommendations,
		users:           users,
		admin:           admin,
		email:           email,
		telegram:        telegram,
		clock:           time.Now,
	}
}

// Run is the RegisterJob callback — called by the scheduler once per tick.
// It returns a human-readable status string that the admin Jobs page displays
// in LastResult.
//
// Gating (in order):
//  1. global on/off flag
//  2. once-per-month idempotency
//  3. send-day check (only the RunNow path skips this)
func (s *DigestService) Run(ctx context.Context) string {
	return s.run(ctx, false)
}

// RunNow is like Run but bypasses the send-day gate — for admin "run now"
// the feature should behave like a normal monthly send (still honours the
// once-per-month guard) but it shouldn't wait for the configured send day.
func (s *DigestService) RunNow(ctx context.Context) string {
	return s.run(ctx, true)
}

func (s *DigestService) run(ctx context.Context, skipSendDay bool) string {
	now := s.clock()
	currentMonth := now.Format("2006-01")

	if reason, skip := s.checkGates(now, currentMonth, skipSendDay); skip {
		return reason
	}

	content, err := s.assembleContent(now)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("monthly-digest: failed to assemble content")
		return "failed: " + err.Error()
	}

	if len(content.NewBooks) == 0 && len(content.TopRecommended) == 0 {
		if err := s.admin.UpsertSetting("monthly_digest_last_handled_month", currentMonth); err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("monthly-digest: failed to mark month handled")
		}
		return "nothing to send"
	}

	recipients, err := s.users.ListDigestRecipients(ctx)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("monthly-digest: failed to list recipients")
		return "failed: " + err.Error()
	}

	sent, failed := s.sendAll(ctx, recipients, content)

	if err := s.admin.UpsertSetting("monthly_digest_last_handled_month", currentMonth); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("monthly-digest: failed to mark month handled")
	}

	if failed > 0 {
		return fmt.Sprintf("sent %d, failed %d", sent, failed)
	}
	return fmt.Sprintf("sent %d", sent)
}

// checkGates evaluates the enabled/idempotency/send-day gates in order and
// reports the first one that should stop the run, if any.
func (s *DigestService) checkGates(now time.Time, currentMonth string, skipSendDay bool) (reason string, skip bool) {
	if enabled, _ := s.admin.GetSetting("monthly_digest_enabled"); enabled != "true" {
		return "disabled", true
	}

	if last, _ := s.admin.GetSetting("monthly_digest_last_handled_month"); last == currentMonth {
		return "already sent this month", true
	}

	if !skipSendDay {
		sendDay := 1
		if v, _ := s.admin.GetSetting("monthly_digest_send_day"); v != "" {
			if d, err := strconv.Atoi(v); err == nil && d >= 1 && d <= 28 {
				sendDay = d
			}
		}
		if now.Day() < sendDay {
			return "waiting for send day", true
		}
	}

	return "", false
}

// assembleContent gathers the new-books and top-recommended sections for the
// previous full calendar month relative to now.
func (s *DigestService) assembleContent(now time.Time) (DigestContent, error) {
	prevMonthStart := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location())
	prevMonthEnd := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	newBooksLimit := 3
	if v, _ := s.admin.GetSetting("monthly_digest_new_books_limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			newBooksLimit = n
		}
	}
	topLimit := 3
	if v, _ := s.admin.GetSetting("monthly_digest_top_recommended_limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			topLimit = n
		}
	}

	newBooks, err := s.books.ListCreatedBetween(prevMonthStart, prevMonthEnd, newBooksLimit)
	if err != nil {
		return DigestContent{}, fmt.Errorf("list new books: %w", err)
	}

	topRecs, err := s.recommendations.ListTopBooks(topLimit)
	if err != nil {
		return DigestContent{}, fmt.Errorf("list top books: %w", err)
	}

	totalBooks, err := s.books.CountAll()
	if err != nil {
		return DigestContent{}, fmt.Errorf("count books: %w", err)
	}

	return DigestContent{
		NewBooks:       newBooks,
		TopRecommended: topRecs,
		PreviousMonth:  prevMonthStart.Format("January 2006"),
		PeriodStart:    prevMonthStart,
		PeriodEnd:      prevMonthEnd,
		TotalBooks:     totalBooks,
	}, nil
}

// sendAll sends the digest to each recipient sequentially. Per-send errors
// are logged and counted rather than aborting the loop. MonthlyDigestEnabled
// (already applied by ListDigestRecipients) is the sole opt-in for the
// digest itself; which channel(s) actually carry it is then decided per
// recipient from their own general notification preferences, same as every
// other notification in this app (see EmailNotificationsEnabled usage in
// loan_workflow.go/wishlist_workflow.go) rather than always emailing
// regardless of that setting.
func (s *DigestService) sendAll(ctx context.Context, recipients []models.User, content DigestContent) (sent, failed int) {
	for _, r := range recipients {
		if r.EmailNotificationsEnabled {
			subject, html := s.render(r, content, false)
			if err := s.email.SendEmail(ctx, r.Email, subject, html); err != nil {
				log.Ctx(ctx).Error().Err(err).Str("to", r.Email).Msg("monthly-digest: send failed")
				failed++
			} else {
				sent++
			}
		}

		// Best-effort second channel, same NotifyAsync-fire-and-forget shape
		// every other event notification uses — a failed Telegram push
		// doesn't affect the email-based sent/failed counts above.
		if r.WantsTelegram() {
			s.telegram.NotifyAsync(ctx, *r.TelegramChatID, s.renderTelegram(content))
		}
	}
	return
}

// render builds the subject line and HTML body for one recipient. isPreview
// is true only for SendTestEmail's admin preview — a real send never reaches
// this function with both sections empty (run() short-circuits before
// sendAll in that case), so the empty-content note below only ever appears
// in a preview, never in an email a member actually receives.
func (s *DigestService) render(recipient models.User, content DigestContent, isPreview bool) (subject, body string) {
	subject = digestSubject(content)

	name := html.EscapeString(recipient.Name)
	if recipient.Name == "" {
		name = "there"
	}

	// Hidden preheader: most inbox lists show this text right after the
	// subject line, so it's worth tailoring rather than leaving clients to
	// fall back to the first visible line ("Hi <name>, ...").
	body = fmt.Sprintf(`<div style="display:none;max-height:0;overflow:hidden;">%s</div>`, digestPreheader(content))
	body += fmt.Sprintf("<p>Hi %s,</p>\n<p>Here's your Bookshelf update for <strong>%s</strong>.</p>\n", name, content.PreviousMonth)
	body += fmt.Sprintf("<p>The library now has <strong>%d</strong> books.</p>\n", content.TotalBooks)

	if isPreview && len(content.NewBooks) == 0 && len(content.TopRecommended) == 0 {
		body += fmt.Sprintf(
			"<p><em>Nothing to report for %s yet — no books were added that month and there are no "+
				"community recommendations. A real digest would not be sent for an empty month like this; "+
				"you're only seeing this because a preview always sends.</em></p>\n",
			content.PreviousMonth,
		)
	}

	if len(content.NewBooks) > 0 {
		body += fmt.Sprintf("<h2>Latest %d additions</h2>\n<ul>\n", len(content.NewBooks))
		for _, b := range content.NewBooks {
			body += fmt.Sprintf("  <li><strong>%s</strong> — %s</li>\n",
				html.EscapeString(b.Title), html.EscapeString(b.Author))
		}
		body += "</ul>\n"
	}

	if len(content.TopRecommended) > 0 {
		body += "<h2>Top picks from the community</h2>\n<ul>\n"
		for _, tr := range content.TopRecommended {
			body += fmt.Sprintf("  <li><strong>%s</strong> — %s (%d recommend)</li>\n",
				html.EscapeString(tr.Book.Title), html.EscapeString(tr.Book.Author), tr.Count)
		}
		body += "</ul>\n"
	}

	body += s.email.Button("/catalog", "Browse the library")
	body += fmt.Sprintf(
		`<p style="font-size:13px;color:#555;">Looking for a book we don't have yet? `+
			`<a href="%s">Post it to the wishlist board</a>.</p>`,
		s.email.URL("/wishlist"),
	)
	body += fmt.Sprintf(
		`<p style="font-size:12px;color:#888;">Don't want these emails? `+
			`<a href="%s">Unsubscribe</a></p>`,
		s.email.UnsubscribeLink(recipient.ID),
	)
	return
}

// renderTelegram builds the Telegram-formatted counterpart to render's email
// body — same content, restyled for Telegram's much smaller HTML subset
// (<b>, <i>, <a href>; no <ul>/<li>/<h2>/inline styles), matching the plain,
// line-based message shape every other Telegram push in this app uses (see
// due_reminder.go, loan_workflow.go).
func (s *DigestService) renderTelegram(content DigestContent) string {
	text := fmt.Sprintf("<b>Bookshelf update — %s</b>\n", content.PreviousMonth)
	text += fmt.Sprintf("The library now has %d books.\n", content.TotalBooks)

	if len(content.NewBooks) > 0 {
		text += fmt.Sprintf("\n<b>Latest %d additions</b>\n", len(content.NewBooks))
		for _, b := range content.NewBooks {
			text += fmt.Sprintf("• <b>%s</b> — %s\n", html.EscapeString(b.Title), html.EscapeString(b.Author))
		}
	}

	if len(content.TopRecommended) > 0 {
		text += "\n<b>Top picks from the community</b>\n"
		for _, tr := range content.TopRecommended {
			text += fmt.Sprintf("• <b>%s</b> — %s (%d recommend)\n",
				html.EscapeString(tr.Book.Title), html.EscapeString(tr.Book.Author), tr.Count)
		}
	}

	text += fmt.Sprintf("\n<a href=\"%s\">Browse the library</a>", s.email.URL("/catalog"))
	return text
}

// digestSubject builds a subject line reflecting the actual content of the
// digest, rather than a static "Bookshelf digest" label, since a subject
// that names what's inside earns more opens.
func digestSubject(content DigestContent) string {
	switch {
	case len(content.NewBooks) > 0 && len(content.TopRecommended) > 0:
		return fmt.Sprintf("%d new books + this month's top pick — %s", len(content.NewBooks), content.PreviousMonth)
	case len(content.NewBooks) > 0:
		return fmt.Sprintf("%d new books added to the library — %s", len(content.NewBooks), content.PreviousMonth)
	case len(content.TopRecommended) > 0:
		return fmt.Sprintf("This month's top picks from the community — %s", content.PreviousMonth)
	default:
		return fmt.Sprintf("Bookshelf digest — %s", content.PreviousMonth)
	}
}

// digestPreheader builds the one-line teaser most inbox lists show right
// after the subject, mirroring digestSubject's content-aware logic.
func digestPreheader(content DigestContent) string {
	switch {
	case len(content.NewBooks) > 0 && len(content.TopRecommended) > 0:
		return fmt.Sprintf("%d new books added this month, plus the community's top pick.", len(content.NewBooks))
	case len(content.NewBooks) > 0:
		return fmt.Sprintf("%d new books were added to the library this month.", len(content.NewBooks))
	case len(content.TopRecommended) > 0:
		return "See what the community's been recommending this month."
	default:
		return "Your monthly Bookshelf update."
	}
}

// SendTestEmail assembles the previous month's content and sends one email to
// adminUser. Bypasses all gating (enabled flag, send-day, idempotency) and
// does not update monthly_digest_last_handled_month — it's a preview only.
func (s *DigestService) SendTestEmail(ctx context.Context, adminUser models.User) (string, error) {
	content, err := s.assembleContent(s.clock())
	if err != nil {
		return "", fmt.Errorf("assemble content: %w", err)
	}

	subject, html := s.render(adminUser, content, true)
	if err := s.email.SendEmail(ctx, adminUser.Email, subject, html); err != nil {
		return "", fmt.Errorf("send: %w", err)
	}
	return adminUser.Email, nil
}

// ErrTelegramNotLinked is returned by SendTestTelegram when adminUser has no
// linked Telegram chat to send the preview to.
var ErrTelegramNotLinked = errors.New("telegram not linked")

// SendTestTelegram is SendTestEmail's Telegram counterpart: assembles the
// previous month's content and pushes one preview message to adminUser's
// linked Telegram chat. Same bypass-all-gating, preview-only contract —
// doesn't touch monthly_digest_last_handled_month. Uses the synchronous
// Notify (not NotifyAsync) so the caller learns whether delivery actually
// succeeded, same as the profile "send test notification" action.
func (s *DigestService) SendTestTelegram(ctx context.Context, adminUser models.User) error {
	if adminUser.TelegramChatID == nil {
		return ErrTelegramNotLinked
	}

	content, err := s.assembleContent(s.clock())
	if err != nil {
		return fmt.Errorf("assemble content: %w", err)
	}

	if err := s.telegram.Notify(ctx, *adminUser.TelegramChatID, s.renderTelegram(content)); err != nil {
		return fmt.Errorf("send: %w", err)
	}
	return nil
}
