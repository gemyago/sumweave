#!/usr/bin/env bash

set -euo pipefail

readonly repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly bootstrap_dsn="${POSTGRES_BOOTSTRAP_DSN:-}"
readonly external_mode="${POSTGRES_MANAGED_EXTERNALLY:-}"
readonly migration_cover_dir="${SUMWEAVE_FINANCE_MIGRATION_COVER_DIR:-}"
readonly expected_migration_cover_dir="${repository_root}/finance/.cover/postgres-migration"

# Coverage mode operates only in a trusted repository worktree with no
# concurrent writer. The lexical, canonical, and static symlink checks below
# reject unsafe inputs and pre-existing paths; they do not close a TOCTOU race.

if [[ -z "${bootstrap_dsn}" ]]; then
  printf '%s\n' 'POSTGRES_BOOTSTRAP_DSN is required for PostgreSQL bootstrap' >&2
  exit 2
fi

if [[ -n "${migration_cover_dir}" && "${migration_cover_dir}" != "${expected_migration_cover_dir}" ]]; then
  printf 'SUMWEAVE_FINANCE_MIGRATION_COVER_DIR must equal %s\n' "${expected_migration_cover_dir}" >&2
  exit 2
fi

validate_migration_cover_dir() {
  local migration_cover_parent="${repository_root}/finance/.cover"
  local resolved_migration_cover_parent
  local resolved_migration_cover_dir

  if [[ -L "${migration_cover_parent}" || -L "${migration_cover_dir}" ]]; then
    printf '%s\n' 'finance migration coverage directory must not use symlinks' >&2
    exit 2
  fi
  if [[ -e "${migration_cover_parent}" && ! -d "${migration_cover_parent}" ]]; then
    printf '%s\n' 'finance migration coverage parent must be a directory' >&2
    exit 2
  fi
  if [[ ! -e "${migration_cover_parent}" ]]; then
    mkdir -- "${migration_cover_parent}"
  fi
  if [[ -L "${migration_cover_parent}" || ! -d "${migration_cover_parent}" ]]; then
    printf '%s\n' 'finance migration coverage parent must be a directory without symlinks' >&2
    exit 2
  fi
  resolved_migration_cover_parent="$(cd "${migration_cover_parent}" && pwd -P)"
  if [[ "${resolved_migration_cover_parent}" != "${migration_cover_parent}" ]]; then
    printf '%s\n' 'finance migration coverage parent must be repository-owned' >&2
    exit 2
  fi

  if [[ -e "${migration_cover_dir}" && ! -d "${migration_cover_dir}" ]]; then
    printf '%s\n' 'finance migration coverage directory must be a directory' >&2
    exit 2
  fi
  if [[ ! -e "${migration_cover_dir}" ]]; then
    mkdir -- "${migration_cover_dir}"
  fi
  if [[ -L "${migration_cover_parent}" || -L "${migration_cover_dir}" || ! -d "${migration_cover_dir}" ]]; then
    printf '%s\n' 'finance migration coverage directory must be a directory without symlinks' >&2
    exit 2
  fi
  resolved_migration_cover_dir="$(cd "${migration_cover_dir}" && pwd -P)"
  if [[ "${resolved_migration_cover_dir}" != "${expected_migration_cover_dir}" ]]; then
    printf '%s\n' 'finance migration coverage directory must be repository-owned' >&2
    exit 2
  fi
}

if [[ -n "${migration_cover_dir}" ]]; then
  validate_migration_cover_dir
fi

run_psql_query() {
  local query="$1"

  if [[ "${external_mode}" == "1" ]]; then
    psql --dbname="${bootstrap_dsn}" --no-psqlrc --tuples-only --no-align --command="${query}"
    return
  fi

  docker compose -f "${repository_root}/compose.yaml" exec -T postgres \
    psql --dbname="${bootstrap_dsn}" --no-psqlrc --tuples-only --no-align --command="${query}"
}

run_psql_file() {
  local file="$1"

  if [[ "${external_mode}" == "1" ]]; then
    psql --dbname="${bootstrap_dsn}" --no-psqlrc --set ON_ERROR_STOP=1 --file="${file}"
    return
  fi

  docker compose -f "${repository_root}/compose.yaml" exec -T postgres \
    psql --dbname="${bootstrap_dsn}" --no-psqlrc --set ON_ERROR_STOP=1 --file=- < "${file}"
}

for _ in {1..30}; do
  if run_psql_query 'SELECT 1' >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! run_psql_query 'SELECT 1' >/dev/null 2>&1; then
  printf '%s\n' 'POSTGRES_BOOTSTRAP_DSN is unreachable' >&2
  exit 1
fi

if [[ "$(run_psql_query "SELECT rolsuper FROM pg_roles WHERE rolname = current_user")" != "t" ]]; then
  printf '%s\n' 'POSTGRES_BOOTSTRAP_DSN must authenticate as a PostgreSQL superuser' >&2
  exit 1
fi

run_psql_file "${repository_root}/scripts/postgres/bootstrap-cluster.sql"
run_psql_file "${repository_root}/scripts/postgres/configure-privileges.sql"

readonly local_migrator_dsn="postgres://sumweave_migrator:sumweave_migrator_local@${POSTGRES_HOST}:${POSTGRES_PORT}/sumweave_local?sslmode=disable"
readonly test_migrator_dsn="postgres://sumweave_migrator:sumweave_migrator_local@${POSTGRES_HOST}:${POSTGRES_PORT}/sumweave_test?sslmode=disable"
if [[ -n "${migration_cover_dir}" ]]; then
  rm -f -- "${migration_cover_dir}/ready"
  rm -rf -- "${migration_cover_dir}/raw"
  mkdir -p "${migration_cover_dir}/raw"
fi

(
  cd "${repository_root}/apps/sumweave"
  APP_APPLICATION_DATABASE_DSN="${local_migrator_dsn}" \
    APP_AGENTRUNTIME_DATABASE_DSN="${local_migrator_dsn}" \
    go run ./cmd/sumweave db-migrate --env local
  test_migration_command=(go run)
  if [[ -n "${migration_cover_dir}" ]]; then
    test_migration_command=(env "GOCOVERDIR=${migration_cover_dir}/raw" go run -covermode=atomic -coverpkg=github.com/gemyago/sumweave/apps/sumweave/cmd/sumweave,github.com/gemyago/sumweave/finance/...)
  fi
  APP_APPLICATION_DATABASE_DSN="${test_migrator_dsn}" \
    APP_AGENTRUNTIME_DATABASE_DSN="${test_migrator_dsn}" \
    "${test_migration_command[@]}" ./cmd/sumweave db-migrate --env test
)

if [[ -n "${migration_cover_dir}" ]]; then
  if ! find "${migration_cover_dir}/raw" -type f -size +0c -print -quit | grep -q .; then
    printf '%s\n' 'finance migration coverage data is empty' >&2
    exit 1
  fi
  go tool covdata textfmt -i="${migration_cover_dir}/raw" -pkg=github.com/gemyago/sumweave/finance/... -o=/dev/null
  touch "${migration_cover_dir}/raw"
  sleep 1
  touch "${migration_cover_dir}/ready"
fi

run_psql_file "${repository_root}/scripts/postgres/grant-runtime-access.sql"
