#!/usr/bin/env bash

set -euo pipefail

readonly repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly test_dir="$(mktemp -d "${repository_root}/tmp/postgres-bootstrap-contract.XXXXXX")"
trap 'rm -rf "${test_dir}"' EXIT

make_fake_command() {
  local name="$1"
  local content="$2"
  printf '%s\n' "${content}" > "${test_dir}/${name}"
  chmod +x "${test_dir}/${name}"
}

make_fake_command psql '#!/usr/bin/env bash
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
