package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog/log"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// metadataClient is a shared HTTP client with a timeout for all metadata fetches.
var metadataClient = &http.Client{Timeout: 10 * time.Second}

// BookMetadataResult is a normalised search result from any metadata source.
type BookMetadataResult struct {
	Source        string `json:"source"`
	Title         string `json:"title"`
	Author        string `json:"author"`
	ISBN          string `json:"isbn"`
	CoverURL      string `json:"cover_url"`
	Description   string `json:"description"`
	Publisher     string `json:"publisher"`
	PublishedDate string `json:"published_date"`
	PageCount     int    `json:"page_count"`
	Language      string `json:"language"`
	OLKey         string `json:"ol_key"`
	GoogleBooksID string `json:"google_books_id"`
	BookBrainzID  string `json:"bookbrainz_id,omitempty"`
}

const searchCacheTTL = 1 * time.Hour

// MetadataHandler handles book metadata search routes.
type MetadataHandler struct {
	googleBooksAPIKey string
	encryptionSecret  string
	users             repository.UserRepository
	cache             MetadataCache
}

// NewMetadataHandler creates a MetadataHandler.
func NewMetadataHandler(ctx context.Context, googleBooksAPIKey, encryptionSecret string, users repository.UserRepository) *MetadataHandler {
	return &MetadataHandler{
		googleBooksAPIKey: googleBooksAPIKey,
		encryptionSecret:  encryptionSecret,
		users:             users,
		cache:             NewInMemoryMetadataCache(ctx, searchCacheTTL),
	}
}

// RegisterRoutes registers the metadata routes on the given huma API.
func (h *MetadataHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "search-book-metadata",
		Method:      "GET",
		Path:        "/books/metadata/search",
		Tags:        []string{"books"},
		Summary:     "Fan-out metadata search across Open Library and Google Books",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.searchMetadata)

	huma.Register(api, huma.Operation{
		OperationID: "get-ol-description",
		Method:      "GET",
		Path:        "/books/metadata/ol-description",
		Tags:        []string{"books"},
		Summary:     "Fetch work description from Open Library (lazy)",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.getOLDescription)
}

type searchMetadataInput struct {
	Q string `query:"q" required:"true" doc:"Search query (title, author, or ISBN)"`
}

type searchMetadataOutput struct {
	Body []BookMetadataResult
}

type olDescriptionInput struct {
	OLKey string `query:"ol_key" required:"true" doc:"Open Library work key e.g. OL12345W"`
}

type olDescriptionOutput struct {
	Body struct {
		Description string `json:"description"`
	}
}

func (h *MetadataHandler) searchMetadata(ctx context.Context, input *searchMetadataInput) (*searchMetadataOutput, error) {
	q := strings.TrimSpace(input.Q)
	if q == "" {
		return &searchMetadataOutput{Body: []BookMetadataResult{}}, nil
	}

	apiKey := h.resolveGoogleBooksAPIKey(ctx)

	// Cache key incorporates whether Google Books is active so that users with
	// and without a Google Books key do not share cache entries.
	cacheKey := strings.ToLower(q)
	if apiKey != "" {
		cacheKey += "|gbooks"
	}
	if cached, ok := h.cache.Get(cacheKey); ok {
		log.Info().Str("query", q).Msg("metadata search cache hit")
		return &searchMetadataOutput{Body: cached}, nil
	}

	results := fetchAllSources(q, apiKey)

	consolidated := consolidateResults(results)
	h.cache.Set(cacheKey, consolidated)
	return &searchMetadataOutput{Body: consolidated}, nil
}

// resolveGoogleBooksAPIKey prefers the authenticated user's stored key,
// falling back to the server-wide key when unauthenticated, unset, or
// undecryptable.
func (h *MetadataHandler) resolveGoogleBooksAPIKey(ctx context.Context) string {
	apiKey := h.googleBooksAPIKey

	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return apiKey
	}
	user, err := h.users.FindByID(userID)
	if err != nil || user.GoogleBooksAPIKey == "" {
		return apiKey
	}
	decrypted, err := decryptField(user.GoogleBooksAPIKey, h.encryptionSecret)
	if err != nil {
		log.Warn().Err(err).Uint("user_id", userID).Msg("could not decrypt user google books api key")
		return apiKey
	}
	return decrypted
}

