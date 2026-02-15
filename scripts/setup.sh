#!/bin/bash
set -euo pipefail

echo "=== AgentVM Setup ==="
echo ""

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
AGENTVM_DIR="$HOME/.agentvm"

# 1. Check prerequisites
echo "Checking prerequisites..."
MISSING=()
for cmd in lima limactl docker brew; do
    if ! command -v "$cmd" &>/dev/null; then
        MISSING+=("$cmd")
    fi
done

if [ ${#MISSING[@]} -gt 0 ]; then
    echo "Missing dependencies: ${MISSING[*]}"
    echo ""
    echo "Install with:"
    echo "  brew install lima docker"
    echo ""
    read -p "Continue anyway? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# 2. Install optional dependencies
echo ""
echo "Installing optional dependencies..."
if command -v brew &>/dev/null; then
    brew list traefik &>/dev/null || brew install traefik
    brew list dnsmasq &>/dev/null || brew install dnsmasq
    brew list mkcert &>/dev/null || brew install mkcert
fi

# 3. Create directory structure
echo ""
echo "Creating directory structure..."
mkdir -p "$AGENTVM_DIR"/{shared/bin,traefik/dynamic,certs,logs}

# 4. Build binaries
echo ""
echo "Building binaries..."
cd "$PROJECT_DIR"
make build build-harness
make install-harness

# 5. Setup DNS
echo ""
echo "Setting up DNS resolution..."
"$SCRIPT_DIR/setup-dnsmasq.sh"

# 6. Setup TLS certificates
echo ""
echo "Setting up TLS certificates..."
"$SCRIPT_DIR/setup-certs.sh"

# 7. Generate Traefik static config
echo ""
echo "Generating Traefik configuration..."
"$PROJECT_DIR/bin/agentctl" setup

# 8. Create golden master
echo ""
read -p "Create golden master VM now? This takes ~5 minutes. [Y/n] " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Nn]$ ]]; then
    "$PROJECT_DIR/bin/agentctl" master create
fi

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Next steps:"
echo "  1. Start agentd:    agentd (or install launchd plist)"
echo "  2. Start traefik:   traefik --configFile=$AGENTVM_DIR/traefik/traefik.yaml"
echo "  3. Dispatch a task: agentctl dispatch --project myapp --repo URL --prompt '...'"
