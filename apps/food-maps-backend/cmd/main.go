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
		if err := configureLogging(options); err != nil {
			log.Fatal().Err(err).Msg("Invalid log level")
		}

		db, err := setupDatabase(options.DBPath)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to set up database")
		}

		srv := buildServer(db, options)

		registerLifecycleHooks(hooks, srv, db, options)
	})

	cli.Run()
}

func configureLogging(options *Options) error {
	level, err := zerolog.ParseLevel(options.LogLevel)
	if err != nil {
		return err
	}
	zerolog.SetGlobalLevel(level)

	if options.LogFormat == "console" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	return nil
}

func setupDatabase(dbPath string) (*sqlite.SQLiteDB, error) {
	if err := sqlite.InitializeDatabase(dbPath); err != nil {
		return nil, err
	}

	return sqlite.NewSQLiteDB(dbPath)
}

func buildServer(db *sqlite.SQLiteDB, options *Options) *http.Server {
	locationService := service.NewLocationService(db)
	locationHandler := handler.NewLocationHandler(locationService)

	router := chi.NewMux()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	humaAPI := humachi.New(router, huma.DefaultConfig("Food Maps API", "1.0.0"))
	routes := api.NewRouter(locationHandler, humaAPI)
	routes.AddLocationRoutes()

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", options.Port),
		Handler: router,
	}
}

func registerLifecycleHooks(hooks humacli.Hooks, srv *http.Server, db *sqlite.SQLiteDB, options *Options) {
	shutdown := make(chan struct{})

	hooks.OnStart(func() {
		log.Info().
			Str("db_path", options.DBPath).
			Int("port", options.Port).
			Str("log_level", options.LogLevel).
			Str("log_format", options.LogFormat).
			Msg("Starting server...")

		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatal().Err(err).Msg("Server failed")
			}
		}()

		waitForShutdownSignal()

		log.Info().Msg("Shutting down server...")
		shutdownServer(srv, db)
		close(shutdown)
	})

	hooks.OnStop(func() {
		<-shutdown
		log.Info().Msg("Server stopped")
	})
}

func waitForShutdownSignal() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}

func shutdownServer(srv *http.Server, db *sqlite.SQLiteDB) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}

	if err := db.Close(); err != nil {
		log.Error().Err(err).Msg("Failed to close database")
	}
}
