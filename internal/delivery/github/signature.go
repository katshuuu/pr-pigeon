package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

const signatureHeader = "X-Hub-Signature-256"

var (
	errNoSignature      = errors.New("signature header is missing")
	errBadSignature     = errors.New("signature header is malformed")
	errSignatureInvalid = errors.New("signature does not match")
)

// verifySignature checks the HMAC-SHA256 that GitHub puts into X-Hub-Signature-256.
// See https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries
func verifySignature(secret, body []byte, header string) error {
	const prefix = "sha256="
	if header == "" {
		return errNoSignature
	}
	if !strings.HasPrefix(header, prefix) {
		return errBadSignature
	}

	got, err := hex.DecodeString(strings.TrimPrefix(header, prefix))
	if err != nil {
		return errBadSignature
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	if !hmac.Equal(got, mac.Sum(nil)) { // constant-time comparison
		return errSignatureInvalid
	}
	return nil
}
