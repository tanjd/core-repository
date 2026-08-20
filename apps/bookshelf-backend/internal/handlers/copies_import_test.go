package handlers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
)

const validImportJSON = `[
  {"title": "Dune", "author": "Frank Herbert", "isbn": "9780441013593"},
  {"title": "", "author": "No Title"}
]`

func TestDecodeImportRows(t *testing.T) {
	t.Run("json round-trips the export shape", func(t *testing.T) {
		rows, err := decodeImportRows("json", `[{"title":"Dune","ol_key":"OL893415W"}]`)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, "Dune", rows[0].Title)
		require.Equal(t, "OL893415W", rows[0].OLKey)
	})

	t.Run("yaml", func(t *testing.T) {
		rows, err := decodeImportRows("yaml", "- title: Dune\n  author: Frank Herbert\n")
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, "Dune", rows[0].Title)
	})

	t.Run("csv maps columns by header name, tolerating reordering", func(t *testing.T) {
		csv := "author,title\nFrank Herbert,Dune\n"
		rows, err := decodeImportRows("csv", csv)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, "Dune", rows[0].Title)
		require.Equal(t, "Frank Herbert", rows[0].Author)
	})

	t.Run("unknown format is rejected", func(t *testing.T) {
		_, err := decodeImportRows("xml", "<a/>")
		require.Error(t, err)
	})

	t.Run("malformed json is rejected, not partially parsed", func(t *testing.T) {
		_, err := decodeImportRows("json", `[{"title": "Dune"`)
		require.Error(t, err)
	})

	t.Run("malformed csv is rejected", func(t *testing.T) {
		_, err := decodeImportRows("csv", "title,author\n\"unterminated")
		require.Error(t, err)
	})

	t.Run("too many rows is rejected", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("[")
		for i := range maxImportRows + 1 {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(`{"title":"Book"}`)
		}
		b.WriteString("]")
		_, err := decodeImportRows("json", b.String())
		require.Error(t, err)
	})
}

func TestSanitizeImportRow(t *testing.T) {
	row := sanitizeImportRow(exportRow{
		Title:  "  Dune  ",
		Status: "loaned", // must never survive sanitization
	})
	require.Equal(t, "Dune", row.Title)
	require.Empty(t, row.Status, "status from an untrusted file must never be trusted")
}

func TestCopyHandler_ClassifyImportRow(t *testing.T) {
	h, _, books, _ := newCopyHandler("")

	existing := models.Book{Title: "Dune", Author: "Frank Herbert", OLKey: "OL893415W"}
	require.NoError(t, books.Create(&existing))

	t.Run("missing title is skipped", func(t *testing.T) {
		plan := h.classifyImportRow(exportRow{Author: "No Title"})
		require.Equal(t, actionSkipped, plan.action)
		require.Equal(t, "missing title", plan.reason)
	})

	t.Run("matches an existing book by ol_key", func(t *testing.T) {
		plan := h.classifyImportRow(exportRow{Title: "Dune (reissue)", OLKey: "OL893415W"})
		require.Equal(t, actionMatchBook, plan.action)
		require.Equal(t, existing.ID, plan.book.ID)
	})

	t.Run("no matching key creates a new book", func(t *testing.T) {
		plan := h.classifyImportRow(exportRow{Title: "Some Other Book"})
		require.Equal(t, actionCreateBook, plan.action)
	})
}

