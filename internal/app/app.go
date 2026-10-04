// Package app wires all layers together and runs them until shutdown.
package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	_ "github.com/lib/pq" // postgres driver

	tgclient "github.com/katshuuu/pr-pigeon/internal/adapters/clients/telegram"
	"github.com/katshuuu/pr-pigeon/internal/config"
	"github.com/katshuuu/pr-pigeon/internal/delivery/github"
	"github.com/katshuuu/pr-pigeon/internal/delivery/httpserver"
	tgdelivery "github.com/katshuuu/pr-pigeon/internal/delivery/telegram"
	"github.com/katshuuu/pr-pigeon/internal/repository/memory"
	"github.com/katshuuu/pr-pigeon/internal/repository/postgres"
	"github.com/katshuuu/pr-pigeon/internal/router"
	"github.com/katshuuu/pr-pigeon/internal/service"
)

const WebhookPath = "/api/v1/github/webhook"

type repository interface {
	service.SubscriptionRepository
	service.SubscriberFinder
}

func Run(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	repo, closeRepo, err := newRepository(ctx, cfg.DB, log)
	if err != nil {
		return err
	}
	defer closeRepo()

	tgclient.UseSlog(log)
	api, err := tgclient.NewBotAPI(cfg.Telegram.Token, cfg.Telegram.Debug)
	if err != nil {
		return err
	}
	log.Info("telegram bot authorized", "username", api.Self.UserName)

	sender := tgclient.New(api, log)
	subs := service.NewSubscriptions(repo, log)
	notifier := service.NewNotifier(repo, sender, log)

	listener := tgdelivery.NewListener(api, router.New(subs, log), sender, log)
	server := httpserver.New(cfg.HTTP, newHTTPHandler(cfg, notifier, log), log)

	return runAll(ctx, log,
		task{"http server", server.Run},
		task{"telegram listener", listener.Run},
	)
}

func newHTTPHandler(cfg config.Config, n github.Notifier, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST "+WebhookPath, github.NewHandler(cfg.GitHub.Secret, n, cfg.GitHub.NotifyTimeout, log))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	return httpserver.Chain(mux, httpserver.LogRequests(log), httpserver.Recover(log))
}

type task struct {
	name string
	run  func(context.Context) error
}

// runAll starts tasks concurrently. When one of them stops, the rest are cancelled.
func runAll(ctx context.Context, log *slog.Logger, tasks ...task) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	for _, t := range tasks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer cancel()
			if err := t.run(ctx); err != nil {
				log.Error("task failed", "task", t.name, "err", err)
				mu.Lock()
				errs = append(errs, fmt.Errorf("%s: %w", t.name, err))
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return errors.Join(errs...)
}

func newRepository(ctx context.Context, cfg config.DBConfig, log *slog.Logger) (repository, func(), error) {
	if !cfg.Enabled() {
		log.Warn("DB_HOST is not set, using in-memory storage: linked accounts will be lost on restart")
		return memory.New(), func() {}, nil
	}

	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)

	if err := ping(ctx, db, log); err != nil {
		db.Close()
		return nil, nil, err
	}

	repo := postgres.New(db)
	if err := repo.Migrate(ctx); err != nil {
		db.Close()
		return nil, nil, err
	}

	log.Info("connected to postgres", "host", cfg.Host, "db", cfg.Name)
	return repo, func() { _ = db.Close() }, nil
}

// ping retries for a few seconds: in docker-compose the database may still be starting.
func ping(ctx context.Context, db *sql.DB, log *slog.Logger) error {
	const attempts = 5
	var err error
	for i := 1; i <= attempts; i++ {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err = db.PingContext(pingCtx)
		cancel()
		if err == nil {
			return nil
		}
		log.Warn("postgres is not ready yet", "attempt", i, "err", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(i) * time.Second):
		}
	}
	return fmt.Errorf("ping postgres: %w", err)
}
