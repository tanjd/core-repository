package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/danielgtaylor/huma/v2/humacli"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/tanjd/core-repository/apps/food-maps-backend/api"
	"github.com/tanjd/core-repository/apps/food-maps-backend/handler"
	"github.com/tanjd/core-repository/apps/food-maps-backend/repository/sqlite"
	"github.com/tanjd/core-repository/apps/food-maps-backend/service"

	_ "github.com/danielgtaylor/huma/v2/formats/cbor"
)

type Options struct {
	Port      int    `help:"Port to listen on" short:"p" default:"8080"`
	DBPath    string `help:"Path to SQLite database" default:"data/food-maps.db"`
	LogLevel  string `help:"Log level (debug, info, warn, error)" default:"debug"`
	LogFormat string `help:"Log format (json, console)" default:"console"`
}

func main() {
	cli := humacli.New(func(hooks humacli.Hooks, options *Options) {
		// Configure logging
		level, err := zerolog.ParseLevel(options.LogLevel)
		if err != nil {
			log.Fatal().Err(err).Msg("Invalid log level")
		}
		zerolog.SetGlobalLevel(level)

		if options.LogFormat == "console" {
			log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		}

		// Initialize database
		if err := sqlite.InitializeDatabase(options.DBPath); err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize database")
		}

		// Connect to database
		db, err := sqlite.NewSQLiteDB(options.DBPath)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to connect to database")
		}

		// Initialize services
		locationService := service.NewLocationService(db)

		// Initialize handlers
		locationHandler := handler.NewLocationHandler(locationService)

		// Create router with middleware
		router := chi.NewMux()
		router.Use(middleware.Logger)
		router.Use(middleware.Recoverer)

		// Create API with Chi adapter
		humaAPI := humachi.New(router, huma.DefaultConfig("Food Maps API", "1.0.0"))

		// Initialize and add routes
		routes := api.NewRouter(locationHandler, humaAPI)
		routes.AddLocationRoutes()

		// Create server
		srv := &http.Server{
			Addr:    fmt.Sprintf(":%d", options.Port),
			Handler: router,
		}

		// Channel to signal server shutdown
		shutdown := make(chan struct{})

		hooks.OnStart(func() {
			log.Info().
				Str("db_path", options.DBPath).
				Int("port", options.Port).
				Str("log_level", options.LogLevel).
				Str("log_format", options.LogFormat).
				Msg("Starting server...")

			// Start server in a goroutine
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Fatal().Err(err).Msg("Server failed")
				}
			}()

			// Wait for interrupt signal
			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
			<-quit

			log.Info().Msg("Shutting down server...")

			// Create shutdown context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Shutdown server
			if err := srv.Shutdown(ctx); err != nil {
				log.Error().Err(err).Msg("Server forced to shutdown")
			}

			// Close database connection
			if err := db.Close(); err != nil {
				log.Error().Err(err).Msg("Failed to close database")
			}

			close(shutdown)
		})

		hooks.OnStop(func() {
			// Wait for server to shutdown
			<-shutdown
			log.Info().Msg("Server stopped")
		})
	})

	cli.Run()
}
