// Package router parses bot commands and dispatches them to the service layer.
// It knows nothing about Telegram itself: it gets a plain Message and returns
// the reply text (HTML).
package router

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"unicode"

	"github.com/katshuuu/pr-pigeon/internal/domain"
)

type SubscriptionService interface {
	Link(ctx context.Context, chatID int64, rawLogin string) (domain.Subscription, error)
	Unlink(ctx context.Context, chatID int64) (domain.Subscription, error)
	Get(ctx context.Context, chatID int64) (domain.Subscription, error)
}

type Message struct {
	ChatID    int64
	FirstName string
	Text      string
}

type handlerFunc func(ctx context.Context, msg Message, args string) string

type Router struct {
	subs     SubscriptionService
	log      *slog.Logger
	handlers map[string]handlerFunc
}

func New(subs SubscriptionService, log *slog.Logger) *Router {
	r := &Router{subs: subs, log: log.With("component", "router")}
	r.handlers = map[string]handlerFunc{
		"start":  r.start,
		"help":   r.help,
		"link":   r.link,
		"github": r.link,
		"unlink": r.unlink,
		"me":     r.me,
	}
	return r
}

// Route returns the reply for an incoming message. An empty string means "say nothing".
func (r *Router) Route(ctx context.Context, msg Message) string {
	cmd, args, ok := parseCommand(msg.Text)
	if !ok {
		return "Я понимаю только команды 🙂 Загляните в /help — там всё коротко."
	}

	h, found := r.handlers[cmd]
	if !found {
		r.log.Debug("unknown command", "chat_id", msg.ChatID, "command", cmd)
		return fmt.Sprintf("Не знаю команду /%s 🤔 Что я умею — в /help.", html.EscapeString(cmd))
	}

	r.log.Debug("command received", "chat_id", msg.ChatID, "command", cmd)
	return h(ctx, msg, args)
}

// parseCommand splits "/link@PigeonBot octocat" into ("link", "octocat").
func parseCommand(text string) (cmd, args string, ok bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return "", "", false
	}

	head, rest := text, ""
	if i := strings.IndexFunc(text, unicode.IsSpace); i >= 0 {
		head, rest = text[:i], text[i:]
	}

	head = strings.TrimPrefix(head, "/")
	if at := strings.IndexByte(head, '@'); at >= 0 {
		head = head[:at] // commands in group chats come with the bot's username
	}
	if head == "" {
		return "", "", false
	}
	return strings.ToLower(head), strings.TrimSpace(rest), true
}

func (r *Router) start(_ context.Context, msg Message, _ string) string {
	name := "коллега"
	if msg.FirstName != "" {
		name = html.EscapeString(msg.FirstName)
	}
	return fmt.Sprintf(`Привет, %s! 🕊

Я — PR Pigeon, почтовый голубь для код-ревью. Слежу за pull request'ами на GitHub и прилетаю, когда:
• на вас назначили PR или позвали на ревью;
• в вашем PR появилось ревью, комментарий или решённое обсуждение;
• PR смержили или закрыли.

Чтобы начать, расскажите свой ник на GitHub:
<code>/link octocat</code>`, name)
}

func (r *Router) help(context.Context, Message, string) string {
	return `Что я умею:

/link <code>ник</code> — привязать GitHub-аккаунт к этому чату
/me — показать, кто сейчас привязан
/unlink — отвязать аккаунт и больше не присылать уведомления
/help — эта подсказка

Можно прислать ник, @ник или ссылку на профиль — разберусь.`
}

func (r *Router) link(ctx context.Context, msg Message, args string) string {
	if args == "" {
		return "Укажите ник после команды, например: <code>/link octocat</code>"
	}

	sub, err := r.subs.Link(ctx, msg.ChatID, args)
	switch {
	case errors.Is(err, domain.ErrInvalidLogin):
		return fmt.Sprintf("Хм, «%s» не похоже на ник GitHub. Ник состоит из латиницы, цифр и дефисов, максимум 39 символов.",
			html.EscapeString(args))
	case err != nil:
		return r.internalError(msg, "link", err)
	}

	return fmt.Sprintf("Готово! Уведомления для <b>@%s</b> теперь летят в этот чат ✉️\n\n"+
		"Проверить: назначьте себя assignee в любом PR репозитория с настроенным вебхуком.",
		html.EscapeString(sub.GitHubLogin))
}

func (r *Router) unlink(ctx context.Context, msg Message, _ string) string {
	sub, err := r.subs.Unlink(ctx, msg.ChatID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return "Этот чат пока ни к какому нику не привязан. Привязать — /link"
	case err != nil:
		return r.internalError(msg, "unlink", err)
	}
	return fmt.Sprintf("Отвязал <b>@%s</b>. Больше не побеспокою — но /link всегда рядом 👋",
		html.EscapeString(sub.GitHubLogin))
}

func (r *Router) me(ctx context.Context, msg Message, _ string) string {
	sub, err := r.subs.Get(ctx, msg.ChatID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return "Пока ни к кому не привязан. Расскажите ник: <code>/link octocat</code>"
	case err != nil:
		return r.internalError(msg, "me", err)
	}
	return fmt.Sprintf("Этот чат получает уведомления для <b>@%s</b> (с %s).",
		html.EscapeString(sub.GitHubLogin), sub.UpdatedAt.Format("02.01.2006"))
}

func (r *Router) internalError(msg Message, cmd string, err error) string {
	r.log.Error("command failed", "chat_id", msg.ChatID, "command", cmd, "err", err)
	return "Что-то пошло не так на моей стороне 😔 Попробуйте чуть позже."
}
