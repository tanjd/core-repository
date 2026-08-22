package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeISBN(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "already ISBN-13", in: "978-0-13-468599-1", want: "9780134685991"},
		{name: "ISBN-10 converts to ISBN-13", in: "0-13-468599-7", want: "9780134685991"},
		{name: "ISBN-10 with X check digit", in: "080442957X", want: "9780804429573"},
		{name: "invalid length", in: "123", want: ""},
		{name: "non-numeric ISBN-13", in: "97801346859XX", want: ""},
		{name: "empty string", in: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeISBN(tt.in))
		})
	}
}

func TestNormalizeTitleAuthor(t *testing.T) {
	tests := []struct {
		name         string
		title        string
		author       string
		wantEqualsTo string
	}{
		{
			name:  "case and punctuation are normalized the same",
			title: "The Go Programming Language", author: "Donovan, Alan",
			wantEqualsTo: normalizeTitleAuthor("the go programming language", "donovan alan"),
		},
		{
			name:  "extra whitespace collapses",
			title: "  Go   in   Action  ", author: "Kennedy",
			wantEqualsTo: normalizeTitleAuthor("go in action", "kennedy"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantEqualsTo, normalizeTitleAuthor(tt.title, tt.author))
		})
	}
}

func TestConsolidateResults_DeduplicatesByISBN(t *testing.T) {
	results := []BookMetadataResult{
		{Source: "openlibrary", Title: "Go in Action", Author: "Kennedy", ISBN: "978-1-61729-176-9", CoverURL: "ol-cover.jpg"},
		{Source: "google_books", Title: "Go in Action", Author: "William Kennedy", ISBN: "9781617291769", Description: "A great book"},
	}

	got := consolidateResults(results)

	assert.Len(t, got, 1, "same ISBN across sources should merge into one result")
	assert.Equal(t, "google_books", got[0].Source, "google_books should win the source-priority tiebreak")
	assert.Equal(t, "William Kennedy", got[0].Author, "should take Author from the higher-priority source")
	assert.Equal(t, "ol-cover.jpg", got[0].CoverURL, "should fall back to a lower-priority source for fields the winner lacks")
	assert.Equal(t, "A great book", got[0].Description)
}

func TestConsolidateResults_DeduplicatesByTitleAuthorWhenNoISBN(t *testing.T) {
	results := []BookMetadataResult{
		{Source: "openlibrary", Title: "Go in Action", Author: "Kennedy", PageCount: 300},
		{Source: "bookbrainz", Title: "go in action", Author: "kennedy", PageCount: 0},
	}

	got := consolidateResults(results)

	assert.Len(t, got, 1, "matching normalized title+author should merge even without an ISBN")
	assert.Equal(t, 300, got[0].PageCount)
}

