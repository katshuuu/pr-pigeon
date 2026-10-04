package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/katshuuu/pr-pigeon/internal/domain"
)

var testPR = domain.PullRequest{
	Repo:      "katshuuu/pr-pigeon",
	Number:    7,
	Title:     "Add <retry> for telegram client",
	URL:       "https://github.com/katshuuu/pr-pigeon/pull/7",
	Author:    "alice",
	Assignees: []string{"bob", "katshuuu"},
}

func TestRecipients(t *testing.T) {
	tests := []struct {
		name string
		ev   domain.PREvent
		want []string
	}{
		{
			name: "self assignment still notifies",
			ev:   domain.PREvent{Kind: domain.EventPRAssigned, PR: testPR, Actor: "katshuuu", Target: "katshuuu"},
			want: []string{"katshuuu"},
		},
		{
			name: "team review request has no person",
			ev:   domain.PREvent{Kind: domain.EventReviewRequested, PR: testPR},
			want: nil,
		},
		{
			name: "comment skips its author",
			ev:   domain.PREvent{Kind: domain.EventReviewComment, PR: testPR, Actor: "BOB"},
			want: []string{"katshuuu", "alice"},
		},
		{
			name: "author who is also assignee counted once",
			ev: domain.PREvent{
				Kind:  domain.EventPRMerged,
				PR:    domain.PullRequest{Author: "alice", Assignees: []string{"Alice", "bob"}},
				Actor: "bob",
			},
			want: []string{"Alice"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := recipients(tt.ev); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("recipients() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNotifier_Notify(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	repo.subs[100] = domain.Subscription{ChatID: 100, GitHubLogin: "KatShuuu"}
	repo.subs[200] = domain.Subscription{ChatID: 200, GitHubLogin: "alice"}
	sender := &fakeSender{}
	n := NewNotifier(repo, sender, discardLogger())

	sent, err := n.Notify(ctx, domain.PREvent{
		Kind: domain.EventPRAssigned, PR: testPR, Actor: "alice", Target: "katshuuu",
	})
	if err != nil || sent != 1 {
		t.Fatalf("Notify = %d, %v", sent, err)
	}

	msg := sender.sent[0]
	if msg.chatID != 100 {
		t.Errorf("sent to %d, want 100", msg.chatID)
	}
	for _, want := range []string{
		"На вас назначен pull request:",
		"<b>Add &lt;retry&gt; for telegram client</b>",
		`<a href="https://github.com/katshuuu/pr-pigeon/pull/7">katshuuu/pr-pigeon#7</a>`,
		"@alice",
	} {
		if !strings.Contains(msg.text, want) {
			t.Errorf("message %q does not contain %q", msg.text, want)
		}
	}
}

func TestNotifier_PartialFailure(t *testing.T) {
	repo := newFakeRepo()
	repo.subs[1] = domain.Subscription{ChatID: 1, GitHubLogin: "bob"}
	repo.subs[2] = domain.Subscription{ChatID: 2, GitHubLogin: "alice"}
	sender := &fakeSender{failTo: map[int64]bool{1: true}}
	n := NewNotifier(repo, sender, discardLogger())

	sent, err := n.Notify(context.Background(), domain.PREvent{
		Kind: domain.EventPRMerged, PR: testPR, Actor: "katshuuu",
	})
	if sent != 1 || err == nil {
		t.Fatalf("Notify = %d, %v; want 1 and an error", sent, err)
	}
}

func TestNotifier_LookupError(t *testing.T) {
	repo := newFakeRepo()
	repo.failWith = errors.New("db is down")
	n := NewNotifier(repo, &fakeSender{}, discardLogger())

	_, err := n.Notify(context.Background(), domain.PREvent{Kind: domain.EventPRAssigned, PR: testPR, Target: "bob"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNotifier_SkipsSilentEvents(t *testing.T) {
	sender := &fakeSender{}
	n := NewNotifier(newFakeRepo(), sender, discardLogger())

	events := []domain.PREvent{
		{Kind: domain.EventReviewSubmitted, PR: testPR, Review: domain.ReviewCommented},
		{Kind: domain.EventReviewSubmitted, PR: testPR, Review: "dismissed"},
		{Kind: "something_new", PR: testPR},
		{Kind: domain.EventReviewRequested, PR: testPR},
	}
	for _, ev := range events {
		if sent, err := n.Notify(context.Background(), ev); sent != 0 || err != nil {
			t.Errorf("Notify(%s) = %d, %v", ev.Kind, sent, err)
		}
	}
	if len(sender.sent) != 0 {
		t.Errorf("unexpected messages: %v", sender.sent)
	}
}

func TestRender(t *testing.T) {
	long := strings.Repeat("я", maxQuoteLen+50)
	tests := []struct {
		name string
		ev   domain.PREvent
		want []string
	}{
		{"unassigned", domain.PREvent{Kind: domain.EventPRUnassigned, PR: testPR}, []string{"С вас сняли"}},
		{"review requested", domain.PREvent{Kind: domain.EventReviewRequested, PR: testPR}, []string{"Вас ждут на ревью", "@alice"}},
		{"merged", domain.PREvent{Kind: domain.EventPRMerged, PR: testPR}, []string{"смержен"}},
		{"closed", domain.PREvent{Kind: domain.EventPRClosed, PR: testPR}, []string{"закрыт без слияния"}},
		{
			"approved",
			domain.PREvent{Kind: domain.EventReviewSubmitted, PR: testPR, Actor: "bob", Review: domain.ReviewApproved},
			[]string{"✅ Ревью от @bob: approve"},
		},
		{
			"changes requested with long body",
			domain.PREvent{
				Kind: domain.EventReviewSubmitted, PR: testPR, Actor: "bob",
				Review: domain.ReviewChangesRequested, Body: long, URL: testPR.URL + "#pullrequestreview-1",
			},
			[]string{"Ревью от @bob: нужны правки", "…</blockquote>", "Открыть на GitHub"},
		},
		{
			"comment",
			domain.PREvent{
				Kind: domain.EventReviewComment, PR: testPR, Actor: "bob",
				Body: "a && b", Path: "internal/app/app.go",
			},
			[]string{"<code>internal/app/app.go</code>", "a &amp;&amp; b"},
		},
		{"resolved", domain.PREvent{Kind: domain.EventThreadResolved, PR: testPR, Actor: "bob"}, []string{"решённым (@bob)"}},
		{"unresolved", domain.PREvent{Kind: domain.EventThreadUnresolved, PR: testPR}, []string{"снова открыто (кто-то)"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, ok := render(tt.ev)
			if !ok {
				t.Fatal("render returned !ok")
			}
			for _, w := range tt.want {
				if !strings.Contains(text, w) {
					t.Errorf("%q does not contain %q", text, w)
				}
			}
		})
	}
}
