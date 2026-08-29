package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
)

// --- fakes ---

// digestAdminRepo is a minimal AdminRepository fake for the digest tests.
// It stores settings in a map and records UpsertSetting calls.
type digestAdminRepo struct {
	stubAdminRepo
	mu       sync.Mutex
	settings map[string]string
}

func newDigestAdminRepo(settings map[string]string) *digestAdminRepo {
	if settings == nil {
		settings = map[string]string{}
	}
	return &digestAdminRepo{settings: settings}
}

func (r *digestAdminRepo) GetSetting(key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.settings[key], nil
}

func (r *digestAdminRepo) UpsertSetting(key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.settings[key] = value
	return nil
}

// fakeEmailService records sends without opening a real SMTP connection.
// errOnNth, if > 0, returns an error for exactly that recipient index (1-based).
type fakeEmailService struct {
	mu       sync.Mutex
	calls    int
	sent     []string // recipient emails, in order
	lastHTML string   // body of the most recent successful send
	errOnNth int
}

func (f *fakeEmailService) SendEmail(_ context.Context, recipient, _, html string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.errOnNth > 0 && f.calls == f.errOnNth {
		return errors.New("smtp: simulated failure")
	}
	f.sent = append(f.sent, recipient)
	f.lastHTML = html
	return nil
}

func (f *fakeEmailService) URL(path string) string { return "http://localhost:3000" + path }

func (f *fakeEmailService) Button(_, label string) string {
	return fmt.Sprintf("<button>%s</button>", label)
}

func (f *fakeEmailService) UnsubscribeLink(_ uint) string {
	return "http://localhost:3000/unsubscribe?token=test"
}

// --- helpers ---

// digestSettings returns the minimal settings needed for a digest run that
// should succeed: feature enabled, send on day 1, small limits, no
// last-handled-month (so any run is "fresh").
func digestSettings() map[string]string {
	return map[string]string{
		"monthly_digest_enabled":               "true",
		"monthly_digest_send_day":              "1",
		"monthly_digest_new_books_limit":       "10",
		"monthly_digest_top_recommended_limit": "5",
	}
}

// newDigestService builds a DigestService with the given clock and repos.
func newDigestService(
	books repository.BookRepository,
	recs repository.RecommendationRepository,
	users repository.UserRepository,
	admin repository.AdminRepository,
	email digestEmailer,
	clock func() time.Time,
) *DigestService {
	return &DigestService{
		books:           books,
		recommendations: recs,
		users:           users,
		admin:           admin,
		email:           email,
		clock:           clock,
	}
}

func seedRecipients(t *testing.T, users *repotest.UserRepository, n int) []models.User {
	t.Helper()
	var out []models.User
	for i := range n {
		u := models.User{
			Name:                 fmt.Sprintf("User %d", i),
			Email:                fmt.Sprintf("user%d@example.com", i),
			Verified:             true,
			MonthlyDigestEnabled: true,
		}
		require.NoError(t, users.Create(&u))
		out = append(out, u)
	}
	return out
}

// --- tests ---

func TestDigestService_Run_DisabledGating(t *testing.T) {
	admin := newDigestAdminRepo(map[string]string{
		"monthly_digest_enabled":  "false",
		"monthly_digest_send_day": "1",
	})
	today := time.Date(2025, 5, 1, 9, 0, 0, 0, time.Local)
	svc := newDigestService(
		repotest.NewBookRepository(),
		repotest.NewRecommendationRepository(nil),
		repotest.NewUserRepository(),
		admin,
		&fakeEmailService{},
		func() time.Time { return today },
	)
	result := svc.Run(context.Background())
	assert.Equal(t, "disabled", result)
}

func TestDigestService_Run_AlreadySentThisMonth(t *testing.T) {
	today := time.Date(2025, 5, 1, 9, 0, 0, 0, time.Local)
	admin := newDigestAdminRepo(map[string]string{
		"monthly_digest_enabled":            "true",
		"monthly_digest_send_day":           "1",
		"monthly_digest_last_handled_month": "2025-05",
	})
	svc := newDigestService(
		repotest.NewBookRepository(),
		repotest.NewRecommendationRepository(nil),
		repotest.NewUserRepository(),
		admin,
		&fakeEmailService{},
		func() time.Time { return today },
	)
	result := svc.Run(context.Background())
	assert.Equal(t, "already sent this month", result)
}

