package services

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// AuthorBackfillService finds an author for books that have none — e.g. a
// book added via a metadata source that never captured an author, or one
// entered manually and left blank. Unlike DescriptionReconciliationService,
// this has no in-catalog "free" pass: an author is expected to already be
// present on any sibling edition worth matching against (bucketByWorkKey
// itself requires a non-empty Author to group books at all), so there's no
// donor data to borrow from in-catalog — every candidate here goes straight
// to an external lookup.
type AuthorBackfillService struct {
	books              repository.BookRepository
	googleBooksKeyPool *GoogleBooksKeyPool
	hardcoverAPIKey    string
	client             *http.Client
}

// NewAuthorBackfillService creates an AuthorBackfillService. hardcoverAPIKey
// may be empty, in which case resolveExternalDataWithPool simply skips the
// Hardcover step (see externalLookupSteps).
func NewAuthorBackfillService(books repository.BookRepository, googleBooksKeyPool *GoogleBooksKeyPool, hardcoverAPIKey string) *AuthorBackfillService {
	return &AuthorBackfillService{
		books:              books,
		googleBooksKeyPool: googleBooksKeyPool,
		hardcoverAPIKey:    hardcoverAPIKey,
		client:             &http.Client{Timeout: 15 * time.Second},
	}
}

// Run backfills Author across the catalog and returns a human-readable
// summary for JobStatus.LastResult, matching the signature RegisterJob
// expects (same shape as CoverBackfillService.Run).
func (s *AuthorBackfillService) Run(ctx context.Context) string {
	books, err := s.books.List("", "title", false)
	if err != nil {
		log.Error().Err(err).Msg("author-backfill: failed to list books")
		return "failed: " + err.Error()
	}

	candidates := authorBackfillCandidates(books)

	backfilled := 0
	lines := make([]string, 0, len(candidates))
booksLoop:
	for i, book := range candidates {
		if i > 0 {
			select {
			case <-ctx.Done():
				break booksLoop
			case <-time.After(coverBackfillSpacing):
			}
		}
		ok, detail := s.backfillOne(ctx, &book)
		if ok {
			backfilled++
			lines = append(lines, fmt.Sprintf("✓ %s", book.Title))
		} else {
			lines = append(lines, fmt.Sprintf("✗ %s — %s", book.Title, detail))
		}
	}

	result := fmt.Sprintf("backfilled %d of %d books", backfilled, len(candidates))
	log.Info().Int("backfilled", backfilled).Int("candidates", len(candidates)).Msg("author-backfill: complete")
	if len(lines) == 0 {
		return result
	}
	return result + "\n" + strings.Join(lines, "\n")
}

// authorBackfillCandidates returns books with no author but at least one
// external key to look one up by.
func authorBackfillCandidates(books []models.Book) []models.Book {
	var candidates []models.Book
	for _, book := range books {
		if book.Author != "" {
			continue
		}
		if book.OLKey == "" && book.GoogleBooksID == "" && book.ISBN == "" {
			continue
		}
		candidates = append(candidates, book)
	}
	return candidates
}

// backfillOne resolves and saves an author for one book. Any failure is
// logged and treated as "no author found this run" — never aborts the batch.
func (s *AuthorBackfillService) backfillOne(ctx context.Context, book *models.Book) (bool, string) {
	data, attempts := resolveExternalDataWithPool(ctx, s.client, *book, s.googleBooksKeyPool, s.hardcoverAPIKey, wantedFields{author: true})
	if data.author == "" {
		return false, attemptsSummary(attempts)
	}

	book.Author = data.author
	if err := s.books.Save(book); err != nil {
		log.Warn().Err(err).Uint("book_id", book.ID).Msg("author-backfill: failed to save book")
		return false, "failed to save book: " + err.Error()
	}
	return true, ""
}
