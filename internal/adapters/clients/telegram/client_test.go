package telegram

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type fakeAPI struct {
	got tgbotapi.Chattable
	err error
}

func (f *fakeAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	f.got = c
	return tgbotapi.Message{}, f.err
}

func TestSendMessage(t *testing.T) {
	api := &fakeAPI{}
	c := New(api, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := c.SendMessage(context.Background(), 42, "<b>hi</b>"); err != nil {
		t.Fatal(err)
	}
	msg, ok := api.got.(tgbotapi.MessageConfig)
	if !ok {
		t.Fatalf("sent %T", api.got)
	}
	if msg.ChatID != 42 || msg.ParseMode != tgbotapi.ModeHTML || !msg.DisableWebPagePreview {
		t.Errorf("unexpected message config: %+v", msg)
	}
}

func TestSendMessageErrors(t *testing.T) {
	api := &fakeAPI{err: errors.New("Forbidden: bot was blocked by the user")}
	c := New(api, slog.New(slog.NewTextHandler(io.Discard, nil)))

	err := c.SendMessage(context.Background(), 1, "x")
	if err == nil || !strings.Contains(err.Error(), "chat 1") {
		t.Errorf("err = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	api.got = nil
	if err := c.SendMessage(ctx, 1, "x"); !errors.Is(err, context.Canceled) || api.got != nil {
		t.Errorf("cancelled ctx: err = %v, sent = %v", err, api.got)
	}
}

func TestSlogAdapter(t *testing.T) {
	var buf bytes.Buffer
	a := slogAdapter{log: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))}
	a.Println("Failed to get updates, retrying in 3 seconds...")
	a.Printf("Endpoint: %s", "getMe")

	out := buf.String()
	if !strings.Contains(out, "level=WARN") || !strings.Contains(out, "Endpoint: getMe") {
		t.Errorf("unexpected log output: %s", out)
	}
}
