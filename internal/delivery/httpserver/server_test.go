package httpserver

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/katshuuu/pr-pigeon/internal/config"
)

func TestChainAndMiddleware(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /boom", func(http.ResponseWriter, *http.Request) { panic("oops") })
	mux.HandleFunc("GET /teapot", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })

	h := Chain(mux, LogRequests(log), Recover(log))

	for path, want := range map[string]int{"/healthz": 200, "/boom": 500, "/teapot": 418} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != want {
			t.Errorf("%s: status = %d, want %d", path, w.Code, want)
		}
	}

	out := buf.String()
	for _, want := range []string{"panic in http handler", "level=ERROR msg=\"http request\"", "level=WARN", "level=DEBUG"} {
		if !strings.Contains(out, want) {
			t.Errorf("log does not contain %q:\n%s", want, out)
		}
	}
}

func TestTLSConfig(t *testing.T) {
	c := TLSConfig()
	if c.MinVersion != tls.VersionTLS12 || len(c.CipherSuites) == 0 {
		t.Errorf("weak tls config: %+v", c)
	}
	for _, id := range c.CipherSuites {
		for _, insecure := range tls.InsecureCipherSuites() {
			if id == insecure.ID {
				t.Errorf("insecure suite %s", insecure.Name)
			}
		}
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}

func TestServerRunAndShutdown(t *testing.T) {
	cfg := config.HTTPConfig{
		Addr:              freeAddr(t),
		ReadHeaderTimeout: time.Second,
		ReadTimeout:       time.Second,
		WriteTimeout:      time.Second,
		IdleTimeout:       time.Second,
		ShutdownTimeout:   time.Second,
	}
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })
	srv := New(cfg, h, slog.New(slog.NewTextHandler(io.Discard, nil)))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx) }()

	var err error
	for range 50 {
		var resp *http.Response
		if resp, err = http.Get("http://" + cfg.Addr); err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("server did not start: %v", err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop")
	}
}

func TestServerTLSMissingCert(t *testing.T) {
	cfg := config.HTTPConfig{
		Addr:            freeAddr(t),
		TLSCertFile:     "/nonexistent/server.crt",
		TLSKeyFile:      "/nonexistent/server.key",
		ShutdownTimeout: time.Second,
	}
	srv := New(cfg, http.NotFoundHandler(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if srv.srv.TLSConfig == nil {
		t.Fatal("TLS config must be set")
	}
	if err := srv.Run(context.Background()); err == nil {
		t.Fatal("expected error for missing certificate")
	}
}
