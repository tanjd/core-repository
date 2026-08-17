package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// statusRecorder wraps http.ResponseWriter to capture the status code and
// bytes written, since net/http gives no way to read them back afterwards.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// newRequestID returns a short random hex ID for correlating one access-log
// line with a request; not cryptographically meaningful, just unique enough
// to grep by.
func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// RequestLogger logs one line per request (method, path, status, duration)
// after it completes, and attaches a request-scoped sub-logger (carrying
// request_id) to the request context so handlers and services downstream can
// log via zerolog.Ctx(ctx) and have their lines correlate with this one.
// Health checks are logged at Debug to keep routine polling out of the
// default log level; everything else is leveled by status code (5xx →
// Error, 4xx → Warn, else → Info).
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := newRequestID()
		w.Header().Set("X-Request-Id", id)

		sublogger := log.With().Str("request_id", id).Logger()
		r = r.WithContext(sublogger.WithContext(r.Context()))

		rec := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(rec, r)

		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}

		var level zerolog.Level
		switch {
		case r.URL.Path == "/health":
			level = zerolog.DebugLevel
		case status >= 500:
			level = zerolog.ErrorLevel
		case status >= 400:
			level = zerolog.WarnLevel
		default:
			level = zerolog.InfoLevel
		}

		sublogger.WithLevel(level).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", status).
			Int("bytes", rec.bytes).
			Dur("duration", time.Since(start)).
			Msg("request")
	})
}
