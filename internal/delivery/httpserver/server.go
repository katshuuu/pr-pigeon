// Package httpserver configures the HTTP(S) server and common middleware.
package httpserver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/katshuuu/pr-pigeon/internal/config"
)

type Server struct {
	srv *http.Server
	cfg config.HTTPConfig
	log *slog.Logger
}

func New(cfg config.HTTPConfig, h http.Handler, log *slog.Logger) *Server {
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           h,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout, // slowloris protection
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    64 << 10,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
	if cfg.TLSEnabled() {
		srv.TLSConfig = TLSConfig()
	}
	return &Server{srv: srv, cfg: cfg, log: log.With("component", "http_server")}
}

// TLSConfig follows the "intermediate" Mozilla profile: TLS 1.2+ with AEAD ciphers only.
// TLS 1.3 suites are not configurable in Go and are secure by default.
func TLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion:       tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		},
	}
}

// Run serves until ctx is cancelled, then shuts down gracefully.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.log.Info("http server started", "addr", s.cfg.Addr, "tls", s.cfg.TLSEnabled())

		var err error
		if s.cfg.TLSEnabled() {
			err = s.srv.ListenAndServeTLS(s.cfg.TLSCertFile, s.cfg.TLSKeyFile)
		} else {
			s.log.Warn("TLS is disabled: use it only behind a TLS-terminating proxy (ngrok, nginx)")
			err = s.srv.ListenAndServe()
		}
		if !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("listen: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	s.log.Info("shutting down http server", "timeout", s.cfg.ShutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
	defer cancel()
	if err := s.srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
