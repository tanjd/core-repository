// Package main is the entry point for the bookshelf API server.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/config"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/db"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/handlers"
	appmiddleware "github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
	gormrepo "github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository/gorm"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/services"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	// Load .env if present. No-op in production where vars are injected by the runtime.
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load configuration")
	}

	// Configure logger: pretty console output in dev, structured JSON in production.
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if cfg.Env != "prd" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	}
	// zerolog.Ctx(ctx) falls back to this when ctx carries no request-scoped
	// sub-logger (e.g. the scheduler's background ctx), instead of a disabled
	// no-op logger.
	zerolog.DefaultContextLogger = &log.Logger

	// Log the full startup config — LogFields redacts anything tagged
	// sensitive:"true" on Config, so this never leaks a secret.
	log.Info().
		Str("version", version).
		Fields(cfg.LogFields()).
		Msg("bookshelf starting")
	if cfg.JWTSecret == "dev-secret-change-me" {
		if cfg.Env != "dev" {
			log.Fatal().Msg("JWT_SECRET must be set to a non-default value when ENV is not \"dev\" — anyone who reads this public repo knows the default and can forge admin tokens")
		}
		log.Warn().Msg("JWT_SECRET is set to the default value — change it before deploying to production")
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open database")
	}

	db.Seed(database)

	coversDir := "./data/covers"
	if err := os.MkdirAll(coversDir, 0o750); err != nil {
		log.Fatal().Err(err).Msg("failed to create covers dir")
	}

	// Repositories — only this layer and db.Open ever import gorm.io/gorm.
	userRepo := gormrepo.NewUserRepository(database)
	bookRepo := gormrepo.NewBookRepository(database)
	copyRepo := gormrepo.NewCopyRepository(database)
	loanRepo := gormrepo.NewLoanRequestRepository(database)
	notifRepo := gormrepo.NewNotificationRepository(database)
	adminRepo := gormrepo.NewAdminRepository(database)
	waitlistRepo := gormrepo.NewWaitlistRepository(database)
	regVerificationRepo := gormrepo.NewRegistrationVerificationRepository(database)

	// Services
	emailSvc := services.NewEmailService(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.EmailFrom, cfg.Env, cfg.DevEmailOverride)
	smsSvc := services.NewMockSMSService()
	workflow := services.NewLoanWorkflow(copyRepo, loanRepo, notifRepo, userRepo, waitlistRepo, emailSvc)
	scheduler := services.NewScheduler(bookRepo, adminRepo, coversDir, cfg.MetadataRefreshInterval)

	seedYAMLConfig(cfg.AppConfigPath, adminRepo)

	// Create the root context early so it can be passed to handlers that run
	// background goroutines (e.g. the metadata cache eviction loop).
	ctx, cancel := context.WithCancel(context.Background())

	// Resolve encryption secret: use ENCRYPTION_SECRET if set, otherwise fall
	// back to JWT_SECRET so existing deployments keep working unchanged.
	encryptionSecret := cfg.EncryptionSecret
	if encryptionSecret == "" {
		encryptionSecret = cfg.JWTSecret
	}

	// Handlers
	authH := handlers.NewAuthHandler(userRepo, adminRepo, copyRepo, regVerificationRepo, cfg.JWTSecret, encryptionSecret, emailSvc, smsSvc, cfg.Env)
	metadataH := handlers.NewMetadataHandler(ctx, cfg.GoogleBooksAPIKey, encryptionSecret, userRepo)
	bookH := handlers.NewBookHandler(bookRepo, userRepo, coversDir)
	copyH := handlers.NewCopyHandler(copyRepo, userRepo, notifRepo, waitlistRepo, adminRepo)
	loanH := handlers.NewLoanRequestHandler(copyRepo, loanRepo, adminRepo, userRepo, workflow)
	notifH := handlers.NewNotificationHandler(notifRepo)
	adminH := handlers.NewAdminHandler(adminRepo, cfg.GoogleBooksAPIKey)
	jobsH := handlers.NewJobsHandler(scheduler)
	waitlistH := handlers.NewWaitlistHandler(copyRepo, waitlistRepo)

	// Router
	mux := http.NewServeMux()

	// Static file serving for locally cached book covers. Directory-listing
	// requests (trailing slash, e.g. GET /covers/) are rejected with 404
	// rather than falling through to http.FileServer's default listing.
	coverFS := http.StripPrefix("/covers/", http.FileServer(http.Dir(coversDir)))
	mux.Handle("/covers/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		// Filenames are content-addressed (SHA-256 of the source URL), so a
		// given path's content never changes — safe to cache indefinitely.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		coverFS.ServeHTTP(w, r)
	}))

	// Health check (plain net/http, outside huma)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","version":"` + version + `"}`))
	})

	// Huma API — auto-generates OpenAPI spec at /openapi.yaml + /openapi.json
	// and serves interactive docs at /docs
	apiConfig := huma.DefaultConfig("Bookshelf API", version)
	apiConfig.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearer": {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
		},
	}
	api := humago.New(mux, apiConfig)

	// Register routes
	authH.RegisterRoutes(api)
	metadataH.RegisterRoutes(api)
	bookH.RegisterRoutes(api)
	copyH.RegisterRoutes(api)
	loanH.RegisterRoutes(api)
	notifH.RegisterRoutes(api)
	adminH.RegisterRoutes(api)
	jobsH.RegisterRoutes(api)
	waitlistH.RegisterRoutes(api)

	// Middleware chain: security headers → request logging → CORS → auth enrichment → mux
	corsHandler := cors.New(cors.Options{
		AllowedOrigins: cfg.CORSOrigins,
		AllowedMethods: []string{
			http.MethodGet, http.MethodPost, http.MethodPatch,
			http.MethodPut, http.MethodDelete, http.MethodOptions,
		},
		AllowedHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
	})

	handler := appmiddleware.SecurityHeaders(appmiddleware.RequestLogger(corsHandler.Handler(appmiddleware.SetAuth(cfg.JWTSecret)(appmiddleware.RequireActiveUser(userRepo)(mux)))))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start the background scheduler.
	var wg sync.WaitGroup
	wg.Go(func() { scheduler.Start(ctx) })

	// Graceful shutdown on SIGINT/SIGTERM.
	done := make(chan struct{})
	go waitForShutdownSignal(cancel, srv, database, &wg, done)

	log.Info().Str("port", cfg.Port).Msg("listening")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		cancel()
		log.Fatal().Err(err).Msg("server failed")
	}
	<-done
}

