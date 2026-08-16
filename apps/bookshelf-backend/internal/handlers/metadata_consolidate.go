package handlers

import (
	"regexp"
	"sort"
	"strings"
)

// nonAlphanumSpace matches any character that is not a lowercase letter, digit, or space.
var nonAlphanumSpace = regexp.MustCompile(`[^a-z0-9 ]+`)

// sourcePriority returns a numeric priority for a source (lower = higher priority).
func sourcePriority(source string) int {
	switch source {
	case "google_books":
		return 0
	case "openlibrary":
		return 1
	default:
		return 2
	}
}

// normalizeISBN strips hyphens/spaces and converts ISBN-10 to ISBN-13.
// Returns "" if the input doesn't produce a valid 10- or 13-digit ISBN.
func normalizeISBN(s string) string {
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ToUpper(s)

	switch len(s) {
	case 13:
		return normalizedISBN13(s)
	case 10:
		return isbn10ToISBN13(s)
	default:
		return ""
	}
}

// normalizedISBN13 returns s if it's all digits, else "".
func normalizedISBN13(s string) string {
	for _, c := range s {
		if c < '0' || c > '9' {
			return ""
		}
	}
	return s
}

// isbn10ToISBN13 validates a 10-character ISBN-10 (last char may be 'X' = 10)
// and converts it to ISBN-13: prepend "978", drop the old check digit,
// compute a new one. Returns "" if s isn't a valid ISBN-10.
func isbn10ToISBN13(s string) string {
	for i, c := range s {
		if i == 9 {
			if c != 'X' && (c < '0' || c > '9') {
				return ""
			}
		} else if c < '0' || c > '9' {
			return ""
		}
	}
	prefix := "978" + s[:9]
	check := isbn13CheckDigit(prefix)
	return prefix + string([]byte{'0' + check})
}

// isbn13CheckDigit computes the ISBN-13 check digit for a 12-digit string.
func isbn13CheckDigit(digits string) byte {
	sum := 0
	for i, c := range digits {
		d := int(c - '0')
		if i%2 == 0 {
			sum += d
		} else {
			sum += d * 3
		}
	}
	return byte((10 - (sum % 10)) % 10)
}

// normalizeTitleAuthor returns a deduplication key from title and author.
func normalizeTitleAuthor(title, author string) string {
	norm := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		s = nonAlphanumSpace.ReplaceAllString(s, "")
		s = strings.Join(strings.Fields(s), " ")
		return s
	}
	return norm(title) + "|" + norm(author)
}

// deduplicateIntoGroups groups results that refer to the same book.
// ISBN (normalized to ISBN-13) is the primary key; title+author is the fallback.
func deduplicateIntoGroups(results []BookMetadataResult) [][]BookMetadataResult {
	groups := [][]BookMetadataResult{}
	isbnIndex := map[string]int{}
	titleAuthorIndex := map[string]int{}

	for _, r := range results {
		normISBN := normalizeISBN(r.ISBN)
		normTA := normalizeTitleAuthor(r.Title, r.Author)

		idx, found := findExistingGroup(isbnIndex, titleAuthorIndex, normISBN, normTA)
		if found {
			groups[idx] = append(groups[idx], r)
			registerISBN(isbnIndex, normISBN, idx)
			continue
		}

		idx = len(groups)
		groups = append(groups, []BookMetadataResult{r})
		registerISBN(isbnIndex, normISBN, idx)
		if normTA != "|" {
			titleAuthorIndex[normTA] = idx
		}
	}

	return groups
}

// findExistingGroup looks up an existing group index by ISBN first, falling
// back to the title+author key.
func findExistingGroup(isbnIndex, titleAuthorIndex map[string]int, normISBN, normTA string) (int, bool) {
	if normISBN != "" {
		if i, ok := isbnIndex[normISBN]; ok {
			return i, true
		}
	}
	if normTA != "|" {
		if i, ok := titleAuthorIndex[normTA]; ok {
			return i, true
		}
	}
	return -1, false
}

