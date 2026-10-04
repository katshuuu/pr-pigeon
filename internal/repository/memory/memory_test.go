package memory

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/katshuuu/pr-pigeon/internal/domain"
)

func TestRepository(t *testing.T) {
	ctx := context.Background()
	r := New()

	first := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	_ = r.Upsert(ctx, domain.Subscription{ChatID: 2, GitHubLogin: "Octocat", CreatedAt: first, UpdatedAt: first})
	_ = r.Upsert(ctx, domain.Subscription{ChatID: 1, GitHubLogin: "octocat"})
	_ = r.Upsert(ctx, domain.Subscription{ChatID: 3, GitHubLogin: "someone-else"})

	ids, _ := r.ChatIDsByLogin(ctx, "OCTOCAT")
	if !reflect.DeepEqual(ids, []int64{1, 2}) {
		t.Errorf("ChatIDsByLogin = %v", ids)
	}

	// re-linking keeps the original creation time
	later := first.Add(time.Hour)
	_ = r.Upsert(ctx, domain.Subscription{ChatID: 2, GitHubLogin: "hubot", CreatedAt: later, UpdatedAt: later})
	sub, err := r.Get(ctx, 2)
	if err != nil || sub.GitHubLogin != "hubot" || !sub.CreatedAt.Equal(first) || !sub.UpdatedAt.Equal(later) {
		t.Errorf("Get = %+v, %v", sub, err)
	}

	if err := r.Delete(ctx, 2); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := r.Delete(ctx, 2); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("second Delete = %v", err)
	}
	if _, err := r.Get(ctx, 2); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get deleted = %v", err)
	}
}

func TestRepositoryConcurrent(t *testing.T) {
	ctx := context.Background()
	r := New()
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = r.Upsert(ctx, domain.Subscription{ChatID: int64(i), GitHubLogin: "octocat"})
		}()
		go func() {
			defer wg.Done()
			_, _ = r.ChatIDsByLogin(ctx, "octocat")
		}()
	}
	wg.Wait()

	ids, _ := r.ChatIDsByLogin(ctx, "octocat")
	if len(ids) != 50 {
		t.Errorf("got %d ids, want 50", len(ids))
	}
}
