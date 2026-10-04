// Package telegram is the incoming side of Telegram: it receives updates via
// long polling, hands them to the router and sends the replies back.
package telegram

import (
	"context"
	"log/slog"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/katshuuu/pr-pigeon/internal/router"
)

const (
	pollTimeout  = 30 // seconds, Telegram long polling
	replyTimeout = 10 * time.Second
)

type UpdatesSource interface {
	GetUpdatesChan(cfg tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	StopReceivingUpdates()
}

type Router interface {
	Route(ctx context.Context, msg router.Message) string
}

type Sender interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
}

type Listener struct {
	src    UpdatesSource
	router Router
	sender Sender
	log    *slog.Logger
}

func NewListener(src UpdatesSource, r Router, s Sender, log *slog.Logger) *Listener {
	return &Listener{src: src, router: r, sender: s, log: log.With("component", "telegram_listener")}
}

// Run blocks until ctx is cancelled or the updates channel is closed.
func (l *Listener) Run(ctx context.Context) error {
	cfg := tgbotapi.NewUpdate(0)
	cfg.Timeout = pollTimeout
	cfg.AllowedUpdates = []string{"message"}

	updates := l.src.GetUpdatesChan(cfg)
	defer l.src.StopReceivingUpdates()

	l.log.Info("listening for telegram updates")
	for {
		select {
		case <-ctx.Done():
			l.log.Info("telegram listener stopped")
			return nil
		case upd, ok := <-updates:
			if !ok {
				return nil
			}
			l.handle(ctx, upd)
		}
	}
}

func (l *Listener) handle(ctx context.Context, upd tgbotapi.Update) {
	msg := upd.Message
	if msg == nil || msg.Text == "" {
		return
	}

	log := l.log.With("update_id", upd.UpdateID, "chat_id", msg.Chat.ID)
	in := router.Message{ChatID: msg.Chat.ID, Text: msg.Text}
	if msg.From != nil {
		in.FirstName = msg.From.FirstName
	}

	ctx, cancel := context.WithTimeout(ctx, replyTimeout)
	defer cancel()

	reply := l.router.Route(ctx, in)
	if reply == "" {
		return
	}
	if err := l.sender.SendMessage(ctx, msg.Chat.ID, reply); err != nil {
		log.Error("failed to reply", "err", err)
	}
}
