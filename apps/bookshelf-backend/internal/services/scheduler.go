package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// defaultInterval is used when no valid interval is configured.
const defaultInterval = 24 * time.Hour

// coverRefreshConcurrency bounds how many cover downloads run in parallel,
// so a large catalog doesn't run this job for a very long time while still
// avoiding an unbounded burst of outbound requests.
const coverRefreshConcurrency = 6

// JobStatus describes the current state of a background job.
type JobStatus struct {
	Name       string     `json:"name"`
	Running    bool       `json:"running"`
	Interval   string     `json:"interval"`
	LastRunAt  *time.Time `json:"last_run_at"`
	NextRunAt  *time.Time `json:"next_run_at"`
	LastResult string     `json:"last_result"`
}

// nextRunAt computes when a job is next due, given its last run time and
// configured interval. Returns nil if it has never run yet (the scheduler
// runs every job once shortly after boot, so this only applies briefly).
func nextRunAt(lastRunAt *time.Time, interval time.Duration) *time.Time {
	if lastRunAt == nil {
		return nil
	}
	next := lastRunAt.Add(interval)
	return &next
}

// Scheduler runs periodic background tasks such as refreshing book cover
// images. Cover-refresh is the scheduler's original built-in job; additional
// named jobs (e.g. "backup") are attached via RegisterJob, sharing the same
// ticker/trigger/status machinery via the job type below.
type Scheduler struct {
	books     repository.BookRepository
	admin     repository.AdminRepository
	coversDir string
	fallback  time.Duration // interval used when setting is absent/invalid
	client    *http.Client
	trigger   chan struct{}

	mu         sync.RWMutex
	running    bool
	lastRunAt  *time.Time
	lastResult string

	extra []*job
}

// job holds the ticker state for one RegisterJob-added background task.
type job struct {
	name       string
	settingKey string
	fallback   time.Duration
	run        func(ctx context.Context) string
	trigger    chan struct{}

	mu         sync.RWMutex
	running    bool
	lastRunAt  *time.Time
	lastResult string
}

// NewScheduler creates a Scheduler.
// intervalStr is the fallback duration string (e.g. "24h") used when the
// admin setting "cover_refresh_interval" is absent. Defaults to 24h on error.
func NewScheduler(books repository.BookRepository, admin repository.AdminRepository, coversDir, intervalStr string) *Scheduler {
	fallback, err := time.ParseDuration(intervalStr)
	if err != nil || fallback <= 0 {
		fallback = defaultInterval
	}
	return &Scheduler{
		books:     books,
		admin:     admin,
		coversDir: coversDir,
		fallback:  fallback,
		client:    &http.Client{Timeout: 15 * time.Second},
		trigger:   make(chan struct{}, 1),
	}
}

// interval reads the configured interval from admin settings, falling back to s.fallback.
func (s *Scheduler) interval() time.Duration {
	if s.admin != nil {
		if val, err := s.admin.GetSetting("cover_refresh_interval"); err == nil && val != "" {
			if d, err := time.ParseDuration(val); err == nil && d > 0 {
				return d
			}
		}
	}
	return s.fallback
}

// RegisterJob attaches an additional named background job to the scheduler,
// alongside the built-in cover-refresh job. settingKey names the
// AdminRepository setting holding its configured interval (as a Go duration
// string); fallback is used when that setting is absent/invalid. run
// performs one job execution and returns a human-readable result string for
// JobStatus.LastResult. Must be called before Start.
func (s *Scheduler) RegisterJob(name, settingKey string, fallback time.Duration, run func(ctx context.Context) string) {
	s.extra = append(s.extra, &job{
		name:       name,
		settingKey: settingKey,
		fallback:   fallback,
		run:        run,
		trigger:    make(chan struct{}, 1),
	})
}

// Status returns the current status of every job — the built-in
// cover-refresh job first, then any RegisterJob-added jobs in registration order.
func (s *Scheduler) Status() []JobStatus {
	statuses := []JobStatus{s.coverRefreshStatus()}
	for _, j := range s.extra {
		statuses = append(statuses, j.status(s.jobInterval(j)))
	}
	return statuses
}

func (s *Scheduler) coverRefreshStatus() JobStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	interval := s.interval()
	return JobStatus{
		Name:       "cover-refresh",
		Running:    s.running,
		Interval:   interval.String(),
		LastRunAt:  s.lastRunAt,
		NextRunAt:  nextRunAt(s.lastRunAt, interval),
		LastResult: s.lastResult,
	}
}