func TestConsolidateResults_KeepsDistinctISBNEditionsSeparateEvenWithMatchingTitleAuthor(t *testing.T) {
	results := []BookMetadataResult{
		{Source: "openlibrary", Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769"},
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9780134190440"},
	}

	got := consolidateResults(results)

	assert.Len(t, got, 2, "two results with distinct, valid ISBNs must never be folded together via the title+author fallback, even when title+author match")
}

func TestConsolidateResults_ISBNLessResultStillMergesWithSameEditionCarryingAnISBN(t *testing.T) {
	// BookBrainz never sets ISBN (see copies_import.go's findFuzzyMatch doc
	// comment) — its hit for an edition another source reports with an ISBN
	// must still merge into one result, not be treated as a second, distinct
	// edition. Regression test for an over-correction of the fix above: an
	// early version blocked the title+author fallback whenever the *incoming*
	// result had any ISBN, which also wrongly blocked this legitimate case.
	results := []BookMetadataResult{
		{Source: "bookbrainz", Title: "Go in Action", Author: "Kennedy"},
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769", Description: "A great book"},
	}

	got := consolidateResults(results)

	assert.Len(t, got, 1, "an ISBN-less hit for the same edition must still merge with an ISBN-bearing hit for it")
	assert.Equal(t, "9781617291769", got[0].ISBN)
}

func TestConsolidateResults_ISBNLessResultThenTwoDistinctISBNEditionsStayThreeSeparate(t *testing.T) {
	// Order matters: the ISBN-less result arrives first and claims the
	// title+author group; the first ISBN-bearing arrival attaches its ISBN
	// to that same group (legitimate merge, previous test); a second,
	// different-ISBN arrival must still get its own group rather than
	// silently absorbing into the now-ISBN-claimed group.
	results := []BookMetadataResult{
		{Source: "bookbrainz", Title: "Go in Action", Author: "Kennedy"},
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769"},
		{Source: "openlibrary", Title: "Go in Action", Author: "Kennedy", ISBN: "9780134190440"},
	}

	got := consolidateResults(results)

	assert.Len(t, got, 2)
}

func TestConsolidateResults_KeepsDistinctBooksSeparate(t *testing.T) {
	results := []BookMetadataResult{
		{Source: "openlibrary", Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769"},
		{Source: "openlibrary", Title: "The Go Programming Language", Author: "Donovan", ISBN: "9780134190440"},
	}

	got := consolidateResults(results)

	assert.Len(t, got, 2)
}

func TestConsolidateResults_RanksByCompletenessThenTitle(t *testing.T) {
	results := []BookMetadataResult{
		{Source: "bookbrainz", Title: "Sparse Book", Author: "A"},
		{Source: "google_books", Title: "Complete Book", Author: "B", CoverURL: "c.jpg", Description: "d", ISBN: "1", Publisher: "p", PageCount: 100},
	}

	got := consolidateResults(results)

	assert.Len(t, got, 2)
	assert.Equal(t, "Complete Book", got[0].Title, "more complete result should rank first")
	assert.Equal(t, "Sparse Book", got[1].Title)
}

func TestConsolidateResults_RankTieBreakIsCaseInsensitive(t *testing.T) {
	// Regression test for a real-world ranking bug: Open Library's title
	// casing is inconsistent across editions of the same work — the genuine
	// English edition of "Church Discipline" by Jonathan Leeman is indexed as
	// "Church discipline" (lowercase "d"), while a Burmese translation is
	// indexed as "Church Discipline (Burmese)" (capital "D"). Neither carries
	// a cover from Open Library's search endpoint, so both tie on
	// scoreResult (ISBN only), and a case-sensitive tie-break let the
	// capitalized Burmese title sort first purely as an ASCII-ordering
	// artifact, not because it was actually the better match.
	results := []BookMetadataResult{
		{Source: "openlibrary", Title: "Church Discipline (Burmese)", Author: "Jonathan Leeman", ISBN: "9781955768849"},
		{Source: "openlibrary", Title: "Church discipline", Author: "Jonathan Leeman", ISBN: "9781433532368"},
	}

	got := consolidateResults(results)

	require.Len(t, got, 2)
	assert.Equal(t, "Church discipline", got[0].Title, "the genuine English edition must rank first, not a differently-cased translated sibling")
	assert.Equal(t, "Church Discipline (Burmese)", got[1].Title)
}

func TestConsolidateResults_EmptyInput(t *testing.T) {
	got := consolidateResults(nil)
	assert.Empty(t, got)
}

func TestConsolidateResults_EnrichesSparseEditionFromRicherEdition(t *testing.T) {
	results := []BookMetadataResult{
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769", Description: "A great book"},
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9780134190440"},
	}

	got := consolidateResults(results)

	assert.Len(t, got, 2)
	var rich, sparse BookMetadataResult
	for _, r := range got {
		if r.ISBN == "9781617291769" {
			rich = r
		} else {
			sparse = r
		}
	}
	assert.Equal(t, "A great book", sparse.Description, "sparse edition should be backfilled from its richer sibling")
	assert.Equal(t, []string{"description"}, sparse.EnrichedFields)
	assert.Equal(t, "A great book", rich.Description, "richer edition's own description should be untouched")
	assert.Empty(t, rich.EnrichedFields, "the donor edition itself was not enriched")
	assert.Empty(t, sparse.CoverURL, "CoverURL is never cross-filled")
}

func TestConsolidateResults_DoesNotCrossFillEditionSpecificFields(t *testing.T) {
	results := []BookMetadataResult{
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769", Publisher: "Manning", PublishedDate: "2015", PageCount: 300},
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9780134190440"},
	}

	got := consolidateResults(results)

	for _, r := range got {
		if r.ISBN == "9780134190440" {
			assert.Empty(t, r.Publisher, "Publisher is edition-specific, never cross-filled")
			assert.Empty(t, r.PublishedDate, "PublishedDate is edition-specific, never cross-filled")
			assert.Zero(t, r.PageCount, "PageCount is edition-specific, never cross-filled")
		}
	}
}

func TestConsolidateResults_NeverOverwritesAlreadyPopulatedField(t *testing.T) {
	results := []BookMetadataResult{
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769", Description: "Better description"},
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9780134190440", Description: "Own description"},
	}

	got := consolidateResults(results)

	for _, r := range got {
		if r.ISBN == "9780134190440" {
			assert.Equal(t, "Own description", r.Description, "an already-populated field is never overwritten")
			assert.Empty(t, r.EnrichedFields)
		}
	}
}

func TestConsolidateResults_DoesNotCopyIdentityFieldsAcrossEditions(t *testing.T) {
	results := []BookMetadataResult{
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769", OLKey: "OL1W", GoogleBooksID: "gb1", BookBrainzID: "bb1"},
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9780134190440"},
	}

	got := consolidateResults(results)

	for _, r := range got {
		if r.ISBN == "9780134190440" {
			assert.Empty(t, r.OLKey)
			assert.Empty(t, r.GoogleBooksID)
			assert.Empty(t, r.BookBrainzID)
		}
	}
}

func TestConsolidateResults_ExcludesEmptyTitleOrAuthorFromBucketing(t *testing.T) {
	results := []BookMetadataResult{
		{Source: "openlibrary", Title: "", Author: "Kennedy", ISBN: "9781617291769", Description: "d1"},
		{Source: "openlibrary", Title: "Go in Action", Author: "", ISBN: "9780134190440", Description: "d2"},
	}

	got := consolidateResults(results)

	for _, r := range got {
		assert.Empty(t, r.WorkKey, "entries with an empty Title or Author must not be bucketed")
		assert.Empty(t, r.EnrichedFields)
	}
}

func TestConsolidateResults_SkipsDescriptionOnLanguageMismatch(t *testing.T) {
	results := []BookMetadataResult{
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769", Description: "Une bonne description", Language: "fr"},
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9780134190440", Language: "en"},
	}

	got := consolidateResults(results)

	for _, r := range got {
		if r.ISBN == "9780134190440" {
			assert.Empty(t, r.Description, "a donor description in a different language must not be backfilled")
			assert.Empty(t, r.EnrichedFields)
		}
	}
}

func TestConsolidateResults_FillsDescriptionWhenLanguageUnsetOnEitherSide(t *testing.T) {
	results := []BookMetadataResult{
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769", Description: "A great book", Language: "en"},
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9780134190440"},
	}

	got := consolidateResults(results)

	for _, r := range got {
		if r.ISBN == "9780134190440" {
			assert.Equal(t, "A great book", r.Description, "an unset target Language should proceed best-effort")
		}
	}
}

func TestConsolidateResults_MultipleDonorsPicksDeterministically(t *testing.T) {
	results := []BookMetadataResult{
		{Source: "bookbrainz", Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769", Description: "BookBrainz description"},
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9780134190440", Description: "Google description"},
		{Source: "openlibrary", Title: "Go in Action", Author: "Kennedy", ISBN: "9780134685991"},
	}

	got := consolidateResults(results)

	for _, r := range got {
		if r.ISBN == "9780134685991" {
			assert.Equal(t, "Google description", r.Description, "google_books has the highest source priority among donors")
		}
	}
}

func TestConsolidateResults_ThreeEditionsFillIndependentlyWithoutCascading(t *testing.T) {
	results := []BookMetadataResult{
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769", Description: "Original description"},
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9780134190440"},
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9780134685991"},
	}

	got := consolidateResults(results)

	for _, r := range got {
		if r.ISBN != "9781617291769" {
			assert.Equal(t, "Original description", r.Description)
			assert.Equal(t, []string{"description"}, r.EnrichedFields)
		}
	}
}

func TestConsolidateResults_SingleEditionBucketIsNoop(t *testing.T) {
	results := []BookMetadataResult{
		{Source: "openlibrary", Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769"},
	}

	assert.NotPanics(t, func() {
		got := consolidateResults(results)
		assert.Len(t, got, 1)
		assert.Empty(t, got[0].Description)
		assert.NotEmpty(t, got[0].WorkKey)
	})
}

func TestConsolidateResults_EnrichedDescriptionIsAlwaysLabeled(t *testing.T) {
	results := []BookMetadataResult{
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769", Description: "A great book"},
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9780134190440"},
	}

	got := consolidateResults(results)

	for _, r := range got {
		hasBackfilledDescription := r.Description != "" && r.ISBN == "9780134190440"
		if hasBackfilledDescription {
			assert.Contains(t, r.EnrichedFields, "description")
		}
		if len(r.EnrichedFields) == 0 {
			continue
		}
		assert.Contains(t, r.EnrichedFields, "description", "no code path may set EnrichedFields for a field it didn't actually backfill")
	}
}

func TestConsolidateResults_WorkKeyMatchesBucketKey(t *testing.T) {
	results := []BookMetadataResult{
		{Source: "openlibrary", Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769"},
		{Source: "openlibrary", Title: "go in action", Author: "kennedy", ISBN: "9780134190440"},
	}

	got := consolidateResults(results)

	want := normalizeTitleAuthor("Go in Action", "Kennedy")
	for _, r := range got {
		assert.Equal(t, want, r.WorkKey)
	}
}

func TestConsolidateResults_FullPipeline_MultiSourceMergePlusCrossEditionBackfill(t *testing.T) {
	// Realistic fan-out shape: one edition (hardcover) is reported by two
	// sources and merges via ISBN into a single rich result; a second,
	// distinct-ISBN edition (paperback) is reported by only one source and
	// arrives sparse. Exercises deduplicateIntoGroups' ISBN-based merge and
	// enrichAcrossEditions' cross-edition backfill together, not in
	// isolation — the actual order both run in inside consolidateResults.
	results := []BookMetadataResult{
		{Source: "openlibrary", Title: "Go in Action", Author: "Kennedy", ISBN: "978-1-61729-176-9", CoverURL: "ol-cover.jpg"},
		{Source: "google_books", Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769", Description: "A great book"},
		{Source: "openlibrary", Title: "Go in Action", Author: "Kennedy", ISBN: "9780134190440"},
	}

	got := consolidateResults(results)

	require.Len(t, got, 2, "hardcover's two source hits merge by ISBN into one; paperback stays its own distinct-ISBN result")

	var hardcover, paperback BookMetadataResult
	for _, r := range got {
		if r.ISBN == "9781617291769" {
			hardcover = r
		} else {
			paperback = r
		}
	}

	assert.Equal(t, "google_books", hardcover.Source, "multi-source merge still picks fields by source priority")
	assert.Equal(t, "ol-cover.jpg", hardcover.CoverURL, "multi-source merge still fills gaps from the lower-priority source")
	assert.Equal(t, "A great book", hardcover.Description)
	assert.Empty(t, hardcover.EnrichedFields, "hardcover's description came from its own source, not a cross-edition backfill")

	assert.Equal(t, "9780134190440", paperback.ISBN)
	assert.Equal(t, "A great book", paperback.Description, "paperback backfilled from the hardcover's merged description")
	assert.Equal(t, []string{"description"}, paperback.EnrichedFields)
	assert.Empty(t, paperback.CoverURL, "CoverURL is never cross-filled, even though the hardcover has one")
}

func TestConsolidateResults_KnownLimitation_AuthorFormatDivergenceFromMergeBlocksBackfill(t *testing.T) {
	// enrichAcrossEditions buckets by the *merged* result's chosen Title/
	// Author (whichever source won by sourcePriority), not each source's raw
	// value. If a multi-source edition's winning source formats the author
	// differently than a sparse edition's only source does (e.g. "William
	// Kennedy" vs "Kennedy" — a real-world provider inconsistency, not a
	// contrived one), their bucket keys diverge and the backfill silently
	// doesn't fire, even though a human would recognize them as the same
	// work. This documents that as accepted current behavior — consistent
	// with the spec's explicit "exact-after-normalization only, no fuzzy
	// matching" design — not a bug to fix here.
	results := []BookMetadataResult{
		{Source: "openlibrary", Title: "Go in Action", Author: "Kennedy", ISBN: "9781617291769"},
		{Source: "google_books", Title: "Go in Action", Author: "William Kennedy", ISBN: "9781617291769", Description: "A great book"},
		{Source: "openlibrary", Title: "Go in Action", Author: "Kennedy", ISBN: "9780134190440"},
	}

	got := consolidateResults(results)

	require.Len(t, got, 2)
	for _, r := range got {
		if r.ISBN == "9780134190440" {
			assert.Empty(t, r.Description, "known limitation: author-format divergence from the merge step prevents this backfill")
		}
	}
}

func TestBestTitleAuthorForExpansion_PicksHighestPriorityCompleteResult(t *testing.T) {
	results := []BookMetadataResult{
		{Source: "bookbrainz", Title: "Church Discipline", Author: "Jonathan Leeman"},
		{Source: "openlibrary", Title: "", Author: ""},
		{Source: "google_books", Title: "Church Discipline", Author: "Jonathan Leeman", Description: "..."},
	}

	title, author := bestTitleAuthorForExpansion(results)

	assert.Equal(t, "Church Discipline", title)
	assert.Equal(t, "Jonathan Leeman", author, "google_books outranks bookbrainz by source priority")
}

func TestBestTitleAuthorForExpansion_SkipsResultsMissingEitherField(t *testing.T) {
	results := []BookMetadataResult{
		{Source: "google_books", Title: "Church Discipline", Author: ""},
		{Source: "openlibrary", Title: "", Author: "Jonathan Leeman"},
		{Source: "bookbrainz", Title: "Church Discipline", Author: "Jonathan Leeman"},
	}

	title, author := bestTitleAuthorForExpansion(results)

	assert.Equal(t, "Church Discipline", title)
	assert.Equal(t, "Jonathan Leeman", author)
}

func TestBestTitleAuthorForExpansion_ReturnsEmptyWhenNoResultHasBothFields(t *testing.T) {
	results := []BookMetadataResult{
		{Source: "google_books", Title: "Church Discipline", Author: ""},
		{Source: "openlibrary", Title: "", ISBN: "9781433532337"},
	}

	title, author := bestTitleAuthorForExpansion(results)

	assert.Empty(t, title)
	assert.Empty(t, author)
}

func TestBestTitleAuthorForExpansion_EmptyInput(t *testing.T) {
	title, author := bestTitleAuthorForExpansion(nil)

	assert.Empty(t, title)
	assert.Empty(t, author)
}

func TestPromoteQueriedEdition_MovesMatchToFront(t *testing.T) {
	sorted := []BookMetadataResult{
		{Title: "Sibling A", ISBN: "9781111111111"},
		{Title: "Sibling B", ISBN: "9782222222222"},
		{Title: "Queried Edition", ISBN: "9783333333333"},
	}

	got := promoteQueriedEdition(sorted, "9783333333333")

	require.Len(t, got, 3)
	assert.Equal(t, "Queried Edition", got[0].Title)
	assert.Equal(t, "Sibling A", got[1].Title, "relative order of the rest is preserved")
	assert.Equal(t, "Sibling B", got[2].Title)
}

func TestPromoteQueriedEdition_NoopWhenAlreadyFirst(t *testing.T) {
	sorted := []BookMetadataResult{
		{Title: "Queried Edition", ISBN: "9783333333333"},
		{Title: "Sibling A", ISBN: "9781111111111"},
	}

	got := promoteQueriedEdition(sorted, "9783333333333")

	assert.Equal(t, "Queried Edition", got[0].Title)
	assert.Equal(t, "Sibling A", got[1].Title)
}

func TestPromoteQueriedEdition_NoopWhenQueriedISBNEmpty(t *testing.T) {
	sorted := []BookMetadataResult{
		{Title: "Sibling A", ISBN: "9781111111111"},
		{Title: "Queried Edition", ISBN: "9783333333333"},
	}

	got := promoteQueriedEdition(sorted, "")

	assert.Equal(t, "Sibling A", got[0].Title, "no queried ISBN means nothing to promote")
}

func TestPromoteQueriedEdition_NoopWhenNoMatch(t *testing.T) {
	sorted := []BookMetadataResult{
		{Title: "Sibling A", ISBN: "9781111111111"},
		{Title: "Sibling B", ISBN: "9782222222222"},
	}

	got := promoteQueriedEdition(sorted, "9789999999999")

	assert.Equal(t, "Sibling A", got[0].Title)
	assert.Equal(t, "Sibling B", got[1].Title)
}

func TestPromoteQueriedEdition_MatchesAcrossISBNFormats(t *testing.T) {
	// The merged result's ISBN may be hyphenated/ISBN-10 depending on which
	// source won the field by priority; promotion must normalize both sides.
	sorted := []BookMetadataResult{
		{Title: "Sibling A", ISBN: "9781111111111"},
		{Title: "Queried Edition", ISBN: "978-1-433532-33-7"},
	}

	got := promoteQueriedEdition(sorted, normalizeISBN("9781433532337"))

	assert.Equal(t, "Queried Edition", got[0].Title)
}

func TestConsolidateResults_PromoteQueriedEdition_ExactISBNBeatsHigherScoringSibling(t *testing.T) {
	// Regression test for the real-world case that motivated
	// promoteQueriedEdition: searching ISBN 9781433532337 ("Church
	// Discipline" by Jonathan Leeman) returns the correct English edition
	// from Google Books (cover, description, publisher, but PageCount: 0 —
	// Google's own catalog entry is incomplete on that one field) plus,
	// via expandSiblingEditions, a Burmese translation that Google reports
	// with a non-zero PageCount, which genuinely outscores the correct
	// edition under scoreResult. Without promotion, the Burmese edition
	// would rank first purely because of a better-documented minor field.
	results := []BookMetadataResult{
		{
			Source: "google_books", Title: "Church Discipline", Author: "Jonathan Leeman",
			ISBN: "9781433532337", CoverURL: "cover.jpg", Description: "How the Church Protects the Name of Jesus",
			Publisher: "9marks", PageCount: 0,
		},
		{
			Source: "google_books", Title: "Church Discipline (Burmese)", Author: "Jonathan Leeman",
			ISBN: "9781955768849", CoverURL: "burmese-cover.jpg", Description: "...",
			Publisher: "Building Healthy Churches (Burmese)", PageCount: 194,
		},
	}

	consolidated := consolidateResults(results)
	require.Len(t, consolidated, 2)
	require.Equal(t, "Church Discipline (Burmese)", consolidated[0].Title, "sanity check: the Burmese edition does genuinely outscore the exact match on completeness alone")

	got := promoteQueriedEdition(consolidated, normalizeISBN("9781433532337"))

	assert.Equal(t, "Church Discipline", got[0].Title, "the exact-ISBN edition must win the top slot regardless of completeness score")
}