// fetchAllSources queries Open Library, Google Books (if apiKey is set), and
// BookBrainz concurrently, merging all results. A per-source failure is
// logged and otherwise ignored.
func fetchAllSources(q, apiKey string) []BookMetadataResult {
	var mu sync.Mutex
	var results []BookMetadataResult
	var wg sync.WaitGroup

	fetch := func(name string, fn func() ([]BookMetadataResult, error)) {
		defer wg.Done()
		items, err := fn()
		if err != nil {
			log.Warn().Err(err).Msg(name + " search failed")
			return
		}
		mu.Lock()
		results = append(results, items...)
		mu.Unlock()
	}

	wg.Add(1)
	go fetch("open library", func() ([]BookMetadataResult, error) { return fetchOpenLibrary(q) })

	if apiKey != "" {
		wg.Add(1)
		go fetch("google books", func() ([]BookMetadataResult, error) { return fetchGoogleBooks(q, apiKey) })
	}

	wg.Add(1)
	go fetch("bookbrainz", func() ([]BookMetadataResult, error) { return fetchBookBrainz(q) })

	wg.Wait()
	return results
}

func (h *MetadataHandler) getOLDescription(_ context.Context, input *olDescriptionInput) (*olDescriptionOutput, error) {
	workKey := strings.TrimPrefix(input.OLKey, "/works/")
	apiURL := fmt.Sprintf("https://openlibrary.org/works/%s.json", url.PathEscape(workKey))

	resp, err := metadataClient.Get(apiURL) //nolint:noctx,gosec
	if err != nil {
		return nil, huma.Error502BadGateway("could not reach Open Library")
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return &olDescriptionOutput{}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, huma.Error502BadGateway("could not read Open Library response")
	}

	var work struct {
		Description json.RawMessage `json:"description"`
	}
	if err := json.Unmarshal(body, &work); err != nil {
		return &olDescriptionOutput{}, nil
	}

	var out olDescriptionOutput
	// description may be a plain string or {"type":..., "value": "..."}
	var plain string
	if err := json.Unmarshal(work.Description, &plain); err == nil {
		out.Body.Description = plain
	} else {
		var obj struct {
			Value string `json:"value"`
		}
		if err := json.Unmarshal(work.Description, &obj); err == nil {
			out.Body.Description = obj.Value
		}
	}
	return &out, nil
}

