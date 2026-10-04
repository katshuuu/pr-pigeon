package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

func base() map[string]string {
	return map[string]string{
		"TELEGRAM_BOT_TOKEN": "123:abc",
		"GITHUB_SECRET":      "s3cret",
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := load(env(base()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTP.Addr != ":8080" {
		t.Errorf("addr = %q, want :8080", cfg.HTTP.Addr)
	}
	if cfg.HTTP.TLSEnabled() {
		t.Error("TLS should be disabled by default")
	}
	if cfg.DB.Enabled() {
		t.Error("DB should be disabled without DB_HOST")
	}
	if cfg.Log.Level != slog.LevelInfo || cfg.Log.Format != "json" {
		t.Errorf("log = %+v", cfg.Log)
	}
	if cfg.HTTP.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("read header timeout = %v", cfg.HTTP.ReadHeaderTimeout)
	}
}

func TestLoadOverrides(t *testing.T) {
	m := base()
	m["PORT"] = "8443"
	m["TLS_CERT_FILE"] = "/certs/server.crt"
	m["TLS_KEY_FILE"] = "/certs/server.key"
	m["LOG_LEVEL"] = "DEBUG"
	m["LOG_FORMAT"] = "text"
	m["DB_HOST"] = "db"
	m["DB_USERNAME"] = "pigeon"
	m["DB_PASSWORD"] = "p@ss:word"
	m["HTTP_WRITE_TIMEOUT"] = "20s"

	cfg, err := load(env(m))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTP.Addr != ":8443" || !cfg.HTTP.TLSEnabled() {
		t.Errorf("http = %+v", cfg.HTTP)
	}
	if cfg.Log.Level != slog.LevelDebug {
		t.Errorf("level = %v", cfg.Log.Level)
	}
	want := "postgres://pigeon:p%40ss%3Aword@db:5432/pigeon?sslmode=disable"
	if got := cfg.DB.DSN(); got != want {
		t.Errorf("dsn = %q, want %q", got, want)
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(map[string]string)
		wantErr string
	}{
		{"no token", func(m map[string]string) { delete(m, "TELEGRAM_BOT_TOKEN") }, "TELEGRAM_BOT_TOKEN is required"},
		{"blank secret", func(m map[string]string) { m["GITHUB_SECRET"] = "  " }, "GITHUB_SECRET is required"},
		{"half tls", func(m map[string]string) { m["TLS_CERT_FILE"] = "a.crt" }, "must be set together"},
		{"bad level", func(m map[string]string) { m["LOG_LEVEL"] = "loud" }, "unknown level"},
		{"bad format", func(m map[string]string) { m["LOG_FORMAT"] = "xml" }, "LOG_FORMAT"},
		{"bad duration", func(m map[string]string) { m["HTTP_READ_TIMEOUT"] = "soon" }, "HTTP_READ_TIMEOUT"},
		{"bad bool", func(m map[string]string) { m["TELEGRAM_DEBUG"] = "maybe" }, "TELEGRAM_DEBUG"},
		{"db without user", func(m map[string]string) { m["DB_HOST"] = "db" }, "DB_USERNAME"},
		{"notify timeout too big", func(m map[string]string) { m["NOTIFY_TIMEOUT"] = "30s" }, "NOTIFY_TIMEOUT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := base()
			tt.mutate(m)
			_, err := load(env(m))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}
