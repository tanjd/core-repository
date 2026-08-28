// Package main is the entry point for the bookshelf API server.
package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	requireNonDefaultSecret(cfg.Env, "JWT_SECRET", cfg.JWTSecret, "dev-secret-change-me", "forge admin tokens")
	requireNonDefaultSecret(cfg.Env, "ENCRYPTION_SECRET", cfg.EncryptionSecret, "dev-encryption-secret-change-me", "decrypt stored secrets")

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open database")
	}

	db.Seed(database)

	coversDir := "./data/covers"
	if err := os.MkdirAll(coversDir, 0o750); err != nil {
		log.Fatal().Err(err).Msg("failed to create covers dir")
	}

	backupsDir := "./data/backups"
	if err := os.MkdirAll(backupsDir, 0o750); err != nil {
		log.Fatal().Err(err).Msg("failed to create backups dir")
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
	announcementRepo := gormrepo.NewAnnouncementRepository(database)
	wishlistRepo := gormrepo.NewWishlistRequestRepository(database)
	recommendationRepo := gormrepo.NewRecommendationRepository(database)

	// Services
	emailSvc := services.NewEmailService(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.EmailFrom, cfg.Env, cfg.DevEmailOverride, cfg.FrontendOrigin)
	smsSvc := services.NewMockSMSService()
	workflow := services.NewLoanWorkflow(copyRepo, loanRepo, notifRepo, userRepo, waitlistRepo, emailSvc)
	wishlistWorkflow := services.NewWishlistWorkflow(wishlistRepo, notifRepo, userRepo, emailSvc)
	registrationWorkflow := services.NewRegistrationWorkflow(adminRepo, notifRepo, bookRepo, emailSvc)

	sqlDB, err := database.DB()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get underlying sql.DB")
	}
	schemaVersion, err := db.MigrationVersion(sqlDB)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read schema migration version")
	}
	backupSvc := services.NewBackupService(sqlDB, adminRepo, cfg.DBPath, coversDir, backupsDir)
	googleBooksKeyPool := services.NewGoogleBooksKeyPool(cfg.GoogleBooksAPIKeys)
	descriptionReconciliationSvc := services.NewDescriptionReconciliationService(bookRepo, googleBooksKeyPool)
	coverBackfillSvc := services.NewCoverBackfillService(bookRepo, coversDir, googleBooksKeyPool)

	scheduler := services.NewScheduler(bookRepo, adminRepo, coversDir, cfg.MetadataRefreshInterval)
	scheduler.RegisterJob("backup", "backup_interval", 24*time.Hour, backupSvc.CreateSnapshot)
	scheduler.RegisterJob("description-reconciliation", "description_reconciliation_interval", 24*time.Hour, descriptionReconciliationSvc.Run)
	scheduler.RegisterJob("cover-backfill", "cover_backfill_interval", 24*time.Hour, coverBackfillSvc.Run)
	// Sweeps abandoned signups out of registration_verifications. A row is
	// deleted as soon as its code is submitted, right or wrong, so this only
	// catches the ones nobody ever came back to — which for the email channel
	// still hold a bcrypt hash of the password that was typed. Hourly is far
	// more often than the 15-minute TTL needs, and the delete is a single
	// indexed statement.
	scheduler.RegisterJob("registration-prune", "registration_prune_interval", time.Hour,
		pruneRegistrationVerifications(regVerificationRepo))

	seedYAMLConfig(cfg.AppConfigPath, adminRepo)

	// Create the root context early so it can be passed to handlers that run
	// background goroutines (e.g. the metadata cache eviction loop).
	ctx, cancel := context.WithCancel(context.Background())

	// Handlers
	authH := handlers.NewAuthHandler(userRepo, adminRepo, copyRepo, regVerificationRepo, cfg.JWTSecret, cfg.EncryptionSecret, emailSvc, smsSvc, registrationWorkflow, cfg.Env, cfg.RegisterRateLimitBurst, cfg.RegisterSendRateLimitBurst, cfg.LoginRateLimitAttempts)
	metadataH := handlers.NewMetadataHandler(ctx, googleBooksKeyPool, cfg.EncryptionSecret, userRepo)
	bookH := handlers.NewBookHandler(bookRepo, userRepo, coversDir, wishlistWorkflow, recommendationRepo)
	copyH := handlers.NewCopyHandler(copyRepo, userRepo, notifRepo, waitlistRepo, adminRepo, bookRepo, wishlistRepo, coversDir, wishlistWorkflow, recommendationRepo)
	loanH := handlers.NewLoanRequestHandler(copyRepo, loanRepo, adminRepo, userRepo, workflow)
	notifH := handlers.NewNotificationHandler(notifRepo)
	adminH := handlers.NewAdminHandler(adminRepo, copyRepo, loanRepo, googleBooksKeyPool, registrationWorkflow, recommendationRepo)
	jobsH := handlers.NewJobsHandler(scheduler)
	backupH := handlers.NewBackupHandler(backupSvc)
	waitlistH := handlers.NewWaitlistHandler(copyRepo, waitlistRepo)
	announcementH := handlers.NewAnnouncementHandler(announcementRepo)
	wishlistH := handlers.NewWishlistHandler(wishlistRepo, bookRepo, wishlistWorkflow)
	recommendationH := handlers.NewRecommendationHandler(recommendationRepo)

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
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":         "ok",
			"version":        version,
			"schema_version": schemaVersion,
		})
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
	backupH.RegisterRoutes(api)
	waitlistH.RegisterRoutes(api)
	announcementH.RegisterRoutes(api)
	wishlistH.RegisterRoutes(api)
	recommendationH.RegisterRoutes(api)

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

// requireNonDefaultSecret fatally exits outside dev if secret is still set to
// its checked-in default — anyone who reads this public repo knows the
// default value and can exploit it (describe what, via consequence, in the
// log message).
func requireNonDefaultSecret(env, name, secret, defaultValue, consequence string) {
	if secret != defaultValue {
		return
	}
	if env != "dev" {
		log.Fatal().Msgf("%s must be set to a non-default value when ENV is not \"dev\" — anyone who reads this public repo knows the default and can %s", name, consequence)
	}
	log.Warn().Msgf("%s is set to the default value — change it before deploying to production", name)
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

// pruneRegistrationVerifications returns a scheduler job that deletes every
// registration_verifications row past its expiry — the abandoned-signup
// sweep described at its RegisterJob call site. Pulled out of main as its
// own function (rather than an inline closure) purely to keep main's own
// cognitive-complexity score down; the logic itself is a one-line delegate.
func pruneRegistrationVerifications(repo repository.RegistrationVerificationRepository) func(context.Context) string {
	return func(context.Context) string {
		n, err := repo.DeleteExpired(time.Now())
		if err != nil {
			return "failed: " + err.Error()
		}
		return fmt.Sprintf("pruned %d expired registration verification(s)", n)
	}
}
