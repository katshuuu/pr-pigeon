package github

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/katshuuu/pr-pigeon/internal/domain"
)

var testSecret = []byte("it's a secret to everybody")

func sign(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

type fakeNotifier struct {
	got  []domain.PREvent
	sent int
	err  error
}

func (f *fakeNotifier) Notify(ctx context.Context, ev domain.PREvent) (int, error) {
	if _, ok := ctx.Deadline(); !ok {
		return 0, errors.New("notify must be called with a deadline")
	}
	f.got = append(f.got, ev)
	return f.sent, f.err
}

func newHandler(n Notifier) *Handler {
	return NewHandler(string(testSecret), n, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func request(event string, body []byte, signature string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/github/webhook", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(eventHeader, event)
	r.Header.Set(deliveryHeader, "72d3162e-cc78-11e3-81ab-4c9367dc0958")
	if signature != "" {
		r.Header.Set(signatureHeader, signature)
	}
	return r
}

func TestHandler_Events(t *testing.T) {
	tests := []struct {
		fixture string
		event   string
		want    domain.PREvent
	}{
		{
			fixture: "pull_request_assigned.json",
			event:   "pull_request",
			want:    domain.PREvent{Kind: domain.EventPRAssigned, Actor: "alice-dev", Target: "katshuuu"},
		},
		{
			fixture: "pull_request_review_requested.json",
			event:   "pull_request",
			want:    domain.PREvent{Kind: domain.EventReviewRequested, Actor: "alice-dev", Target: "katshuuu"},
		},
		{
			fixture: "pull_request_closed_merged.json",
			event:   "pull_request",
			want:    domain.PREvent{Kind: domain.EventPRMerged, Actor: "bob-reviewer"},
		},
		{
			fixture: "pull_request_review_submitted.json",
			event:   "pull_request_review",
			want: domain.PREvent{
				Kind: domain.EventReviewSubmitted, Actor: "bob-reviewer",
				Review: domain.ReviewChangesRequested,
				Body:   "Логика ретраев ок, но давай вынесем backoff в конфиг.",
				URL:    "https://github.com/katshuuu/pr-pigeon/pull/7#pullrequestreview-9001",
			},
		},
		{
			fixture: "pull_request_review_comment_created.json",
			event:   "pull_request_review_comment",
			want: domain.PREvent{
				Kind: domain.EventReviewComment, Actor: "bob-reviewer",
				Body: "Тут лучше `errors.As`, чтобы достать RetryAfter.",
				Path: "internal/adapters/clients/telegram/client.go",
				URL:  "https://github.com/katshuuu/pr-pigeon/pull/7#discussion_r5150",
			},
		},
		{
			fixture: "pull_request_review_thread_resolved.json",
			event:   "pull_request_review_thread",
			want: domain.PREvent{
				Kind: domain.EventThreadResolved, Actor: "alice-dev",
				Path: "internal/adapters/clients/telegram/client.go",
				URL:  "https://github.com/katshuuu/pr-pigeon/pull/7#discussion_r5150",
			},
		},
	}

	wantPR := domain.PullRequest{
		Repo:      "katshuuu/pr-pigeon",
		Number:    7,
		Title:     "Retry Telegram sends on 429",
		URL:       "https://github.com/katshuuu/pr-pigeon/pull/7",
		Author:    "alice-dev",
		Assignees: []string{"katshuuu"},
	}

	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			n := &fakeNotifier{sent: 1}
			body := fixture(t, tt.fixture)
			w := httptest.NewRecorder()

			newHandler(n).ServeHTTP(w, request(tt.event, body, sign(testSecret, body)))

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", w.Code, w.Body)
			}
			var resp response
			_ = json.Unmarshal(w.Body.Bytes(), &resp)
			if resp.Status != "ok" || resp.Notified != 1 {
				t.Errorf("response = %+v", resp)
			}
			if len(n.got) != 1 {
				t.Fatalf("notifier called %d times", len(n.got))
			}

			got := n.got[0]
			tt.want.PR = wantPR
			if got.Kind != tt.want.Kind || got.Actor != tt.want.Actor || got.Target != tt.want.Target ||
				got.Review != tt.want.Review || got.Body != tt.want.Body || got.Path != tt.want.Path || got.URL != tt.want.URL {
				t.Errorf("event =\n%+v\nwant\n%+v", got, tt.want)
			}
			if got.PR.Repo != wantPR.Repo || got.PR.Title != wantPR.Title || got.PR.Author != wantPR.Author ||
				len(got.PR.Assignees) != 1 || got.PR.Assignees[0] != "katshuuu" {
				t.Errorf("pr = %+v", got.PR)
			}
		})
	}
}