func TestDigestService_Run_WaitingForSendDay(t *testing.T) {
	today := time.Date(2025, 5, 5, 9, 0, 0, 0, time.Local)
	admin := newDigestAdminRepo(map[string]string{
		"monthly_digest_enabled":  "true",
		"monthly_digest_send_day": "10",
	})
	svc := newDigestService(
		repotest.NewBookRepository(),
		repotest.NewRecommendationRepository(nil),
		repotest.NewUserRepository(),
		admin,
		&fakeEmailService{},
		func() time.Time { return today },
	)
	result := svc.Run(context.Background())
	assert.Equal(t, "waiting for send day", result)
}

func TestDigestService_Run_EmptyPeriod_MarksHandled(t *testing.T) {
	today := time.Date(2025, 5, 1, 9, 0, 0, 0, time.Local)
	admin := newDigestAdminRepo(digestSettings())
	email := &fakeEmailService{}
	users := repotest.NewUserRepository()
	seedRecipients(t, users, 2)

	svc := newDigestService(
		repotest.NewBookRepository(),
		repotest.NewRecommendationRepository(nil),
		users,
		admin,
		email,
		func() time.Time { return today },
	)
	result := svc.Run(context.Background())
	assert.Equal(t, "nothing to send", result)
	assert.Empty(t, email.sent, "no emails sent when content is empty")

	handled, _ := admin.GetSetting("monthly_digest_last_handled_month")
	assert.Equal(t, "2025-05", handled, "month still marked handled even when empty")
}

func TestDigestService_Run_SendsToAllRecipients(t *testing.T) {
	today := time.Date(2025, 5, 1, 9, 0, 0, 0, time.Local)
	prevMonthStart := time.Date(2025, 4, 1, 0, 0, 0, 0, time.Local)
	prevMonthEnd := time.Date(2025, 5, 1, 0, 0, 0, 0, time.Local)

	books := repotest.NewBookRepository()
	book := models.Book{Title: "April Book", Author: "A", CreatedAt: prevMonthStart.Add(time.Hour)}
	require.NoError(t, books.Create(&book))

	users := repotest.NewUserRepository()
	recipients := seedRecipients(t, users, 3)

	admin := newDigestAdminRepo(digestSettings())
	email := &fakeEmailService{}

	svc := newDigestService(
		books,
		repotest.NewRecommendationRepository(nil),
		users,
		admin,
		email,
		func() time.Time { return today },
	)

	// Verify the book actually falls in the prev-month window before testing Run.
	booksInWindow, err := books.ListCreatedBetween(prevMonthStart, prevMonthEnd, 10)
	require.NoError(t, err)
	require.Len(t, booksInWindow, 1)

	result := svc.Run(context.Background())
	assert.True(t, strings.HasPrefix(result, "sent"), "result should start with 'sent': "+result)

	var sentAddrs []string
	for _, r := range recipients {
		sentAddrs = append(sentAddrs, r.Email)
	}
	assert.ElementsMatch(t, sentAddrs, email.sent)

	handled, _ := admin.GetSetting("monthly_digest_last_handled_month")
	assert.Equal(t, "2025-05", handled)
}

func TestDigestService_Run_Idempotent_SameMonth(t *testing.T) {
	today := time.Date(2025, 5, 1, 9, 0, 0, 0, time.Local)
	admin := newDigestAdminRepo(digestSettings())
	email := &fakeEmailService{}
	users := repotest.NewUserRepository()
	seedRecipients(t, users, 1)

	books := repotest.NewBookRepository()
	book := models.Book{Title: "B", Author: "A", CreatedAt: time.Date(2025, 4, 15, 0, 0, 0, 0, time.Local)}
	require.NoError(t, books.Create(&book))

	svc := newDigestService(books, repotest.NewRecommendationRepository(nil), users, admin, email, func() time.Time { return today })

	r1 := svc.Run(context.Background())
	r2 := svc.Run(context.Background())

	assert.True(t, strings.HasPrefix(r1, "sent"), r1)
	assert.Equal(t, "already sent this month", r2)
	assert.Len(t, email.sent, 1, "second run should not send again")
}

