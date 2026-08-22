// Package bookmatch holds normalized-title+author matching, shared by
// internal/handlers (search-result dedup/enrichment, catalog import fuzzy
// match) and internal/services (catalog description reconciliation).
// internal/handlers already depends on internal/services, so this can't live
// in either of those packages without creating an import cycle.
package bookmatch

import (
	"regexp"
	"strings"
)

// nonAlphanumSpace matches any character that is not a lowercase letter, digit, or space.
var nonAlphanumSpace = regexp.MustCompile(`[^a-z0-9 ]+`)

// NormalizeTitleAuthor returns a matching key from title and author: lowercased,
// punctuation stripped, whitespace collapsed. Used both as a fuzzy-match key
// (title+author equality, no similarity scoring) and to bucket distinct
// editions of the same work for cross-edition metadata enrichment.
func NormalizeTitleAuthor(title, author string) string {
	norm := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		s = nonAlphanumSpace.ReplaceAllString(s, "")
		s = strings.Join(strings.Fields(s), " ")
		return s
	}
	return norm(title) + "|" + norm(author)
}
