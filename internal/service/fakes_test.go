package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"

	"github.com/katshuuu/pr-pigeon/internal/domain"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeRepo struct {
	subs      map[int64]domain.Subscription
	failWith  error
	deleteErr error
}

func newFakeRepo() *fakeRepo { return &fakeRepo{subs: map[int64]domain.Subscription{}} }

func (r *fakeRepo) Upsert(_ context.Context, s domain.Subscription) error {
	if r.failWith != nil {
		return r.failWith
	}
	r.subs[s.ChatID] = s
	return nil
}

func (r *fakeRepo) Get(_ context.Context, chatID int64) (domain.Subscription, error) {
	if r.failWith != nil {
		return domain.Subscription{}, r.failWith
	}
	s, ok := r.subs[chatID]
	if !ok {
		return domain.Subscription{}, domain.ErrNotFound
	}
	return s, nil
}

func (r *fakeRepo) Delete(_ context.Context, chatID int64) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	delete(r.subs, chatID)
	return nil
}

func (r *fakeRepo) ChatIDsByLogin(_ context.Context, login string) ([]int64, error) {
	if r.failWith != nil {
		return nil, r.failWith
	}
	var ids []int64
	for id, s := range r.subs {
		if strings.EqualFold(s.GitHubLogin, login) {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

type sentMsg struct {
	chatID int64
	text   string
}

type fakeSender struct {
	mu     sync.Mutex
	sent   []sentMsg
	failTo map[int64]bool
}

func (s *fakeSender) SendMessage(_ context.Context, chatID int64, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failTo[chatID] {
		return errors.New("telegram is down")
	}
	s.sent = append(s.sent, sentMsg{chatID, text})
	return nil
}
