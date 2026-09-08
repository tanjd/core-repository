// Package textutil holds small text-normalization helpers shared across
// handlers and repositories.
package textutil

import (
	"regexp"
	"strings"
)

// nonAlphanumSpace matches any character that is not a lowercase letter, digit, or space.
var nonAlphanumSpace = regexp.MustCompile(`[^a-z0-9 ]+`)

// nonAlphanum matches any character that is not a lowercase letter or digit.
var nonAlphanum = regexp.MustCompile(`[^a-z0-9]+`)

// NormalizeText lowercases s, strips punctuation, and collapses whitespace —
// used to compare user-entered text (e.g. a search query) against stored
// title/author text regardless of punctuation or spacing differences.
func NormalizeText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlphanumSpace.ReplaceAllString(s, "")
	return strings.Join(strings.Fields(s), " ")
}

// NormalizeSearchKey lowercases s and strips every non-alphanumeric
// character, including whitespace — unlike NormalizeText, it collapses
// "C. S. Lewis" and "CS Lewis" to the same "cslewis" key, so a search term
// can match stored text regardless of how initials/words are spaced or
// punctuated.
func NormalizeSearchKey(s string) string {
	return nonAlphanum.ReplaceAllString(strings.ToLower(s), "")
}
