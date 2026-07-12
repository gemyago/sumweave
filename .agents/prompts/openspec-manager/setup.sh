#!/usr/bin/env bash
# Refresh generated OpenCode agents for openspec-manager.
# Usage: .agents/prompts/openspec-manager/setup.sh [path-to-yaml]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG="${1:-$SCRIPT_DIR/config.yaml}"

"$SCRIPT_DIR/sync-opencode-agents.sh" "$CONFIG"
