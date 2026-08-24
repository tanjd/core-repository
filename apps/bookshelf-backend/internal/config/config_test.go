package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// configEnvKeys lists every env var Config reads. Tests use it to force a
// clean slate — the devcontainer/CI environment may already export some of
// these (e.g. a real JWT_SECRET for docker-compose), and t.Setenv(key, "")
// reliably falls back to envDefault regardless of what's ambient.
var configEnvKeys = []string{
	"PORT", "DB_PATH", "JWT_SECRET", "ENCRYPTION_SECRET", "CORS_ORIGINS",
	"SMTP_HOST", "SMTP_PORT", "SMTP_USERNAME", "SMTP_PASSWORD", "EMAIL_FROM",
	"DEV_EMAIL_OVERRIDE", "ENV", "GOOGLE_BOOKS_API_KEY", "METADATA_REFRESH_INTERVAL",
	"APP_CONFIG_PATH", "REGISTER_RATE_LIMIT_BURST",
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range configEnvKeys {
		t.Setenv(key, "")
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	clearConfigEnv(t)

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, "8000", cfg.Port)
	assert.Equal(t, "dev-secret-change-me", cfg.JWTSecret)
	assert.Equal(t, []string{"http://localhost:3000"}, cfg.CORSOrigins)
}

func TestLoadReadsEnv(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("PORT", "9000")
	t.Setenv("CORS_ORIGINS", "https://a.example.com,https://b.example.com")

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, "9000", cfg.Port)
	assert.Equal(t, []string{"https://a.example.com", "https://b.example.com"}, cfg.CORSOrigins)
}

func TestLogFieldsRedactsSensitiveValues(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("JWT_SECRET", "super-secret-token")
	t.Setenv("SMTP_PASSWORD", "hunter2")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	cfg, err := Load()
	require.NoError(t, err)

	fields := cfg.LogFields()

	assert.Equal(t, "***REDACTED***", fields["jwt_secret"], "secret value must never appear in logs")
	assert.Equal(t, "***REDACTED***", fields["smtp_password"])
	assert.Equal(t, "(unset)", fields["encryption_secret"], "unset secrets should say so without a fake value")
	assert.Equal(t, "smtp.example.com", fields["smtp_host"], "non-sensitive fields log their real value")
}

func TestLogFieldsJoinsSliceFields(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("CORS_ORIGINS", "https://a.example.com,https://b.example.com")
	cfg, err := Load()
	require.NoError(t, err)

	fields := cfg.LogFields()

	assert.Equal(t, "https://a.example.com, https://b.example.com", fields["cors_origins"])
}
