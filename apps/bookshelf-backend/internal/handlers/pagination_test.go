package handlers

import (
	"context"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
)

// TestPaginationParams_BindsFromQueryString is a regression test for a real
// production bug: PaginationParams used to be named paginationParams
// (unexported). Go's reflect.StructField.IsExported() treats an anonymously
// embedded field as unexported whenever its *type* name is unexported, even
// though Page/PageSize are themselves capitalized — so huma's field walker
// silently skipped it and Page/PageSize always bound to their zero value, no
// matter what the request's query string said. Every list endpoint
// (books, admin users, announcements, loan requests, notifications,
// wishlist) always served page 1 regardless of ?page=N.
//
// A direct-call test like TestListBooks_Unauthenticated above doesn't catch
// this: constructing listBooksInput{} in Go and setting .Page by hand always
// works via normal field promotion, exported or not. Only a real request
// through huma's query-binding path (via humatest here) exercises the
// reflection code that broke.
func TestPaginationParams_BindsFromQueryString(t *testing.T) {
	type input struct {
		PaginationParams
	}

	_, api := humatest.New(t)
	huma.Register(api, huma.Operation{
		OperationID: "test-list",
		Method:      "GET",
		Path:        "/items",
	}, func(_ context.Context, in *input) (*struct{}, error) {
		page, pageSize := in.normalize(20)
		assert.Equal(t, 4, page)
		assert.Equal(t, 50, pageSize)
		return nil, nil
	})

	resp := api.Get("/items?page=4&page_size=50")
	assert.Equal(t, 204, resp.Code)
}
