#!/usr/bin/env bash

set -euo pipefail

readonly repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly bootstrap_dsn="${POSTGRES_BOOTSTRAP_DSN:-}"
readonly external_mode="${POSTGRES_MANAGED_EXTERNALLY:-}"

if [[ -z "${bootstrap_dsn}" ]]; then
  printf '%s\n' 'POSTGRES_BOOTSTRAP_DSN is required for PostgreSQL bootstrap' >&2
  exit 2
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

(
  cd "${repository_root}/apps/sumweave"
  APP_APPLICATION_DATABASE_DSN="${local_migrator_dsn}" \
    APP_AGENTRUNTIME_DATABASE_DSN="${local_migrator_dsn}" \
    go run ./cmd/sumweave db-migrate --env local
  APP_APPLICATION_DATABASE_DSN="${test_migrator_dsn}" \
    APP_AGENTRUNTIME_DATABASE_DSN="${test_migrator_dsn}" \
    go run ./cmd/sumweave db-migrate --env test
)

run_psql_file "${repository_root}/scripts/postgres/grant-runtime-access.sql"
