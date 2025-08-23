package sqlite

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

// InitializeDatabase creates the SQLite database and runs the initial schema
func InitializeDatabase(dbPath string) error {
	// Ensure the directory exists
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// Create database connection
	db, err := NewSQLiteDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close database")
		}
	}()

	// Execute schema
	if _, err := db.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	log.Info().Str("path", dbPath).Msg("Database initialized successfully")
	return nil
}
