// Package memory is an in-process storage. Handy for local runs and tests,
// but everything is gone after a restart.
package memory

import (
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/katshuuu/pr-pigeon/internal/domain"
)

type Repository struct {
	mu   sync.RWMutex
	subs map[int64]domain.Subscription
}

func New() *Repository {
	return &Repository{subs: make(map[int64]domain.Subscription)}
}

func (r *Repository) Upsert(_ context.Context, sub domain.Subscription) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if old, ok := r.subs[sub.ChatID]; ok {
		sub.CreatedAt = old.CreatedAt
	}
	r.subs[sub.ChatID] = sub
	return nil
}

func (r *Repository) Get(_ context.Context, chatID int64) (domain.Subscription, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sub, ok := r.subs[chatID]
	if !ok {
		return domain.Subscription{}, domain.ErrNotFound
	}
	return sub, nil
}

func (r *Repository) Delete(_ context.Context, chatID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.subs[chatID]; !ok {
		return domain.ErrNotFound
	}
	delete(r.subs, chatID)
	return nil
}

// ChatIDsByLogin matches logins case-insensitively, the same way GitHub does.
func (r *Repository) ChatIDsByLogin(_ context.Context, login string) ([]int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var ids []int64
	for id, sub := range r.subs {
		if strings.EqualFold(sub.GitHubLogin, login) {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids, nil
}