func TestHandler_Rejections(t *testing.T) {
	valid := fixture(t, "pull_request_assigned.json")

	tests := []struct {
		name   string
		req    func() *http.Request
		status int
		result string
	}{
		{
			name:   "no signature",
			req:    func() *http.Request { return request("pull_request", valid, "") },
			status: http.StatusUnauthorized,
		},
		{
			name:   "wrong secret",
			req:    func() *http.Request { return request("pull_request", valid, sign([]byte("nope"), valid)) },
			status: http.StatusUnauthorized,
		},
		{
			name:   "sha1 signature",
			req:    func() *http.Request { return request("pull_request", valid, "sha1=deadbeef") },
			status: http.StatusUnauthorized,
		},
		{
			name:   "not hex",
			req:    func() *http.Request { return request("pull_request", valid, "sha256=zz") },
			status: http.StatusUnauthorized,
		},
		{
			name: "tampered body",
			req: func() *http.Request {
				sig := sign(testSecret, valid)
				return request("pull_request", bytes.Replace(valid, []byte("katshuuu"), []byte("mallory"), 1), sig)
			},
			status: http.StatusUnauthorized,
		},
		{
			name: "form encoded",
			req: func() *http.Request {
				r := request("pull_request", valid, sign(testSecret, valid))
				r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				return r
			},
			status: http.StatusUnsupportedMediaType,
		},
		{
			name: "too large",
			req: func() *http.Request {
				big := bytes.Repeat([]byte("a"), maxBodyBytes+1)
				return request("pull_request", big, sign(testSecret, big))
			},
			status: http.StatusRequestEntityTooLarge,
		},
		{
			name: "broken json",
			req: func() *http.Request {
				b := []byte(`{"action": "assigned",`)
				return request("pull_request", b, sign(testSecret, b))
			},
			status: http.StatusBadRequest,
		},
		{
			name: "ping",
			req: func() *http.Request {
				b := []byte(`{"zen": "Keep it logically awesome."}`)
				return request("ping", b, sign(testSecret, b))
			},
			status: http.StatusOK,
			result: "pong",
		},
		{
			name: "unsupported event",
			req: func() *http.Request {
				b := []byte(`{"ref": "refs/heads/main"}`)
				return request("push", b, sign(testSecret, b))
			},
			status: http.StatusAccepted,
			result: "ignored",
		},
		{
			name: "uninteresting action",
			req: func() *http.Request {
				b := fixture(t, "pull_request_synchronize.json")
				return request("pull_request", b, sign(testSecret, b))
			},
			status: http.StatusAccepted,
			result: "ignored",
		},
		{
			name: "team review request",
			req: func() *http.Request {
				b := fixture(t, "pull_request_review_requested_team.json")
				return request("pull_request", b, sign(testSecret, b))
			},
			status: http.StatusAccepted,
			result: "ignored",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &fakeNotifier{}
			w := httptest.NewRecorder()
			newHandler(n).ServeHTTP(w, tt.req())

			if w.Code != tt.status {
				t.Fatalf("status = %d, want %d, body = %s", w.Code, tt.status, w.Body)
			}
			if tt.result != "" && !strings.Contains(w.Body.String(), `"status":"`+tt.result+`"`) {
				t.Errorf("body = %s", w.Body)
			}
			if len(n.got) != 0 {
				t.Errorf("notifier must not be called, got %v", n.got)
			}
		})
	}
}

func TestHandler_NotifierError(t *testing.T) {
	n := &fakeNotifier{err: errors.New("telegram is down")}
	body := fixture(t, "pull_request_assigned.json")
	w := httptest.NewRecorder()

	newHandler(n).ServeHTTP(w, request("pull_request", body, sign(testSecret, body)))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestParseEvent_IgnoredActions(t *testing.T) {
	cases := map[string]string{
		"pull_request":                `{"action": "assigned"}`,
		"pull_request_review":         `{"action": "dismissed"}`,
		"pull_request_review_comment": `{"action": "edited"}`,
		"pull_request_review_thread":  `{"action": "something_new"}`,
	}
	for event, body := range cases {
		ev, err := parseEvent(event, []byte(body))
		if err != nil || ev != nil {
			t.Errorf("parseEvent(%s, %s) = %v, %v", event, body, ev, err)
		}
	}

	for _, event := range []string{"pull_request_review", "pull_request_review_comment", "pull_request_review_thread"} {
		if _, err := parseEvent(event, []byte(`[]`)); err == nil {
			t.Errorf("parseEvent(%s) with array: expected error", event)
		}
	}
}

func TestParseEvent_ClosedWithoutMerge(t *testing.T) {
	ev, err := parseEvent("pull_request", []byte(`{"action":"closed","pull_request":{"merged":false}}`))
	if err != nil || ev == nil || ev.Kind != domain.EventPRClosed {
		t.Fatalf("got %+v, %v", ev, err)
	}

	ev, err = parseEvent("pull_request", []byte(`{"action":"unassigned","assignee":{"login":"bob"}}`))
	if err != nil || ev == nil || ev.Kind != domain.EventPRUnassigned || ev.Target != "bob" {
		t.Fatalf("got %+v, %v", ev, err)
	}

	ev, err = parseEvent("pull_request_review_thread", []byte(`{"action":"unresolved","thread":{"comments":[]}}`))
	if err != nil || ev == nil || ev.Kind != domain.EventThreadUnresolved || ev.URL != "" {
		t.Fatalf("got %+v, %v", ev, err)
	}
}