func TestDigestService_Run_ManualRunBypassesSendDay(t *testing.T) {
	// today is day 5, send day is 10 — a manual "run now" should still proceed.
	today := time.Date(2025, 5, 5, 9, 0, 0, 0, time.Local)
	admin := newDigestAdminRepo(map[string]string{
		"monthly_digest_enabled":               "true",
		"monthly_digest_send_day":              "10",
		"monthly_digest_new_books_limit":       "10",
		"monthly_digest_top_recommended_limit": "5",
	})
	email := &fakeEmailService{}
	users := repotest.NewUserRepository()
	seedRecipients(t, users, 1)

	books := repotest.NewBookRepository()
	book := models.Book{Title: "B", Author: "A", CreatedAt: time.Date(2025, 4, 15, 0, 0, 0, 0, time.Local)}
	require.NoError(t, books.Create(&book))

	svc := newDigestService(books, repotest.NewRecommendationRepository(nil), users, admin, email, func() time.Time { return today })

	result := svc.RunNow(context.Background())
	assert.True(t, strings.HasPrefix(result, "sent"), result)
}

func TestDigestService_Run_SendLoopToleratesFailure(t *testing.T) {
	today := time.Date(2025, 5, 1, 9, 0, 0, 0, time.Local)
	admin := newDigestAdminRepo(digestSettings())
	// errOnNth=2 means the 2nd recipient will get an error from SendEmail.
	email := &fakeEmailService{errOnNth: 2}
	users := repotest.NewUserRepository()
	seedRecipients(t, users, 5)

	books := repotest.NewBookRepository()
	book := models.Book{Title: "B", Author: "A", CreatedAt: time.Date(2025, 4, 15, 0, 0, 0, 0, time.Local)}
	require.NoError(t, books.Create(&book))

	svc := newDigestService(books, repotest.NewRecommendationRepository(nil), users, admin, email, func() time.Time { return today })

	result := svc.Run(context.Background())
	assert.Contains(t, result, "sent 4", result)
	assert.Contains(t, result, "failed 1", result)
	assert.Len(t, email.sent, 4, "4 of 5 recipients got the email")

	handled, _ := admin.GetSetting("monthly_digest_last_handled_month")
	assert.Equal(t, "2025-05", handled, "month marked handled even with partial failure")
}

func TestDigestService_SendTestEmail(t *testing.T) {
	today := time.Date(2025, 5, 15, 9, 0, 0, 0, time.Local)
	admin := newDigestAdminRepo(map[string]string{
		// feature disabled + month already handled — SendTestEmail must bypass both
		"monthly_digest_enabled":               "false",
		"monthly_digest_last_handled_month":    "2025-05",
		"monthly_digest_new_books_limit":       "10",
		"monthly_digest_top_recommended_limit": "5",
	})
	email := &fakeEmailService{}
	users := repotest.NewUserRepository()

	adminUser := models.User{Name: "Admin", Email: "admin@example.com"}
	require.NoError(t, users.Create(&adminUser))

	books := repotest.NewBookRepository()
	book := models.Book{Title: "April Book", Author: "A", CreatedAt: time.Date(2025, 4, 15, 0, 0, 0, 0, time.Local)}
	require.NoError(t, books.Create(&book))

	svc := newDigestService(books, repotest.NewRecommendationRepository(nil), users, admin, email, func() time.Time { return today })

	recipient, err := svc.SendTestEmail(context.Background(), adminUser)
	require.NoError(t, err)
	assert.Equal(t, adminUser.Email, recipient)
	assert.Equal(t, []string{adminUser.Email}, email.sent)

	// last_handled_month must NOT be updated by a test email
	handled, _ := admin.GetSetting("monthly_digest_last_handled_month")
	assert.Equal(t, "2025-05", handled, "SendTestEmail must not update last_handled_month")
}

func TestDigestService_SendTestEmail_EmptyContent_ShowsPreviewNote(t *testing.T) {
	today := time.Date(2025, 5, 15, 9, 0, 0, 0, time.Local)
	admin := newDigestAdminRepo(digestSettings())
	email := &fakeEmailService{}
	users := repotest.NewUserRepository()

	adminUser := models.User{Name: "Admin", Email: "admin@example.com"}
	require.NoError(t, users.Create(&adminUser))

	// No books and no recommendations at all — a real Run() would skip
	// sending entirely for this period; SendTestEmail should still send,
	// but with a note explaining the preview is empty.
	svc := newDigestService(
		repotest.NewBookRepository(), repotest.NewRecommendationRepository(nil),
		users, admin, email, func() time.Time { return today },
	)

	_, err := svc.SendTestEmail(context.Background(), adminUser)
	require.NoError(t, err)
	assert.Contains(t, email.lastHTML, "Nothing to report for April 2025")
}
