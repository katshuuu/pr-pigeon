// Package config reads application settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Telegram TelegramConfig
	HTTP     HTTPConfig
	GitHub   GitHubConfig
	DB       DBConfig
	Log      LogConfig
}

type TelegramConfig struct {
	Token string
	Debug bool
}

type HTTPConfig struct {
	Addr        string
	TLSCertFile string
	TLSKeyFile  string

	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

// TLSEnabled reports whether both certificate and key are configured.
func (c HTTPConfig) TLSEnabled() bool {
	return c.TLSCertFile != "" && c.TLSKeyFile != ""
}

type GitHubConfig struct {
	Secret        string
	NotifyTimeout time.Duration
}

type DBConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	Name     string
	SSLMode  string
}

// Enabled reports whether PostgreSQL should be used instead of in-memory storage.
func (c DBConfig) Enabled() bool { return c.Host != "" }

// DSN builds a postgres:// connection string with properly escaped credentials.
func (c DBConfig) DSN() string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.Username, c.Password),
		Host:   net.JoinHostPort(c.Host, c.Port),
		Path:   "/" + c.Name,
	}
	q := url.Values{}
	q.Set("sslmode", c.SSLMode)
	u.RawQuery = q.Encode()
	return u.String()
}

type LogConfig struct {
	Level  slog.Level
	Format string // "json" or "text"
}

// Load reads configuration from the process environment.
func Load() (Config, error) {
	return load(os.LookupEnv)
}

func load(lookup func(string) (string, bool)) (Config, error) {
	r := &reader{lookup: lookup}

	cfg := Config{
		Telegram: TelegramConfig{
			Token: r.required("TELEGRAM_BOT_TOKEN"),
			Debug: r.bool("TELEGRAM_DEBUG", false),
		},
		HTTP: HTTPConfig{
			Addr:              normalizeAddr(r.str("PORT", ":8080")),
			TLSCertFile:       r.str("TLS_CERT_FILE", ""),
			TLSKeyFile:        r.str("TLS_KEY_FILE", ""),
			ReadHeaderTimeout: r.duration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			ReadTimeout:       r.duration("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:      r.duration("HTTP_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:       r.duration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout:   r.duration("SHUTDOWN_TIMEOUT", 10*time.Second),
		},
		GitHub: GitHubConfig{
			Secret:        r.required("GITHUB_SECRET"),
			NotifyTimeout: r.duration("NOTIFY_TIMEOUT", 8*time.Second),
		},
		DB: DBConfig{
			Host:     r.str("DB_HOST", ""),
			Port:     r.str("DB_PORT", "5432"),
			Username: r.str("DB_USERNAME", ""),
			Password: r.str("DB_PASSWORD", ""),
			Name:     r.str("DB_NAME", "pigeon"),
			SSLMode:  r.str("DB_SSLMODE", "disable"),
		},
		Log: LogConfig{
			Level:  r.level("LOG_LEVEL", slog.LevelInfo),
			Format: strings.ToLower(r.str("LOG_FORMAT", "json")),
		},
	}

	r.validate(cfg)
	if len(r.errs) > 0 {
		return Config{}, errors.Join(r.errs...)
	}
	return cfg, nil
}

// normalizeAddr lets PORT be either "8080" or ":8080".
func normalizeAddr(v string) string {
	if _, err := strconv.Atoi(v); err == nil {
		return ":" + v
	}
	return v
}

type reader struct {
	lookup func(string) (string, bool)
	errs   []error
}

func (r *reader) str(key, def string) string {
	if v, ok := r.lookup(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

func (r *reader) required(key string) string {
	v := r.str(key, "")
	if v == "" {
		r.errs = append(r.errs, fmt.Errorf("%s is required", key))
	}
	return v
}

func (r *reader) bool(key string, def bool) bool {
	raw := r.str(key, "")
	if raw == "" {
		return def
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		r.errs = append(r.errs, fmt.Errorf("%s: %q is not a bool", key, raw))
		return def
	}
	return v
}

func (r *reader) duration(key string, def time.Duration) time.Duration {
	raw := r.str(key, "")
	if raw == "" {
		return def
	}
	v, err := time.ParseDuration(raw)
	if err != nil || v <= 0 {
		r.errs = append(r.errs, fmt.Errorf("%s: %q is not a positive duration (e.g. 10s)", key, raw))
		return def
	}
	return v
}

func (r *reader) level(key string, def slog.Level) slog.Level {
	raw := r.str(key, "")
	if raw == "" {
		return def
	}
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(raw)); err != nil {
		r.errs = append(r.errs, fmt.Errorf("%s: unknown level %q (debug, info, warn, error)", key, raw))
		return def
	}
	return lvl
}

func (r *reader) validate(cfg Config) {
	if (cfg.HTTP.TLSCertFile == "") != (cfg.HTTP.TLSKeyFile == "") {
		r.errs = append(r.errs, errors.New("TLS_CERT_FILE and TLS_KEY_FILE must be set together"))
	}
	if cfg.Log.Format != "json" && cfg.Log.Format != "text" {
		r.errs = append(r.errs, fmt.Errorf("LOG_FORMAT: %q, expected json or text", cfg.Log.Format))
	}
	if cfg.DB.Enabled() && cfg.DB.Username == "" {
		r.errs = append(r.errs, errors.New("DB_USERNAME is required when DB_HOST is set"))
	}
	// GitHub waits ~10s for a response, and the notification must be sent
	// before the server gives up on writing it.
	if cfg.GitHub.NotifyTimeout >= cfg.HTTP.WriteTimeout {
		r.errs = append(r.errs, errors.New("NOTIFY_TIMEOUT must be less than HTTP_WRITE_TIMEOUT"))
	}
}
