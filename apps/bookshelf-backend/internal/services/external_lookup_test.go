package services

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
)

// stubResponse is a canned response for one URL substring match.
type stubResponse struct {
	body        string
	contentType string // defaults to "application/json" if empty
}

// stubRoundTripper routes requests by substring match against the request
// URL, avoiding any real network call — Open Library/Google Books URLs are
// hardcoded in external_lookup.go, so interception happens at the transport
// level rather than by pointing the client at a local server.
type stubRoundTripper struct {
	responses map[string]stubResponse
	calls     []string
}

func (rt *stubRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.calls = append(rt.calls, req.URL.String())
	for substr, resp := range rt.responses {
		if strings.Contains(req.URL.String(), substr) {
			header := make(http.Header)
			ct := resp.contentType
			if ct == "" {
				ct = "application/json"
			}
			header.Set("Content-Type", ct)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(resp.body)),
				Header:     header,
			}, nil
		}
	}
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(bytes.NewBufferString("")),
		Header:     make(http.Header),
	}, nil
}

func newStubClient(responses map[string]string) (*http.Client, *stubRoundTripper) {
	converted := make(map[string]stubResponse, len(responses))
	for k, v := range responses {
		converted[k] = stubResponse{body: v}
	}
	rt := &stubRoundTripper{responses: converted}
	return &http.Client{Transport: rt}, rt
}

func TestResolveExternalData_PrefersOLKeyOverGoogleBooksIDOverISBN(t *testing.T) {
	// OLKey's Open Library lookup can only ever contribute a cover (never a
	// description — see lookupOpenLibraryCover), so it must not short-circuit
	// the GoogleBooksID lookup that follows it: the merged result should have
	// both fields, sourced from whichever source could actually supply each,
	// and the ISBN branch should never be reached since both are now filled.
	client, rt := newStubClient(map[string]string{
		"bibkeys=OLID:OL1M": `{"OLID:OL1M":{"cover":{"large":"https://covers.openlibrary.org/b/id/1-L.jpg"}}}`,
		"volumes/GB1":       `{"volumeInfo":{"description":"from google"}}`,
		"bibkeys=ISBN:123":  `{"ISBN:123":{"cover":{"large":"https://covers.openlibrary.org/b/id/2-L.jpg"}}}`,
	})

	book := models.Book{OLKey: "OL1M", GoogleBooksID: "GB1", ISBN: "123"}
	data, _ := resolveExternalData(t.Context(), client, book, "test-key", "", wantedFields{cover: true, description: true})

	if data.coverURL != "https://covers.openlibrary.org/b/id/1-L.jpg" {
		t.Fatalf("expected OLKey result to win for cover, got %+v", data)
	}
	if data.description != "from google" {
		t.Fatalf("expected GoogleBooksID result to fill description, got %+v", data)
	}
	if len(rt.calls) != 2 {
		t.Fatalf("expected OLKey then GoogleBooksID lookups (stopping before ISBN, since both fields were filled), got %v", rt.calls)
	}
}

func TestResolveExternalData_FallsThroughWhenOLKeyEmpty(t *testing.T) {
	client, _ := newStubClient(map[string]string{
		"volumes/GB1": `{"volumeInfo":{"description":"from google","imageLinks":{"thumbnail":"https://books.google.com/gb1.jpg"}}}`,
	})

	book := models.Book{GoogleBooksID: "GB1"}
	data, _ := resolveExternalData(t.Context(), client, book, "test-key", "", wantedFields{cover: true, description: true})

	if data.coverURL != "https://books.google.com/gb1.jpg" || data.description != "from google" {
		t.Fatalf("expected Google Books result, got %+v", data)
	}
}

func TestResolveExternalData_FallsThroughOnEmptySource(t *testing.T) {
	client, _ := newStubClient(map[string]string{
		"bibkeys=OLID:OL1M": `{"OLID:OL1M":{}}`, // no cover
		"bibkeys=ISBN:123":  `{"ISBN:123":{"cover":{"large":"https://covers.openlibrary.org/b/id/3-L.jpg"}}}`,
	})

	book := models.Book{OLKey: "OL1M", ISBN: "123"}
	data, _ := resolveExternalData(t.Context(), client, book, "", "", wantedFields{cover: true, description: true})

	if data.coverURL != "https://covers.openlibrary.org/b/id/3-L.jpg" {
		t.Fatalf("expected fallthrough to ISBN result, got %+v", data)
	}
}

