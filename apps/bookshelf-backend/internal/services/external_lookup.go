package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
)

// externalBookData is what a direct by-key lookup against an external
// catalog can contribute towards backfilling a stored Book row.
type externalBookData struct {
	coverURL    string
	description string
}

// empty reports whether the lookup found nothing usable.
func (d externalBookData) empty() bool {
	return d.coverURL == "" && d.description == ""
}

// lookupOpenLibraryCover looks up a cover by ISBN or OpenLibrary edition key
// via the Books API (jscmd=data), which — unlike the raw
// covers.openlibrary.org/b/<key>/<value>-L.jpg image endpoint — only includes
// a "cover" object when a real cover actually exists, rather than returning
// HTTP 200 with a tiny placeholder image for "no cover". This endpoint does
// not carry a description field (that lives on the separate Work record),
// so Open Library only ever contributes a cover here.
func lookupOpenLibraryCover(ctx context.Context, client *http.Client, bibkey string) (externalBookData, error) {
	// bibkey is always "ISBN:<digits>" or "OLID:<alphanumeric>" (see callers
	// in resolveExternalData) — no characters requiring escaping, and Open
	// Library's own docs show the colon unescaped in this param.
	reqURL := "https://openlibrary.org/api/books?bibkeys=" + bibkey + "&format=json&jscmd=data"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return externalBookData{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return externalBookData{}, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return externalBookData{}, fmt.Errorf("open library books api returned %d", resp.StatusCode)
	}

	var parsed map[string]struct {
		Cover struct {
			Large  string `json:"large"`
			Medium string `json:"medium"`
			Small  string `json:"small"`
		} `json:"cover"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return externalBookData{}, err
	}

	entry, ok := parsed[bibkey]
	if !ok {
		return externalBookData{}, nil
	}

	switch {
	case entry.Cover.Large != "":
		return externalBookData{coverURL: entry.Cover.Large}, nil
	case entry.Cover.Medium != "":
		return externalBookData{coverURL: entry.Cover.Medium}, nil
	case entry.Cover.Small != "":
		return externalBookData{coverURL: entry.Cover.Small}, nil
	default:
		return externalBookData{}, nil
	}
}

// googleBooksVolumeInfo is the subset of a Google Books "volume" resource
// (whether returned directly by ID or as a search result item) both
// lookupGoogleBooksData and lookupGoogleBooksByISBN need.
type googleBooksVolumeInfo struct {
	Description string `json:"description"`
	ImageLinks  struct {
		Thumbnail      string `json:"thumbnail"`
		SmallThumbnail string `json:"smallThumbnail"`
	} `json:"imageLinks"`
}

func (v googleBooksVolumeInfo) toExternalBookData() externalBookData {
	cover := v.ImageLinks.Thumbnail
	if cover == "" {
		cover = v.ImageLinks.SmallThumbnail
	}
	return externalBookData{coverURL: cover, description: v.Description}
}

// doGoogleBooksRequest issues a GET against the Google Books API and decodes
// the JSON response into dest. Returns (false, nil) without decoding if
// apiKey is empty — mirrors fetchGoogleBooks' existing behavior in
// metadata.go — or if the API returns a non-200 (e.g. 404 for no match).
func doGoogleBooksRequest(ctx context.Context, client *http.Client, reqURL, apiKey string, dest any) (bool, error) {
	if apiKey == "" {
		return false, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL+"&key="+url.QueryEscape(apiKey), nil)
	if err != nil {
		return false, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return false, err
	}
	return true, nil
}

// lookupGoogleBooksData looks up a cover and description by Google Books
// volume ID — the precise, no-ambiguity path used when the book already has
// a GoogleBooksID on record.
func lookupGoogleBooksData(ctx context.Context, client *http.Client, volumeID, apiKey string) (externalBookData, error) {
	reqURL := "https://www.googleapis.com/books/v1/volumes/" + url.PathEscape(volumeID) + "?alt=json"
	var parsed struct {
		VolumeInfo googleBooksVolumeInfo `json:"volumeInfo"`
	}
	ok, err := doGoogleBooksRequest(ctx, client, reqURL, apiKey, &parsed)
	if err != nil || !ok {
		return externalBookData{}, err
	}
	return parsed.VolumeInfo.toExternalBookData(), nil
}

// lookupGoogleBooksByISBN searches Google Books by ISBN — a fallback for
// books that only ever had an ISBN captured (e.g. added via a metadata
// source, like BookBrainz, that never set GoogleBooksID), so they aren't
// stuck depending solely on Open Library, which doesn't always have a cover
// or description either. Takes the first search result, same as how
// createBook's original metadata search would have surfaced one.
func lookupGoogleBooksByISBN(ctx context.Context, client *http.Client, isbn, apiKey string) (externalBookData, error) {
	reqURL := "https://www.googleapis.com/books/v1/volumes?q=isbn:" + url.QueryEscape(isbn)
	var parsed struct {
		Items []struct {
			VolumeInfo googleBooksVolumeInfo `json:"volumeInfo"`
		} `json:"items"`
	}
	ok, err := doGoogleBooksRequest(ctx, client, reqURL, apiKey, &parsed)
	if err != nil || !ok || len(parsed.Items) == 0 {
		return externalBookData{}, err
	}
	return parsed.Items[0].VolumeInfo.toExternalBookData(), nil
}

// resolveExternalData tries book's external keys in the same trust order
// findExistingBook uses (OLKey, then GoogleBooksID, then ISBN), returning the
// first source that yields any usable data. A source erroring or coming back
// empty falls through to the next rather than aborting the lookup. The ISBN
// branch tries Open Library first (free, no API key needed) before Google
// Books by ISBN search, to minimize paid/key-gated calls.
func resolveExternalData(ctx context.Context, client *http.Client, book models.Book, googleBooksAPIKey string) externalBookData {
	if book.OLKey != "" {
		if data, err := lookupOpenLibraryCover(ctx, client, "OLID:"+book.OLKey); err == nil && !data.empty() {
			return data
		}
	}
	if book.GoogleBooksID != "" {
		if data, err := lookupGoogleBooksData(ctx, client, book.GoogleBooksID, googleBooksAPIKey); err == nil && !data.empty() {
			return data
		}
	}
	if book.ISBN != "" {
		if data, err := lookupOpenLibraryCover(ctx, client, "ISBN:"+book.ISBN); err == nil && !data.empty() {
			return data
		}
		if data, err := lookupGoogleBooksByISBN(ctx, client, book.ISBN, googleBooksAPIKey); err == nil && !data.empty() {
			return data
		}
	}
	return externalBookData{}
}