// registerISBN records normISBN → idx if normISBN is non-empty and not already indexed.
func registerISBN(isbnIndex map[string]int, normISBN string, idx int) {
	if normISBN == "" {
		return
	}
	if _, ok := isbnIndex[normISBN]; !ok {
		isbnIndex[normISBN] = idx
	}
}

// mergeGroup merges a group of results (same book, multiple sources) into one.
// Google Books fields take priority, then Open Library, then BookBrainz.
func mergeGroup(group []BookMetadataResult) BookMetadataResult {
	// Sort by source priority so we pick fields from the best source first
	sorted := make([]BookMetadataResult, len(group))
	copy(sorted, group)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sourcePriority(sorted[i].Source) < sourcePriority(sorted[j].Source)
	})

	// For each field, take the first non-empty/non-zero value in source-priority order.
	return BookMetadataResult{
		Source:        sorted[0].Source,
		Title:         firstNonEmpty(sorted, func(r BookMetadataResult) string { return r.Title }),
		Author:        firstNonEmpty(sorted, func(r BookMetadataResult) string { return r.Author }),
		ISBN:          firstNonEmpty(sorted, func(r BookMetadataResult) string { return r.ISBN }),
		CoverURL:      firstNonEmpty(sorted, func(r BookMetadataResult) string { return r.CoverURL }),
		Description:   firstNonEmpty(sorted, func(r BookMetadataResult) string { return r.Description }),
		Publisher:     firstNonEmpty(sorted, func(r BookMetadataResult) string { return r.Publisher }),
		PublishedDate: firstNonEmpty(sorted, func(r BookMetadataResult) string { return r.PublishedDate }),
		PageCount:     firstNonZero(sorted, func(r BookMetadataResult) int { return r.PageCount }),
		Language:      firstNonEmpty(sorted, func(r BookMetadataResult) string { return r.Language }),
		// Accumulate all source IDs
		OLKey:         firstNonEmpty(sorted, func(r BookMetadataResult) string { return r.OLKey }),
		GoogleBooksID: firstNonEmpty(sorted, func(r BookMetadataResult) string { return r.GoogleBooksID }),
		BookBrainzID:  firstNonEmpty(sorted, func(r BookMetadataResult) string { return r.BookBrainzID }),
	}
}

// firstNonEmpty returns get(r) for the first r where it's non-empty, else "".
func firstNonEmpty(sorted []BookMetadataResult, get func(BookMetadataResult) string) string {
	for _, r := range sorted {
		if v := get(r); v != "" {
			return v
		}
	}
	return ""
}

// firstNonZero returns get(r) for the first r where it's non-zero, else 0.
func firstNonZero(sorted []BookMetadataResult, get func(BookMetadataResult) int) int {
	for _, r := range sorted {
		if v := get(r); v != 0 {
			return v
		}
	}
	return 0
}

// scoreResult returns a completeness score for ranking.
func scoreResult(r BookMetadataResult) int {
	score := 0
	if r.CoverURL != "" {
		score += 2
	}
	if r.Description != "" {
		score += 2
	}
	if r.ISBN != "" {
		score++
	}
	if r.Publisher != "" {
		score++
	}
	if r.PageCount > 0 {
		score++
	}
	// Bonus for multi-source confidence
	sources := 0
	if r.OLKey != "" {
		sources++
	}
	if r.GoogleBooksID != "" {
		sources++
	}
	if r.BookBrainzID != "" {
		sources++
	}
	if sources > 1 {
		score += sources - 1
	}
	return score
}

// consolidateResults deduplicates, merges, and ranks results from all sources.
func consolidateResults(results []BookMetadataResult) []BookMetadataResult {
	if len(results) == 0 {
		return []BookMetadataResult{}
	}

	groups := deduplicateIntoGroups(results)

	merged := make([]BookMetadataResult, 0, len(groups))
	for _, group := range groups {
		merged = append(merged, mergeGroup(group))
	}

	sort.SliceStable(merged, func(i, j int) bool {
		si, sj := scoreResult(merged[i]), scoreResult(merged[j])
		if si != sj {
			return si > sj
		}
		return merged[i].Title < merged[j].Title
	})

	return merged
}
