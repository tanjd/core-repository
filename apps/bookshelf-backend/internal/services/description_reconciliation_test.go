package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
)

// newReconciliationDeps wires a fake BookRepository (with a CopyRepository,
// since List only returns books with at least one copy) and the service
// under test.
func newReconciliationDeps() (*DescriptionReconciliationService, *repotest.BookRepository, *repotest.CopyRepository) {
	books := repotest.NewBookRepository()
	copies := repotest.NewCopyRepository()
	books.SetCopies(copies)
	return NewDescriptionReconciliationService(books), books, copies
}

// addCatalogBook creates book and gives it a copy, so it's visible to List.
func addCatalogBook(t *testing.T, books *repotest.BookRepository, copies *repotest.CopyRepository, book models.Book) models.Book {
	t.Helper()
	require.NoError(t, books.Create(&book))
	require.NoError(t, copies.Create(&models.Copy{BookID: book.ID, Status: "available"}))
	return book
}

func findBook(t *testing.T, books *repotest.BookRepository, id uint) models.Book {
	t.Helper()
	b, err := books.GetByIDWithCopies(id)
	require.NoError(t, err)
	return *b
}

func TestDescriptionReconciliation_BackfillsSparseBookFromRicherSibling(t *testing.T) {
	svc, books, copies := newReconciliationDeps()
	rich := addCatalogBook(t, books, copies, models.Book{Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769", Description: "A great book"})
	sparse := addCatalogBook(t, books, copies, models.Book{Title: "Go in Action", Author: "Kennedy", ISBN: "9780134190440"})

	result := svc.Run(context.Background())

	assert.Equal(t, "backfilled 1 of 2 books", result)
	got := findBook(t, books, sparse.ID)
	assert.Equal(t, "A great book", got.Description)
	assert.True(t, got.DescriptionEnriched)
	donor := findBook(t, books, rich.ID)
	assert.False(t, donor.DescriptionEnriched, "the donor itself was not backfilled")
}

func TestDescriptionReconciliation_NeverOverwritesAlreadyPopulatedDescription(t *testing.T) {
	svc, books, copies := newReconciliationDeps()
	addCatalogBook(t, books, copies, models.Book{Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769", Description: "Better description"})
	own := addCatalogBook(t, books, copies, models.Book{Title: "Go in Action", Author: "Kennedy", ISBN: "9780134190440", Description: "Own description"})

	svc.Run(context.Background())

	got := findBook(t, books, own.ID)
	assert.Equal(t, "Own description", got.Description)
	assert.False(t, got.DescriptionEnriched)
}

func TestDescriptionReconciliation_ExcludesEmptyTitleOrAuthorFromBucketing(t *testing.T) {
	svc, books, copies := newReconciliationDeps()
	a := addCatalogBook(t, books, copies, models.Book{Title: "", Author: "Kennedy", ISBN: "9781617291769", Description: "d1"})
	b := addCatalogBook(t, books, copies, models.Book{Title: "Go in Action", Author: "", ISBN: "9780134190440", Description: "d2"})

	result := svc.Run(context.Background())

	assert.Equal(t, "backfilled 0 of 2 books", result)
	assert.False(t, findBook(t, books, a.ID).DescriptionEnriched)
	assert.False(t, findBook(t, books, b.ID).DescriptionEnriched)
}

func TestDescriptionReconciliation_SkipsDescriptionOnLanguageMismatch(t *testing.T) {
	svc, books, copies := newReconciliationDeps()
	addCatalogBook(t, books, copies, models.Book{Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769", Description: "Une bonne description", Language: "fr"})
	target := addCatalogBook(t, books, copies, models.Book{Title: "Go in Action", Author: "Kennedy", ISBN: "9780134190440", Language: "en"})

	svc.Run(context.Background())

	got := findBook(t, books, target.ID)
	assert.Empty(t, got.Description)
	assert.False(t, got.DescriptionEnriched)
}

func TestDescriptionReconciliation_LanguageGuardSurvivesCascadeThroughEarlierBackfill(t *testing.T) {
	// Regression test: donor selection must use a stable, pre-write snapshot,
	// not the live (mutating) books slice. A bug here previously let an
	// English-language target (d) get backfilled with a French description
	// because it "saw" an intermediate, already-backfilled book (a, whose own
	// Language field is empty) as an eligible donor — bypassing the language
	// guard, since the guard only checks the immediate donor's own Language,
	// not the true original source's.
	svc, books, copies := newReconciliationDeps()
	// Created in ID order: a (no language, gets backfilled first since it's
	// processed before d) must not become a laundering path for c's French
	// description to reach d.
	a := addCatalogBook(t, books, copies, models.Book{Title: "Go in Action", Author: "Kennedy", ISBN: "1"})
	addCatalogBook(t, books, copies, models.Book{Title: "Go in Action", Author: "Kennedy", ISBN: "2", Language: "fr"})
	addCatalogBook(t, books, copies, models.Book{Title: "Go in Action", Author: "Kennedy", ISBN: "3", Description: "Une bonne description", Language: "fr"})
	d := addCatalogBook(t, books, copies, models.Book{Title: "Go in Action", Author: "Kennedy", ISBN: "4", Language: "en"})

	result := svc.Run(context.Background())

	assert.Equal(t, "backfilled 2 of 4 books", result)
	assert.Equal(t, "Une bonne description", findBook(t, books, a.ID).Description, "unset-language target still fills best-effort")
	assert.Empty(t, findBook(t, books, d.ID).Description, "English target must never receive a French description, even via a cascade")
}

func TestDescriptionReconciliation_ThreeBooksDeterministicDonorByLowestID(t *testing.T) {
	svc, books, copies := newReconciliationDeps()
	// Created in this order, so IDs ascend: first (lowest ID) has no
	// description, second has one and must win as donor, third (higher ID,
	// also has a description) must never be picked over the second.
	first := addCatalogBook(t, books, copies, models.Book{Title: "Go in Action", Author: "Kennedy", ISBN: "1"})
	second := addCatalogBook(t, books, copies, models.Book{Title: "Go in Action", Author: "Kennedy", ISBN: "2", Description: "Second's description"})
	addCatalogBook(t, books, copies, models.Book{Title: "Go in Action", Author: "Kennedy", ISBN: "3", Description: "Third's description"})

	svc.Run(context.Background())

	got := findBook(t, books, first.ID)
	assert.Equal(t, "Second's description", got.Description)
	assert.True(t, got.DescriptionEnriched)
	assert.NotEqual(t, 0, second.ID)
}

func TestDescriptionReconciliation_IdempotentOnRerun(t *testing.T) {
	svc, books, copies := newReconciliationDeps()
	addCatalogBook(t, books, copies, models.Book{Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769", Description: "A great book"})
	addCatalogBook(t, books, copies, models.Book{Title: "Go in Action", Author: "Kennedy", ISBN: "9780134190440"})

	first := svc.Run(context.Background())
	second := svc.Run(context.Background())

	assert.Equal(t, "backfilled 1 of 2 books", first)
	assert.Equal(t, "backfilled 0 of 2 books", second, "nothing left to backfill on a repeat run")
}

func TestDescriptionReconciliation_EmptyCatalogNoPanic(t *testing.T) {
	svc, _, _ := newReconciliationDeps()

	assert.NotPanics(t, func() {
		result := svc.Run(context.Background())
		assert.Equal(t, "backfilled 0 of 0 books", result)
	})
}

func TestDescriptionReconciliation_SingleBookInCatalogNoPanic(t *testing.T) {
	svc, books, copies := newReconciliationDeps()
	addCatalogBook(t, books, copies, models.Book{Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769"})

	assert.NotPanics(t, func() {
		result := svc.Run(context.Background())
		assert.Equal(t, "backfilled 0 of 1 books", result)
	})
}
