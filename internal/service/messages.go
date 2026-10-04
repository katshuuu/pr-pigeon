package service

import (
	"fmt"
	"html"
	"strings"
	"unicode/utf8"

	"github.com/katshuuu/pr-pigeon/internal/domain"
)

// Messages use Telegram's HTML parse mode, so everything that comes from
// GitHub must be escaped.

const maxQuoteLen = 300

func render(ev domain.PREvent) (string, bool) {
	var b strings.Builder

	switch ev.Kind {
	case domain.EventPRAssigned:
		fmt.Fprintf(&b, "📌 На вас назначен pull request: %s\n", prLine(ev.PR))
		fmt.Fprintf(&b, "Автор: %s", user(ev.PR.Author))

	case domain.EventPRUnassigned:
		fmt.Fprintf(&b, "🕊 С вас сняли pull request: %s\n", prLine(ev.PR))
		b.WriteString("Можно выдохнуть и вернуться к своим задачам.")

	case domain.EventReviewRequested:
		fmt.Fprintf(&b, "👀 Вас ждут на ревью: %s\n", prLine(ev.PR))
		fmt.Fprintf(&b, "Автор: %s", user(ev.PR.Author))

	case domain.EventPRMerged:
		fmt.Fprintf(&b, "🎉 Pull request смержен: %s\n", prLine(ev.PR))
		b.WriteString("Спасибо всем, кто ревьюил!")

	case domain.EventPRClosed:
		fmt.Fprintf(&b, "🗂 Pull request закрыт без слияния: %s", prLine(ev.PR))

	case domain.EventReviewSubmitted:
		icon, verdict, ok := reviewVerdict(ev.Review)
		if !ok {
			return "", false
		}
		// A "commented" review without a summary is just a wrapper around inline
		// comments, and those arrive as separate events.
		if ev.Review == domain.ReviewCommented && strings.TrimSpace(ev.Body) == "" {
			return "", false
		}
		fmt.Fprintf(&b, "%s Ревью от %s: %s\n%s", icon, user(ev.Actor), verdict, prLine(ev.PR))
		writeQuote(&b, ev.Body)
		writeOpenLink(&b, ev)

	case domain.EventReviewComment:
		fmt.Fprintf(&b, "💬 Новый комментарий от %s\n%s", user(ev.Actor), prLine(ev.PR))
		if ev.Path != "" {
			fmt.Fprintf(&b, "\n📄 <code>%s</code>", esc(ev.Path))
		}
		writeQuote(&b, ev.Body)
		writeOpenLink(&b, ev)

	case domain.EventThreadResolved:
		fmt.Fprintf(&b, "🧵 Обсуждение отмечено решённым (%s)\n%s", user(ev.Actor), prLine(ev.PR))
		writeOpenLink(&b, ev)

	case domain.EventThreadUnresolved:
		fmt.Fprintf(&b, "🧵 Обсуждение снова открыто (%s)\n%s", user(ev.Actor), prLine(ev.PR))
		writeOpenLink(&b, ev)

	default:
		return "", false
	}

	return b.String(), true
}

func reviewVerdict(s domain.ReviewState) (icon, verdict string, ok bool) {
	switch s {
	case domain.ReviewApproved:
		return "✅", "approve", true
	case domain.ReviewChangesRequested:
		return "🛠", "нужны правки", true
	case domain.ReviewCommented:
		return "💬", "есть комментарии", true
	default:
		return "", "", false
	}
}

// prLine renders "<b>Title</b> — owner/repo#42" with the second part as a link.
func prLine(pr domain.PullRequest) string {
	return fmt.Sprintf(`<b>%s</b> — <a href="%s">%s#%d</a>`,
		esc(pr.Title), esc(pr.URL), esc(pr.Repo), pr.Number)
}

func user(login string) string {
	if login == "" {
		return "кто-то"
	}
	return "@" + esc(login)
}

func writeQuote(b *strings.Builder, body string) {
	body = strings.TrimSpace(body)
	if body == "" {
		return
	}
	fmt.Fprintf(b, "\n<blockquote>%s</blockquote>", esc(truncate(body, maxQuoteLen)))
}

func writeOpenLink(b *strings.Builder, ev domain.PREvent) {
	if ev.URL == "" || ev.URL == ev.PR.URL {
		return
	}
	fmt.Fprintf(b, "\n<a href=\"%s\">Открыть на GitHub →</a>", esc(ev.URL))
}

func truncate(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return strings.TrimSpace(string(runes[:limit])) + "…"
}

func esc(s string) string { return html.EscapeString(s) }
