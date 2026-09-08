package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
)

// externalBookData is what a direct by-key lookup against an external
// catalog can contribute towards backfilling a stored Book row.
type externalBookData struct {
	coverURL    string
	description string
	author      string
}

// empty reports whether the lookup found nothing usable.
func (d externalBookData) empty() bool {
	return d.coverURL == "" && d.description == "" && d.author == ""
}

// wantedFields tells resolveExternalData which fields a caller actually
// needs, so it can stop trying further sources as soon as those specific
// fields are filled rather than always waiting on every field this package
// knows how to look up (e.g. AuthorBackfillService only wants author, and
// shouldn't keep querying sources after an early one already supplied it,
// just because cover/description are still empty).
type wantedFields struct {
	cover       bool
	description bool
	author      bool
}

// satisfies reports whether d already has everything want asks for.
func (want wantedFields) satisfies(d externalBookData) bool {
	if want.cover && d.coverURL == "" {
		return false
	}
	if want.description && d.description == "" {
		return false
	}
	if want.author && d.author == "" {
		return false
	}
	return true
}

// joinAuthors collapses multiple credited authors into Book.Author's single
// string column — this backfill/reconciliation path never introduces a
// separate Author entity (see apps/bookshelf-backend/CLAUDE.md's product
// scope), so every source's author list is joined the same way rather than
// picking just the first and silently dropping co-authors.
func joinAuthors(names []string) string {
	var nonEmpty []string
	for _, n := range names {
		if n != "" {
			nonEmpty = append(nonEmpty, n)
		}
	}
	return strings.Join(nonEmpty, "; ")
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

// lookupOpenLibraryCover looks up a cover and author(s) by ISBN or
// OpenLibrary edition key via the Books API (jscmd=data), which — unlike the
// raw covers.openlibrary.org/b/<key>/<value>-L.jpg image endpoint — only
// includes a "cover" object when a real cover actually exists, rather than
// returning HTTP 200 with a tiny placeholder image for "no cover". This
// endpoint does not carry a description field (that lives on the separate
// Work record), so Open Library never contributes a description here.
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
		Authors []struct {
			Name string `json:"name"`
		} `json:"authors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return externalBookData{}, err
	}

	entry, ok := parsed[bibkey]
	if !ok {
		return externalBookData{}, nil
	}

	data := externalBookData{}
	switch {
	case entry.Cover.Large != "":
		data.coverURL = entry.Cover.Large
	case entry.Cover.Medium != "":
		data.coverURL = entry.Cover.Medium
	case entry.Cover.Small != "":
		data.coverURL = entry.Cover.Small
	}
	if len(entry.Authors) > 0 {
		names := make([]string, len(entry.Authors))
		for i, a := range entry.Authors {
			names[i] = a.Name
		}
		data.author = joinAuthors(names)
	}
	return data, nil
}

// googleBooksVolumeInfo is the subset of a Google Books "volume" resource
// (whether returned directly by ID or as a search result item) both
// lookupGoogleBooksData and lookupGoogleBooksByISBN need.
type googleBooksVolumeInfo struct {
	Description string   `json:"description"`
	Authors     []string `json:"authors"`
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
	return externalBookData{coverURL: cover, description: v.Description, author: joinAuthors(v.Authors)}
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
	// fields restricts the response to just what toExternalBookData reads —
	// see https://developers.google.com/books/docs/v1/performance, "Using
	// partial response" — instead of the full volume resource (which also
	// carries authors, categories, saleInfo, etc. we never look at).
	reqURL := "https://www.googleapis.com/books/v1/volumes/" + url.PathEscape(volumeID) + "?alt=json&fields=" + url.QueryEscape("volumeInfo(description,authors,imageLinks)")
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
	// Same partial-response restriction as lookupGoogleBooksData, scoped to
	// the search response's items array.
	reqURL := "https://www.googleapis.com/books/v1/volumes?q=isbn:" + url.QueryEscape(isbn) +
		"&fields=" + url.QueryEscape("items(volumeInfo(description,authors,imageLinks))")
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

// hardcoverGraphQLEndpoint mirrors internal/handlers/metadata.go's constant
// of the same name — duplicated rather than imported, since handlers already
// imports services and Go doesn't allow the reverse.
const hardcoverGraphQLEndpoint = "https://api.hardcover.app/v1/graphql"

// hardcoverLookupByISBNQuery is a minimal version of metadata.go's
// hardcoverSearchByISBNQuery, scoped to just the fields
// resolveExternalData/mergeExternalLookup can use (cover + description +
// author) — this backfill/reconciliation path never persists the richer
// fields (publisher, language, ratings) that query also fetches.
const hardcoverLookupByISBNQuery = `
query BookLookupByIsbn($isbn: String!) {
  books(where: { editions: { isbn_13: { _eq: $isbn } } }) {
    description
    image { url }
    cached_contributors { author { name } contribution }
  }
}
`

// hardcoverLookupRateLimiter spaces this package's own Hardcover requests to
// stay within its free-tier 60 req/min cap — a separate limiter instance
// from internal/handlers/metadata.go's (services can't import handlers to
// share one), though in practice it rarely binds here: coverBackfillSpacing
// already paces at most one book — so at most one Hardcover call — every
// 1.5s, well under the 1/s this enforces.
var hardcoverLookupRateLimiter = &minIntervalLimiter{minInterval: time.Second}

// minIntervalLimiter enforces a minimum gap between successive calls,
// blocking callers (or returning early on ctx cancellation) rather than
// rejecting them outright. Mirrors internal/handlers/metadata.go's type of
// the same name.
type minIntervalLimiter struct {
	mu          sync.Mutex
	minInterval time.Duration
	next        time.Time
}

func (l *minIntervalLimiter) wait(ctx context.Context) {
	l.mu.Lock()
	now := time.Now()
	scheduled := now
	if l.next.After(now) {
		scheduled = l.next
	}
	l.next = scheduled.Add(l.minInterval)
	l.mu.Unlock()

	if d := time.Until(scheduled); d > 0 {
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
		}
	}
}

// hardcoverLookupContributor mirrors internal/handlers/metadata.go's
// hardcoverContributor — duplicated rather than imported, since handlers
// already imports services and Go doesn't allow the reverse.
type hardcoverLookupContributor struct {
	Author struct {
		Name string `json:"name"`
	} `json:"author"`
	Contribution string `json:"contribution"`
}

// hardcoverLookupResponse is the "data"/"errors" envelope for
// hardcoverLookupByISBNQuery.
type hardcoverLookupResponse struct {
	Data struct {
		Books []struct {
			Description        string                       `json:"description"`
			CachedContributors []hardcoverLookupContributor `json:"cached_contributors"`
			Image              struct {
				URL string `json:"url"`
			} `json:"image"`
		} `json:"books"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// hardcoverLookupAuthors returns every contributor credited as an author
// (contribution unset or "Author", matching Hardcover's own display rule —
// see hardcoverAuthorFromContributors in internal/handlers/metadata.go, which
// this mirrors but keeps every match instead of just the first).
func hardcoverLookupAuthors(contributors []hardcoverLookupContributor) []string {
	var names []string
	for _, c := range contributors {
		if c.Contribution == "" || strings.EqualFold(c.Contribution, "author") {
			if c.Author.Name != "" {
				names = append(names, c.Author.Name)
			}
		}
	}
	return names
}

// lookupHardcoverByISBN looks up a cover and description by ISBN via
// Hardcover's GraphQL API — the same source metadata search's Hardcover
// provider uses (internal/handlers/metadata.go's fetchHardcoverByISBN), but
// reimplemented minimally here since Book has no HardcoverID field to look
// up by directly (only ISBN), and this backfill/reconciliation sweep has no
// use for the free-text search path search's fan-out relies on.
func lookupHardcoverByISBN(ctx context.Context, client *http.Client, isbn, apiKey string) (externalBookData, string, error) {
	if apiKey == "" {
		return externalBookData{}, "no api key configured", nil
	}
	hardcoverLookupRateLimiter.wait(ctx)

	payload, err := json.Marshal(struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}{Query: hardcoverLookupByISBNQuery, Variables: map[string]any{"isbn": isbn}})
	if err != nil {
		return externalBookData{}, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hardcoverGraphQLEndpoint, bytes.NewReader(payload))
	if err != nil {
		return externalBookData{}, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return externalBookData{}, "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return externalBookData{}, fmt.Sprintf("http %d", resp.StatusCode), nil
	}

	var parsed hardcoverLookupResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return externalBookData{}, "", err
	}
	if len(parsed.Errors) > 0 {
		return externalBookData{}, "", fmt.Errorf("hardcover returned GraphQL errors: %s", parsed.Errors[0].Message)
	}
	if len(parsed.Data.Books) == 0 {
		return externalBookData{}, "no results", nil
	}

	book := parsed.Data.Books[0]
	return externalBookData{
		coverURL:    book.Image.URL,
		description: book.Description,
		author:      joinAuthors(hardcoverLookupAuthors(book.CachedContributors)),
	}, "", nil
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
// needed), then Google Books by ISBN search, then Hardcover by ISBN — kept
// last since it's the newest/least-tested source here and, unlike the other
// two, self-rate-limits (see hardcoverLookupRateLimiter). A step whose key
// is empty on book, or whose required API key isn't configured, is omitted
// rather than run.
func externalLookupSteps(ctx context.Context, client *http.Client, book models.Book, googleBooksAPIKey, hardcoverAPIKey string) []externalLookupStep {
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
		if hardcoverAPIKey != "" {
			steps = append(steps, externalLookupStep{"hardcover(isbn)", func() (externalBookData, string, error) {
				return lookupHardcoverByISBN(ctx, client, book.ISBN, hardcoverAPIKey)
			}})
		}
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
		if merged.author == "" {
			merged.author = data.author
		}
	case status != "":
		*attempts = append(*attempts, lookupAttempt{source, status})
	default:
		*attempts = append(*attempts, lookupAttempt{source, "no cover/description"})
	}
}