func TestLookupGoogleBooksData_SkipsWhenNoAPIKey(t *testing.T) {
	client, rt := newStubClient(map[string]string{
		"volumes/GB1": `{"volumeInfo":{"description":"should not be seen"}}`,
	})

	data, _, err := lookupGoogleBooksData(t.Context(), client, "GB1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !data.empty() {
		t.Fatalf("expected empty result when no API key configured, got %+v", data)
	}
	if len(rt.calls) != 0 {
		t.Fatalf("expected no HTTP call when no API key configured, got %v", rt.calls)
	}
}

func TestResolveExternalData_ISBNFallsThroughToGoogleBooksSearch(t *testing.T) {
	// Mirrors the real "Nine Marks of a Healthy Church" case: a book with
	// only an ISBN on record (no OLKey/GoogleBooksID captured at add-time),
	// where Open Library has nothing but Google Books' by-ISBN search does.
	client, rt := newStubClient(map[string]string{
		"bibkeys=ISBN:9781433578113": `{"ISBN:9781433578113":{}}`, // Open Library: no cover
		"volumes?q=isbn:9781433578113": `{"items":[{"volumeInfo":{
			"description":"A short, accessible guide to identifying a healthy church.",
			"imageLinks":{"thumbnail":"https://books.google.com/books/content?id=4qErzgEACAAJ"}
		}}]}`,
	})

	book := models.Book{ISBN: "9781433578113"}
	data, _ := resolveExternalData(t.Context(), client, book, "test-key", "", wantedFields{cover: true, description: true})

	if data.coverURL != "https://books.google.com/books/content?id=4qErzgEACAAJ" {
		t.Fatalf("expected Google Books ISBN-search cover, got %+v", data)
	}
	if data.description != "A short, accessible guide to identifying a healthy church." {
		t.Fatalf("expected Google Books ISBN-search description, got %+v", data)
	}
	if len(rt.calls) != 2 {
		t.Fatalf("expected Open Library then Google Books, got %v", rt.calls)
	}
}

func TestResolveExternalData_ISBNGoogleBooksSearchSkippedWithoutAPIKey(t *testing.T) {
	client, rt := newStubClient(map[string]string{
		"bibkeys=ISBN:111":   `{"ISBN:111":{}}`,
		"volumes?q=isbn:111": `{"items":[{"volumeInfo":{"description":"should not be seen"}}]}`,
	})

	book := models.Book{ISBN: "111"}
	data, _ := resolveExternalData(t.Context(), client, book, "", "", wantedFields{cover: true, description: true})

	if !data.empty() {
		t.Fatalf("expected empty result without an API key, got %+v", data)
	}
	if len(rt.calls) != 1 {
		t.Fatalf("expected only the Open Library call, Google Books search should be skipped, got %v", rt.calls)
	}
}

func TestResolveExternalData_ISBNFallsThroughToHardcover(t *testing.T) {
	// Neither Open Library nor Google Books has anything for this ISBN, but
	// Hardcover does — Hardcover must be tried as the last-resort ISBN
	// source, and only when a key is configured.
	client, rt := newStubClient(map[string]string{
		"bibkeys=ISBN:222":   `{"ISBN:222":{}}`,
		"volumes?q=isbn:222": `{"items":[]}`,
		"api.hardcover.app":  `{"data":{"books":[{"description":"from hardcover","image":{"url":"https://assets.hardcover.app/cover.jpg"}}]}}`,
	})

	book := models.Book{ISBN: "222"}
	data, attempts := resolveExternalData(t.Context(), client, book, "test-key", "hardcover-key", wantedFields{cover: true, description: true})

	if data.coverURL != "https://assets.hardcover.app/cover.jpg" || data.description != "from hardcover" {
		t.Fatalf("expected Hardcover result, got %+v (attempts: %v)", data, attempts)
	}
	if len(rt.calls) != 3 {
		t.Fatalf("expected Open Library, Google Books, then Hardcover, got %v", rt.calls)
	}
}

