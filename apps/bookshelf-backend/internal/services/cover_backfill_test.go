package services

import (
	"net/http"
	"strings"
	"testing"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
)

func TestCoverBackfillCandidates_FiltersCorrectly(t *testing.T) {
	books := []models.Book{
		{ID: 1, CoverURL: "", ISBN: "111"},                  // candidate
		{ID: 2, CoverURL: "/api/covers/x.jpg", ISBN: "222"}, // already has a cover
		{ID: 3, CoverURL: "", OLKey: "OL1M"},                // candidate
		{ID: 4, CoverURL: ""},                               // no external key at all
	}

	got := coverBackfillCandidates(books)

	if len(got) != 2 {
		t.Fatalf("expected 2 candidates, got %d: %+v", len(got), got)
	}
	if got[0].ID != 1 || got[1].ID != 3 {
		t.Fatalf("unexpected candidates: %+v", got)
	}
}

func TestCoverBackfillService_Run_OneFailureDoesNotStopTheRun(t *testing.T) {
	client, _ := newStubClient(map[string]string{
		"bibkeys=ISBN:111": `{"ISBN:111":{}}`, // no cover found
		"bibkeys=ISBN:222": `{"ISBN:222":{"cover":{"large":"https://covers.openlibrary.org/b/id/9-L.jpg"}}}`,
	})

	repo := &stubBookRepo{books: []models.Book{
		{ID: 1, ISBN: "111", Title: "No Cover Anywhere"},
		{ID: 2, ISBN: "222", Title: "Has A Cover"},
	}}

	svc := &CoverBackfillService{books: repo, coversDir: t.TempDir(), client: client}

	// services.DownloadCover builds its own http.Client internally rather
	// than accepting one, so it falls back to http.DefaultTransport — swap
	// that for the test's duration to also serve the resolved cover URL's
	// image bytes without making a real network call.
	rt, ok := client.Transport.(*stubRoundTripper)
	if !ok {
		t.Fatal("expected client.Transport to be a *stubRoundTripper")
	}
	rt.responses["covers.openlibrary.org/b/id/9-L.jpg"] = stubResponse{
		body:        "fake-jpeg-bytes",
		contentType: "image/jpeg",
	}
	originalTransport := http.DefaultTransport
	http.DefaultTransport = rt
	defer func() { http.DefaultTransport = originalTransport }()

	result := svc.Run(t.Context())

	if !strings.HasPrefix(result, "backfilled 1 of 2 books") {
		t.Fatalf("expected 1 of 2 backfilled (one has no external cover), got %q", result)
	}
	if !strings.Contains(result, "✓ Has A Cover") {
		t.Fatalf("expected success line for the backfilled book, got %q", result)
	}
	if !strings.Contains(result, "✗ No Cover Anywhere — openlibrary(isbn): no cover/description; google_books(isbn): no api key configured") {
		t.Fatalf("expected failure line explaining why, got %q", result)
	}
}

func TestCoverBackfillService_Run_NoCandidatesIsANoop(t *testing.T) {
	repo := &stubBookRepo{books: []models.Book{
		{ID: 1, CoverURL: "/api/covers/already-set.jpg", ISBN: "111"},
	}}

	svc := NewCoverBackfillService(repo, t.TempDir(), "")
	result := svc.Run(t.Context())

	if result != "backfilled 0 of 0 books" {
		t.Fatalf("expected no candidates, got %q", result)
	}
}
