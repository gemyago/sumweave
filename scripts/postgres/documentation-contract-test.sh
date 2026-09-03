#!/usr/bin/env bash

set -euo pipefail

readonly repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly local_config="${repository_root}/apps/sumweave/internal/config/local.yaml"
readonly test_config="${repository_root}/apps/sumweave/internal/config/test.yaml"
readonly default_config="${repository_root}/apps/sumweave/internal/config/default.yaml"
readonly contract_parser="${repository_root}/scripts/postgres/documentation-contract-parser.py"
readonly contract_test_dir="$(mktemp -d "${repository_root}/tmp/postgres-documentation-contract.XXXXXX")"

cleanup_contract_tests() {
  rm -rf -- "${contract_test_dir}"
}
trap cleanup_contract_tests EXIT

require_text() {
  local file="$1"
  local expected="$2"

  if ! grep -Fq -- "${expected}" "${file}"; then
    printf 'missing PostgreSQL documentation/config contract in %s: %s\n' "${file}" "${expected}" >&2
    exit 1
  fi
}

forbid_text() {
  local file="$1"
  local forbidden="$2"

  if grep -Fq -- "${forbidden}" "${file}"; then
    printf 'stale SQLite guidance in %s: %s\n' "${file}" "${forbidden}" >&2
    exit 1
  fi
}

require_absent() {
  local path="$1"

  if [[ -e "${path}" ]]; then
    printf 'obsolete PostgreSQL helper remains active: %s\n' "${path}" >&2
    exit 1
  fi
}

validate_psql_guides() {
  if [[ "$#" -ne 3 ]]; then
    printf '%s\n' 'the PostgreSQL command contract must inspect exactly three guides' >&2
    return 1
  fi

  python3 "${contract_parser}" psql "$@"
}

validate_api_only_guides() {
  if [[ "$#" -ne 5 ]]; then
    printf '%s\n' 'the API-only lifecycle contract must inspect exactly five guides' >&2
    return 1
  fi

  python3 "${contract_parser}" api "$@"
}

require_text "${local_config}" 'postgres://sumweave_runtime:sumweave_runtime_local@127.0.0.1:55432/sumweave_local?sslmode=disable'
require_text "${test_config}" 'postgres://sumweave_runtime:sumweave_runtime_local@127.0.0.1:55432/sumweave_test?sslmode=disable'
require_text "${default_config}" 'Shared PostgreSQL database for finance, auth, jobs, and dispatch.'
require_text "${repository_root}/docs/ARCHITECTURE.md" 'make postgres-bootstrap'
require_text "${repository_root}/docs/ARCHITECTURE.md" 'make postgres-verify'
require_text "${repository_root}/docs/database-backed-state-plan.md" 'sumweave_owner'
require_text "${repository_root}/docs/database-backed-state-plan.md" 'sumweave_migrator'
require_text "${repository_root}/docs/database-backed-state-plan.md" 'sumweave_runtime'
require_text "${repository_root}/docs/manual-e2e/README.md" 'make postgres-bootstrap'
require_text "${repository_root}/docs/local-https.md" 'make postgres-bootstrap'
require_text "${repository_root}/apps/sumweave/AGENTS.md" 'make postgres-bootstrap'
require_text "${repository_root}/runtime/AGENTS.md" 'database-independent'
require_text "${repository_root}/finance/AGENTS.md" 'database-free'
require_absent "${repository_root}/docs/manual-e2e/postgres-local.compose.yml"
require_absent "${repository_root}/docs/manual-e2e/postgres-init/01-local-roles.sql"

# Every active local setup entry point uses the same bootstrap prerequisite.
for setup_entry_point in \
  "${repository_root}/README.md" \
  "${repository_root}/CONTRIBUTING.md" \
  "${repository_root}/AGENTS.md" \
  "${repository_root}/apps/sumweave/AGENTS.md" \
  "${repository_root}/docs/local-https.md" \
  "${repository_root}/docs/manual-e2e/README.md" \
  "${repository_root}/docs/manual-e2e/postgres-local-verification.md"; do
  require_text "${setup_entry_point}" 'make postgres-bootstrap'
done
require_text "${repository_root}/CONTRIBUTING.md" 'pm2 start ecosystem.config.js'
forbid_text "${repository_root}/CONTRIBUTING.md" 'go run ./cmd/sumweave db-migrate --env local'

