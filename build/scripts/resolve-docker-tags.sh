#!/usr/bin/env bash
# Adapted from gemyago/golang-backend-boilerplate@798f0dc9fd753481d0d698d8232ea08df44185b6.
set -euo pipefail

sanitize() {
  local value="${1//[^[:alnum:].-]/-}"
  [[ "$value" =~ ^[[:alnum:]] ]] || value="x-${value}"
  printf '%s' "${value:0:128}"
}

resolve() {
  local ref="$1" sha="$2" stable="$3" latest="${4:-false}"
  sha="${sha:0:7}"
  if [[ "$ref" == refs/tags/* || "$ref" == tags/* ]]; then
    local tag="${ref#refs/tags/}"; tag="${tag#tags/}"
    local tags="git-tag-$(sanitize "$tag")"
    if [[ "$tag" =~ ^v?([0-9]+)(\.([0-9]+))?(\.([0-9]+))?(-.+)?$ ]]; then
      local major="${BASH_REMATCH[1]}" minor="${BASH_REMATCH[3]}" patch="${BASH_REMATCH[5]}" prerelease="${BASH_REMATCH[6]}"
      local full="v${major}"; [[ -n "$minor" ]] && full+=".${minor}"; [[ -n "$patch" ]] && full+=".${patch}"; full+="$prerelease"
      tags+=" $(sanitize "$full")"
      if [[ -z "$prerelease" ]]; then
        [[ -n "$minor" ]] && tags+=" v${major}.${minor}-latest"
        tags+=" v${major}-latest latest"
      fi
    fi
    printf '%s\n' "$tags"
    return
  fi
  local branch="${ref#refs/heads/}" branch_tag
  branch_tag="$(sanitize "$branch")"
  if [[ ",$stable," == *",$branch,"* ]]; then
    printf 'latest-%s git-commit-%s\n' "$branch_tag" "$(sanitize "$sha")"
  else
    printf '%s git-commit-%s\n' "$branch_tag" "$(sanitize "$sha")"
  fi
}

if [[ "${1:-}" == "--self-test" ]]; then
  [[ "$(resolve main abc123456 main)" == "latest-main git-commit-abc1234" ]]
  [[ "$(resolve feature/a abc123456 main)" == "feature-a git-commit-abc1234" ]]
  [[ "$(resolve refs/tags/v1.2.3 abc123456 main)" == "git-tag-v1.2.3 v1.2.3 v1.2-latest v1-latest latest" ]]
  [[ "$(resolve refs/tags/v1.2.3-rc1 abc123456 main)" == "git-tag-v1.2.3-rc1 v1.2.3-rc1" ]]
  exit 0
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
    --stable-branches) stable="$2"; shift 2 ;;
    --git-ref) ref="$2"; shift 2 ;;
    --commit-sha) sha="$2"; shift 2 ;;
    *) echo "unknown option: $1" >&2; exit 1 ;;
  esac
done
[[ -n "$stable" && -n "$ref" && -n "$sha" ]] || { echo "stable branches, git ref, and commit sha are required" >&2; exit 1; }
resolve "$ref" "$sha" "$stable"