// seedYAMLConfig seeds settings from bookshelf.yaml if it exists (YAML values override DB defaults).
func seedYAMLConfig(path string, adminRepo repository.AdminRepository) {
	kvs, yamlErr := handlers.LoadYAMLConfig(path)
	if yamlErr != nil {
		log.Warn().Err(yamlErr).Str("path", path).Msg("could not parse bookshelf.yaml — using DB defaults")
		return
	}
	if len(kvs) == 0 {
		return
	}
	for k, v := range kvs {
		if upsertErr := adminRepo.UpsertSetting(k, v); upsertErr != nil {
			log.Warn().Err(upsertErr).Str("key", k).Msg("could not apply YAML setting")
		}
	}
	log.Info().Str("path", path).Int("keys", len(kvs)).Msg("seeded settings from YAML")
}

// waitForShutdownSignal blocks until SIGINT/SIGTERM, then gracefully shuts down
// srv, waits for background goroutines tracked by wg to exit, and closes database.
// Closes done as its last step so the caller can block until shutdown is complete.
func waitForShutdownSignal(cancel context.CancelFunc, srv *http.Server, database *gorm.DB, wg *sync.WaitGroup, done chan<- struct{}) {
	defer close(done)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("shutting down server")
	cancel()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	if shutErr := srv.Shutdown(shutCtx); shutErr != nil {
		log.Error().Err(shutErr).Msg("server shutdown error")
	}

	wg.Wait()

	sqlDB, err := database.DB()
	if err != nil {
		log.Error().Err(err).Msg("failed to get underlying sql.DB for shutdown")
		return
	}
	if err := sqlDB.Close(); err != nil {
		log.Error().Err(err).Msg("database close error")
	}
}
