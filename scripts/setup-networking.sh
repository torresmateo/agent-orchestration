#!/bin/bash
set -euo pipefail

DOMAIN="agents.test"
AGENTVM_DIR="$HOME/.agentvm"

echo "=== AgentVM Networking Setup (HTTP-only) ==="
echo ""

# Step 1: dnsmasq
echo "--- Step 1: dnsmasq ---"
if ! command -v dnsmasq &>/dev/null; then
    echo "Installing dnsmasq..."
    brew install dnsmasq
fi

bash "$(dirname "$0")/setup-dnsmasq.sh"
echo "  dnsmasq configured for *.$DOMAIN -> 127.0.0.1"

# Step 2: Install traefik
echo ""
echo "--- Step 2: traefik ---"
if ! command -v traefik &>/dev/null; then
    echo "Installing traefik..."
    brew install traefik
fi
echo "  traefik installed: $(traefik version 2>&1 | head -1)"

# Step 3: Generate HTTP-only traefik.yaml
echo ""
echo "--- Step 3: Generate HTTP-only traefik config ---"
mkdir -p "$AGENTVM_DIR/traefik/dynamic"

cat > "$AGENTVM_DIR/traefik/traefik.yaml" <<EOF
# Traefik static configuration for agentvm (HTTP-only)
entryPoints:
  web:
    address: ":80"

providers:
  file:
    directory: "$AGENTVM_DIR/traefik/dynamic"
    watch: true

api:
  dashboard: true
  insecure: true

log:
  level: INFO
EOF
echo "  Written $AGENTVM_DIR/traefik/traefik.yaml"

# Step 4: Update agentvm config for HTTP-only
echo ""
echo "--- Step 4: Enable httpOnly in agentvm config ---"
CONFIG="$AGENTVM_DIR/config.yaml"
if [ -f "$CONFIG" ]; then
    if grep -q "httpOnly" "$CONFIG"; then
        sed -i '' 's/httpOnly: false/httpOnly: true/' "$CONFIG"
    else
        # Append httpOnly to network section
        sed -i '' '/^network:/a\
  httpOnly: true' "$CONFIG"
    fi
    echo "  Updated $CONFIG with httpOnly: true"
else
    echo "  Warning: config.yaml not found, run 'agentctl setup' first"
fi

# Step 5: Verify DNS
echo ""
echo "--- Step 5: Verify DNS ---"
sleep 1
if dig +short test.$DOMAIN @127.0.0.1 2>/dev/null | grep -q "127.0.0.1"; then
    echo "  DNS resolution works: test.$DOMAIN -> 127.0.0.1"
else
    echo "  Warning: DNS resolution not working yet. Try: sudo brew services restart dnsmasq"
fi

echo ""
echo "=== Networking setup complete ==="
echo ""
echo "To start traefik:"
echo "  sudo traefik --configFile=$AGENTVM_DIR/traefik/traefik.yaml"
echo ""
echo "Or in the background:"
echo "  sudo traefik --configFile=$AGENTVM_DIR/traefik/traefik.yaml &"
echo ""
echo "Traefik dashboard: http://localhost:8080"