psql_guides=(
  "${repository_root}/docs/manual-e2e/finance-fx-refresh-e2e.md" \
  "${repository_root}/docs/manual-e2e/finance-scheduled-sync-lifecycle-e2e.md" \
  "${repository_root}/docs/manual-e2e/postgres-local-verification.md"
)
# Compose uses 55432 inside and outside the container; inspect every executable
# psql invocation in exactly these three guides.
validate_psql_guides "${psql_guides[@]}"

# API-only guides isolate the consumer before resetting shared PostgreSQL state
# and restore the normal PM2 backend when the guide is complete.
api_only_guides=(
  "${repository_root}/docs/manual-e2e/finance-account-csv-import-e2e.md" \
  "${repository_root}/docs/manual-e2e/finance-transaction-csv-import-e2e.md" \
  "${repository_root}/docs/manual-e2e/synthetic-provider-flow-e2e.md" \
  "${repository_root}/docs/manual-e2e/finance-fx-refresh-e2e.md" \
  "${repository_root}/docs/manual-e2e/finance-scheduled-sync-lifecycle-e2e.md"
)
validate_api_only_guides "${api_only_guides[@]}"
for api_only_guide in "${api_only_guides[@]}"; do
  require_text "${api_only_guide}" 'make postgres-bootstrap'
  require_text "${api_only_guide}" 'owns restoring the normal PM2 backend'
done

copy_with_mutation() {
  local source="$1"
  local destination="$2"
  local old_text="$3"
  local new_text="$4"

  python3 - "${source}" "${destination}" "${old_text}" "${new_text}" <<'PY'
import sys

source, destination, old_text, new_text = sys.argv[1:]
with open(source, encoding="utf-8") as source_file:
    content = source_file.read()
if old_text not in content:
    raise SystemExit(f"mutation text not found in {source}: {old_text}")
with open(destination, "w", encoding="utf-8") as destination_file:
    destination_file.write(content.replace(old_text, new_text, 1))
PY
}

copy_with_early_restart() {
  local source="$1"
  local destination="$2"

  python3 - "${source}" "${destination}" <<'PY'
import sys

source, destination = sys.argv[1:]
with open(source, encoding="utf-8") as source_file:
    content = source_file.read()
worker = "go run ./cmd/sumweave jobs worker --once --env local\n"
restart = "pm2 start ecosystem.config.js\n"
if worker not in content or restart not in content:
    raise SystemExit("could not construct early-restart mutation")
content = content.replace(worker, restart + worker, 1)
prefix, suffix = content.rsplit(restart, 1)
content = prefix + "# moved PM2 restart\n" + suffix
with open(destination, "w", encoding="utf-8") as destination_file:
    destination_file.write(content)
PY
}

assert_rejected() {
  local name="$1"
  shift
  local output="${contract_test_dir}/${name}.out"

  if "$@" >"${output}" 2>&1; then
    printf 'adversarial documentation mutation was accepted: %s\n' "${name}" >&2
    exit 1
  fi
}

# Keep the contract self-testing without modifying checked-in guides. These
# mutations specifically guard against reverting to presence-only assertions.
readonly bad_psql_port="${contract_test_dir}/bad-psql-port.md"
copy_with_mutation "${psql_guides[0]}" "${bad_psql_port}" \
  'psql -p 55432' 'psql -p 5432'
assert_rejected bad-psql-port validate_psql_guides \
  "${bad_psql_port}" "${psql_guides[1]}" "${psql_guides[2]}"

readonly escaped_psql_quote="${contract_test_dir}/escaped-psql-quote.md"
copy_with_mutation "${psql_guides[0]}" "${escaped_psql_quote}" \
  '-c "' $'-c \\"'
assert_rejected escaped-psql-quote validate_psql_guides \
  "${escaped_psql_quote}" "${psql_guides[1]}" "${psql_guides[2]}"

readonly printf_psql_prose="${contract_test_dir}/printf-psql-prose.md"
copy_with_mutation "${psql_guides[0]}" "${printf_psql_prose}" \
  '```bash' $'```bash\nprintf "psql -p 5432"\n'
validate_psql_guides \
  "${printf_psql_prose}" "${psql_guides[1]}" "${psql_guides[2]}"

readonly nonshell_psql_fence="${contract_test_dir}/nonshell-psql-fence.md"
copy_with_mutation "${psql_guides[0]}" "${nonshell_psql_fence}" \
  '```bash' $'```text\npsql -p 5432\n```\n\n```bash'
validate_psql_guides \
  "${nonshell_psql_fence}" "${psql_guides[1]}" "${psql_guides[2]}"

readonly compound_psql_first_invalid="${contract_test_dir}/compound-psql-first-invalid.md"
copy_with_mutation "${psql_guides[0]}" "${compound_psql_first_invalid}" \
  '```bash' $'```bash\npsql -p 5432 -c "SELECT 1" && psql -p 55432 -c "SELECT 1"\n'
