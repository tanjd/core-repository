package handlers

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoogleBooksQueryFor(t *testing.T) {
	tests := []struct {
		name string
		q    string
		want string
	}{
		{
			name: "ISBN-13 gets the isbn: operator",
			q:    "9781433532337",
			want: "isbn:9781433532337",
		},
		{
			name: "hyphenated ISBN-13 normalizes then gets the isbn: operator",
			q:    "978-1-433-53233-7",
			want: "isbn:9781433532337",
		},
		{
			name: "ISBN-10 is normalized to ISBN-13 before the isbn: operator",
			q:    "1433532336",
			want: "isbn:9781433532337",
		},
		{
			name: "title/author query passes through unchanged",
			q:    "Church Discipline Jonathan Leeman",
			want: "Church Discipline Jonathan Leeman",
		},
		{
			name: "empty query passes through unchanged",
			q:    "",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, googleBooksQueryFor(tt.q))
		})
	}
}

func TestGoogleBooksStatusError(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   string
	}{
		{
			name:   "429 is classified as rate limit",
			status: http.StatusTooManyRequests,
			want:   "google books rate limit exceeded (status 429)",
		},
		{
			name:   "401 is classified as a rejected key",
			status: http.StatusUnauthorized,
			want:   "google books rejected the API key (status 401)",
		},
		{
			name:   "403 is classified as a rejected key",
			status: http.StatusForbidden,
			want:   "google books rejected the API key (status 403)",
		},
		{
			name:   "503 is classified as service unavailable",
			status: http.StatusServiceUnavailable,
			want:   "google books service unavailable, try again shortly (status 503)",
		},
		{
			name:   "unrecognized status falls back to a generic message",
			status: http.StatusTeapot,
			want:   "google books returned unexpected status 418",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.EqualError(t, googleBooksStatusError(tt.status), tt.want)
		})
	}
}

func TestRunFetchSources(t *testing.T) {
	ok := func(items ...BookMetadataResult) func() ([]BookMetadataResult, error) {
		return func() ([]BookMetadataResult, error) { return items, nil }
	}
	fail := func() ([]BookMetadataResult, error) { return nil, errors.New("boom") }

	t.Run("all sources succeed with no results", func(t *testing.T) {
		results, hadError := runFetchSources(context.Background(), "q", []fetchSource{
			{"a", ok()},
			{"b", ok()},
		})
		assert.Empty(t, results)
		assert.False(t, hadError)
	})

	t.Run("a source errors and results end up empty", func(t *testing.T) {
		results, hadError := runFetchSources(context.Background(), "q", []fetchSource{
			{"a", fail},
			{"b", ok()},
		})
		assert.Empty(t, results)
		assert.True(t, hadError)
	})

	t.Run("a source errors but another returns results", func(t *testing.T) {
		want := BookMetadataResult{Title: "found"}
		results, hadError := runFetchSources(context.Background(), "q", []fetchSource{
			{"a", fail},
			{"b", ok(want)},
		})
		assert.Equal(t, []BookMetadataResult{want}, results)
		assert.True(t, hadError)
	})
}
