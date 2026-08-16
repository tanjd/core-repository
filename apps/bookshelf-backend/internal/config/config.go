// Package config provides application configuration loaded from environment variables.
package config

import (
	"os"
	"strings"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Port                    string
	DBPath                  string
	JWTSecret               string
	EncryptionSecret        string
	CORSOrigins             []string
	ResendAPIKey            string
	EmailFrom               string
	DevEmailOverride        string
	Env                     string
	GoogleBooksAPIKey       string
	MetadataRefreshInterval string
	AppConfigPath           string
}

// Load reads configuration from environment variables, applying defaults where
// values are absent.
func Load() *Config {
	return &Config{
		Port:                    getEnv("PORT", "8000"),
		DBPath:                  getEnv("DB_PATH", "./data/bookshelf.db"),
		JWTSecret:               getEnv("JWT_SECRET", "dev-secret-change-me"),
		EncryptionSecret:        getEnv("ENCRYPTION_SECRET", ""),
		CORSOrigins:             strings.Split(getEnv("CORS_ORIGINS", "http://localhost:3000"), ","),
		ResendAPIKey:            getEnv("RESEND_API_KEY", ""),
		EmailFrom:               getEnv("EMAIL_FROM", "noreply@bookshelf.local"),
		DevEmailOverride:        getEnv("DEV_EMAIL_OVERRIDE", ""),
		Env:                     getEnv("ENV", "dev"),
		GoogleBooksAPIKey:       getEnv("GOOGLE_BOOKS_API_KEY", ""),
		MetadataRefreshInterval: getEnv("METADATA_REFRESH_INTERVAL", "24h"),
		AppConfigPath:           getEnv("APP_CONFIG_PATH", "./bookshelf.yaml"),
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
