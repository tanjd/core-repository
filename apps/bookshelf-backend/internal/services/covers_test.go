package services

import "testing"

func TestIsCoverURLAllowed(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"open library", "https://covers.openlibrary.org/b/id/1-L.jpg", true},
		{"google books", "https://books.google.com/books/content?id=1", true},
		{"google books usercontent", "https://books.googleusercontent.com/books/content?id=1", true},
		{"readmill", "https://cover.books.readmill.com/1.jpg", true},
		{"hardcover", "https://assets.hardcover.app/editions/1/cover.jpg", true},
		{"hardcover subdomain", "https://cdn.assets.hardcover.app/1.jpg", true},
		{"untrusted host", "https://evil.example.com/cover.jpg", false},
		{"untrusted host resembling an allowed one", "https://notassets.hardcover.app/1.jpg", false},
		{"non-http scheme", "file:///etc/passwd", false},
		{"invalid url", "://bad", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCoverURLAllowed(tt.url); got != tt.want {
				t.Errorf("IsCoverURLAllowed(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}
