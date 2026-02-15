#!/bin/bash
set -euo pipefail

DOMAIN="agents.test"

echo "Setting up dnsmasq for *.$DOMAIN..."

# Install dnsmasq if not present
if ! command -v dnsmasq &>/dev/null; then
    echo "Installing dnsmasq..."
    brew install dnsmasq
fi

# Configure dnsmasq
DNSMASQ_CONF_DIR="/opt/homebrew/etc/dnsmasq.d"
mkdir -p "$DNSMASQ_CONF_DIR"

echo "address=/$DOMAIN/127.0.0.1" > "$DNSMASQ_CONF_DIR/agentvm.conf"
echo "  Written $DNSMASQ_CONF_DIR/agentvm.conf"

# Ensure dnsmasq.conf includes the .d directory
DNSMASQ_CONF="/opt/homebrew/etc/dnsmasq.conf"
if ! grep -q "conf-dir=$DNSMASQ_CONF_DIR" "$DNSMASQ_CONF" 2>/dev/null; then
    echo "conf-dir=$DNSMASQ_CONF_DIR/,*.conf" >> "$DNSMASQ_CONF"
    echo "  Updated $DNSMASQ_CONF to include conf-dir"
fi

# Restart dnsmasq
echo "Restarting dnsmasq..."
sudo brew services restart dnsmasq

# Create resolver entry
echo "Creating /etc/resolver/$DOMAIN..."
sudo mkdir -p /etc/resolver
echo "nameserver 127.0.0.1" | sudo tee "/etc/resolver/$DOMAIN" > /dev/null

echo "DNS setup complete. Test with: dig test.$DOMAIN @127.0.0.1"
