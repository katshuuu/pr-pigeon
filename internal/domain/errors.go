package domain

import "errors"

var (
	ErrNotFound     = errors.New("not found")
	ErrInvalidLogin = errors.New("invalid github login")
)
