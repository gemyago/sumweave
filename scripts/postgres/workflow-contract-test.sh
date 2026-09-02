#!/usr/bin/env bash

set -euo pipefail

readonly repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly postgres_workflow="${repository_root}/.github/workflows/postgres-verify.yml"
readonly routine_workflow="${repository_root}/.github/workflows/tests-run.yml"
readonly root_makefile="${repository_root}/Makefile"

require_line() {
  local file="$1"
  local expected="$2"

  if ! grep -Fqx "${expected}" "${file}"; then
    printf 'missing expected workflow contract line in %s: %s\n' "${file}" "${expected}" >&2
    exit 1
  fi
}

forbid_text() {
  local file="$1"
  local forbidden="$2"

  if grep -Fq "${forbidden}" "${file}"; then
    printf 'unexpected routine PostgreSQL setup in %s: %s\n' "${file}" "${forbidden}" >&2
    exit 1
  fi
}

forbid_target_postgres_setup() {
  local file="$1"

  if ! awk '
    /^test:/ { in_test = 1; next }
    in_test && /^[^[:space:]]/ { exit }
    in_test && /(postgres-|postgres_test|SUMWEAVE_POSTGRES_TEST_DSN|pg_isready|docker compose)/ { exit 1 }
    END { if (!in_test) exit 1 }
  ' "${file}"; then
    printf 'routine test target must not invoke PostgreSQL setup: %s\n' "${file}" >&2
    exit 1
  fi
}

require_line "${postgres_workflow}" 'name: PostgreSQL verification'
require_line "${postgres_workflow}" '  workflow_dispatch:'
require_line "${postgres_workflow}" '  postgres-verify:'
require_line "${postgres_workflow}" '    runs-on: ubuntu-24.04'
require_line "${postgres_workflow}" '      - name: Start PostgreSQL service'
require_line "${postgres_workflow}" '          sudo systemctl start postgresql.service'
require_line "${postgres_workflow}" '          pg_isready --host=127.0.0.1 --port=5432 --dbname=postgres'
require_line "${postgres_workflow}" '      - name: Configure ephemeral PostgreSQL administrator'
require_line "${postgres_workflow}" '          sudo -u postgres psql --set ON_ERROR_STOP=1 --dbname postgres --command "ALTER ROLE postgres PASSWORD '\''sumweave_postgres_ci'\''"'
require_line "${postgres_workflow}" '          echo "POSTGRES_BOOTSTRAP_DSN=postgres://postgres:sumweave_postgres_ci@127.0.0.1:5432/postgres?sslmode=disable" >> "$GITHUB_ENV"'
require_line "${postgres_workflow}" '      - name: Run PostgreSQL verification'
require_line "${postgres_workflow}" '          POSTGRES_MANAGED_EXTERNALLY=1 \'
require_line "${postgres_workflow}" '          POSTGRES_HOST=127.0.0.1 \'
require_line "${postgres_workflow}" '          POSTGRES_PORT=5432 \'
require_line "${postgres_workflow}" '          make postgres-verify'

require_line "${routine_workflow}" 'on:'
require_line "${routine_workflow}" '  workflow_call:'
require_line "${routine_workflow}" '        run: npx nx affected --target=lint --parallel=3'
require_line "${routine_workflow}" '        run: npx nx affected --target=test --parallel=3'
forbid_text "${routine_workflow}" 'postgres'
forbid_text "${routine_workflow}" 'pg_isready'
forbid_text "${routine_workflow}" 'docker compose'

forbid_target_postgres_setup "${root_makefile}"
for module in runtime finance apps/sumweave; do
  forbid_target_postgres_setup "${repository_root}/${module}/Makefile"
done
