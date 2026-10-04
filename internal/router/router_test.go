package router

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/katshuuu/pr-pigeon/internal/domain"
)

type fakeSubs struct {
	sub domain.Subscription
	err error
}

func (f *fakeSubs) Link(_ context.Context, chatID int64, raw string) (domain.Subscription, error) {
	if f.err != nil {
		return domain.Subscription{}, f.err
	}
	login, err := domain.NormalizeLogin(raw)
	if err != nil {
		return domain.Subscription{}, err
	}
	return domain.Subscription{ChatID: chatID, GitHubLogin: login}, nil
}

func (f *fakeSubs) Unlink(context.Context, int64) (domain.Subscription, error) { return f.sub, f.err }
func (f *fakeSubs) Get(context.Context, int64) (domain.Subscription, error)    { return f.sub, f.err }

func newRouter(f *fakeSubs) *Router {
	return New(f, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestParseCommand(t *testing.T) {
	tests := []struct {
		in        string
		cmd, args string
		ok        bool
	}{
		{"/start", "start", "", true},
		{"  /LINK   octocat  ", "link", "octocat", true},
		{"/link@PrPigeonBot @octocat", "link", "@octocat", true},
		{"/link\noctocat", "link", "octocat", true},
		{"hello", "", "", false},
		{"/", "", "", false},
		{"/@bot", "", "", false},
	}
	for _, tt := range tests {
		cmd, args, ok := parseCommand(tt.in)
		if cmd != tt.cmd || args != tt.args || ok != tt.ok {
			t.Errorf("parseCommand(%q) = %q, %q, %v", tt.in, cmd, args, ok)
		}
	}
}

func TestRoute(t *testing.T) {
	linked := domain.Subscription{ChatID: 1, GitHubLogin: "katshuuu", UpdatedAt: time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)}
	dbErr := errors.New("db is down")

	tests := []struct {
		name string
		subs *fakeSubs
		text string
		want string
	}{
		{"start with name", &fakeSubs{}, "/start", "Привет, <Катя>"},
		{"help", &fakeSubs{}, "/help", "/unlink"},
		{"plain text", &fakeSubs{}, "привет", "только команды"},
		{"unknown", &fakeSubs{}, "/dance<b>", "Не знаю команду /dance&lt;b&gt;"},
		{"link ok", &fakeSubs{}, "/link https://github.com/katshuuu", "<b>@katshuuu</b>"},
		{"link alias", &fakeSubs{}, "/github katshuuu", "<b>@katshuuu</b>"},
		{"link empty", &fakeSubs{}, "/link", "Укажите ник"},
		{"link invalid", &fakeSubs{}, "/link <script>", "«&lt;script&gt;» не похоже"},
		{"link db error", &fakeSubs{err: dbErr}, "/link octocat", "не так на моей стороне"},
		{"me", &fakeSubs{sub: linked}, "/me", "<b>@katshuuu</b> (с 04.10.2026)"},
		{"me not linked", &fakeSubs{err: domain.ErrNotFound}, "/me", "Пока ни к кому"},
		{"me db error", &fakeSubs{err: dbErr}, "/me", "не так на моей стороне"},
		{"unlink", &fakeSubs{sub: linked}, "/unlink", "Отвязал <b>@katshuuu</b>"},
		{"unlink not linked", &fakeSubs{err: domain.ErrNotFound}, "/unlink", "ни к какому нику"},
		{"unlink db error", &fakeSubs{err: dbErr}, "/unlink", "не так на моей стороне"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRouter(tt.subs)
			got := r.Route(context.Background(), Message{ChatID: 1, FirstName: "<Катя>", Text: tt.text})
			want := strings.ReplaceAll(tt.want, "<Катя>", "&lt;Катя&gt;")
			if !strings.Contains(got, want) {
				t.Errorf("reply %q does not contain %q", got, want)
			}
		})
	}
}

func TestStartWithoutName(t *testing.T) {
	got := newRouter(&fakeSubs{}).Route(context.Background(), Message{Text: "/start"})
	if !strings.Contains(got, "Привет, коллега") {
		t.Errorf("reply = %q", got)
	}
}
