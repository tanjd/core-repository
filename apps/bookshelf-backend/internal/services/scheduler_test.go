package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repotest"
)

// stubAdminRepo is a minimal AdminRepository fake used elsewhere in this
// package (backup_test.go, digest_test.go) purely as an embedding base to
// satisfy the interface, with individual methods overridden as needed —
// scheduler tests in this file use the fuller repotest.AdminRepository
// instead, since they need a working UpsertSetting.
type stubAdminRepo struct {
	repository.AdminRepository
}

func (stubAdminRepo) GetSetting(_ string) (string, error) {
	return "", nil
}

type stubBookRepo struct {
	repository.BookRepository
	books []models.Book
	mu    sync.Mutex
	saved int
}

func (r *stubBookRepo) List(_, _ string, _ bool) ([]models.Book, error) {
	return r.books, nil
}

func (r *stubBookRepo) Save(_ *models.Book) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.saved++
	return nil
}

func TestRefreshCovers_ConcurrentDownloadsSaveAllBooks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fake-jpeg-bytes"))
	}))
	defer srv.Close()

	const n = 20
	books := make([]models.Book, n)
	for i := range books {
		books[i] = models.Book{ID: uint(i + 1), CoverURL: srv.URL + "/cover.jpg"}
	}
	repo := &stubBookRepo{books: books}

	sched := NewScheduler(repo, repotest.NewAdminRepository(), t.TempDir(), "24h")
	sched.refreshCovers(context.Background())

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.saved != n {
		t.Fatalf("expected %d books saved, got %d", n, repo.saved)
	}
}

func TestScheduler_RegisterJob_StatusIncludesEveryJob(t *testing.T) {
	sched := NewScheduler(&stubBookRepo{}, repotest.NewAdminRepository(), t.TempDir(), "24h")
	sched.RegisterJob("backup", "backup_interval", time.Hour, func(context.Context) string {
		return "ok"
	})

	statuses := sched.Status()
	if len(statuses) != 2 {
		t.Fatalf("expected 2 job statuses, got %d", len(statuses))
	}
	if statuses[0].Name != "cover-refresh" {
		t.Fatalf("expected first status to be cover-refresh, got %q", statuses[0].Name)
	}
	if statuses[1].Name != "backup" {
		t.Fatalf("expected second status to be backup, got %q", statuses[1].Name)
	}
	if statuses[1].Interval != "1h0m0s" {
		t.Fatalf("expected backup job's fallback interval, got %q", statuses[1].Interval)
	}
}

func TestScheduler_TriggerNow_RunsRegisteredJobAndReportsUnknown(t *testing.T) {
	sched := NewScheduler(&stubBookRepo{}, repotest.NewAdminRepository(), t.TempDir(), "24h")
	ran := make(chan struct{}, 1)
	sched.RegisterJob("backup", "backup_interval", time.Hour, func(context.Context) string {
		ran <- struct{}{}
		return "done"
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sched.runJobLoop(ctx, sched.extra[0])

	if !sched.TriggerNow("backup") {
		t.Fatal("expected TriggerNow(\"backup\") to find the registered job")
	}
	select {
	case <-ran:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for triggered job to run")
	}

	if sched.TriggerNow("no-such-job") {
		t.Fatal("expected TriggerNow for an unknown job name to return false")
	}
}

func TestHydrateLastRunAt_RoundTripsThroughAdminRepository(t *testing.T) {
	admin := repotest.NewAdminRepository()

	if got := hydrateLastRunAt(admin, "backup_last_run_at"); got != nil {
		t.Fatalf("expected nil for an unset key, got %v", got)
	}

	want := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	if err := admin.UpsertSetting("backup_last_run_at", want.Format(time.RFC3339)); err != nil {
		t.Fatalf("UpsertSetting failed: %v", err)
	}
	got := hydrateLastRunAt(admin, "backup_last_run_at")
	if got == nil || !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}

	if err := admin.UpsertSetting("backup_last_run_at", "not-a-time"); err != nil {
		t.Fatalf("UpsertSetting failed: %v", err)
	}
	if got := hydrateLastRunAt(admin, "backup_last_run_at"); got != nil {
		t.Fatalf("expected nil for a malformed stored value, got %v", got)
	}
}

func TestIsDue(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		last     *time.Time
		interval time.Duration
		want     bool
	}{
		{"never run", nil, time.Hour, true},
		{"interval elapsed", timePtr(now.Add(-2 * time.Hour)), time.Hour, true},
		{"interval not yet elapsed", timePtr(now.Add(-10 * time.Minute)), time.Hour, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDue(tt.last, tt.interval); got != tt.want {
				t.Fatalf("isDue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunJob_PersistsLastRunAtToAdminRepository(t *testing.T) {
	admin := repotest.NewAdminRepository()
	sched := NewScheduler(&stubBookRepo{}, admin, t.TempDir(), "24h")
	sched.RegisterJob("backup", "backup_interval", time.Hour, func(context.Context) string {
		return "ok"
	})

	before := time.Now()
	sched.runJob(context.Background(), sched.extra[0])
	after := time.Now()

	raw, err := admin.GetSetting("backup_last_run_at")
	if err != nil {
		t.Fatalf("expected backup_last_run_at to be persisted, got error: %v", err)
	}
	got, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("expected a parseable RFC3339 timestamp, got %q: %v", raw, err)
	}
	if got.Before(before.Add(-time.Second)) || got.After(after.Add(time.Second)) {
		t.Fatalf("expected persisted timestamp near [%v, %v], got %v", before, after, got)
	}
}

func TestIsDue_RespectsPersistedLastRun(t *testing.T) {
	const key = "backup_last_run_at"
	const interval = time.Hour

	t.Run("recently persisted run is not due", func(t *testing.T) {
		admin := repotest.NewAdminRepository()
		if err := admin.UpsertSetting(key, time.Now().Add(-5*time.Minute).UTC().Format(time.RFC3339)); err != nil {
			t.Fatalf("UpsertSetting failed: %v", err)
		}
		if isDue(hydrateLastRunAt(admin, key), interval) {
			t.Fatal("expected a job run 5 minutes ago with a 1h interval to not be due")
		}
	})

	t.Run("stale persisted run is due", func(t *testing.T) {
		admin := repotest.NewAdminRepository()
		if err := admin.UpsertSetting(key, time.Now().Add(-2*time.Hour).UTC().Format(time.RFC3339)); err != nil {
			t.Fatalf("UpsertSetting failed: %v", err)
		}
		if !isDue(hydrateLastRunAt(admin, key), interval) {
			t.Fatal("expected a job run 2 hours ago with a 1h interval to be due")
		}
	})

	t.Run("missing persisted run is due", func(t *testing.T) {
		admin := repotest.NewAdminRepository()
		if !isDue(hydrateLastRunAt(admin, key), interval) {
			t.Fatal("expected a job with no persisted last-run to be due (first-ever boot)")
		}
	})
}

func timePtr(t time.Time) *time.Time {
	return &t
}
