// Package github receives GitHub webhooks over HTTP.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"github.com/katshuuu/pr-pigeon/internal/domain"
)

const (
	eventHeader    = "X-GitHub-Event"
	deliveryHeader = "X-GitHub-Delivery"

	// GitHub caps payloads at 25 MB, but PR events are a few dozen KB at most.
	maxBodyBytes = 1 << 20
)

type Notifier interface {
	Notify(ctx context.Context, ev domain.PREvent) (int, error)
}

type Handler struct {
	secret        []byte
	notifier      Notifier
	notifyTimeout time.Duration
	log           *slog.Logger
}

func NewHandler(secret string, n Notifier, notifyTimeout time.Duration, log *slog.Logger) *Handler {
	return &Handler{
		secret:        []byte(secret),
		notifier:      n,
		notifyTimeout: notifyTimeout,
		log:           log.With("component", "github_webhook"),
	}
}

type response struct {
	Status   string `json:"status"`
	Notified int    `json:"notified,omitempty"`
	Error    string `json:"error,omitempty"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	event := r.Header.Get(eventHeader)
	log := h.log.With("event", event, "delivery_id", r.Header.Get(deliveryHeader))

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			log.Warn("payload too large", "limit", tooLarge.Limit)
			writeJSON(w, http.StatusRequestEntityTooLarge, response{Status: "error", Error: "payload too large"})
			return
		}
		log.Warn("failed to read body", "err", err)
		writeJSON(w, http.StatusBadRequest, response{Status: "error", Error: "cannot read body"})
		return
	}

	// Check the signature before looking at anything else in the request.
	if err := verifySignature(h.secret, body, r.Header.Get(signatureHeader)); err != nil {
		log.Warn("rejected webhook with invalid signature", "err", err, "remote_addr", r.RemoteAddr)
		writeJSON(w, http.StatusUnauthorized, response{Status: "error", Error: "invalid signature"})
		return
	}

	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" {
		log.Warn("unexpected content type", "content_type", r.Header.Get("Content-Type"))
		writeJSON(w, http.StatusUnsupportedMediaType, response{
			Status: "error",
			Error:  "set webhook content type to application/json",
		})
		return
	}

	if event == "ping" {
		log.Info("ping from github, webhook is configured correctly")
		writeJSON(w, http.StatusOK, response{Status: "pong"})
		return
	}

	ev, err := parseEvent(event, body)
	switch {
	case errors.Is(err, errUnsupportedEvent):
		log.Debug("event type is not handled")
		writeJSON(w, http.StatusAccepted, response{Status: "ignored"})
		return
	case err != nil:
		log.Warn("bad payload", "err", err)
		writeJSON(w, http.StatusBadRequest, response{Status: "error", Error: "malformed payload"})
		return
	case ev == nil:
		log.Debug("action is not handled")
		writeJSON(w, http.StatusAccepted, response{Status: "ignored"})
		return
	}

	log = log.With("kind", ev.Kind, "repo", ev.PR.Repo, "pr", ev.PR.Number)
	log.Info("webhook accepted")

	// Finish sending even if GitHub drops the connection, but don't hang forever.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), h.notifyTimeout)
	defer cancel()

	sent, err := h.notifier.Notify(ctx, *ev)
	if err != nil {
		log.Error("notification failed", "err", err, "notified", sent)
		writeJSON(w, http.StatusInternalServerError, response{Status: "error", Notified: sent, Error: "notification failed"})
		return
	}
	writeJSON(w, http.StatusOK, response{Status: "ok", Notified: sent})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
