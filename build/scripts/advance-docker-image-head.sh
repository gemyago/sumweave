#!/usr/bin/env bash
set -euo pipefail

# Run in the checked-out repository after publishing SOURCE_SHA successfully.
: "${SOURCE_SHA:?SOURCE_SHA must identify the published commit}"
tag_name=${TAG_NAME:-docker-image-head}
remote=${GIT_REMOTE:-origin}
tag_ref="refs/tags/$tag_name"
git check-ref-format "$tag_ref"
source_commit=$(git rev-parse --verify --end-of-options "$SOURCE_SHA^{commit}")

current=$(git ls-remote --refs -- "$remote" "$tag_ref")
current=${current%%$'\t'*}
if [[ -n "$current" ]]; then
  git fetch --no-tags -- "$remote" "$current"
  current_commit=$(git rev-parse "$current^{commit}")
  if ! git merge-base --is-ancestor "$current_commit" "$source_commit"; then
    echo "Published source does not advance $tag_name; leaving it unchanged."
    exit 0
  fi
  if [[ "$current_commit" == "$source_commit" ]]; then
    echo "$tag_name already points to the published source."
    exit 0
  fi
fi
# Compare-and-swap protects initial creation too (an empty expected ref).
# Concurrent updates fail rather than overwriting another publisher's tag.
git push --force-with-lease="$tag_ref:$current" -- "$remote" "$source_commit:$tag_ref"
