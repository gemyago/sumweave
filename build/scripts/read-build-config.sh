#!/usr/bin/env bash
# Copied from gemyago/golang-backend-boilerplate@798f0dc9fd753481d0d698d8232ea08df44185b6.
set -euo pipefail

key="${1:-}"
if [[ -z "$key" ]]; then
  echo "usage: $0 <key>" >&2
  exit 1
fi

value="$(grep -m 1 "^${key}[[:space:]]*=" "$(dirname "$0")/../build.cfg" | cut -d= -f2- | xargs)"
if [[ -z "$value" ]]; then
  echo "missing build config key: $key" >&2
  exit 1
fi
printf '%s\n' "$value"
