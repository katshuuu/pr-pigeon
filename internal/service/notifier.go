package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/katshuuu/pr-pigeon/internal/domain"
)

type SubscriberFinder interface {
	ChatIDsByLogin(ctx context.Context, login string) ([]int64, error)
}

// Sender delivers a ready-to-show message to a chat.
type Sender interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
}

type Notifier struct {
	subs   SubscriberFinder
	sender Sender
	log    *slog.Logger
}

func NewNotifier(subs SubscriberFinder, sender Sender, log *slog.Logger) *Notifier {
	return &Notifier{subs: subs, sender: sender, log: log.With("component", "notifier")}
}

// Notify works out who cares about the event and sends them a message.
// It returns the number of chats notified; a failure for one chat does not stop the others.
func (n *Notifier) Notify(ctx context.Context, ev domain.PREvent) (int, error) {
	log := n.log.With("kind", ev.Kind, "repo", ev.PR.Repo, "pr", ev.PR.Number)

	text, ok := render(ev)
	if !ok {
		log.Debug("nothing to say about this event, skipping")
		return 0, nil
	}

	logins := recipients(ev)
	if len(logins) == 0 {
		log.Debug("event has no recipients")
		return 0, nil
	}

	var (
		sent int
		errs []error
		seen = make(map[int64]struct{})
	)
	for _, login := range logins {
		chats, err := n.subs.ChatIDsByLogin(ctx, login)
		if err != nil {
			errs = append(errs, fmt.Errorf("find chats for %s: %w", login, err))
			continue
		}
		if len(chats) == 0 {
			log.Debug("login has no linked chats", "github_login", login)
			continue
		}

		for _, chatID := range chats {
			if _, dup := seen[chatID]; dup {
				continue
			}
			seen[chatID] = struct{}{}

			if err := n.sender.SendMessage(ctx, chatID, text); err != nil {
				log.Error("failed to send notification", "chat_id", chatID, "github_login", login, "err", err)
				errs = append(errs, err)
				continue
			}
			sent++
			log.Info("notification sent", "chat_id", chatID, "github_login", login)
		}
	}

	return sent, errors.Join(errs...)
}

// recipients decides whose chats should get the event.
//
// Assignment and review requests go to the person they are about, even when
// people assign themselves (that is exactly how the bot is usually tested).
// Everything else goes to assignees and the author, but never to the actor:
// nobody needs a ping about their own comment.
func recipients(ev domain.PREvent) []string {
	switch ev.Kind {
	case domain.EventPRAssigned, domain.EventPRUnassigned, domain.EventReviewRequested:
		if ev.Target == "" {
			return nil
		}
		return []string{ev.Target}
	}

	candidates := append([]string{}, ev.PR.Assignees...)
	candidates = append(candidates, ev.PR.Author)

	out := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, login := range candidates {
		key := strings.ToLower(login)
		if login == "" || strings.EqualFold(login, ev.Actor) {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, login)
	}
	return out
}
