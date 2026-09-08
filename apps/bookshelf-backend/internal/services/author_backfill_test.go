package services

import (
	"strings"
	"testing"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
)

func TestAuthorBackfillCandidates_FiltersCorrectly(t *testing.T) {
	books := []models.Book{
		{ID: 1, Author: "", ISBN: "111"},                // candidate
		{ID: 2, Author: "Existing Author", ISBN: "222"}, // already has an author
		{ID: 3, Author: "", OLKey: "OL1M"},              // candidate
		{ID: 4, Author: ""},                             // no external key at all
	}

	got := authorBackfillCandidates(books)

	if len(got) != 2 {
		t.Fatalf("expected 2 candidates, got %d: %+v", len(got), got)
	}
	if got[0].ID != 1 || got[1].ID != 3 {
		t.Fatalf("unexpected candidates: %+v", got)
	}
}

func TestAuthorBackfillService_Run_OneFailureDoesNotStopTheRun(t *testing.T) {
	client, _ := newStubClient(map[string]string{
		"bibkeys=ISBN:111": `{"ISBN:111":{}}`, // no author found
		"bibkeys=ISBN:222": `{"ISBN:222":{"authors":[{"name":"Found Author"}]}}`,
	})

	repo := &stubBookRepo{books: []models.Book{
		{ID: 1, ISBN: "111", Title: "No Author Anywhere"},
		{ID: 2, ISBN: "222", Title: "Has An Author"},
	}}

	svc := &AuthorBackfillService{books: repo, client: client, googleBooksKeyPool: NewGoogleBooksKeyPool(nil)}

	result := svc.Run(t.Context())

	if !strings.HasPrefix(result, "backfilled 1 of 2 books") {
		t.Fatalf("expected 1 of 2 backfilled (one has no external author), got %q", result)
	}
	if !strings.Contains(result, "✓ Has An Author") {
		t.Fatalf("expected success line for the backfilled book, got %q", result)
	}
	if !strings.Contains(result, "✗ No Author Anywhere — openlibrary(isbn): no cover/description; google_books(isbn): no api key configured") {
		t.Fatalf("expected failure line explaining why, got %q", result)
	}
}

func TestAuthorBackfillService_Run_NoCandidatesIsANoop(t *testing.T) {
	repo := &stubBookRepo{books: []models.Book{
		{ID: 1, Author: "Already Set", ISBN: "111"},
	}}

	svc := NewAuthorBackfillService(repo, NewGoogleBooksKeyPool(nil), "")
	result := svc.Run(t.Context())

	if result != "backfilled 0 of 0 books" {
		t.Fatalf("expected no candidates, got %q", result)
	}
}

func TestAuthorBackfillService_Run_JoinsMultipleAuthors(t *testing.T) {
	client, _ := newStubClient(map[string]string{
		"bibkeys=ISBN:333": `{"ISBN:333":{"authors":[{"name":"Author One"},{"name":"Author Two"}]}}`,
	})

	repo := &stubBookRepo{books: []models.Book{
		{ID: 1, ISBN: "333", Title: "Co-Authored Book"},
	}}

	svc := &AuthorBackfillService{books: repo, client: client, googleBooksKeyPool: NewGoogleBooksKeyPool(nil)}

	result := svc.Run(t.Context())

	if !strings.HasPrefix(result, "backfilled 1 of 1 books") {
		t.Fatalf("expected the book to be backfilled, got %q", result)
	}
	if !strings.Contains(result, "✓ Co-Authored Book") {
		t.Fatalf("expected success line for the backfilled book, got %q", result)
	}
}