func (j *job) status(interval time.Duration) JobStatus {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return JobStatus{
		Name:       j.name,
		Running:    j.running,
		Interval:   interval.String(),
		LastRunAt:  j.lastRunAt,
		NextRunAt:  nextRunAt(j.lastRunAt, interval),
		LastResult: j.lastResult,
	}
}

// jobInterval reads the configured interval for j from admin settings,
// falling back to j.fallback.
func (s *Scheduler) jobInterval(j *job) time.Duration {
	if s.admin != nil {
		if val, err := s.admin.GetSetting(j.settingKey); err == nil && val != "" {
			if d, err := time.ParseDuration(val); err == nil && d > 0 {
				return d
			}
		}
	}
	return j.fallback
}

// TriggerNow requests an immediate run of the named job ("cover-refresh" or
// any name passed to RegisterJob). Non-blocking; ignored if already queued.
// Returns false if no job with that name is registered.
func (s *Scheduler) TriggerNow(name string) bool {
	if name == "cover-refresh" {
		select {
		case s.trigger <- struct{}{}:
		default:
		}
		return true
	}
	for _, j := range s.extra {
		if j.name != name {
			continue
		}
		select {
		case j.trigger <- struct{}{}:
		default:
		}
		return true
	}
	return false
}

// Start launches the scheduler goroutine. It runs until ctx is cancelled.
// It uses a 1-minute base tick and checks the configured interval on each tick,
// so interval changes take effect within a minute.
func (s *Scheduler) Start(ctx context.Context) {
	log.Info().Dur("interval", s.interval()).Msg("scheduler started")

	var extraWg sync.WaitGroup
	for _, j := range s.extra {
		extraWg.Add(1)
		go func(j *job) {
			defer extraWg.Done()
			s.runJobLoop(ctx, j)
		}(j)
	}

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	// Run once on startup after a short delay to avoid blocking server startup.
	go func() {
		select {
		case <-time.After(30 * time.Second):
			s.refreshCovers(ctx)
		case <-ctx.Done():
		}
	}()

	for {
		select {
		case <-ticker.C:
			// Only run if enough time has elapsed since the last run.
			s.mu.RLock()
			last := s.lastRunAt
			s.mu.RUnlock()
			if last != nil && time.Since(*last) < s.interval() {
				continue
			}
			s.refreshCovers(ctx)
		case <-s.trigger:
			s.refreshCovers(ctx)
		case <-ctx.Done():
			log.Info().Msg("scheduler stopped")
			extraWg.Wait()
			return
		}
	}
}

// runJobLoop drives one RegisterJob-added job's ticker/trigger loop, mirroring
// the built-in cover-refresh loop in Start above.
func (s *Scheduler) runJobLoop(ctx context.Context, j *job) {
	interval := s.jobInterval(j)
	log.Info().Str("job", j.name).Dur("interval", interval).Msg("scheduler: job registered")
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	go func() {
		select {
		case <-time.After(30 * time.Second):
			s.runJob(ctx, j)
		case <-ctx.Done():
		}
	}()

	for {
		select {
		case <-ticker.C:
			j.mu.RLock()
			last := j.lastRunAt
			j.mu.RUnlock()
			if last != nil && time.Since(*last) < s.jobInterval(j) {
				continue
			}
			s.runJob(ctx, j)
		case <-j.trigger:
			s.runJob(ctx, j)
		case <-ctx.Done():
			log.Info().Str("job", j.name).Msg("scheduler: job stopped")
			return
		}
	}
}

// runJob executes j.run once, recording running/lastRunAt/lastResult.
// Skips the run (logging instead) if j is already running.
func (s *Scheduler) runJob(ctx context.Context, j *job) {
	j.mu.Lock()
	if j.running {
		j.mu.Unlock()
		log.Info().Str("job", j.name).Msg("scheduler: job already running, skipping")
		return
	}
	j.running = true
	now := time.Now()
	j.lastRunAt = &now
	j.lastResult = "running…"
	j.mu.Unlock()

	result := j.run(ctx)

	j.mu.Lock()
	j.running = false
	j.lastResult = result
	j.mu.Unlock()
}

