#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "Building harness binary..."
cd "$PROJECT_DIR"
make build-harness
make install-harness

echo "Creating golden master VM..."
"$PROJECT_DIR/bin/agentctl" master create

echo "Done. Master VM is ready for cloning."
