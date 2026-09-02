#!/usr/bin/env bash

set -euo pipefail

readonly repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly test_dir="$(mktemp -d "${repository_root}/tmp/postgres-bootstrap-contract.XXXXXX")"
readonly migration_cover_parent="${repository_root}/finance/.cover"
readonly migration_cover_dir="${migration_cover_parent}/postgres-migration"
readonly saved_migration_cover_parent="${test_dir}/saved-cover"
coverage_parent_was_present=0
coverage_parent_is_test_owned=0

# This contract covers the static state inspected by bootstrap. Replacing an
# entry after validation is outside its trusted, non-concurrently-mutated
# worktree contract and is intentionally not treated as a supported race.

restore_migration_cover_parent() {
  if [[ "${coverage_parent_is_test_owned}" -eq 1 ]] && [[ -e "${migration_cover_parent}" || -L "${migration_cover_parent}" ]]; then
    rm -rf -- "${migration_cover_parent}"
  fi
  if [[ "${coverage_parent_was_present}" -eq 1 ]]; then
    mv -- "${saved_migration_cover_parent}" "${migration_cover_parent}"
  fi
}

cleanup() {
  restore_migration_cover_parent
  rm -rf -- "${test_dir}"
}
trap cleanup EXIT

if [[ -e "${migration_cover_parent}" || -L "${migration_cover_parent}" ]]; then
  mv -- "${migration_cover_parent}" "${saved_migration_cover_parent}"
  coverage_parent_was_present=1
fi
coverage_parent_is_test_owned=1

make_fake_command() {
  local name="$1"
  local content="$2"
  printf '%s\n' "${content}" > "${test_dir}/${name}"
  chmod +x "${test_dir}/${name}"
}

make_fake_command psql '#!/usr/bin/env bash
if [[ -n "${POSTGRES_PSQL_LOG:-}" ]]; then
  printf "%s\n" "$*" >> "${POSTGRES_PSQL_LOG}"
fi
case "${POSTGRES_PSQL_RESULT}" in
  reachable)
    if [[ "$*" == *"rolsuper"* ]]; then
      printf "f\\n"
    else
      printf "1\\n"
    fi
    ;;
  *) exit 1 ;;
esac'
make_fake_command go '#!/usr/bin/env bash
printf "%s\\n" "$*" >> "${POSTGRES_GO_LOG}"'

assert_coverage_dir_rejected() {
  local name="$1"
  local cover_dir="$2"
  local output="${test_dir}/${name}.out"
  local psql_log="${test_dir}/${name}.psql.log"

  if POSTGRES_MANAGED_EXTERNALLY=1 POSTGRES_BOOTSTRAP_DSN='postgres://coverage-guard' \
    SUMWEAVE_FINANCE_MIGRATION_COVER_DIR="${cover_dir}" \
    PATH="${test_dir}:${PATH}" POSTGRES_PSQL_LOG="${psql_log}" \
    "${repository_root}/scripts/postgres/bootstrap.sh" >"${output}" 2>&1; then
    printf 'expected coverage path case to fail: %s\n' "${name}" >&2
    exit 1
  fi
  test ! -e "${psql_log}"
}

readonly traversal_cover_dir="${migration_cover_dir}/../postgres-migration"
assert_coverage_dir_rejected traversal "${traversal_cover_dir}"
grep -Fq 'SUMWEAVE_FINANCE_MIGRATION_COVER_DIR must equal' "${test_dir}/traversal.out"

readonly ancestor_sentinel="${test_dir}/ancestor-sentinel"
mkdir -- "${ancestor_sentinel}"
touch "${ancestor_sentinel}/unchanged"
ln -s -- "${ancestor_sentinel}" "${migration_cover_parent}"
assert_coverage_dir_rejected ancestor-symlink "${migration_cover_dir}"
grep -Fq 'finance migration coverage directory must not use symlinks' "${test_dir}/ancestor-symlink.out"
test -f "${ancestor_sentinel}/unchanged"
test ! -e "${ancestor_sentinel}/postgres-migration"
rm -- "${migration_cover_parent}"

mkdir -- "${migration_cover_parent}"
readonly target_sentinel="${test_dir}/target-sentinel"
mkdir -- "${target_sentinel}"
touch "${target_sentinel}/unchanged"
touch "${target_sentinel}/ready"
ln -s -- "${target_sentinel}" "${migration_cover_dir}"
assert_coverage_dir_rejected target-symlink "${migration_cover_dir}"
grep -Fq 'finance migration coverage directory must not use symlinks' "${test_dir}/target-symlink.out"
test -f "${target_sentinel}/unchanged"
test -f "${target_sentinel}/ready"
test ! -e "${target_sentinel}/raw"
rm -rf -- "${migration_cover_parent}"

if POSTGRES_MANAGED_EXTERNALLY=1 PATH="${test_dir}:${PATH}" \
  "${repository_root}/scripts/postgres/bootstrap.sh" >"${test_dir}/missing.out" 2>&1; then
  printf '%s\n' 'expected missing bootstrap DSN to fail' >&2
  exit 1
fi
grep -Fq 'POSTGRES_BOOTSTRAP_DSN is required' "${test_dir}/missing.out"

if POSTGRES_MANAGED_EXTERNALLY=1 POSTGRES_BOOTSTRAP_DSN='postgres://unreachable' \
  PATH="${test_dir}:${PATH}" POSTGRES_PSQL_RESULT=unreachable \
  "${repository_root}/scripts/postgres/bootstrap.sh" >"${test_dir}/unreachable.out" 2>&1; then
  printf '%s\n' 'expected unreachable bootstrap DSN to fail' >&2
  exit 1
fi
grep -Fq 'POSTGRES_BOOTSTRAP_DSN is unreachable' "${test_dir}/unreachable.out"

if POSTGRES_MANAGED_EXTERNALLY=1 POSTGRES_BOOTSTRAP_DSN='postgres://insufficient' \
  PATH="${test_dir}:${PATH}" POSTGRES_PSQL_RESULT=reachable POSTGRES_GO_LOG="${test_dir}/go.log" \
  "${repository_root}/scripts/postgres/bootstrap.sh" >"${test_dir}/insufficient.out" 2>&1; then
  printf '%s\n' 'expected insufficient bootstrap DSN to fail' >&2
  exit 1
fi
grep -Fq 'POSTGRES_BOOTSTRAP_DSN must authenticate as a PostgreSQL superuser' "${test_dir}/insufficient.out"
test ! -e "${test_dir}/go.log"
