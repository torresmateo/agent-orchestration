#!/bin/bash
set -euo pipefail

echo "=== AgentVM Teardown ==="
echo ""
echo "This will stop all services and delete all VMs."
read -p "Are you sure? [y/N] " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted."
    exit 0
fi

# Stop launchd services
echo "Stopping services..."
launchctl unload ~/Library/LaunchAgents/com.agentvm.agentd.plist 2>/dev/null || true
launchctl unload ~/Library/LaunchAgents/com.agentvm.traefik.plist 2>/dev/null || true

# Kill any running agentd/traefik processes
pkill -f agentd 2>/dev/null || true
pkill -f "traefik.*agentvm" 2>/dev/null || true

# Delete all agent VMs
echo "Deleting VMs..."
for vm in $(limactl list --json 2>/dev/null | jq -r '.name' | grep -E '^(warm-|agent-|active-)'); do
    echo "  Deleting $vm..."
    limactl delete --force "$vm" 2>/dev/null || true
done

# Delete master
read -p "Delete golden master VM too? [y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "  Deleting agent-master..."
    limactl delete --force agent-master 2>/dev/null || true
fi

# Clean runtime state
echo "Cleaning runtime state..."
rm -rf ~/.agentvm/pool-state.json
rm -rf ~/.agentvm/registry.json
rm -rf ~/.agentvm/traefik/dynamic/*.yaml

echo ""
echo "Teardown complete."
echo "To fully remove, also delete ~/.agentvm/"
