package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"gopkg.in/yaml.v3"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
)

// exportRow is one row of an owned-books export — a flattened Book+Copy,
// shaped the same way regardless of output format.
type exportRow struct {
	Title         string `json:"title" yaml:"title"`
	Author        string `json:"author" yaml:"author"`
	ISBN          string `json:"isbn" yaml:"isbn"`
	OLKey         string `json:"ol_key" yaml:"ol_key"`
	GoogleBooksID string `json:"google_books_id" yaml:"google_books_id"`
	Publisher     string `json:"publisher" yaml:"publisher"`
	PublishedDate string `json:"published_date" yaml:"published_date"`
	PageCount     int    `json:"page_count" yaml:"page_count"`
	Language      string `json:"language" yaml:"language"`
	Condition     string `json:"condition" yaml:"condition"`
	Notes         string `json:"notes" yaml:"notes"`
	Status        string `json:"status" yaml:"status"`
}

// exportRowHeader/exportRowValues keep the CSV column order in sync with
// exportRow's fields without relying on struct-tag reflection (encoding/csv
// has no tag support of its own). ol_key/google_books_id ride along
// specifically so an import can dedup against an existing catalog using the
// same OLKey/GoogleBooksID-first precedence BookHandler.findExistingBook
// already uses for manual "add a book" — ISBN alone can be shared across
// distinct editions, so it's the weakest of the three signals.
var exportRowHeader = []string{
	"title", "author", "isbn", "ol_key", "google_books_id", "publisher",
	"published_date", "page_count", "language", "condition", "notes", "status",
}

func (r exportRow) exportRowValues() []string {
	return []string{
		r.Title, r.Author, r.ISBN, r.OLKey, r.GoogleBooksID, r.Publisher, r.PublishedDate,
		strconv.Itoa(r.PageCount), r.Language, r.Condition, r.Notes, r.Status,
	}
}

// buildExportRows flattens owned copies (with their preloaded Book) into
// export rows, one per copy.
func buildExportRows(copies []models.Copy) []exportRow {
	rows := make([]exportRow, len(copies))
	for i, c := range copies {
		rows[i] = exportRow{
			Title:         c.Book.Title,
			Author:        c.Book.Author,
			ISBN:          c.Book.ISBN,
			OLKey:         c.Book.OLKey,
			GoogleBooksID: c.Book.GoogleBooksID,
			Publisher:     c.Book.Publisher,
			PublishedDate: c.Book.PublishedDate,
			PageCount:     c.Book.PageCount,
			Language:      c.Book.Language,
			Condition:     c.Condition,
			Notes:         c.Notes,
			Status:        c.Status,
		}
	}
	return rows
}

// exportFormats maps a validated `format` query value to its content type
// and download filename.
var exportFormats = map[string]struct{ contentType, filename string }{
	"json": {"application/json", "my-books.json"},
	"yaml": {"application/yaml", "my-books.yaml"},
	"csv":  {"text/csv", "my-books.csv"},
}

// encodeExportRows writes rows to w in the given format. format must already
// be validated against exportFormats.
func encodeExportRows(w io.Writer, format string, rows []exportRow) error {
	switch format {
	case "yaml":
		return yaml.NewEncoder(w).Encode(rows)
	case "csv":
		cw := csv.NewWriter(w)
		if err := cw.Write(exportRowHeader); err != nil {
			return err
		}
		for _, r := range rows {
			if err := cw.Write(r.exportRowValues()); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	default:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}
}

type exportMyCopiesInput struct {
	Format string `query:"format" doc:"Export format: json (default), yaml, or csv"`
}

func (h *CopyHandler) exportMyCopies(ctx context.Context, input *exportMyCopiesInput) (*huma.StreamResponse, error) {
	ownerID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	format := input.Format
	if format == "" {
		format = "json"
	}
	spec, ok := exportFormats[format]
	if !ok {
		return nil, huma.Error400BadRequest("format must be one of: json, yaml, csv")
	}

	copies, err := h.copies.ListByOwnerID(ownerID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch copies")
	}
	rows := buildExportRows(copies)

	return &huma.StreamResponse{
		Body: func(sctx huma.Context) {
			sctx.SetHeader("Content-Type", spec.contentType)
			sctx.SetHeader("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, spec.filename))
			_ = encodeExportRows(sctx.BodyWriter(), format, rows)
		},
	}, nil
}