// fetchOpenLibrary calls the OL search API and returns normalised results.
func fetchOpenLibrary(q string) ([]BookMetadataResult, error) {
	log.Info().Str("query", q).Msg("searching Open Library")
	apiURL := fmt.Sprintf(
		"https://openlibrary.org/search.json?q=%s&fields=key,title,author_name,isbn,cover_i&limit=10",
		url.QueryEscape(q),
	)
	resp, err := metadataClient.Get(apiURL) //nolint:noctx,gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open library returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Docs []struct {
			Key        string   `json:"key"`
			Title      string   `json:"title"`
			AuthorName []string `json:"author_name"`
			ISBN       []string `json:"isbn"`
			CoverI     int64    `json:"cover_i"`
		} `json:"docs"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	log.Info().Str("query", q).Int("results", len(payload.Docs)).Msg("Open Library search complete")
	results := make([]BookMetadataResult, 0, len(payload.Docs))
	for _, doc := range payload.Docs {
		r := BookMetadataResult{
			Source: "openlibrary",
			Title:  doc.Title,
			OLKey:  doc.Key,
		}
		if len(doc.AuthorName) > 0 {
			r.Author = doc.AuthorName[0]
		}
		if len(doc.ISBN) > 0 {
			r.ISBN = doc.ISBN[0]
		}
		if doc.CoverI > 0 {
			r.CoverURL = fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-M.jpg", doc.CoverI)
		}
		results = append(results, r)
	}
	return results, nil
}

// validateGoogleBooksAPIKey makes a minimal test call to verify the key is accepted by Google Books.
func validateGoogleBooksAPIKey(key string) error {
	apiURL := fmt.Sprintf(
		"https://www.googleapis.com/books/v1/volumes?q=test&key=%s&maxResults=1",
		url.QueryEscape(key),
	)
	resp, err := metadataClient.Get(apiURL) //nolint:noctx,gosec
	if err != nil {
		return fmt.Errorf("could not reach Google Books API: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("google Books API rejected the key (status %d)", resp.StatusCode)
	}
	return nil
}

type googleBooksIndustryIdentifier struct {
	Type       string `json:"type"`
	Identifier string `json:"identifier"`
}

type googleBooksImageLinks struct {
	Thumbnail string `json:"thumbnail"`
}

type googleBooksVolumeInfo struct {
	Title               string                          `json:"title"`
	Authors             []string                        `json:"authors"`
	Publisher           string                          `json:"publisher"`
	PublishedDate       string                          `json:"publishedDate"`
	Description         string                          `json:"description"`
	PageCount           int                             `json:"pageCount"`
	Language            string                          `json:"language"`
	IndustryIdentifiers []googleBooksIndustryIdentifier `json:"industryIdentifiers"`
	ImageLinks          googleBooksImageLinks           `json:"imageLinks"`
}

type googleBooksItem struct {
	ID         string                `json:"id"`
	VolumeInfo googleBooksVolumeInfo `json:"volumeInfo"`
}

type googleBooksSearchResponse struct {
	Items []googleBooksItem `json:"items"`
}

// googleVolumeToResult converts a single Google Books API item into a normalised result.
func googleVolumeToResult(item googleBooksItem) BookMetadataResult {
	vi := item.VolumeInfo
	r := BookMetadataResult{
		Source:        "google_books",
		GoogleBooksID: item.ID,
		Title:         vi.Title,
		Publisher:     vi.Publisher,
		PublishedDate: vi.PublishedDate,
		Description:   vi.Description,
		PageCount:     vi.PageCount,
		Language:      vi.Language,
		ISBN:          preferredISBN(vi.IndustryIdentifiers),
	}
	if len(vi.Authors) > 0 {
		r.Author = vi.Authors[0]
	}
	if thumb := vi.ImageLinks.Thumbnail; thumb != "" {
		r.CoverURL = strings.Replace(thumb, "http://", "https://", 1)
	}
	return r
}

// preferredISBN returns the ISBN-13 identifier if present, else ISBN-10, else "".
func preferredISBN(ids []googleBooksIndustryIdentifier) string {
	for _, id := range ids {
		if id.Type == "ISBN_13" {
			return id.Identifier
		}
	}
	for _, id := range ids {
		if id.Type == "ISBN_10" {
			return id.Identifier
		}
	}
	return ""
}

// fetchGoogleBooks calls the Google Books API and returns normalised results.
func fetchGoogleBooks(q, apiKey string) ([]BookMetadataResult, error) {
	log.Info().Str("query", q).Msg("searching Google Books")
	apiURL := fmt.Sprintf(
		"https://www.googleapis.com/books/v1/volumes?q=%s&key=%s&maxResults=10",
		url.QueryEscape(q),
		url.QueryEscape(apiKey),
	)
	resp, err := metadataClient.Get(apiURL) //nolint:noctx,gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google books returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload googleBooksSearchResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	log.Info().Str("query", q).Int("results", len(payload.Items)).Msg("Google Books search complete")
	results := make([]BookMetadataResult, 0, len(payload.Items))
	for _, item := range payload.Items {
		results = append(results, googleVolumeToResult(item))
	}
	return results, nil
}

// fetchBookBrainz calls the BookBrainz search API and returns normalised results.
// BookBrainz does not provide cover images.
func fetchBookBrainz(q string) ([]BookMetadataResult, error) {
	log.Debug().Str("query", q).Msg("searching BookBrainz")
	apiURL := fmt.Sprintf(
		"https://api.bookbrainz.org/1/search?q=%s&type=edition&size=10",
		url.QueryEscape(q),
	)
	resp, err := metadataClient.Get(apiURL) //nolint:noctx,gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bookbrainz returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Results []struct {
			BBID         string `json:"bbid"`
			DefaultAlias struct {
				Name string `json:"name"`
			} `json:"default-alias"`
			AuthorCredit struct {
				Names []struct {
					Name string `json:"name"`
				} `json:"names"`
			} `json:"author-credit"`
		} `json:"search-results"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	log.Debug().Str("query", q).Int("results", len(payload.Results)).Msg("BookBrainz search complete")
	results := make([]BookMetadataResult, 0, len(payload.Results))
	for _, item := range payload.Results {
		if item.DefaultAlias.Name == "" {
			continue
		}
		r := BookMetadataResult{
			Source:       "bookbrainz",
			Title:        item.DefaultAlias.Name,
			BookBrainzID: item.BBID,
		}
		if len(item.AuthorCredit.Names) > 0 {
			r.Author = item.AuthorCredit.Names[0].Name
		}
		results = append(results, r)
	}
	return results, nil
}
