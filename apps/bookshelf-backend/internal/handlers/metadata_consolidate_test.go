package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestConsolidateResults_EmptyInput(t *testing.T) {
	got := consolidateResults(nil)
	assert.Empty(t, got)
}
