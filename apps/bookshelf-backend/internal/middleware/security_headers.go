package middleware

import "net/http"

// SecurityHeaders sets baseline defensive response headers on every request.
// CSP is intentionally omitted here — this backend is a JSON API plus
// /covers/ static file serving and huma's auto-generated /docs (Swagger UI,
// which needs inline scripts/styles); a meaningful CSP belongs on the
// frontend (apps/bookshelf/next.config.ts), the only HTML-rendering surface.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}
