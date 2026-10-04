package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/katshuuu/pr-pigeon/internal/config"
	"github.com/katshuuu/pr-pigeon/internal/domain"
	"github.com/katshuuu/pr-pigeon/internal/repository/memory"
)

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

type nopNotifier struct{}

func (nopNotifier) Notify(context.Context, domain.PREvent) (int, error) { return 0, nil }

func TestHTTPRoutes(t *testing.T) {
	cfg := config.Config{GitHub: config.GitHubConfig{Secret: "s", NotifyTimeout: time.Second}}
	h := newHTTPHandler(cfg, nopNotifier{}, discard())

	tests := []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/healthz", http.StatusOK},
		{http.MethodPost, WebhookPath, http.StatusUnauthorized}, // no signature
		{http.MethodGet, WebhookPath, http.StatusMethodNotAllowed},
		{http.MethodGet, "/", http.StatusNotFound},
	}
	for _, tt := range tests {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))
		if w.Code != tt.want {
			t.Errorf("%s %s = %d, want %d", tt.method, tt.path, w.Code, tt.want)
		}
	}
}

func TestRunAllStopsOthers(t *testing.T) {
	boom := errors.New("boom")
	err := runAll(context.Background(), discard(),
		task{"failing", func(context.Context) error { return boom }},
		task{"waiting", func(ctx context.Context) error { <-ctx.Done(); return nil }},
	)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

func TestRunAllCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := runAll(ctx, discard(), task{"waiting", func(ctx context.Context) error { <-ctx.Done(); return nil }})
	if err != nil {
		t.Fatal(err)
	}
}

func TestNewRepositoryMemory(t *testing.T) {
	repo, closeFn, err := newRepository(context.Background(), config.DBConfig{}, discard())
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	if _, ok := repo.(*memory.Repository); !ok {
		t.Errorf("repo = %T, want memory", repo)
	}
}

func TestNewRepositoryPostgresUnavailable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	cfg := config.DBConfig{Host: "127.0.0.1", Port: "1", Username: "u", Name: "db", SSLMode: "disable"}
	if _, _, err := newRepository(ctx, cfg, discard()); err == nil {
		t.Fatal("expected error")
	}
}
