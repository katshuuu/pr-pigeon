#!/usr/bin/env sh
# Sends a signed webhook to a running bot, the same way GitHub does.
#
#   GITHUB_SECRET=change-me ./scripts/send-test-webhook.sh \
#       internal/delivery/github/testdata/pull_request_assigned.json pull_request
set -eu

FILE="${1:?payload file}"
EVENT="${2:-pull_request}"
URL="${URL:-https://localhost:8443/api/v1/github/webhook}"
: "${GITHUB_SECRET:?GITHUB_SECRET is not set}"

SIG="sha256=$(openssl dgst -sha256 -hmac "$GITHUB_SECRET" < "$FILE" | sed 's/^.* //')"

curl -sS -k -X POST "$URL" \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: $EVENT" \
  -H "X-GitHub-Delivery: local-$(date +%s)" \
  -H "X-Hub-Signature-256: $SIG" \
  --data-binary "@$FILE"
echo