func TestCopyHandler_PreviewImportBooks(t *testing.T) {
	h, _, _, _ := newCopyHandler("")

	t.Run("unauthenticated is unauthorized", func(t *testing.T) {
		_, err := h.previewImportBooks(fakeAuthedCtxNone(), &importInput{})
		require.Error(t, err)
		assertStatus(t, err, 401)
	})

	t.Run("invalid format is rejected", func(t *testing.T) {
		input := &importInput{}
		input.Body.Format = "xml"
		input.Body.Content = "irrelevant"
		_, err := h.previewImportBooks(fakeAuthedCtx(t, 1, "user"), input)
		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("plans rows without writing anything", func(t *testing.T) {
		input := &importInput{}
		input.Body.Format = "json"
		input.Body.Content = validImportJSON
		out, err := h.previewImportBooks(fakeAuthedCtx(t, 1, "user"), input)
		require.NoError(t, err)
		require.Len(t, out.Body.Rows, 2)
		require.Equal(t, actionCreateBook, out.Body.Rows[0].Action)
		require.Equal(t, actionSkipped, out.Body.Rows[1].Action)
		require.Equal(t, 1, out.Body.Summary.BooksCreated)
		require.Equal(t, 1, out.Body.Summary.Skipped)

		mine, err := h.copies.ListByOwnerID(1)
		require.NoError(t, err)
		require.Empty(t, mine, "preview must never persist anything")
	})
}

func TestCopyHandler_ImportBooks(t *testing.T) {
	t.Run("unauthenticated is unauthorized", func(t *testing.T) {
		h, _, _, _ := newCopyHandler("")
		_, err := h.importBooks(fakeAuthedCtxNone(), &importInput{})
		require.Error(t, err)
		assertStatus(t, err, 401)
	})

	t.Run("creates new books and copies owned by the caller, skipping invalid rows", func(t *testing.T) {
		h, copies, _, _ := newCopyHandler("")
		input := &importInput{}
		input.Body.Format = "json"
		input.Body.Content = validImportJSON

		out, err := h.importBooks(fakeAuthedCtx(t, 7, "user"), input)
		require.NoError(t, err)
		require.Equal(t, 1, out.Body.Summary.BooksCreated)
		require.Equal(t, 1, out.Body.Summary.CopiesCreated)
		require.Equal(t, 1, out.Body.Summary.Skipped)

		mine, err := copies.ListByOwnerID(7)
		require.NoError(t, err)
		require.Len(t, mine, 1)
		require.Equal(t, uint(7), mine[0].OwnerID)
		require.Equal(t, "available", mine[0].Status, "imported copies always start available, regardless of file content")
	})

	t.Run("status from the file is never trusted, even if set to loaned", func(t *testing.T) {
		h, copies, _, _ := newCopyHandler("")
		input := &importInput{}
		input.Body.Format = "json"
		input.Body.Content = `[{"title": "Dune", "status": "loaned"}]`

		_, err := h.importBooks(fakeAuthedCtx(t, 1, "user"), input)
		require.NoError(t, err)

		mine, err := copies.ListByOwnerID(1)
		require.NoError(t, err)
		require.Len(t, mine, 1)
		require.Equal(t, "available", mine[0].Status)
	})

	t.Run("matches an existing book by isbn and adds a new copy to it", func(t *testing.T) {
		h, copies, books, _ := newCopyHandler("")
		existing := models.Book{Title: "Dune", Author: "Frank Herbert", ISBN: "9780441013593"}
		require.NoError(t, books.Create(&existing))

		input := &importInput{}
		input.Body.Format = "json"
		input.Body.Content = `[{"title": "Dune", "isbn": "9780441013593"}]`

		out, err := h.importBooks(fakeAuthedCtx(t, 1, "user"), input)
		require.NoError(t, err)
		require.Equal(t, 1, out.Body.Summary.BooksMatched)
		require.Equal(t, 0, out.Body.Summary.BooksCreated)

		mine, err := copies.ListByOwnerID(1)
		require.NoError(t, err)
		require.Len(t, mine, 1)
		require.Equal(t, existing.ID, mine[0].BookID, "must attach to the existing book, not create a duplicate")
	})

	t.Run("one bad row does not block the rest of the batch", func(t *testing.T) {
		h, copies, _, _ := newCopyHandler("")
		input := &importInput{}
		input.Body.Format = "json"
		input.Body.Content = `[{"title": ""}, {"title": "Good Book"}]`

		out, err := h.importBooks(fakeAuthedCtx(t, 1, "user"), input)
		require.NoError(t, err)
		require.Equal(t, actionSkipped, out.Body.Rows[0].Action)
		require.Equal(t, actionCreateBook, out.Body.Rows[1].Action)

		mine, err := copies.ListByOwnerID(1)
		require.NoError(t, err)
		require.Len(t, mine, 1)
	})

	t.Run("stops adding copies once max_copies_per_user is reached", func(t *testing.T) {
		h, copies, _, _ := newCopyHandler("")
		require.NoError(t, h.admin.UpsertSetting("max_copies_per_user", "1"))

		input := &importInput{}
		input.Body.Format = "json"
		input.Body.Content = `[{"title": "First"}, {"title": "Second"}]`

		out, err := h.importBooks(fakeAuthedCtx(t, 1, "user"), input)
		require.NoError(t, err)
		require.Equal(t, actionCreateBook, out.Body.Rows[0].Action)
		require.Equal(t, actionSkipped, out.Body.Rows[1].Action)

		mine, err := copies.ListByOwnerID(1)
		require.NoError(t, err)
		require.Len(t, mine, 1)
	})

	t.Run("malformed file is rejected wholesale, not partially imported", func(t *testing.T) {
		h, copies, _, _ := newCopyHandler("")
		input := &importInput{}
		input.Body.Format = "json"
		input.Body.Content = `[{"title": "Dune"`

		_, err := h.importBooks(fakeAuthedCtx(t, 1, "user"), input)
		require.Error(t, err)
		assertStatus(t, err, 400)

		mine, err := copies.ListByOwnerID(1)
		require.NoError(t, err)
		require.Empty(t, mine)
	})
}
