package handlers

import (
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
