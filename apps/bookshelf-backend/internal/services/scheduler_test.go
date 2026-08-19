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
)

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

type stubAdminRepo struct {
	repository.AdminRepository
}

func (stubAdminRepo) GetSetting(_ string) (string, error) {
	return "", nil
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

	sched := NewScheduler(repo, stubAdminRepo{}, t.TempDir(), "24h")
	sched.refreshCovers(context.Background())

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.saved != n {
		t.Fatalf("expected %d books saved, got %d", n, repo.saved)
	}
}

func TestScheduler_RegisterJob_StatusIncludesEveryJob(t *testing.T) {
	sched := NewScheduler(&stubBookRepo{}, stubAdminRepo{}, t.TempDir(), "24h")
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
	sched := NewScheduler(&stubBookRepo{}, stubAdminRepo{}, t.TempDir(), "24h")
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
