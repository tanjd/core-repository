package services

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// coverBackfillSpacing is the delay between each book's external lookup
// during a backfill run — this job (unlike refreshCovers' concurrent
// cache-download retries) makes a fresh outbound request per book against
// Open Library/Google Books, so it's paced sequentially rather than run
// through a concurrent pool.
const coverBackfillSpacing = 1500 * time.Millisecond

// CoverBackfillService finds a cover for books that have none — e.g. a book
// first added when no cover was available, whose Book row later survived
// having its last Copy removed (see maybeDeleteOrphanedBook in
// internal/handlers/copies.go) and was never re-added through the UI, which
// is the only other path that backfills a cover (see backfillCover in
// internal/handlers/books.go). refreshCovers/refreshBookCover in
// scheduler.go don't help here — they only retry a cover that already has an
// external URL but failed to cache locally, never discover one that's
// missing entirely.
type CoverBackfillService struct {
	books          repository.BookRepository
	coversDir      string
	googleBooksKey string
	client         *http.Client
}

// NewCoverBackfillService creates a CoverBackfillService.
func NewCoverBackfillService(books repository.BookRepository, coversDir, googleBooksKey string) *CoverBackfillService {
	return &CoverBackfillService{
		books:          books,
		coversDir:      coversDir,
		googleBooksKey: googleBooksKey,
		client:         &http.Client{Timeout: 15 * time.Second},
	}
}

// Run backfills covers across the catalog and returns a human-readable
// summary for JobStatus.LastResult, matching the signature RegisterJob
// expects.
func (s *CoverBackfillService) Run(ctx context.Context) string {
	books, err := s.books.List("", "title", false)
	if err != nil {
		log.Error().Err(err).Msg("cover-backfill: failed to list books")
		return "failed: " + err.Error()
	}

	candidates := coverBackfillCandidates(books)

	backfilled := 0
booksLoop:
	for i, book := range candidates {
		if i > 0 {
			select {
			case <-ctx.Done():
				break booksLoop
			case <-time.After(coverBackfillSpacing):
			}
		}
		if s.backfillOne(ctx, &book) {
			backfilled++
		}
	}

	result := fmt.Sprintf("backfilled %d of %d books", backfilled, len(candidates))
	log.Info().Int("backfilled", backfilled).Int("candidates", len(candidates)).Msg("cover-backfill: complete")
	return result
}

// coverBackfillCandidates returns books with no cover but at least one
// external key to look one up by.
func coverBackfillCandidates(books []models.Book) []models.Book {
	var candidates []models.Book
	for _, book := range books {
		if book.CoverURL != "" {
			continue
		}
		if book.OLKey == "" && book.GoogleBooksID == "" && book.ISBN == "" {
			continue
		}
		candidates = append(candidates, book)
	}
	return candidates
}

// backfillOne resolves and saves a cover for one book. Any failure is
// logged and treated as "no cover found this run" — never aborts the batch.
func (s *CoverBackfillService) backfillOne(ctx context.Context, book *models.Book) bool {
	data := resolveExternalData(ctx, s.client, *book, s.googleBooksKey)
	if data.coverURL == "" {
		return false
	}

	if !IsCoverURLAllowed(data.coverURL) {
		log.Warn().Uint("book_id", book.ID).Msg("cover-backfill: resolved cover URL host not in allowlist, skipping")
		return false
	}

	localPath, err := DownloadCover(ctx, data.coverURL, s.coversDir)
	if err != nil {
		log.Warn().Err(err).Uint("book_id", book.ID).Msg("cover-backfill: cover download failed")
		return false
	}
	if localPath == "" {
		localPath = data.coverURL
	}

	book.CoverURL = localPath
	if err := s.books.Save(book); err != nil {
		log.Warn().Err(err).Uint("book_id", book.ID).Msg("cover-backfill: failed to save book")
		return false
	}
	return true
}
