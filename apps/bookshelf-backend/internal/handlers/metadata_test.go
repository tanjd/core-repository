package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
)

// fakeDoer is an httpDoer test double — see metadataClient's seam comment in
// metadata.go for why fetch functions go through the interface at all.
type fakeDoer struct {
	do func(*http.Request) (*http.Response, error)
}

func (f fakeDoer) Do(req *http.Request) (*http.Response, error) { return f.do(req) }

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

// withFakeMetadataClient swaps the package-level metadataClient for the
// duration of the calling test, restoring it on cleanup.
func withFakeMetadataClient(t *testing.T, do func(*http.Request) (*http.Response, error)) {
	t.Helper()
	orig := metadataClient
	metadataClient = fakeDoer{do: do}
	t.Cleanup(func() { metadataClient = orig })
}

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

func TestMinIntervalLimiter_SpacesCalls(t *testing.T) {
	l := &minIntervalLimiter{minInterval: 20 * time.Millisecond}
	start := time.Now()
	l.wait(context.Background())
	l.wait(context.Background())
	assert.GreaterOrEqual(t, time.Since(start), 20*time.Millisecond)
}

func TestFetchHardcoverByISBN_MapsBookAndEditionFields(t *testing.T) {
	body := `{"data":{"books":[{"slug":"go-in-action","title":"Go in Action",` +
		`"description":"desc","cached_contributors":[{"author":{"name":"Kennedy"},"contribution":"Author"}],` +
		`"pages":300,"release_year":2015,"image":{"url":"https://covers.example/x.jpg"},` +
		`"editions":[{"publisher":{"name":"Manning"},"isbn_13":"9781617291769","language":{"code2":"en"}}]}]}}`

	var gotAuth string
	withFakeMetadataClient(t, func(req *http.Request) (*http.Response, error) {
		gotAuth = req.Header.Get("Authorization")
		assert.Equal(t, hardcoverGraphQLEndpoint, req.URL.String())
		return jsonResponse(http.StatusOK, body), nil
	})

	results, err := fetchHardcoverByISBN(context.Background(), "9781617291769", "test-key")
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Equal(t, "Bearer test-key", gotAuth)
	r := results[0]
	assert.Equal(t, "hardcover", r.Source)
	assert.Equal(t, "go-in-action", r.HardcoverID)
	assert.Equal(t, "Go in Action", r.Title)
	assert.Equal(t, "Kennedy", r.Author)
	assert.Equal(t, "desc", r.Description)
	assert.Equal(t, 300, r.PageCount)
	assert.Equal(t, "Manning", r.Publisher)
	assert.Equal(t, "9781617291769", r.ISBN)
	assert.Equal(t, "en", r.Language)
	assert.Equal(t, "2015", r.PublishedDate, "falls back to release_year when release_date is absent")
	assert.Equal(t, "https://covers.example/x.jpg", r.CoverURL)
}

func TestFetchHardcoverByISBN_NoEditionsStillReturnsBookLevelResult(t *testing.T) {
	body := `{"data":{"books":[{"slug":"solo","title":"Solo","editions":[]}]}}`
	withFakeMetadataClient(t, func(_ *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, body), nil
	})

	results, err := fetchHardcoverByISBN(context.Background(), "9781617291769", "test-key")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Solo", results[0].Title)
	assert.Empty(t, results[0].ISBN)
}

func TestFetchHardcoverBySearch_MapsSearchDocuments(t *testing.T) {
	searchBody := `{"data":{"search":{"results":{"hits":[{"document":{"slug":"go-in-action","title":"Go in Action",` +
		`"description":"desc","contributions":[{"author":{"name":"Kennedy"}}],` +
		`"isbns":["1617291769","9781617291769"],"pages":300,"release_date":"2015-11-01",` +
		`"image":{"url":"https://covers.example/x.jpg"}}}]}}}}`
	editionsBody := `{"data":{"books":[{"editions":[{"publisher":{"name":"Manning"},"isbn_13":"9781617291769","language":{"code2":"en"}}]}]}}`

	var editionsReqBody string
	withFakeMetadataClient(t, func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		if strings.Contains(string(body), "BookEditionsBySlug") {
			editionsReqBody = string(body)
			return jsonResponse(http.StatusOK, editionsBody), nil
		}
		return jsonResponse(http.StatusOK, searchBody), nil
	})

	results, err := fetchHardcoverBySearch(context.Background(), "go in action kennedy", "test-key")
	require.NoError(t, err)
	require.Len(t, results, 1)

	r := results[0]
	assert.Equal(t, "go-in-action", r.HardcoverID)
	assert.Equal(t, "Go in Action", r.Title)
	assert.Equal(t, "Kennedy", r.Author)
	assert.Equal(t, "9781617291769", r.ISBN, "prefers the ISBN-13-shaped entry")
	assert.Equal(t, "2015-11-01", r.PublishedDate)
	assert.Equal(t, "Manning", r.Publisher, "backfilled from the top-hit editions follow-up")
	assert.Equal(t, "en", r.Language, "backfilled from the top-hit editions follow-up")
	assert.Contains(t, editionsReqBody, "go-in-action", "editions lookup is scoped to the top hit's slug")
}

