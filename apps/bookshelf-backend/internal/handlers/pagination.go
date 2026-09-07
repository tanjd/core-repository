package handlers

// PaginationParams is embedded into list-endpoint input structs to share the page/page_size
// query params and their huma doc tags across handlers.
//
// Must be exported: an anonymously-embedded field is only treated as exported (and so only
// recursed into) by huma's reflection-based field walker (findInType in huma.go) when its type
// name is exported too — an unexported embedded struct's fields never get bound to query
// params, no matter what tags they carry. Renaming this from paginationParams (its name at
// introduction) fixed a real production regression: every list-endpoint request always saw
// Page/PageSize as their zero value, so pagination silently defaulted to page 1 forever.
type PaginationParams struct {
	Page     int `query:"page" minimum:"1" doc:"Page number (default 1)"`
	PageSize int `query:"page_size" minimum:"1" maximum:"100" doc:"Items per page"`
}

// normalize clamps Page/PageSize to their defaults when unset (huma leaves them at their zero
// value rather than applying "default N" from the doc tag, since these aren't declared with a
// `default:` tag).
func (p PaginationParams) normalize(defaultPageSize int) (page, pageSize int) {
	page = p.Page
	if page < 1 {
		page = 1
	}
	pageSize = p.PageSize
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	return page, pageSize
}
