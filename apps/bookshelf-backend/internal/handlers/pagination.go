package handlers

// paginationParams is embedded into list-endpoint input structs to share the page/page_size
// query params and their huma doc tags across handlers.
type paginationParams struct {
	Page     int `query:"page" minimum:"1" doc:"Page number (default 1)"`
	PageSize int `query:"page_size" minimum:"1" maximum:"100" doc:"Items per page"`
}

// normalize clamps Page/PageSize to their defaults when unset (huma leaves them at their zero
// value rather than applying "default N" from the doc tag, since these aren't declared with a
// `default:` tag).
func (p paginationParams) normalize(defaultPageSize int) (page, pageSize int) {
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