func TestFetchHardcoverBySearch_EditionsBackfillFailureDoesNotFailSearch(t *testing.T) {
	searchBody := `{"data":{"search":{"results":{"hits":[{"document":{"slug":"go-in-action","title":"Go in Action",` +
		`"isbns":["9781617291769"]}}]}}}}`

	withFakeMetadataClient(t, func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		if strings.Contains(string(body), "BookEditionsBySlug") {
			return jsonResponse(http.StatusInternalServerError, `{}`), nil
		}
		return jsonResponse(http.StatusOK, searchBody), nil
	})

	results, err := fetchHardcoverBySearch(context.Background(), "go in action", "test-key")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Go in Action", results[0].Title)
	assert.Empty(t, results[0].Publisher, "editions backfill failure is swallowed, not a hard search failure")
}

func TestFetchHardcoverBySearch_NoHitsSkipsEditionsBackfill(t *testing.T) {
	body := `{"data":{"search":{"results":{"hits":[]}}}}`

	calls := 0
	withFakeMetadataClient(t, func(_ *http.Request) (*http.Response, error) {
		calls++
		return jsonResponse(http.StatusOK, body), nil
	})

	results, err := fetchHardcoverBySearch(context.Background(), "no such book", "test-key")
	require.NoError(t, err)
	assert.Empty(t, results)
	assert.Equal(t, 1, calls, "no top hit to enrich, so no second call is made")
}

func TestFetchHardcover_NonOKStatusIsAnError(t *testing.T) {
	withFakeMetadataClient(t, func(_ *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusUnauthorized, `{}`), nil
	})

	_, err := fetchHardcover(context.Background(), "some title", "bad-key")
	assert.ErrorContains(t, err, "hardcover returned 401")
}

func TestFetchHardcover_GraphQLErrorsOnHTTP200IsAnError(t *testing.T) {
	withFakeMetadataClient(t, func(_ *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"errors":[{"message":"field 'isbn' is required"}],"data":null}`), nil
	})

	_, err := fetchHardcover(context.Background(), "some title", "test-key")
	assert.ErrorContains(t, err, "field 'isbn' is required")
}

func TestResolveHardcoverAPIKey_PrefersDecryptedUserKeyOverServerFallback(t *testing.T) {
	const secret = "test-encryption-secret-32bytes!"
	encrypted, err := encryptField("user-hardcover-key", secret)
	require.NoError(t, err)

	users := repotest.NewUserRepository()
	require.NoError(t, users.Save(&models.User{ID: 1, HardcoverAPIKey: encrypted}))

	h := &MetadataHandler{hardcoverAPIKey: "server-key", encryptionSecret: secret, users: users}
	ctx := fakeAuthedCtx(t, 1, "user")

	assert.Equal(t, "user-hardcover-key", h.resolveHardcoverAPIKey(ctx))
}

func TestResolveHardcoverAPIKey_FallsBackToServerKey(t *testing.T) {
	users := repotest.NewUserRepository()
	require.NoError(t, users.Save(&models.User{ID: 1}))

	h := &MetadataHandler{hardcoverAPIKey: "server-key", encryptionSecret: "test-encryption-secret-32bytes!", users: users}

	t.Run("no authenticated user", func(t *testing.T) {
		assert.Equal(t, "server-key", h.resolveHardcoverAPIKey(context.Background()))
	})

	t.Run("user has no stored key", func(t *testing.T) {
		assert.Equal(t, "server-key", h.resolveHardcoverAPIKey(fakeAuthedCtx(t, 1, "user")))
	})

	t.Run("stored key undecryptable with the current secret", func(t *testing.T) {
		encrypted, err := encryptField("user-hardcover-key", "a-different-secret-entirely!!!!")
		require.NoError(t, err)
		require.NoError(t, users.Save(&models.User{ID: 1, HardcoverAPIKey: encrypted}))
		assert.Equal(t, "server-key", h.resolveHardcoverAPIKey(fakeAuthedCtx(t, 1, "user")))
	})
}

func TestHardcoverAuthorFromContributors(t *testing.T) {
	author := func(name string) (a struct {
		Name string `json:"name"`
	}) {
		a.Name = name
		return a
	}
	tests := []struct {
		name string
		in   []hardcoverContributor
		want string
	}{
		{
			name: "unset contribution defaults to author",
			in:   []hardcoverContributor{{Contribution: "", Author: author("Kennedy")}},
			want: "Kennedy",
		},
		{
			name: "non-author contribution is skipped",
			in:   []hardcoverContributor{{Contribution: "Illustrator", Author: author("Someone")}},
			want: "",
		},
		{
			name: "empty input",
			in:   nil,
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, hardcoverAuthorFromContributors(tt.in))
		})
	}
}

func TestPreferredHardcoverISBN(t *testing.T) {
	assert.Equal(t, "9781617291769", preferredHardcoverISBN([]string{"1617291769", "9781617291769"}))
	assert.Equal(t, "1617291769", preferredHardcoverISBN([]string{"1617291769"}))
	assert.Empty(t, preferredHardcoverISBN(nil))
}