// refreshCovers downloads and caches cover images for books that still have
// external URLs (i.e. not yet cached locally).
func (s *Scheduler) refreshCovers(ctx context.Context) {
	if s.coversDir == "" {
		return
	}
	if !s.beginRun() {
		log.Info().Msg("scheduler: cover refresh already running, skipping")
		return
	}
	defer s.endRun()

	log.Info().Msg("scheduler: starting cover refresh")

	books, err := s.books.List("", "title", false)
	if err != nil {
		log.Error().Err(err).Msg("scheduler: failed to list books")
		s.setLastResult("failed: " + err.Error())
		return
	}

	var refreshed atomic.Int64
	var wg sync.WaitGroup
	sem := make(chan struct{}, coverRefreshConcurrency)

booksLoop:
	for _, book := range books {
		select {
		case <-ctx.Done():
			break booksLoop
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(book models.Book) {
			defer wg.Done()
			defer func() { <-sem }()
			if s.refreshBookCover(&book) {
				refreshed.Add(1)
			}
		}(book)
	}
	wg.Wait()

	result := fmt.Sprintf("refreshed %d of %d books", refreshed.Load(), len(books))
	log.Info().Int64("refreshed", refreshed.Load()).Int("total", len(books)).Msg("scheduler: cover refresh complete")
	s.setLastResult(result)
}

// beginRun marks the job as running (recording the start time), unless
// already running. Returns false if a run was already in progress.
func (s *Scheduler) beginRun() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return false
	}
	s.running = true
	now := time.Now()
	s.lastRunAt = &now
	s.lastResult = "running…"
	return true
}

// endRun marks the job as no longer running.
func (s *Scheduler) endRun() {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
}

// setLastResult records the outcome of the most recent run.
func (s *Scheduler) setLastResult(result string) {
	s.mu.Lock()
	s.lastResult = result
	s.mu.Unlock()
}

// refreshBookCover downloads and caches book's cover if it has one hosted
// externally (not already a locally-cached path). Returns true if the book
// was updated. Deliberately doesn't check services.IsCoverURLAllowed —
// unlike the backfill jobs, this only ever re-fetches a CoverURL that was
// already allowlist-checked once when it was first accepted (at createBook
// or backfillCover time in internal/handlers/books.go), so it's not a new
// untrusted-input surface.
func (s *Scheduler) refreshBookCover(book *models.Book) bool {
	if book.CoverURL == "" || strings.HasPrefix(book.CoverURL, "/") {
		return false
	}

	localPath, dlErr := s.downloadCover(book.CoverURL)
	if dlErr != nil {
		log.Warn().Err(dlErr).Uint("book_id", book.ID).Msg("scheduler: cover download failed")
		return false
	}
	if localPath == "" || localPath == book.CoverURL {
		return false
	}

	book.CoverURL = localPath
	if saveErr := s.books.Save(book); saveErr != nil {
		log.Warn().Err(saveErr).Uint("book_id", book.ID).Msg("scheduler: failed to save cover path")
		return false
	}
	return true
}

// downloadCover fetches an external image URL and saves it to the coversDir.
// Returns the proxy-accessible path (/api/covers/<filename>) on success.
func (s *Scheduler) downloadCover(externalURL string) (string, error) {
	sum := sha256.Sum256([]byte(externalURL))
	baseName := fmt.Sprintf("%x", sum[:8])

	resp, err := s.client.Get(externalURL) //nolint:noctx
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cover fetch returned %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		return "", fmt.Errorf("unexpected content-type %q", ct)
	}

	ext := ".jpg"
	if strings.Contains(ct, "png") {
		ext = ".png"
	} else if strings.Contains(ct, "webp") {
		ext = ".webp"
	}

	filename := baseName + ext
	destPath := filepath.Join(s.coversDir, filename)

	// Already cached — skip re-download.
	if _, statErr := os.Stat(destPath); statErr == nil {
		return "/api/covers/" + filename, nil
	}

	f, err := os.Create(destPath) //nolint:gosec
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck

	const maxBytes = 10 << 20 // 10 MiB
	if _, err := io.Copy(f, io.LimitReader(resp.Body, maxBytes)); err != nil {
		_ = os.Remove(destPath)
		return "", err
	}

	log.Debug().Str("filename", filename).Msg("scheduler: cover cached")
	return "/api/covers/" + filename, nil
}
