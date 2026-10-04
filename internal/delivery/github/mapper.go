package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/katshuuu/pr-pigeon/internal/domain"
)

var errUnsupportedEvent = errors.New("unsupported event")

// parseEvent turns a raw webhook into a domain event.
// A nil event with nil error means "valid, but we don't care about this action".
func parseEvent(name string, body []byte) (*domain.PREvent, error) {
	switch name {
	case "pull_request":
		var p pullRequestEvent
		if err := json.Unmarshal(body, &p); err != nil {
			return nil, fmt.Errorf("decode %s: %w", name, err)
		}
		return fromPullRequest(p), nil

	case "pull_request_review":
		var p pullRequestReviewEvent
		if err := json.Unmarshal(body, &p); err != nil {
			return nil, fmt.Errorf("decode %s: %w", name, err)
		}
		return fromReview(p), nil

	case "pull_request_review_comment":
		var p pullRequestReviewCommentEvent
		if err := json.Unmarshal(body, &p); err != nil {
			return nil, fmt.Errorf("decode %s: %w", name, err)
		}
		return fromReviewComment(p), nil

	case "pull_request_review_thread":
		var p pullRequestReviewThreadEvent
		if err := json.Unmarshal(body, &p); err != nil {
			return nil, fmt.Errorf("decode %s: %w", name, err)
		}
		return fromReviewThread(p), nil

	default:
		return nil, errUnsupportedEvent
	}
}

func fromPullRequest(p pullRequestEvent) *domain.PREvent {
	ev := &domain.PREvent{PR: toPR(p.PullRequest, p.Repository), Actor: p.Sender.Login}

	switch p.Action {
	case "assigned", "unassigned":
		if p.Assignee == nil {
			return nil
		}
		ev.Kind = domain.EventPRAssigned
		if p.Action == "unassigned" {
			ev.Kind = domain.EventPRUnassigned
		}
		ev.Target = p.Assignee.Login

	case "review_requested":
		// requested_reviewer is absent when a whole team is requested
		if p.RequestedReviewer == nil {
			return nil
		}
		ev.Kind = domain.EventReviewRequested
		ev.Target = p.RequestedReviewer.Login

	case "closed":
		ev.Kind = domain.EventPRClosed
		if p.PullRequest.Merged {
			ev.Kind = domain.EventPRMerged
		}

	default:
		return nil
	}
	return ev
}

func fromReview(p pullRequestReviewEvent) *domain.PREvent {
	if p.Action != "submitted" {
		return nil
	}
	return &domain.PREvent{
		Kind:   domain.EventReviewSubmitted,
		PR:     toPR(p.PullRequest, p.Repository),
		Actor:  p.Sender.Login,
		Review: domain.ReviewState(strings.ToLower(p.Review.State)),
		Body:   p.Review.Body,
		URL:    p.Review.HTMLURL,
	}
}

func fromReviewComment(p pullRequestReviewCommentEvent) *domain.PREvent {
	if p.Action != "created" {
		return nil
	}
	return &domain.PREvent{
		Kind:  domain.EventReviewComment,
		PR:    toPR(p.PullRequest, p.Repository),
		Actor: p.Sender.Login,
		Body:  p.Comment.Body,
		Path:  p.Comment.Path,
		URL:   p.Comment.HTMLURL,
	}
}

func fromReviewThread(p pullRequestReviewThreadEvent) *domain.PREvent {
	ev := &domain.PREvent{PR: toPR(p.PullRequest, p.Repository), Actor: p.Sender.Login}
	switch p.Action {
	case "resolved":
		ev.Kind = domain.EventThreadResolved
	case "unresolved":
		ev.Kind = domain.EventThreadUnresolved
	default:
		return nil
	}
	if len(p.Thread.Comments) > 0 {
		ev.Path = p.Thread.Comments[0].Path
		ev.URL = p.Thread.Comments[0].HTMLURL
	}
	return ev
}

func toPR(pr ghPullRequest, repo ghRepository) domain.PullRequest {
	assignees := make([]string, 0, len(pr.Assignees))
	for _, a := range pr.Assignees {
		assignees = append(assignees, a.Login)
	}
	return domain.PullRequest{
		Repo:      repo.FullName,
		Number:    pr.Number,
		Title:     pr.Title,
		URL:       pr.HTMLURL,
		Author:    pr.User.Login,
		Assignees: assignees,
	}
}