assert_rejected compound-psql-first-invalid validate_psql_guides \
  "${compound_psql_first_invalid}" "${psql_guides[1]}" "${psql_guides[2]}"

readonly assignment_prefixed_wrong_port="${contract_test_dir}/assignment-prefixed-wrong-port.md"
copy_with_mutation "${psql_guides[0]}" "${assignment_prefixed_wrong_port}" \
  '```bash' $'```bash\nPGOPTIONS=--client-min-messages=warning psql -p 5432 -c "SELECT 1"\n'
assert_rejected assignment-prefixed-wrong-port validate_psql_guides \
  "${assignment_prefixed_wrong_port}" "${psql_guides[1]}" "${psql_guides[2]}"

readonly quoted_assignment_wrong_port="${contract_test_dir}/quoted-assignment-wrong-port.md"
copy_with_mutation "${psql_guides[0]}" "${quoted_assignment_wrong_port}" \
  '```bash' $'```bash\nPGOPTIONS=\'--client-min-messages=warning\' psql -p \'5432\' -c "SELECT 1"\n'
assert_rejected quoted-assignment-wrong-port validate_psql_guides \
  "${quoted_assignment_wrong_port}" "${psql_guides[1]}" "${psql_guides[2]}"

readonly compose_project_option_step="${contract_test_dir}/compose-project-option-step.md"
copy_with_mutation "${psql_guides[0]}" "${compose_project_option_step}" \
  'docker compose exec' 'docker compose -p review135 exec'
validate_psql_guides \
  "${compose_project_option_step}" "${psql_guides[1]}" "${psql_guides[2]}"
readonly compose_project_option_wrong_port="${contract_test_dir}/compose-project-option-wrong-port.md"
copy_with_mutation "${compose_project_option_step}" "${compose_project_option_wrong_port}" \
  'psql -p 55432' 'psql -p 5432'
assert_rejected compose-project-option-wrong-port validate_psql_guides \
  "${compose_project_option_wrong_port}" "${psql_guides[1]}" "${psql_guides[2]}"

readonly compose_project_option_attached="${contract_test_dir}/compose-project-option-attached.md"
copy_with_mutation "${psql_guides[0]}" "${compose_project_option_attached}" \
  'docker compose exec' 'docker compose --project-name=review135 exec'
validate_psql_guides \
  "${compose_project_option_attached}" "${psql_guides[1]}" "${psql_guides[2]}"
copy_with_mutation "${compose_project_option_attached}" "${compose_project_option_attached}" \
  'psql -p 55432' 'psql -p 5432'
assert_rejected compose-project-option-attached validate_psql_guides \
  "${compose_project_option_attached}" "${psql_guides[1]}" "${psql_guides[2]}"

readonly compose_project_option_short_attached="${contract_test_dir}/compose-project-option-short-attached.md"
copy_with_mutation "${psql_guides[0]}" "${compose_project_option_short_attached}" \
  'docker compose exec' 'docker compose -preview135 exec'
validate_psql_guides \
  "${compose_project_option_short_attached}" "${psql_guides[1]}" "${psql_guides[2]}"
copy_with_mutation "${compose_project_option_short_attached}" "${compose_project_option_short_attached}" \
  'psql -p 55432' 'psql -p 5432'
assert_rejected compose-project-option-short-attached validate_psql_guides \
  "${compose_project_option_short_attached}" "${psql_guides[1]}" "${psql_guides[2]}"

readonly unsupported_docker_option="${contract_test_dir}/unsupported-docker-option.md"
copy_with_mutation "${psql_guides[0]}" "${unsupported_docker_option}" \
  'docker compose exec' 'docker --unsupported-option compose exec'
assert_rejected unsupported-docker-option validate_psql_guides \
  "${unsupported_docker_option}" "${psql_guides[1]}" "${psql_guides[2]}"

readonly unsupported_exec_option="${contract_test_dir}/unsupported-exec-option.md"
copy_with_mutation "${psql_guides[0]}" "${unsupported_exec_option}" \
  'docker compose exec' 'docker compose exec --unsupported-option'
assert_rejected unsupported-exec-option validate_psql_guides \
  "${unsupported_exec_option}" "${psql_guides[1]}" "${psql_guides[2]}"

readonly quoted_psql_port="${contract_test_dir}/quoted-psql-port.md"
copy_with_mutation "${psql_guides[0]}" "${quoted_psql_port}" \
  'psql -p 55432' "psql -p '55432'"
