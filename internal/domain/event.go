package domain

// EventKind is what happened to a pull request, independent of how we learned about it.
type EventKind string

const (
	EventPRAssigned       EventKind = "pr_assigned"
	EventPRUnassigned     EventKind = "pr_unassigned"
	EventReviewRequested  EventKind = "review_requested"
	EventPRMerged         EventKind = "pr_merged"
	EventPRClosed         EventKind = "pr_closed"
	EventReviewSubmitted  EventKind = "review_submitted"
	EventReviewComment    EventKind = "review_comment"
	EventThreadResolved   EventKind = "thread_resolved"
	EventThreadUnresolved EventKind = "thread_unresolved"
)

type ReviewState string

const (
	ReviewApproved         ReviewState = "approved"
	ReviewChangesRequested ReviewState = "changes_requested"
	ReviewCommented        ReviewState = "commented"
)

type PullRequest struct {
	Repo      string // owner/name
	Number    int
	Title     string
	URL       string
	Author    string
	Assignees []string
}

type PREvent struct {
	Kind  EventKind
	PR    PullRequest
	Actor string // who triggered the event

	// Target is the person the event is about: new assignee or requested reviewer.
	Target string

	Review ReviewState
	Body   string // review or comment text
	Path   string // file the comment is attached to
	URL    string // direct link to the review/comment, if any
}
