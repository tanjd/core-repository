package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"gopkg.in/yaml.v3"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// Importing a file is inherently importing untrusted input — it may come
// from a different bookshelf instance, or be hand-edited/corrupted in
// transit. Every row is therefore treated the same way a manually-typed
// "add a book" submission would be: sanitized, capped in size, matched
// against the catalog through the exact same findExistingBook precedence
// createBook already trusts, and attributed only to the authenticated
// importer. Nothing in the file — an ID, a status, an owner — is ever taken
// at face value.

const (
	// maxImportRows caps how many rows a single import can contain, so a
	// huge or maliciously inflated file can't trigger an unbounded batch.
	maxImportRows = 1000
	// maxImportFieldLen caps free-text fields pulled from an import row.
	maxImportFieldLen = 500
)

type importAction string

const (
	actionCreateBook    importAction = "create_book"
	actionMatchBook     importAction = "match_existing_book"
	actionPossibleMatch importAction = "possible_match"
	actionSkipped       importAction = "skipped"
)

// Decision values a member can send for a possible_match row on commit.
const (
	decisionAcceptMatch = "accept_match"
	decisionCreateNew   = "create_new"
)

type importInput struct {
	Body struct {
		Format  string `json:"format" required:"true" doc:"Import format: json, yaml, or csv"`
		Content string `json:"content" required:"true" maxLength:"2000000" doc:"Raw file contents, as produced by GET /copies/mine/export"`
		// Decisions resolves possible_match rows on commit: 1-based row
		// number (as a string, since JSON object keys are always strings) →
		// "accept_match" or "create_new". A possible_match row with no entry
		// here defaults to "create_new" — see commitImportPlan.
		Decisions map[string]string `json:"decisions,omitempty" doc:"1-based row number to accept_match/create_new, for resolving possible_match rows"`
	}
}

type importRowResult struct {
	Row               int          `json:"row" doc:"1-based row index in the parsed file"`
	Title             string       `json:"title"`
	Action            importAction `json:"action" doc:"create_book, match_existing_book, possible_match, or skipped"`
	Reason            string       `json:"reason,omitempty" doc:"Why the row was skipped, if action is skipped"`
	MatchedBookID     *uint        `json:"matched_book_id,omitempty" doc:"Set for match_existing_book and possible_match"`
	MatchedBookTitle  string       `json:"matched_book_title,omitempty"`
	MatchedBookAuthor string       `json:"matched_book_author,omitempty"`
}

type importSummary struct {
	BooksCreated    int `json:"books_created"`
	BooksMatched    int `json:"books_matched"`
	PossibleMatches int `json:"possible_matches"`
	CopiesCreated   int `json:"copies_created"`
	Skipped         int `json:"skipped"`
}

type importOutput struct {
	Body struct {
		Summary importSummary     `json:"summary"`
		Rows    []importRowResult `json:"rows"`
	}
}

// decodeImportRows parses content in the given format into export rows,
// reusing exportRow as the accepted import shape for round-trip symmetry
// with GET /copies/mine/export. Unlike export, parsing happens entirely
// server-side (the frontend just reads the uploaded file as text) so this
// is the single place format-specific parsing/validation logic lives,
// rather than duplicating a second JSON/YAML/CSV parser in the browser.
func decodeImportRows(format, content string) ([]exportRow, error) {
	switch format {
	case "json":
		var rows []exportRow
		if err := json.Unmarshal([]byte(content), &rows); err != nil {
			return nil, fmt.Errorf("invalid json: %w", err)
		}
		return capImportRows(rows)
	case "yaml":
		var rows []exportRow
		if err := yaml.Unmarshal([]byte(content), &rows); err != nil {
			return nil, fmt.Errorf("invalid yaml: %w", err)
		}
		return capImportRows(rows)
	case "csv":
		rows, err := decodeImportCSV(content)
		if err != nil {
			return nil, err
		}
		return capImportRows(rows)
	default:
		return nil, errors.New("format must be one of: json, yaml, csv")
	}
}

func capImportRows(rows []exportRow) ([]exportRow, error) {
	if len(rows) > maxImportRows {
		return nil, fmt.Errorf("too many rows: %d (max %d)", len(rows), maxImportRows)
	}
	return rows, nil
}

// decodeImportCSV maps columns by header name rather than position, so a
// reordered or partially-edited header row still parses correctly — the
// same tolerance a hand-edited export file needs.
func decodeImportCSV(content string) ([]exportRow, error) {
	r := csv.NewReader(strings.NewReader(content))
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("invalid csv: %w", err)
	}
	if len(records) == 0 {
		return nil, errors.New("csv file is empty")
	}

	colIdx := make(map[string]int, len(records[0]))
	for i, name := range records[0] {
		colIdx[strings.TrimSpace(strings.ToLower(name))] = i
	}
	get := func(row []string, name string) string {
		idx, ok := colIdx[name]
		if !ok || idx >= len(row) {
			return ""
		}
		return row[idx]
	}

	dataRows := records[1:]
	rows := make([]exportRow, len(dataRows))
	for i, rec := range dataRows {
		pageCount, _ := strconv.Atoi(get(rec, "page_count"))
		rows[i] = exportRow{
			Title:         get(rec, "title"),
			Author:        get(rec, "author"),
			ISBN:          get(rec, "isbn"),
			OLKey:         get(rec, "ol_key"),
			GoogleBooksID: get(rec, "google_books_id"),
			Publisher:     get(rec, "publisher"),
			PublishedDate: get(rec, "published_date"),
			PageCount:     pageCount,
			Language:      get(rec, "language"),
			Condition:     get(rec, "condition"),
			Notes:         get(rec, "notes"),
			Status:        get(rec, "status"),
		}
	}
	return rows, nil
}

