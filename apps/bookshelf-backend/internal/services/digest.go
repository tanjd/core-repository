package services

import (
	"context"
	"fmt"
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
}

// DigestService sends the monthly member digest on the configured day.
type DigestService struct {
	books           repository.BookRepository
	recommendations repository.RecommendationRepository
	users           repository.UserRepository
	admin           repository.AdminRepository
	email           digestEmailer
	clock           func() time.Time // injectable for tests; defaults to time.Now
}

// NewDigestService creates a DigestService wired to real repositories.
func NewDigestService(
	books repository.BookRepository,
	recommendations repository.RecommendationRepository,
	users repository.UserRepository,
	admin repository.AdminRepository,
	email *EmailService,
) *DigestService {
	return &DigestService{
		books:           books,
		recommendations: recommendations,
		users:           users,
		admin:           admin,
		email:           email,
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

	newBooksLimit := 10
	if v, _ := s.admin.GetSetting("monthly_digest_new_books_limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			newBooksLimit = n
		}
	}
	topLimit := 5
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

	return DigestContent{
		NewBooks:       newBooks,
		TopRecommended: topRecs,
		PreviousMonth:  prevMonthStart.Format("January 2006"),
		PeriodStart:    prevMonthStart,
		PeriodEnd:      prevMonthEnd,
	}, nil
}

// sendAll sends the digest to each recipient sequentially. Per-send errors
// are logged and counted rather than aborting the loop.
func (s *DigestService) sendAll(ctx context.Context, recipients []models.User, content DigestContent) (sent, failed int) {
	for _, r := range recipients {
		subject, html := s.render(r, content)
		if err := s.email.SendEmail(ctx, r.Email, subject, html); err != nil {
			log.Ctx(ctx).Error().Err(err).Str("to", r.Email).Msg("monthly-digest: send failed")
			failed++
		} else {
			sent++
		}
	}
	return
}

// render builds the subject line and HTML body for one recipient.
func (s *DigestService) render(recipient models.User, content DigestContent) (subject, html string) {
	subject = fmt.Sprintf("Bookshelf digest — %s", content.PreviousMonth)

	name := recipient.Name
	if name == "" {
		name = "there"
	}

	html = fmt.Sprintf("<p>Hi %s,</p>\n<p>Here's your Bookshelf update for <strong>%s</strong>.</p>\n", name, content.PreviousMonth)

	if len(content.NewBooks) > 0 {
		html += "<h2>New additions</h2>\n<ul>\n"
		for _, b := range content.NewBooks {
			html += fmt.Sprintf("  <li><strong>%s</strong> — %s</li>\n", b.Title, b.Author)
		}
		html += "</ul>\n"
	}

	if len(content.TopRecommended) > 0 {
		html += "<h2>Top picks from the community</h2>\n<ul>\n"
		for _, tr := range content.TopRecommended {
			html += fmt.Sprintf("  <li><strong>%s</strong> — %s (%d recommend)</li>\n",
				tr.Book.Title, tr.Book.Author, tr.Count)
		}
		html += "</ul>\n"
	}

	html += s.email.Button("/books", "Browse the library")
	html += fmt.Sprintf(
		`<p style="font-size:12px;color:#888;">Don't want these emails? `+
			`<a href="%s">Unsubscribe</a></p>`,
		s.email.UnsubscribeLink(recipient.ID),
	)
	return
}

// SendTestEmail assembles the previous month's content and sends one email to
// adminUser. Bypasses all gating (enabled flag, send-day, idempotency) and
// does not update monthly_digest_last_handled_month — it's a preview only.
func (s *DigestService) SendTestEmail(ctx context.Context, adminUser models.User) (string, error) {
	content, err := s.assembleContent(s.clock())
	if err != nil {
		return "", fmt.Errorf("assemble content: %w", err)
	}

	subject, html := s.render(adminUser, content)
	if err := s.email.SendEmail(ctx, adminUser.Email, subject, html); err != nil {
		return "", fmt.Errorf("send: %w", err)
	}
	return adminUser.Email, nil
}
