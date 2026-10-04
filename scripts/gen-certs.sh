#!/usr/bin/env sh
# Self-signed certificate for local development. Do not use in production.
set -eu

DIR="${1:-certs}"
mkdir -p "$DIR"

openssl req -x509 -newkey rsa:2048 -sha256 -nodes -days 365 \
  -keyout "$DIR/server.key" -out "$DIR/server.crt" \
  -subj "/CN=localhost" \
  -addext "subjectAltName=DNS:localhost,IP:127.0.0.1" 2>/dev/null

# the container runs as non-root, so the key must be readable by it
chmod 644 "$DIR/server.key"
echo "certificate written to $DIR/"
