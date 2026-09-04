// Package config provides application configuration loaded from environment variables.
package config

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/caarlos0/env/v11"
)

// Config holds all application configuration loaded from environment variables.
// Fields tagged sensitive:"true" hold secrets and must never be logged in the
// clear — see LogFields.
type Config struct {
	Port             string   `env:"PORT" envDefault:"8000"`
	DBPath           string   `env:"DB_PATH" envDefault:"./data/bookshelf.db"`
	JWTSecret        string   `env:"JWT_SECRET" envDefault:"dev-secret-change-me" sensitive:"true"`
	EncryptionSecret string   `env:"ENCRYPTION_SECRET" envDefault:"dev-encryption-secret-change-me" sensitive:"true"`
	CORSOrigins      []string `env:"CORS_ORIGINS" envSeparator:"," envDefault:"http://localhost:3000"`
	FrontendOrigin   string   `env:"FRONTEND_ORIGIN" envDefault:"http://localhost:3000"`
	SMTPHost         string   `env:"SMTP_HOST"`
	SMTPPort         string   `env:"SMTP_PORT" envDefault:"587"`
	SMTPUsername     string   `env:"SMTP_USERNAME"`
	SMTPPassword     string   `env:"SMTP_PASSWORD" sensitive:"true"`
	EmailFrom        string   `env:"EMAIL_FROM" envDefault:"noreply@bookshelf.local"`
	DevEmailOverride string   `env:"DEV_EMAIL_OVERRIDE"`
	Env              string   `env:"ENV" envDefault:"dev"`
	// GoogleBooksAPIKeys is the shared pool of server-wide Google Books API
	// keys, comma-separated. A single value works exactly as the old
	// singular key did; adding more lets requests round-robin across them
	// (services.GoogleBooksKeyPool) to spread load across each key's
	// separate free-tier quota instead of hitting one key's rate limit.
	GoogleBooksAPIKeys      []string `env:"GOOGLE_BOOKS_API_KEY" envSeparator:"," sensitive:"true"`
	MetadataRefreshInterval string   `env:"METADATA_REFRESH_INTERVAL" envDefault:"24h"`
	AppConfigPath           string   `env:"APP_CONFIG_PATH" envDefault:"./bookshelf.yaml"`
	// RegisterRateLimitBurst overrides registerLimiter's burst size
	// (internal/handlers/auth.go) — the default (20) is sized for real
	// community usage. The e2e suite (apps/bookshelf-e2e/playwright.config.ts)
	// raises this substantially: dozens of spec files register accounts
	// concurrently against one shared server, well beyond anything a real
	// community would do in the same window.
	RegisterRateLimitBurst int `env:"REGISTER_RATE_LIMIT_BURST" envDefault:"20"`
	// RegisterSendRateLimitBurst overrides registerSendLimiter's burst size
	// (internal/handlers/auth.go) — the default (30) is sized for real
	// community usage. The e2e suite raises this for the same reason as
	// RegisterRateLimitBurst above: send-email-otp is the first thing every
	// spec that registers its own account hits, and with ~13 spec files ×
	// 2 browser projects registering at roughly the same suite-start
	// instant the default burst was routinely exhausted, producing 429s
	// against tests that had no retry loop of their own.
	RegisterSendRateLimitBurst int `env:"REGISTER_SEND_RATE_LIMIT_BURST" envDefault:"30"`
	// LoginRateLimitAttempts overrides loginLimiter's per-email attempt cap
	// (internal/handlers/auth.go) — the default (5 per 15min) is sized to
	// resist password brute-forcing against one real account. The e2e suite
	// reuses a small, fixed set of seeded accounts across many spec files
	// (loan-request-flow.spec.ts alone logs the same borrower in six times
	// in one run), so a single account can legitimately exceed 5 UI logins
	// well before any brute-force pattern would.
	LoginRateLimitAttempts int `env:"LOGIN_RATE_LIMIT_ATTEMPTS" envDefault:"5"`
	// TelegramBotToken authenticates outbound calls to the Telegram Bot API
	// (services.TelegramService). Empty means Telegram notifications are
	// disabled — same "unset means skip" contract as SMTPHost for email.
	TelegramBotToken string `env:"TELEGRAM_BOT_TOKEN" sensitive:"true"`
	// TelegramBotUsername (no leading @) is returned to the frontend by
	// POST /profile/telegram/link-token, which builds the t.me deep link
	// from it. Deliberately a runtime env var read server-side, not a
	// NEXT_PUBLIC_ Next.js var — the frontend Docker image is built once and
	// reused across deployments (see apps/bookshelf/CLAUDE.md's
	// BACKEND_URL note), and NEXT_PUBLIC_ values get baked in at that build
	// step rather than read at container startup, which would break that.
	TelegramBotUsername string `env:"TELEGRAM_BOT_USERNAME"`
	// TelegramInternalSecret guards POST /internal/telegram/confirm-link,
	// the one endpoint apps/bookshelf-bot calls directly (bot->backend, not
	// user-facing) — see docs/telegram-bot-integration-spec.md. Must match
	// the bot's BOOKSHELF_INTERNAL_TOKEN.
	TelegramInternalSecret string `env:"TELEGRAM_INTERNAL_SECRET" sensitive:"true"`
	// TelegramBotHealthURL, if set, is polled by the admin Jobs page to show
	// whether apps/bookshelf-bot's own process is up (distinct from whether
	// Telegram push delivery works, which needs only TelegramBotToken).
	// Local dev: http://localhost:8080/health. Empty means "not configured"
	// — the admin UI shows that rather than a false "offline".
	TelegramBotHealthURL string `env:"TELEGRAM_BOT_HEALTH_URL"`
}

