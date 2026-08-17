package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

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
