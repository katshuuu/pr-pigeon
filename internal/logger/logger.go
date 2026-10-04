// Package logger builds the application-wide slog logger.
package logger

import (
	"io"
	"log/slog"

	"github.com/katshuuu/pr-pigeon/internal/config"
)

func New(cfg config.LogConfig, w io.Writer) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:     cfg.Level,
		AddSource: cfg.Level <= slog.LevelDebug,
	}

	var h slog.Handler
	if cfg.Format == "text" {
		h = slog.NewTextHandler(w, opts)
	} else {
		h = slog.NewJSONHandler(w, opts)
	}
	return slog.New(h).With("app", "pr-pigeon")
}