// sanitizeImportRow trims and caps free-text fields, and drops Status
// entirely — an imported copy always starts "available", never trusting a
// foreign instance's live loan state.
func sanitizeImportRow(row exportRow) exportRow {
	row.Title = truncate(row.Title, maxImportFieldLen)
	row.Author = truncate(row.Author, maxImportFieldLen)
	row.ISBN = truncate(row.ISBN, 50)
	row.OLKey = truncate(row.OLKey, 100)
	row.GoogleBooksID = truncate(row.GoogleBooksID, 100)
	row.Publisher = truncate(row.Publisher, maxImportFieldLen)
	row.PublishedDate = truncate(row.PublishedDate, 50)
	row.Language = truncate(row.Language, 50)
	row.Condition = truncate(row.Condition, 50)
	row.Notes = truncate(row.Notes, 2000)
	if row.PageCount < 0 {
		row.PageCount = 0
	}
	row.Status = ""
	return row
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}

// rowPlan is a classified import row: what would happen to it, without
// having written anything yet. Shared by preview (which stops here) and
// commit (which additionally executes the plan).
type rowPlan struct {
	row    exportRow
	action importAction
	reason string
	book   *models.Book // set when action is actionMatchBook or actionPossibleMatch
}

// classifyImportRow sanitizes row and decides its plan using the exact same
// dedup precedence createBook already trusts (findExistingBook) — an import
// must never invent a weaker or different matching heuristic. On a miss, a
// normalized title+author comparison against the catalog gets one more,
// weaker try before falling back to creating a new book (findFuzzyMatch).
func (h *CopyHandler) classifyImportRow(row exportRow) rowPlan {
	row = sanitizeImportRow(row)
	if row.Title == "" {
		return rowPlan{row: row, action: actionSkipped, reason: "missing title"}
	}
	if book, err := findExistingBook(h.books, row.OLKey, row.GoogleBooksID, row.ISBN); err == nil {
		return rowPlan{row: row, action: actionMatchBook, book: book}
	}
	if book, err := findFuzzyMatch(h.books, row.Title, row.Author); err == nil {
		return rowPlan{row: row, action: actionPossibleMatch, book: book}
	}
	return rowPlan{row: row, action: actionCreateBook}
}

// findFuzzyMatch compares row's normalized title+author key against every
// catalog book with at least one copy, returning the first exact key match.
// Weaker than findExistingBook: no external key agreement, just title+author
// text equality after normalization — callers must treat the result as a
// suggestion (actionPossibleMatch), never auto-apply it. Mirrors the
// empty/"|"-only key guard findExistingGroup already applies in
// metadata_consolidate.go, so a blank title+author never "matches" every
// other blank row.
func findFuzzyMatch(books repository.BookRepository, title, author string) (*models.Book, error) {
	key := normalizeTitleAuthor(title, author)
	if key == "|" {
		return nil, repository.ErrNotFound
	}
	catalog, err := books.List("", "", false)
	if err != nil {
		return nil, err
	}
	for i := range catalog {
		if normalizeTitleAuthor(catalog[i].Title, catalog[i].Author) == key {
			return &catalog[i], nil
		}
	}
	return nil, repository.ErrNotFound
}

// --- Route registration ---

func (h *CopyHandler) registerImportRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "preview-import-books",
		Method:      "POST",
		Path:        "/copies/mine/import/preview",
		Tags:        []string{"copies"},
		Summary:     "Preview a books import file without writing anything",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.previewImportBooks)

	huma.Register(api, huma.Operation{
		OperationID: "import-books",
		Method:      "POST",
		Path:        "/copies/mine/import",
		Tags:        []string{"copies"},
		Summary:     "Import books from a JSON, YAML, or CSV file into the authenticated user's shelf",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.importBooks)
}

// --- Handlers ---

func (h *CopyHandler) previewImportBooks(ctx context.Context, input *importInput) (*importOutput, error) {
	if _, err := middleware.GetRequiredUserID(ctx); err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}
	rows, err := decodeImportRows(input.Body.Format, input.Body.Content)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}

	out := &importOutput{}
	out.Body.Rows = make([]importRowResult, len(rows))
	for i, row := range rows {
		plan := h.classifyImportRow(row)
		out.Body.Rows[i] = importRowResultFor(i+1, plan.row.Title, plan.action, plan.reason, plan.book)
		tallyImportResult(&out.Body.Summary, plan.action)
	}
	return out, nil
}