func TestResolveExternalData_HardcoverSkippedWithoutAPIKey(t *testing.T) {
	client, rt := newStubClient(map[string]string{
		"bibkeys=ISBN:333":   `{"ISBN:333":{}}`,
		"volumes?q=isbn:333": `{"items":[]}`,
		"api.hardcover.app":  `{"data":{"books":[{"description":"should not be seen"}]}}`,
	})

	book := models.Book{ISBN: "333"}
	data, _ := resolveExternalData(t.Context(), client, book, "test-key", "", wantedFields{cover: true, description: true})

	if !data.empty() {
		t.Fatalf("expected empty result, got %+v", data)
	}
	if len(rt.calls) != 2 {
		t.Fatalf("expected only Open Library and Google Books, Hardcover should be skipped without a key, got %v", rt.calls)
	}
}

func TestResolveExternalData_NoKeysReturnsEmpty(t *testing.T) {
	client, rt := newStubClient(map[string]string{})

	data, _ := resolveExternalData(t.Context(), client, models.Book{}, "test-key", "", wantedFields{cover: true, description: true})

	if !data.empty() {
		t.Fatalf("expected empty result for a book with no external keys, got %+v", data)
	}
	if len(rt.calls) != 0 {
		t.Fatalf("expected no HTTP calls for a book with no external keys, got %v", rt.calls)
	}
}

func TestResolveExternalData_OpenLibraryDecodesAuthors(t *testing.T) {
	client, _ := newStubClient(map[string]string{
		"bibkeys=ISBN:444": `{"ISBN:444":{"authors":[{"name":"Author One"},{"name":"Author Two"}]}}`,
	})

	book := models.Book{ISBN: "444"}
	data, _ := resolveExternalData(t.Context(), client, book, "", "", wantedFields{author: true})

	if data.author != "Author One; Author Two" {
		t.Fatalf("expected joined Open Library authors, got %q", data.author)
	}
}

func TestResolveExternalData_GoogleBooksDecodesAuthors(t *testing.T) {
	client, _ := newStubClient(map[string]string{
		"bibkeys=ISBN:555":   `{"ISBN:555":{}}`,
		"volumes?q=isbn:555": `{"items":[{"volumeInfo":{"authors":["Author One","Author Two"]}}]}`,
	})

	book := models.Book{ISBN: "555"}
	data, _ := resolveExternalData(t.Context(), client, book, "test-key", "", wantedFields{author: true})

	if data.author != "Author One; Author Two" {
		t.Fatalf("expected joined Google Books authors, got %q", data.author)
	}
}

func TestResolveExternalData_HardcoverDecodesAuthors(t *testing.T) {
	client, _ := newStubClient(map[string]string{
		"bibkeys=ISBN:666":   `{"ISBN:666":{}}`,
		"volumes?q=isbn:666": `{"items":[]}`,
		"api.hardcover.app": `{"data":{"books":[{"cached_contributors":[
			{"author":{"name":"Author One"},"contribution":""},
			{"author":{"name":"Illustrator Person"},"contribution":"Illustrator"},
			{"author":{"name":"Author Two"},"contribution":"Author"}
		]}]}}`,
	})

	book := models.Book{ISBN: "666"}
	data, _ := resolveExternalData(t.Context(), client, book, "test-key", "hardcover-key", wantedFields{author: true})

	if data.author != "Author One; Author Two" {
		t.Fatalf("expected joined Hardcover authors excluding the illustrator, got %q", data.author)
	}
}

func TestResolveExternalData_AuthorOnlyWantStopsAsSoonAsAuthorFound(t *testing.T) {
	// OLKey's Open Library lookup can supply an author but never a
	// description/cover in this fixture; when only author is wanted, the
	// loop must stop right there instead of also querying GoogleBooksID.
	client, rt := newStubClient(map[string]string{
		"bibkeys=OLID:OL1M": `{"OLID:OL1M":{"authors":[{"name":"Author One"}]}}`,
		"volumes/GB1":       `{"volumeInfo":{"authors":["Should Not Be Seen"]}}`,
	})

	book := models.Book{OLKey: "OL1M", GoogleBooksID: "GB1"}
	data, _ := resolveExternalData(t.Context(), client, book, "test-key", "", wantedFields{author: true})

	if data.author != "Author One" {
		t.Fatalf("expected Open Library author, got %q", data.author)
	}
	if len(rt.calls) != 1 {
		t.Fatalf("expected to stop after Open Library once author was found, got %v", rt.calls)
	}
}
