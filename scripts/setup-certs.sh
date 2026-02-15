#!/bin/bash
set -euo pipefail

DOMAIN="agents.test"
CERTS_DIR="$HOME/.agentvm/certs"

echo "Setting up TLS certificates for *.$DOMAIN..."

# Install mkcert if not present
if ! command -v mkcert &>/dev/null; then
    echo "Installing mkcert..."
    brew install mkcert
fi

# Install the local CA
echo "Installing local CA (may require password)..."
mkcert -install

# Generate wildcard certificate
mkdir -p "$CERTS_DIR"
cd "$CERTS_DIR"

echo "Generating wildcard certificate..."
mkcert -cert-file "$DOMAIN.pem" -key-file "$DOMAIN-key.pem" \
    "*.$DOMAIN" "*.*.${DOMAIN}" "$DOMAIN"

echo "Certificates written to $CERTS_DIR/"
echo "  $DOMAIN.pem"
echo "  $DOMAIN-key.pem"