// googleBooksWasRateLimited reports whether attempts recorded a 429 from
// Google Books, so a caller using a GoogleBooksKeyPool knows to cool the key
// it just used down rather than picking it again immediately.
func googleBooksWasRateLimited(attempts []lookupAttempt) bool {
	for _, a := range attempts {
		if strings.HasPrefix(a.source, "google_books") && a.status == "http 429" {
			return true
		}
	}
	return false
}

// resolveExternalDataWithPool wraps resolveExternalData, drawing the Google
// Books key to use from pool and cooling it down on a 429 so the caller's
// next lookup round-robins onto a different key instead of retrying the one
// that just got rate-limited. hardcoverAPIKey is passed straight through —
// unlike Google Books, Hardcover has just the one server-wide key (see
// Config.HardcoverAPIKey), no pool to draw from.
func resolveExternalDataWithPool(ctx context.Context, client *http.Client, book models.Book, pool *GoogleBooksKeyPool, hardcoverAPIKey string, want wantedFields) (externalBookData, []lookupAttempt) {
	key := pool.Key()
	data, attempts := resolveExternalData(ctx, client, book, key, hardcoverAPIKey, want)
	if googleBooksWasRateLimited(attempts) {
		pool.MarkRateLimited(key)
	}
	return data, attempts
}

// resolveExternalData tries each of book's applicable external sources in
// turn (see externalLookupSteps), merging in whichever of coverURL/
// description/author each source can supply. A source erroring or coming
// back empty falls through to the next rather than aborting the lookup, and
// sources are tried until every field in want is filled or every source is
// exhausted — an earlier source contributing only a cover (e.g. Open
// Library, which never carries a description — see lookupOpenLibraryCover)
// must not short-circuit a later source that could still supply a still-
// wanted description; a prior "stop at the first source with any usable
// data" version of this function meant a book with an Open Library cover
// would never even be checked against Google Books for a description.
func resolveExternalData(ctx context.Context, client *http.Client, book models.Book, googleBooksAPIKey, hardcoverAPIKey string, want wantedFields) (externalBookData, []lookupAttempt) {
	var attempts []lookupAttempt
	var merged externalBookData

	for _, step := range externalLookupSteps(ctx, client, book, googleBooksAPIKey, hardcoverAPIKey) {
		data, status, err := step.run()
		mergeExternalLookup(&merged, &attempts, step.source, data, status, err)
		if want.satisfies(merged) {
			break
		}
	}
	return merged, attempts
}