// importRowResultFor assembles a row result, surfacing matchedBook's
// identity when set (match_existing_book and possible_match rows).
func importRowResultFor(row int, title string, action importAction, reason string, matchedBook *models.Book) importRowResult {
	result := importRowResult{Row: row, Title: title, Action: action, Reason: reason}
	if matchedBook != nil {
		result.MatchedBookID = &matchedBook.ID
		result.MatchedBookTitle = matchedBook.Title
		result.MatchedBookAuthor = matchedBook.Author
	}
	return result
}

func (h *CopyHandler) importBooks(ctx context.Context, input *importInput) (*importOutput, error) {
	ownerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}
	rows, err := decodeImportRows(input.Body.Format, input.Body.Content)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}

	maxCopies := h.maxCopiesPerUser()
	currentCount, err := h.copies.CountByOwnerID(ownerID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not check existing copies")
	}

	out := &importOutput{}
	out.Body.Rows = make([]importRowResult, len(rows))
	for i, row := range rows {
		plan := h.classifyImportRow(row)
		action, reason, matchedBook := h.commitImportPlan(ctx, ownerID, plan, i+1, input.Body.Decisions, maxCopies, &currentCount)
		out.Body.Rows[i] = importRowResultFor(i+1, plan.row.Title, action, reason, matchedBook)
		tallyImportResult(&out.Body.Summary, action)
	}
	return out, nil
}

// commitImportPlan performs the write(s) for one already-classified import
// row and returns its resolved action, reason, and (for a row that ends up
// attached to an existing book) that book. A problem with one row is
// recorded as a skip and never aborts the rest of the batch — a hostile or
// malformed row must not be able to block the good rows around it.
// currentCount is advanced on success so max_copies_per_user stays accurate
// across the whole import.
//
// A possible_match row is resolved here against decisions[row] (1-based row
// number, matching importRowResult.Row): "accept_match" attaches to
// plan.book same as an exact match_existing_book would; anything else
// (including a missing entry) falls back to creating a new book — see the
// spec's "safe default on no decision". Either way, the *reported* action is
// the resolved one (match_existing_book or create_book), never
// possible_match — a commit always actually does one or the other.
func (h *CopyHandler) commitImportPlan(
	ctx context.Context, ownerID uint, plan rowPlan, row int, decisions map[string]string,
	maxCopies int64, currentCount *int64,
) (importAction, string, *models.Book) {
	if plan.action == actionSkipped {
		return actionSkipped, plan.reason, nil
	}
	if maxCopies > 0 && *currentCount >= maxCopies {
		return actionSkipped, fmt.Sprintf("reached the maximum of %d shared copy/copies", maxCopies), nil
	}

	action := plan.action
	book := plan.book
	if action == actionPossibleMatch {
		if decisions[strconv.Itoa(row)] == decisionAcceptMatch {
			action = actionMatchBook
		} else {
			action = actionCreateBook
			book = nil
		}
	}
	if action == actionCreateBook {
		created, err := h.createImportedBook(ctx, plan.row)
		if err != nil {
			return actionSkipped, "could not create book", nil
		}
		book = created
	}

	bookCopy := models.Copy{
		BookID:    book.ID,
		OwnerID:   ownerID,
		Condition: plan.row.Condition,
		Notes:     plan.row.Notes,
		Status:    "available",
	}
	if err := h.copies.Create(&bookCopy); err != nil {
		return actionSkipped, "could not create copy", nil
	}
	*currentCount++
	if action == actionMatchBook {
		return action, "", book
	}
	return action, "", nil
}

// createImportedBook creates a new Book from an import row. No cover URL or
// description — the export format never carries either — and, same as
// createBook's genuinely-new-book path, runs wishlist auto-fulfillment.
func (h *CopyHandler) createImportedBook(ctx context.Context, row exportRow) (*models.Book, error) {
	book := models.Book{
		Title:         row.Title,
		Author:        row.Author,
		ISBN:          row.ISBN,
		OLKey:         row.OLKey,
		GoogleBooksID: row.GoogleBooksID,
		Publisher:     row.Publisher,
		PublishedDate: row.PublishedDate,
		PageCount:     row.PageCount,
		Language:      row.Language,
	}
	if err := h.books.Create(&book); err != nil {
		return nil, err
	}
	if h.wishlistWorkflow != nil {
		h.wishlistWorkflow.OnBookCreated(ctx, &book) // log-and-continue; never blocks import
	}
	return &book, nil
}

func tallyImportResult(summary *importSummary, action importAction) {
	switch action {
	case actionCreateBook:
		summary.BooksCreated++
		summary.CopiesCreated++
	case actionMatchBook:
		summary.BooksMatched++
		summary.CopiesCreated++
	case actionPossibleMatch:
		summary.PossibleMatches++
	case actionSkipped:
		summary.Skipped++
	}
}
