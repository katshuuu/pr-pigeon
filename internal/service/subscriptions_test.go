package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/katshuuu/pr-pigeon/internal/domain"
)

func TestSubscriptions_LinkGetUnlink(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	svc := NewSubscriptions(repo, discardLogger())
	svc.now = func() time.Time { return time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC) }

	sub, err := svc.Link(ctx, 42, "@katshuuu")
	if err != nil {
		t.Fatalf("Link: %v", err)
	}
	if sub.GitHubLogin != "katshuuu" || sub.ChatID != 42 {
		t.Fatalf("unexpected subscription: %+v", sub)
	}

	got, err := svc.Get(ctx, 42)
	if err != nil || got.GitHubLogin != "katshuuu" {
		t.Fatalf("Get = %+v, %v", got, err)
	}

	removed, err := svc.Unlink(ctx, 42)
	if err != nil || removed.GitHubLogin != "katshuuu" {
		t.Fatalf("Unlink = %+v, %v", removed, err)
	}

	if _, err := svc.Get(ctx, 42); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Get after unlink: %v, want ErrNotFound", err)
	}
}

func TestSubscriptions_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid login", func(t *testing.T) {
		svc := NewSubscriptions(newFakeRepo(), discardLogger())
		if _, err := svc.Link(ctx, 1, "not a login"); !errors.Is(err, domain.ErrInvalidLogin) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("repo failure on link", func(t *testing.T) {
		repo := newFakeRepo()
		repo.failWith = errors.New("db is down")
		svc := NewSubscriptions(repo, discardLogger())
		if _, err := svc.Link(ctx, 1, "octocat"); err == nil {
			t.Fatal("expected error")
		}
		if _, err := svc.Get(ctx, 1); err == nil || errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("Get err = %v, want wrapped db error", err)
		}
	})

	t.Run("unlink not linked", func(t *testing.T) {
		svc := NewSubscriptions(newFakeRepo(), discardLogger())
		if _, err := svc.Unlink(ctx, 1); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("delete failure", func(t *testing.T) {
		repo := newFakeRepo()
		repo.subs[1] = domain.Subscription{ChatID: 1, GitHubLogin: "octocat"}
		repo.deleteErr = errors.New("boom")
		svc := NewSubscriptions(repo, discardLogger())
		if _, err := svc.Unlink(ctx, 1); err == nil {
			t.Fatal("expected error")
		}
	})
}
