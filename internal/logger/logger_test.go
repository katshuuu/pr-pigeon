package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/katshuuu/pr-pigeon/internal/config"
)

func TestJSONLogger(t *testing.T) {
	var buf bytes.Buffer
	log := New(config.LogConfig{Level: slog.LevelInfo, Format: "json"}, &buf)
	log.Debug("hidden")
	log.Info("hello", "k", "v")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("expected exactly one json line, got %q: %v", buf.String(), err)
	}
	if rec["msg"] != "hello" || rec["app"] != "pr-pigeon" || rec["k"] != "v" {
		t.Errorf("unexpected record: %v", rec)
	}
}

func TestTextLoggerDebug(t *testing.T) {
	var buf bytes.Buffer
	log := New(config.LogConfig{Level: slog.LevelDebug, Format: "text"}, &buf)
	log.Debug("visible")
	if !strings.Contains(buf.String(), "msg=visible") || !strings.Contains(buf.String(), "source=") {
		t.Errorf("unexpected output: %q", buf.String())
	}
}
