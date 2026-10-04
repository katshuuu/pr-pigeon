// Package domain holds the core types shared by all layers.
package domain

import (
	"regexp"
	"strings"
	"time"
)

// Subscription links a Telegram chat to a GitHub account.
type Subscription struct {
	ChatID      int64
	GitHubLogin string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

const maxLoginLen = 39

var loginRe = regexp.MustCompile(`^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$`)

// NormalizeLogin accepts what people usually paste into a chat
// ("octocat", "@octocat", "https://github.com/octocat/") and returns a bare login.
func NormalizeLogin(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "@")
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "www.")
	s = strings.TrimPrefix(s, "github.com/")
	s = strings.TrimSuffix(s, "/")

	if s == "" || len(s) > maxLoginLen || !loginRe.MatchString(s) {
		return "", ErrInvalidLogin
	}
	return s, nil
}
