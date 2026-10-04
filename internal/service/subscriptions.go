// Package service contains the business logic of the bot.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/katshuuu/pr-pigeon/internal/domain"
)

type SubscriptionRepository interface {
	Upsert(ctx context.Context, sub domain.Subscription) error
	Get(ctx context.Context, chatID int64) (domain.Subscription, error)
	Delete(ctx context.Context, chatID int64) error
}

type Subscriptions struct {
	repo SubscriptionRepository
	log  *slog.Logger
	now  func() time.Time
}

func NewSubscriptions(repo SubscriptionRepository, log *slog.Logger) *Subscriptions {
	return &Subscriptions{repo: repo, log: log.With("component", "subscriptions"), now: time.Now}
}

// Link remembers which GitHub account belongs to the chat. Re-linking overwrites the old login.
func (s *Subscriptions) Link(ctx context.Context, chatID int64, rawLogin string) (domain.Subscription, error) {
	login, err := domain.NormalizeLogin(rawLogin)
	if err != nil {
		return domain.Subscription{}, err
	}

	now := s.now().UTC()
	sub := domain.Subscription{ChatID: chatID, GitHubLogin: login, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.Upsert(ctx, sub); err != nil {
		return domain.Subscription{}, fmt.Errorf("save subscription: %w", err)
	}

	s.log.Info("github login linked", "chat_id", chatID, "github_login", login)
	return sub, nil
}

// Unlink removes the chat's subscription and returns what was removed.
func (s *Subscriptions) Unlink(ctx context.Context, chatID int64) (domain.Subscription, error) {
	sub, err := s.Get(ctx, chatID)
	if err != nil {
		return domain.Subscription{}, err
	}
	if err := s.repo.Delete(ctx, chatID); err != nil {
		return domain.Subscription{}, fmt.Errorf("delete subscription: %w", err)
	}

	s.log.Info("github login unlinked", "chat_id", chatID, "github_login", sub.GitHubLogin)
	return sub, nil
}

func (s *Subscriptions) Get(ctx context.Context, chatID int64) (domain.Subscription, error) {
	sub, err := s.repo.Get(ctx, chatID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Subscription{}, err
		}
		return domain.Subscription{}, fmt.Errorf("get subscription: %w", err)
	}
	return sub, nil
}
