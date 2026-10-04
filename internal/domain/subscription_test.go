package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeLogin(t *testing.T) {
	tests := []struct {
		in, want string
		ok       bool
	}{
		{"octocat", "octocat", true},
		{"  @Octo-Cat  ", "Octo-Cat", true},
		{"https://github.com/katshuuu/", "katshuuu", true},
		{"github.com/torvalds", "torvalds", true},
		{"", "", false},
		{"-octocat", "", false},
		{"octo--cat", "", false},
		{"octo_cat", "", false},
		{"github.com/user/repo", "", false},
		{"кошка", "", false},
		{strings.Repeat("a", 40), "", false},
		{strings.Repeat("a", 39), strings.Repeat("a", 39), true},
	}
	for _, tt := range tests {
		got, err := NormalizeLogin(tt.in)
		if tt.ok && (err != nil || got != tt.want) {
			t.Errorf("NormalizeLogin(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
		if !tt.ok && !errors.Is(err, ErrInvalidLogin) {
			t.Errorf("NormalizeLogin(%q) error = %v; want ErrInvalidLogin", tt.in, err)
		}
	}
}