validate_psql_guides \
  "${quoted_psql_port}" "${psql_guides[1]}" "${psql_guides[2]}"

readonly attached_escaped_psql_command="${contract_test_dir}/attached-escaped-psql-command.md"
copy_with_mutation "${psql_guides[0]}" "${attached_escaped_psql_command}" \
  '```bash' $'```bash\npsql -p 55432 -c\\"SELECT 1\\"\n'
assert_rejected attached-escaped-psql-command validate_psql_guides \
  "${attached_escaped_psql_command}" "${psql_guides[1]}" "${psql_guides[2]}"

readonly bad_api_order="${contract_test_dir}/bad-api-order.md"
copy_with_mutation "${api_only_guides[0]}" "${bad_api_order}" \
  $'pm2 stop sumweave-api\ndocker compose down -v' \
  $'docker compose down -v\npm2 stop sumweave-api'
assert_rejected bad-api-order validate_api_only_guides \
  "${bad_api_order}" "${api_only_guides[1]}" "${api_only_guides[2]}" \
  "${api_only_guides[3]}" "${api_only_guides[4]}"

readonly reset_before_stop="${contract_test_dir}/reset-before-stop.md"
copy_with_mutation "${api_only_guides[0]}" "${reset_before_stop}" \
  $'pm2 stop sumweave-api\ndocker compose down -v' \
  $'docker compose down -v\npm2 stop sumweave-api\ndocker compose down -v'
assert_rejected reset-before-stop validate_api_only_guides \
  "${reset_before_stop}" "${api_only_guides[1]}" "${api_only_guides[2]}" \
  "${api_only_guides[3]}" "${api_only_guides[4]}"

readonly middle_restart_before_reset="${contract_test_dir}/middle-restart-before-reset.md"
copy_with_mutation "${api_only_guides[0]}" "${middle_restart_before_reset}" \
  $'pm2 stop sumweave-api\ndocker compose down -v' \
  $'pm2 stop sumweave-api\npm2 start ecosystem.config.js\ndocker compose down -v'
assert_rejected middle-restart-before-reset validate_api_only_guides \
  "${middle_restart_before_reset}" "${api_only_guides[1]}" "${api_only_guides[2]}" \
  "${api_only_guides[3]}" "${api_only_guides[4]}"

readonly early_api_restart="${contract_test_dir}/early-api-restart.md"
copy_with_early_restart "${api_only_guides[0]}" "${early_api_restart}"
assert_rejected early-api-restart validate_api_only_guides \
  "${early_api_restart}" "${api_only_guides[1]}" "${api_only_guides[2]}" \
  "${api_only_guides[3]}" "${api_only_guides[4]}"

readonly text_fence_lifecycle="${contract_test_dir}/text-fence-lifecycle.md"
copy_with_mutation "${api_only_guides[0]}" "${text_fence_lifecycle}" \
  'go run ./cmd/sumweave jobs worker --once --env local' '# worker removed'
printf '%s\n' '' '```text' 'go run ./cmd/sumweave jobs worker --once --env local' '```' \
  >>"${text_fence_lifecycle}"
assert_rejected text-fence-lifecycle validate_api_only_guides \
  "${text_fence_lifecycle}" "${api_only_guides[1]}" "${api_only_guides[2]}" \
  "${api_only_guides[3]}" "${api_only_guides[4]}"

readonly quoted_lifecycle_prose="${contract_test_dir}/quoted-lifecycle-prose.md"
copy_with_mutation "${api_only_guides[0]}" "${quoted_lifecycle_prose}" \
  'go run ./cmd/sumweave jobs worker --once --env local' \
  "printf 'go run ./cmd/sumweave jobs worker --once --env local'"
assert_rejected quoted-lifecycle-prose validate_api_only_guides \
  "${quoted_lifecycle_prose}" "${api_only_guides[1]}" "${api_only_guides[2]}" \
  "${api_only_guides[3]}" "${api_only_guides[4]}"

forbid_text "${repository_root}/docs/ARCHITECTURE.md" 'SQLite is local development only'
forbid_text "${repository_root}/docs/database-backed-state-plan.md" 'SQLite is supported for local development'
forbid_text "${repository_root}/docs/manual-e2e/README.md" 'default SQLite local workflow'
forbid_text "${repository_root}/docs/finances-management/design.md" 'SQLite for local development'
forbid_text "${repository_root}/apps/sumweave/doc/architecture.md" 'data/application.db'
forbid_text "${default_config}" 'data/application.db'
forbid_text "${repository_root}/AGENTS.md" 'SQLite as local-dev only storage'
