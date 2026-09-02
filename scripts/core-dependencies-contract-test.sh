#!/usr/bin/env bash

set -euo pipefail

readonly repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly -a core_modules=(runtime finance apps/sumweave)
readonly -a forbidden_modules=(
  github.com/glebarez/go-sqlite
  github.com/glebarez/sqlite
  github.com/ncruces/go-strftime
  gorm.io/driver/sqlite
  github.com/mattn/go-sqlite3
  modernc.org/libc
  modernc.org/mathutil
  modernc.org/memory
  modernc.org/sqlite
)

is_adk_test_residue() {
  local module="$1"
  local dependency="$2"

  [[ "${module}" == 'runtime' || "${module}" == 'apps/sumweave' ]] && case "${dependency}" in
    github.com/glebarez/go-sqlite|github.com/glebarez/sqlite|github.com/ncruces/go-strftime|modernc.org/libc|modernc.org/mathutil|modernc.org/memory|modernc.org/sqlite) return 0 ;;
  esac
  return 1
}

expected_adk_test_why() {
  local module="$1"
  local dependency="$2"
  local dependency_path

  if [[ "${module}" == 'runtime' ]]; then
    dependency_path="github.com/gemyago/sumweave/runtime/internal/sessions
google.golang.org/adk/session/database
google.golang.org/adk/session/database.test"
  else
    dependency_path="github.com/gemyago/sumweave/apps/sumweave
github.com/gemyago/sumweave/runtime/agent
github.com/gemyago/sumweave/runtime/internal/sessions
google.golang.org/adk/session/database
google.golang.org/adk/session/database.test"
  fi

  case "${dependency}" in
    github.com/glebarez/sqlite)
      printf '# %s\n%s\n%s\n' "${dependency}" "${dependency_path}" "${dependency}"
      ;;
    github.com/glebarez/go-sqlite)
      printf '# %s\n%s\ngithub.com/glebarez/sqlite\n%s\n' "${dependency}" "${dependency_path}" "${dependency}"
      ;;
    modernc.org/sqlite)
      printf '# %s\n%s\ngithub.com/glebarez/sqlite\n%s/lib\n' "${dependency}" "${dependency_path}" "${dependency}"
      ;;
    modernc.org/libc)
      printf '# %s\n%s\ngithub.com/glebarez/sqlite\ngithub.com/glebarez/go-sqlite\n%s\n' "${dependency}" "${dependency_path}" "${dependency}"
      ;;
    modernc.org/mathutil|modernc.org/memory)
      printf '# %s\n%s\ngithub.com/glebarez/sqlite\ngithub.com/glebarez/go-sqlite\nmodernc.org/libc\n%s\n' "${dependency}" "${dependency_path}" "${dependency}"
      ;;
    github.com/ncruces/go-strftime)
      printf '# %s\n%s\ngithub.com/glebarez/sqlite\ngithub.com/glebarez/go-sqlite\nmodernc.org/libc\n%s\n' "${dependency}" "${dependency_path}" "${dependency}"
      ;;
  esac
}

assert_no_forbidden_direct_imports() {
  local module="$1"
  local imports

  imports="$(
    cd "${repository_root}/${module}"
    GOWORK=off go list -f '{{join .Imports "\n"}}{{"\n"}}{{join .TestImports "\n"}}{{"\n"}}{{join .XTestImports "\n"}}' ./...
  )"
  if grep -Eq '^(github\.com/glebarez/(go-)?sqlite|github\.com/ncruces/go-strftime|gorm\.io/driver/sqlite|github\.com/mattn/go-sqlite3|modernc\.org/(libc|mathutil|memory|sqlite))($|/)' <<<"${imports}"; then
    printf 'forbidden direct core import in %s\n' "${module}" >&2
    exit 1
  fi
}

assert_no_forbidden_production_dependencies() {
  local module="$1"
  local dependencies

  dependencies="$(
    cd "${repository_root}/${module}"
    GOWORK=off go list -deps ./... | grep -E '^(github\.com/glebarez/(go-)?sqlite|github\.com/ncruces/go-strftime|gorm\.io/driver/sqlite|github\.com/mattn/go-sqlite3|modernc\.org/(libc|mathutil|memory|sqlite))($|/)' || true
  )"
  if [[ -n "${dependencies}" ]]; then
    printf 'forbidden production dependency traversal in %s:\n%s\n' "${module}" "${dependencies}" >&2
    exit 1
  fi
}

assert_adk_test_residue() {
  local module
  local dependency
  local actual
  local expected

  for module in runtime apps/sumweave; do
    for dependency in "${forbidden_modules[@]}"; do
      if ! grep -Fq -- "${dependency}" "${repository_root}/${module}/go.mod"; then
        continue
      fi
      actual="$(
        cd "${repository_root}/${module}"
        GOWORK=off go mod why -m "${dependency}"
      )"
      expected="$(expected_adk_test_why "${module}" "${dependency}")"
      if [[ "${actual}" != "${expected}" ]]; then
        printf 'unexpected ADK test-only dependency path for %s in %s\n' "${dependency}" "${module}" >&2
        exit 1
      fi
    done
  done
}

for module in "${core_modules[@]}"; do
  module_file="${repository_root}/${module}/go.mod"
  for forbidden_module in "${forbidden_modules[@]}"; do
    if ! grep -Fq -- "${forbidden_module}" "${module_file}"; then
      continue
    fi
    if is_adk_test_residue "${module}" "${forbidden_module}" && \
      grep -Eq "^[[:space:]]*${forbidden_module}[[:space:]][^[:space:]]+[[:space:]]+// indirect$" "${module_file}"; then
      continue
    fi
    {
      printf 'forbidden core dependency requirement in %s: %s\n' "${module}" "${forbidden_module}" >&2
      exit 1
    }
  done
  assert_no_forbidden_direct_imports "${module}"
  assert_no_forbidden_production_dependencies "${module}"
done

assert_adk_test_residue
