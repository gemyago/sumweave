#!/usr/bin/env bash
# Copied from gemyago/golang-backend-boilerplate@798f0dc9fd753481d0d698d8232ea08df44185b6 and narrowed to one image.
set -euo pipefail

source_sha=""; target_tags=""; image=""; noop=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --source-commit-sha) source_sha="$2"; shift 2 ;;
    --target-tags) target_tags="$2"; shift 2 ;;
    --image) image="$2"; shift 2 ;;
    --noop) noop=true; shift ;;
    *) echo "unknown option: $1" >&2; exit 1 ;;
  esac
done
[[ -n "$source_sha" && -n "$target_tags" && -n "$image" ]] || { echo "source sha, target tags, and image are required" >&2; exit 1; }
source="${image}:git-commit-${source_sha:0:7}"
for tag in $target_tags; do
  if "$noop"; then printf 'crane tag %q %q\n' "$source" "$tag"; else "$(dirname "$0")/../bin/crane" tag "$source" "$tag"; fi
done
