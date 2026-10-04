package telegram

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/katshuuu/pr-pigeon/internal/router"
)

type fakeSource struct {
	ch      chan tgbotapi.Update
	cfg     tgbotapi.UpdateConfig
	stopped bool
}

func (f *fakeSource) GetUpdatesChan(cfg tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	f.cfg = cfg
	return f.ch
}

func (f *fakeSource) StopReceivingUpdates() { f.stopped = true }

type echoRouter struct{}

func (echoRouter) Route(_ context.Context, m router.Message) string {
	if m.Text == "/silent" {
		return ""
	}
	return m.FirstName + ": " + m.Text
}

type recSender struct {
	mu   sync.Mutex
	msgs []string
	err  error
}

func (s *recSender) SendMessage(_ context.Context, _ int64, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.msgs = append(s.msgs, text)
	return s.err
}

func (s *recSender) all() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.msgs...)
}

func msgUpdate(id int, text string) tgbotapi.Update {
	return tgbotapi.Update{
		UpdateID: id,
		Message: &tgbotapi.Message{
			Text: text,
			Chat: &tgbotapi.Chat{ID: 7},
			From: &tgbotapi.User{FirstName: "Катя"},
		},
	}
}

func TestListener(t *testing.T) {
	src := &fakeSource{ch: make(chan tgbotapi.Update, 4)}
	sender := &recSender{}
	l := NewListener(src, echoRouter{}, sender, slog.New(slog.NewTextHandler(io.Discard, nil)))

	src.ch <- msgUpdate(1, "/start")
	src.ch <- tgbotapi.Update{UpdateID: 2} // e.g. an edited message, nothing to handle
	src.ch <- msgUpdate(3, "/silent")
	src.ch <- msgUpdate(4, "/help")
	close(src.ch)

	if err := l.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	got := sender.all()
	if len(got) != 2 || got[0] != "Катя: /start" || got[1] != "Катя: /help" {
		t.Errorf("replies = %v", got)
	}
	if !src.stopped || src.cfg.Timeout != pollTimeout {
		t.Errorf("stopped = %v, cfg = %+v", src.stopped, src.cfg)
	}
}

func TestListenerStopsOnContext(t *testing.T) {
	src := &fakeSource{ch: make(chan tgbotapi.Update)}
	sender := &recSender{err: errors.New("send failed")}
	l := NewListener(src, echoRouter{}, sender, slog.New(slog.NewTextHandler(io.Discard, nil)))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- l.Run(ctx) }()

	src.ch <- msgUpdate(1, "/start") // reply fails, listener keeps going
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("listener did not stop")
	}
}