// Load reads configuration from environment variables, applying envDefault
// tags where values are absent.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.GoogleBooksAPIKeys = cleanKeys(cfg.GoogleBooksAPIKeys)
	return cfg, nil
}

// cleanKeys trims whitespace around each comma-separated entry and drops
// empty ones (e.g. a trailing comma, or "key1, key2" with a space after the
// comma) so callers never have to defensively re-check.
func cleanKeys(keys []string) []string {
	cleaned := make([]string, 0, len(keys))
	for _, k := range keys {
		if k = strings.TrimSpace(k); k != "" {
			cleaned = append(cleaned, k)
		}
	}
	return cleaned
}

// LogFields flattens every Config field into a map keyed by its lowercased
// env var name, redacting fields tagged sensitive:"true" — so logging the
// full config (e.g. at startup) never leaks a secret, including ones added
// to Config later without a matching update to whatever logs it.
func (c *Config) LogFields() map[string]any {
	v := reflect.ValueOf(*c)
	t := v.Type()

	fields := make(map[string]any, t.NumField())
	for i := range t.NumField() {
		field := t.Field(i)
		name := strings.ToLower(strings.SplitN(field.Tag.Get("env"), ",", 2)[0])

		if field.Tag.Get("sensitive") == "true" {
			fields[name] = redact(v.Field(i))
			continue
		}
		fields[name] = displayValue(v.Field(i))
	}
	return fields
}

// redact reports whether a sensitive field is set, without revealing its
// value(s) — a sensitive slice (e.g. GoogleBooksAPIKeys) reports how many
// entries it holds rather than "(unset)"/"***REDACTED***", since knowing the
// pool size is useful for diagnosing rate-limit issues and reveals nothing
// about the keys themselves.
func redact(fv reflect.Value) string {
	if fv.Kind() == reflect.Slice {
		if fv.Len() == 0 {
			return "(unset)"
		}
		return fmt.Sprintf("***REDACTED*** (%d keys)", fv.Len())
	}
	if fv.String() == "" {
		return "(unset)"
	}
	return "***REDACTED***"
}

// displayValue renders a non-sensitive field for logging, joining slices
// (e.g. CORSOrigins) into a single readable string.
func displayValue(fv reflect.Value) any {
	if fv.Kind() != reflect.Slice {
		return fv.Interface()
	}
	parts := make([]string, fv.Len())
	for i := range parts {
		parts[i] = fmt.Sprint(fv.Index(i).Interface())
	}
	return strings.Join(parts, ", ")
}
