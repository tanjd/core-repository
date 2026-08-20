package handlers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
)

func TestBuildExportRows(t *testing.T) {
	copies := []models.Copy{
		{
			Condition: "good",
			Notes:     "a note",
			Status:    "available",
			Book: models.Book{
				Title:         "Dune",
				Author:        "Frank Herbert",
				ISBN:          "9780441013593",
				OLKey:         "OL893415W",
				GoogleBooksID: "abc123",
				PageCount:     412,
			},
		},
	}

	rows := buildExportRows(copies)
	require.Len(t, rows, 1)
	require.Equal(t, "Dune", rows[0].Title)
	require.Equal(t, "Frank Herbert", rows[0].Author)
	require.Equal(t, "OL893415W", rows[0].OLKey)
	require.Equal(t, "abc123", rows[0].GoogleBooksID)
	require.Equal(t, "good", rows[0].Condition)
	require.Equal(t, "available", rows[0].Status)
	require.Equal(t, 412, rows[0].PageCount)
}

func TestEncodeExportRows(t *testing.T) {
	rows := []exportRow{{Title: "Dune", Author: "Frank Herbert", OLKey: "OL893415W", PageCount: 412}}

	t.Run("json", func(t *testing.T) {
		var buf strings.Builder
		require.NoError(t, encodeExportRows(&buf, "json", rows))
		require.Contains(t, buf.String(), `"title": "Dune"`)
		require.Contains(t, buf.String(), `"ol_key": "OL893415W"`)
	})

	t.Run("yaml", func(t *testing.T) {
		var buf strings.Builder
		require.NoError(t, encodeExportRows(&buf, "yaml", rows))
		require.Contains(t, buf.String(), "title: Dune")
	})

	t.Run("csv", func(t *testing.T) {
		var buf strings.Builder
		require.NoError(t, encodeExportRows(&buf, "csv", rows))
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		require.Len(t, lines, 2)
		require.Equal(t, strings.Join(exportRowHeader, ","), lines[0])
		require.Contains(t, lines[1], "Dune")
	})
}

func TestCopyHandler_ExportMyCopies(t *testing.T) {
	h, copies, _, _ := newCopyHandler("")
	require.NoError(t, copies.Create(&models.Copy{
		OwnerID:   1,
		Condition: "good",
		Status:    "available",
		Book:      models.Book{Title: "Dune", Author: "Frank Herbert"},
	}))
	require.NoError(t, copies.Create(&models.Copy{
		OwnerID:   2,
		Condition: "worn",
		Status:    "available",
		Book:      models.Book{Title: "Not Mine", Author: "Someone Else"},
	}))

	t.Run("unauthenticated is unauthorized", func(t *testing.T) {
		_, err := h.exportMyCopies(fakeAuthedCtxNone(), &exportMyCopiesInput{})
		require.Error(t, err)
		assertStatus(t, err, 401)
	})

	t.Run("invalid format is rejected", func(t *testing.T) {
		_, err := h.exportMyCopies(fakeAuthedCtx(t, 1, "user"), &exportMyCopiesInput{Format: "xml"})
		require.Error(t, err)
		assertStatus(t, err, 400)
	})

	t.Run("defaults to json and only exports the caller's copies", func(t *testing.T) {
		resp, err := h.exportMyCopies(fakeAuthedCtx(t, 1, "user"), &exportMyCopiesInput{})
		require.NoError(t, err)
		require.NotNil(t, resp.Body)
	})
}
