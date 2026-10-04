// Package telegram is the outgoing side of Telegram: it sends messages.
// Today it wraps go-telegram-bot-api; swapping the library should only touch this package.
package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// API is the part of *tgbotapi.BotAPI we use; it keeps the client testable.
type API interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}

type Client struct {
	api API
	log *slog.Logger
}

func New(api API, log *slog.Logger) *Client {
	return &Client{api: api, log: log.With("component", "telegram_client")}
}

// SendMessage sends an HTML-formatted message. Link previews are off:
// a GitHub preview card would take more space than the notification itself.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true

	start := time.Now()
	if _, err := c.api.Send(msg); err != nil {
		return fmt.Errorf("send message to chat %d: %w", chatID, err)
	}
	c.log.Debug("message sent", "chat_id", chatID, "took", time.Since(start))
	return nil
}

// NewBotAPI connects to Telegram and checks the token with getMe.
// The HTTP timeout is longer than the long-polling timeout (30s) on purpose.
func NewBotAPI(token string, debug bool) (*tgbotapi.BotAPI, error) {
	httpClient := &http.Client{Timeout: 60 * time.Second}
	api, err := tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, httpClient)
	if err != nil {
		return nil, fmt.Errorf("connect to telegram: %w", err)
	}
	api.Debug = debug
	return api, nil
}

// UseSlog routes the library's own log output (polling retries etc.) into slog.
func UseSlog(log *slog.Logger) {
	_ = tgbotapi.SetLogger(slogAdapter{log: log.With("component", "tgbotapi")})
}

type slogAdapter struct{ log *slog.Logger }

func (a slogAdapter) Println(v ...interface{}) { a.log.Warn(fmt.Sprint(v...)) }

func (a slogAdapter) Printf(format string, v ...interface{}) {
	a.log.Debug(fmt.Sprintf(format, v...))
}
