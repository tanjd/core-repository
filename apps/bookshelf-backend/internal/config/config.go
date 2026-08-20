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
	Port                    string   `env:"PORT" envDefault:"8000"`
	DBPath                  string   `env:"DB_PATH" envDefault:"./data/bookshelf.db"`
	JWTSecret               string   `env:"JWT_SECRET" envDefault:"dev-secret-change-me" sensitive:"true"`
	EncryptionSecret        string   `env:"ENCRYPTION_SECRET" sensitive:"true"`
	CORSOrigins             []string `env:"CORS_ORIGINS" envSeparator:"," envDefault:"http://localhost:3000"`
	FrontendOrigin          string   `env:"FRONTEND_ORIGIN" envDefault:"http://localhost:3000"`
	SMTPHost                string   `env:"SMTP_HOST"`
	SMTPPort                string   `env:"SMTP_PORT" envDefault:"587"`
	SMTPUsername            string   `env:"SMTP_USERNAME"`
	SMTPPassword            string   `env:"SMTP_PASSWORD" sensitive:"true"`
	EmailFrom               string   `env:"EMAIL_FROM" envDefault:"noreply@bookshelf.local"`
	DevEmailOverride        string   `env:"DEV_EMAIL_OVERRIDE"`
	Env                     string   `env:"ENV" envDefault:"dev"`
	GoogleBooksAPIKey       string   `env:"GOOGLE_BOOKS_API_KEY" sensitive:"true"`
	MetadataRefreshInterval string   `env:"METADATA_REFRESH_INTERVAL" envDefault:"24h"`
	AppConfigPath           string   `env:"APP_CONFIG_PATH" envDefault:"./bookshelf.yaml"`
}

// Load reads configuration from environment variables, applying envDefault
// tags where values are absent.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
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

// redact reports whether a sensitive string field is set, without revealing
// its value.
func redact(fv reflect.Value) string {
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
