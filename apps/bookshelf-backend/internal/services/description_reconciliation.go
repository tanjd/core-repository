package services

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/bookmatch"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// DescriptionReconciliationService backfills a missing Book.Description from
// a sibling edition of the same work already in the catalog — the persisted
// counterpart to the search-time enrichAcrossEditions pass in
// internal/handlers/metadata_consolidate.go (see
// apps/bookshelf-backend/docs/cross-edition-metadata-enrichment.md), which
// only ever touches an ephemeral search response, never a stored Book row.
type DescriptionReconciliationService struct {
	books repository.BookRepository
}

// NewDescriptionReconciliationService creates a DescriptionReconciliationService.
func NewDescriptionReconciliationService(books repository.BookRepository) *DescriptionReconciliationService {
	return &DescriptionReconciliationService{books: books}
}

// Run backfills Description across the catalog and returns a human-readable
// summary for JobStatus.LastResult, matching the signature RegisterJob
// expects (same shape as BackupService.CreateSnapshot).
func (s *DescriptionReconciliationService) Run(_ context.Context) string {
	books, err := s.books.List("", "", false)
	if err != nil {
		log.Error().Err(err).Msg("description-reconciliation: failed to list books")
		return "failed: " + err.Error()
	}

	backfilled := 0
	for _, idxs := range bucketByWorkKey(books) {
		if len(idxs) < 2 {
			continue
		}
		backfilled += s.fillBucketDescriptions(books, idxs)
	}

	result := fmt.Sprintf("backfilled %d of %d books", backfilled, len(books))
	log.Info().Int("backfilled", backfilled).Int("total", len(books)).Msg("description-reconciliation: complete")
	return result
}

// bucketByWorkKey groups books' indices by bookmatch.NormalizeTitleAuthor.
// Books with an empty Title or Author are excluded from bucketing.
func bucketByWorkKey(books []models.Book) map[string][]int {
	buckets := map[string][]int{}
	for i, book := range books {
		if book.Title == "" || book.Author == "" {
			continue
		}
		key := bookmatch.NormalizeTitleAuthor(book.Title, book.Author)
		buckets[key] = append(buckets[key], i)
	}
	return buckets
}

// fillBucketDescriptions backfills each empty-Description book in the bucket
// (indices into books) from the lowest-ID member with a non-empty
// Description, skipping a donor whose Language conflicts with the target's.
// Donor selection is computed against a stable, ID-ascending snapshot taken
// before any writes — never against the live, mutating books slice. Without
// this, an already-backfilled book earlier in ID order could itself act as
// donor for a later book, and since only its Description (not its own
// Language field) gets copied, that later book's language guard could end up
// checked against the wrong (empty) Language instead of the true original
// donor's — silently bypassing the guard. Returns how many books were
// backfilled and persisted.
func (s *DescriptionReconciliationService) fillBucketDescriptions(books []models.Book, idxs []int) int {
	sorted := make([]int, len(idxs))
	copy(sorted, idxs)
	sort.Slice(sorted, func(i, j int) bool {
		return books[sorted[i]].ID < books[sorted[j]].ID
	})

	snapshot := make([]models.Book, len(sorted))
	for i, idx := range sorted {
		snapshot[i] = books[idx]
	}

	backfilled := 0
	for _, idx := range sorted {
		target := &books[idx]
		if target.Description != "" {
			continue
		}
		donor := descriptionDonor(snapshot, target.Language)
		if donor == "" {
			continue
		}
		target.Description = donor
		target.DescriptionEnriched = true
		if err := s.books.Save(target); err != nil {
			log.Warn().Err(err).Uint("book_id", target.ID).Msg("description-reconciliation: failed to save backfilled book")
			continue
		}
		backfilled++
	}
	return backfilled
}

// descriptionDonor returns the first non-empty Description in snapshot
// (already ID-ascending) whose Language doesn't conflict with
// targetLanguage, or "" if none qualifies.
func descriptionDonor(snapshot []models.Book, targetLanguage string) string {
	for _, donor := range snapshot {
		if donor.Description == "" {
			continue
		}
		if targetLanguage != "" && donor.Language != "" &&
			!strings.EqualFold(targetLanguage, donor.Language) {
			continue
		}
		return donor.Description
	}
	return ""
}
