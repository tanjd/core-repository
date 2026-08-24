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

// lookupAttempt records what one source contributed (or why it didn't) for
// a single book, so a caller whose overall lookup came up empty can report
// *why* rather than just "no cover found" — e.g. distinguishing "Open
// Library genuinely has nothing for this edition" from "Google Books quota
// exceeded, never actually checked."
type lookupAttempt struct {
	source string
	status string
}

func (a lookupAttempt) String() string {
	return a.source + ": " + a.status
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
// the JSON response into dest. Returns (false, "no api key configured", nil)
// without decoding if apiKey is empty — mirrors fetchGoogleBooks' existing
// behavior in metadata.go. A non-200 (e.g. 404 for no match, 429 for quota
// exceeded) also returns ok=false, but with the status code in the returned
// string so a caller reporting "nothing found" can distinguish a genuine
// empty result from a request that never actually got answered.
func doGoogleBooksRequest(ctx context.Context, client *http.Client, reqURL, apiKey string, dest any) (bool, string, error) {
	if apiKey == "" {
		return false, "no api key configured", nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL+"&key="+url.QueryEscape(apiKey), nil)
	if err != nil {
		return false, "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("http %d", resp.StatusCode), nil
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return false, "", err
	}
	return true, "", nil
}

// lookupGoogleBooksData looks up a cover and description by Google Books
// volume ID — the precise, no-ambiguity path used when the book already has
// a GoogleBooksID on record.
func lookupGoogleBooksData(ctx context.Context, client *http.Client, volumeID, apiKey string) (externalBookData, string, error) {
	reqURL := "https://www.googleapis.com/books/v1/volumes/" + url.PathEscape(volumeID) + "?alt=json"
	var parsed struct {
		VolumeInfo googleBooksVolumeInfo `json:"volumeInfo"`
	}
	ok, status, err := doGoogleBooksRequest(ctx, client, reqURL, apiKey, &parsed)
	if err != nil || !ok {
		return externalBookData{}, status, err
	}
	return parsed.VolumeInfo.toExternalBookData(), "", nil
}

// lookupGoogleBooksByISBN searches Google Books by ISBN — a fallback for
// books that only ever had an ISBN captured (e.g. added via a metadata
// source, like BookBrainz, that never set GoogleBooksID), so they aren't
// stuck depending solely on Open Library, which doesn't always have a cover
// or description either. Takes the first search result, same as how
// createBook's original metadata search would have surfaced one.
func lookupGoogleBooksByISBN(ctx context.Context, client *http.Client, isbn, apiKey string) (externalBookData, string, error) {
	reqURL := "https://www.googleapis.com/books/v1/volumes?q=isbn:" + url.QueryEscape(isbn)
	var parsed struct {
		Items []struct {
			VolumeInfo googleBooksVolumeInfo `json:"volumeInfo"`
		} `json:"items"`
	}
	ok, status, err := doGoogleBooksRequest(ctx, client, reqURL, apiKey, &parsed)
	if err != nil || !ok {
		return externalBookData{}, status, err
	}
	if len(parsed.Items) == 0 {
		return externalBookData{}, "no results", nil
	}
	return parsed.Items[0].VolumeInfo.toExternalBookData(), "", nil
}

// externalLookupStep is one named source resolveExternalData can try, bound
// to the specific book/client/key it's being resolved for.
type externalLookupStep struct {
	source string
	run    func() (externalBookData, string, error)
}

// externalLookupSteps builds the ordered list of sources to try for book —
// the same trust order findExistingBook uses (OLKey, then GoogleBooksID,
// then ISBN). The ISBN branch tries Open Library first (free, no API key
// needed) before Google Books by ISBN search, to minimize paid/key-gated
// calls. A step whose key is empty on book is omitted rather than run.
func externalLookupSteps(ctx context.Context, client *http.Client, book models.Book, googleBooksAPIKey string) []externalLookupStep {
	var steps []externalLookupStep
	if book.OLKey != "" {
		steps = append(steps, externalLookupStep{"openlibrary(key)", func() (externalBookData, string, error) {
			data, err := lookupOpenLibraryCover(ctx, client, "OLID:"+book.OLKey)
			return data, "", err
		}})
	}
	if book.GoogleBooksID != "" {
		steps = append(steps, externalLookupStep{"google_books(id)", func() (externalBookData, string, error) {
			return lookupGoogleBooksData(ctx, client, book.GoogleBooksID, googleBooksAPIKey)
		}})
	}
	if book.ISBN != "" {
		steps = append(steps, externalLookupStep{"openlibrary(isbn)", func() (externalBookData, string, error) {
			data, err := lookupOpenLibraryCover(ctx, client, "ISBN:"+book.ISBN)
			return data, "", err
		}})
		steps = append(steps, externalLookupStep{"google_books(isbn)", func() (externalBookData, string, error) {
			return lookupGoogleBooksByISBN(ctx, client, book.ISBN, googleBooksAPIKey)
		}})
	}
	return steps
}

// mergeExternalLookup folds one step's result into merged (first non-empty
// value per field wins) and appends what happened to attempts — used to
// report *why* a field is still missing when the overall result is
// incomplete (see CoverBackfillService.backfillOne).
func mergeExternalLookup(merged *externalBookData, attempts *[]lookupAttempt, source string, data externalBookData, status string, err error) {
	switch {
	case err != nil:
		*attempts = append(*attempts, lookupAttempt{source, "error: " + err.Error()})
	case !data.empty():
		*attempts = append(*attempts, lookupAttempt{source, "found"})
		if merged.coverURL == "" {
			merged.coverURL = data.coverURL
		}
		if merged.description == "" {
			merged.description = data.description
		}
	case status != "":
		*attempts = append(*attempts, lookupAttempt{source, status})
	default:
		*attempts = append(*attempts, lookupAttempt{source, "no cover/description"})
	}
}

// resolveExternalData tries each of book's applicable external sources in
// turn (see externalLookupSteps), merging in whichever of coverURL/
// description each source can supply. A source erroring or coming back
// empty falls through to the next rather than aborting the lookup, and
// sources are tried until both fields are filled or every source is
// exhausted — an earlier source contributing only a cover (e.g. Open
// Library, which never carries a description — see lookupOpenLibraryCover)
// must not short-circuit a later source that could still supply the
// description; a prior "stop at the first source with any usable data"
// version of this function meant a book with an Open Library cover would
// never even be checked against Google Books for a description.
func resolveExternalData(ctx context.Context, client *http.Client, book models.Book, googleBooksAPIKey string) (externalBookData, []lookupAttempt) {
	var attempts []lookupAttempt
	var merged externalBookData

	for _, step := range externalLookupSteps(ctx, client, book, googleBooksAPIKey) {
		data, status, err := step.run()
		mergeExternalLookup(&merged, &attempts, step.source, data, status, err)
		if merged.coverURL != "" && merged.description != "" {
			break
		}
	}
	return merged, attempts
}
