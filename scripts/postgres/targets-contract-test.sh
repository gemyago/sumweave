#!/usr/bin/env bash

set -euo pipefail

readonly repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

require_line() {
  local file="$1"
  local expected="$2"

	if ! grep -Fqx -- "${expected}" "${file}"; then
    printf 'missing expected target contract line in %s: %s\n' "${file}" "${expected}" >&2
    exit 1
  fi
}

require_contains() {
  local file="$1"
  local expected="$2"

	if ! grep -Fq -- "${expected}" "${file}"; then
    printf 'missing expected target contract text in %s: %s\n' "${file}" "${expected}" >&2
    exit 1
  fi
}

require_exact_exclusions() {
	local file="$1"
	shift
	local -a expected=("$@")
	local -a actual=()
	local line

	mapfile -t actual < <(sed -E -n 's/^[[:space:]]*-[[:space:]]*([^#[:space:]]+).*/\1/p' "${file}")
	if [[ "${#actual[@]}" -ne "${#expected[@]}" ]]; then
		printf 'unexpected exclusion count in %s\n' "${file}" >&2
		exit 1
	fi
	for line in "${expected[@]}"; do
		if [[ " ${actual[*]} " != *" ${line} "* ]]; then
			printf 'missing expected exclusion in %s: %s\n' "${file}" "${line}" >&2
			exit 1
		fi
	done
}

require_tagged_owner() {
	local source="$1"
	local owner="$2"

	if [[ "${owner}" == 'bootstrap' ]]; then
		return
	fi
	require_line "${repository_root}/${owner}" '//go:build postgres_test'
	if [[ ! -f "${repository_root}/${source}" ]]; then
		printf 'missing routine omission source: %s\n' "${source}" >&2
		exit 1
	fi
}

finance_routine_baseline_exclusions=(
	'mock_.*.go'
	'mock_*.go'
	'mock.go'
)

finance_routine_allowlisted_paths=(
	'^bank_connection_schedule_service.go$'
	'^bank_connection_service.go$'
	'^finance.go$'
	'^finance_cfg.go$'
	'^fixtures/realistic.go$'
	'^focused_services_composition.go$'
	'^fx.go$'
	'^fx_refresh_schedule_service.go$'
	'^imports.go$'
	'^persistence/account_balance_store.go$'
	'^persistence/bank_connection_schedule_store.go$'
	'^persistence/core_store.go$'
	'^persistence/csv_import_store.go$'
	'^persistence/database.go$'
	'^persistence/fx_refresh_schedule_store.go$'
	'^persistence/fx_store.go$'
	'^persistence/instant_predicate.go$'
	'^persistence/migrator.go$'
	'^persistence/models.go$'
	'^persistence/provider_link_persistence.go$'
	'^persistence/provider_snapshot_store.go$'
	'^persistence/provider_sync_state_journal_store.go$'
	'^persistence/provider_sync_store.go$'
	'^persistence/provider_window_sync_persistence.go$'
	'^persistence/store.go$'
	'^persistence/synthetic_pending_start_store.go$'
	'^persistence/synthetic_provider_store.go$'
	'^persistence/transaction_tag_store.go$'
	'^persistence/transfer_candidate_store.go$'
	'^provider_snapshot_service.go$'
	'^provider_sync.go$'
	'^reporting.go$'
	'^service_account_balances.go$'
	'^service_bank_sync.go$'
	'^service_catalog.go$'
	'^service_csv_import.go$'
	'^service_fx.go$'
	'^service_ledger.go$'
	'^service_ledger_contract.go$'
	'^service_reporting.go$'
	'^service_tenant_contract.go$'
	'^service_tenants.go$'
	'^synthetic_link_state_service.go$'
	'^terminal_failure.go$'
	'^timestamp.go$'
	'^transfer_detail_service.go$'
)

finance_routine_owner_sources=(
	'finance/bank_connection_schedule_service_test.go'
	'finance/bank_connection_service_test.go'
	'finance/finance_test.go'
	'finance/finance_cfg_test.go'
	'finance/fixtures/realistic_test.go'
	'finance/focused_public_services_test.go'
	'finance/reporting_fx_test.go'
	'finance/fx_refresh_schedule_service_test.go'
	'finance/imports_test.go'
	'finance/service_account_balances_test.go'
	'finance/persistence/bank_connection_schedule_store_test.go'
	'finance/persistence/store_test.go'
	'finance/persistence/csv_import_store_test.go'
	'finance/persistence/database_test.go'
	'finance/persistence/fx_refresh_schedule_store_test.go'
	'finance/reporting_fx_test.go'
	'finance/persistence/database_test.go'
	'bootstrap'
	'finance/persistence/fixtures_test.go'
	'finance/persistence/provider_link_persistence_test.go'
	'finance/persistence/provider_snapshot_store_test.go'
	'finance/persistence/provider_sync_state_journal_store_test.go'
	'finance/persistence/provider_sync_store_test.go'
	'finance/persistence/provider_window_sync_persistence_test.go'
	'finance/persistence/store_test.go'
	'finance/persistence/synthetic_pending_start_store_test.go'
	'finance/persistence/synthetic_provider_store_test.go'
	'finance/persistence/transaction_tag_store_test.go'
	'finance/persistence/transfer_candidate_store_test.go'
	'finance/provider_snapshot_service_test.go'
	'finance/service_bank_sync_orchestrator_test.go'
	'finance/reporting_fx_test.go'
	'finance/service_account_balances_test.go'
	'finance/service_bank_sync_orchestrator_test.go'
	'finance/service_test.go'
	'finance/imports_test.go'
	'finance/reporting_fx_test.go'
	'finance/service_test.go'
	'finance/service_test.go'
	'finance/service_test.go'
	'finance/service_test.go'
	'finance/service_test.go'
	'finance/synthetic_link_state_service_test.go'
	'finance/terminal_failure_test.go'
	'finance/reporting_fx_test.go'
	'finance/transfer_detail_service_test.go'
)

if [[ "${#finance_routine_allowlisted_paths[@]}" -ne "${#finance_routine_owner_sources[@]}" ]]; then
	printf '%s\n' 'finance routine allowlist and owner mapping must have equal lengths' >&2
	exit 1
fi

require_finance_routine_owner() {
	local path="$1"
	local index
	local source

	for index in "${!finance_routine_allowlisted_paths[@]}"; do
		if [[ "${finance_routine_allowlisted_paths[${index}]}" == "${path}" ]]; then
			source="finance/${path#^}"
			source="${source%\$}"
			require_tagged_owner "${source}" "${finance_routine_owner_sources[${index}]}"
			return
		fi
	done

	printf 'unexpected finance routine exclusion: %s\n' "${path}" >&2
	exit 1
}

require_finance_routine_exclusions() {
	local file="$1"
	local -a actual=()
	local line
	local baseline
	local count
	local i
	local j
	local is_baseline

	mapfile -t actual < <(sed -E -n 's/^[[:space:]]*-[[:space:]]*([^#[:space:]]+).*/\1/p' "${file}")
	for ((i = 0; i < ${#actual[@]}; i++)); do
		for ((j = i + 1; j < ${#actual[@]}; j++)); do
			if [[ "${actual[${i}]}" == "${actual[${j}]}" ]]; then
				printf 'duplicate finance routine exclusion in %s: %s\n' "${file}" "${actual[${i}]}" >&2
				exit 1
			fi
		done
	done

	for baseline in "${finance_routine_baseline_exclusions[@]}"; do
		count=0
		for line in "${actual[@]}"; do
			if [[ "${line}" == "${baseline}" ]]; then
				count=$((count + 1))
			fi
		done
		if [[ "${count}" -ne 1 ]]; then
			printf 'finance baseline exclusion must occur exactly once in %s: %s\n' "${file}" "${baseline}" >&2
			exit 1
		fi
	done

	for line in "${actual[@]}"; do
		is_baseline=0
		for baseline in "${finance_routine_baseline_exclusions[@]}"; do
			if [[ "${line}" == "${baseline}" ]]; then
				is_baseline=1
				break
			fi
		done
		if [[ "${is_baseline}" -eq 0 ]]; then
			require_finance_routine_owner "${line}"
		fi
	done
}

readonly root_makefile="${repository_root}/Makefile"
require_line "${root_makefile}" 'override repository_makefile_dir := $(realpath $(dir $(lastword $(MAKEFILE_LIST))))'
require_line "${root_makefile}" '.NOTPARALLEL: postgres-verify'
require_line "${root_makefile}" 'postgres_test_dsn=postgres://sumweave_runtime:sumweave_runtime_local@$(POSTGRES_HOST):$(POSTGRES_PORT)/sumweave_test?sslmode=disable'
require_line "${root_makefile}" 'ifneq ($(POSTGRES_HOST):$(POSTGRES_PORT),127.0.0.1:55432)'
require_line "${root_makefile}" 'postgres_test_app_env=APP_APPLICATION_DATABASE_DSN="$(postgres_test_dsn)" APP_AGENTRUNTIME_DATABASE_DSN="$(postgres_test_dsn)"'
require_line "${root_makefile}" 'postgres-test-runtime: postgres-bootstrap'
require_line "${root_makefile}" 'postgres-test-finance: export SUMWEAVE_FINANCE_MIGRATION_COVER_DIR=$(repository_makefile_dir)/finance/.cover/postgres-migration'
require_line "${root_makefile}" 'postgres-test-finance: postgres-bootstrap'
require_line "${root_makefile}" 'postgres-test-sumweave: postgres-bootstrap'
require_line "${root_makefile}" 'postgres-verify: export SUMWEAVE_FINANCE_MIGRATION_COVER_DIR=$(repository_makefile_dir)/finance/.cover/postgres-migration'
require_line "${root_makefile}" 'postgres-verify: postgres-test-runtime postgres-test-finance postgres-test-sumweave'
require_contains "${root_makefile}" 'SUMWEAVE_POSTGRES_TEST_DSN="$(postgres_test_dsn)" $(MAKE) -C runtime test-postgres'
require_contains "${root_makefile}" 'SUMWEAVE_POSTGRES_TEST_DSN="$(postgres_test_dsn)" $(MAKE) -C finance test-postgres'
require_contains "${root_makefile}" 'SUMWEAVE_POSTGRES_TEST_DSN="$(postgres_test_dsn)" $(postgres_test_app_env) $(MAKE) -C apps/sumweave test-postgres'
require_contains "${root_makefile}" 'cat tools/firecrawl/.cover/profile.out > $(cover_profile)'
require_contains "${root_makefile}" 'tail -n +2 tools/skills/.cover/profile.out >> $(cover_profile)'
require_contains "${root_makefile}" 'tail -n +2 tools/workspacefs/.cover/profile.out >> $(cover_profile)'
require_contains "${root_makefile}" 'tail -n +2 runtime/.cover/routine.out >> $(cover_profile)'
require_contains "${root_makefile}" 'tail -n +2 finance/.cover/routine.out >> $(cover_profile)'
require_contains "${root_makefile}" 'tail -n +2 apps/sumweave/.cover/routine.out >> $(cover_profile)'
require_contains "${root_makefile}" 'go tool cover -html=$(cover_profile) -o $(cover_dir)/coverage.html'
require_contains "${root_makefile}" '$(go-test-coverage) --badge-file-name $(cover_dir)/coverage.svg --profile $(cover_profile)'

if [[ "$(grep -Fc 'SUMWEAVE_FINANCE_MIGRATION_COVER_DIR=$(repository_makefile_dir)/finance/.cover/postgres-migration' "${root_makefile}")" -ne 2 ]]; then
	printf '%s\n' 'only postgres-test-finance and postgres-verify may own finance migration coverage' >&2
	exit 1
fi

readonly bootstrap_script="${repository_root}/scripts/postgres/bootstrap.sh"
require_contains "${bootstrap_script}" 'migration_cover_dir="${SUMWEAVE_FINANCE_MIGRATION_COVER_DIR:-}"'
require_contains "${bootstrap_script}" 'expected_migration_cover_dir="${repository_root}/finance/.cover/postgres-migration"'
require_contains "${bootstrap_script}" 'Coverage mode operates only in a trusted repository worktree with no'
require_contains "${bootstrap_script}" 'reject unsafe inputs and pre-existing paths; they do not close a TOCTOU race.'
require_contains "${bootstrap_script}" 'if [[ -n "${migration_cover_dir}" && "${migration_cover_dir}" != "${expected_migration_cover_dir}" ]]; then'
require_contains "${bootstrap_script}" 'validate_migration_cover_dir() {'
require_contains "${bootstrap_script}" 'if [[ -L "${migration_cover_parent}" || -L "${migration_cover_dir}" ]]; then'
require_contains "${bootstrap_script}" 'resolved_migration_cover_parent="$(cd "${migration_cover_parent}" && pwd -P)"'
require_contains "${bootstrap_script}" 'if [[ "${resolved_migration_cover_parent}" != "${migration_cover_parent}" ]]; then'
require_contains "${bootstrap_script}" 'resolved_migration_cover_dir="$(cd "${migration_cover_dir}" && pwd -P)"'
require_contains "${bootstrap_script}" 'if [[ "${resolved_migration_cover_dir}" != "${expected_migration_cover_dir}" ]]; then'
require_contains "${bootstrap_script}" 'validate_migration_cover_dir'
require_contains "${bootstrap_script}" 'rm -f -- "${migration_cover_dir}/ready"'
require_contains "${bootstrap_script}" 'rm -rf -- "${migration_cover_dir}/raw"'
require_contains "${bootstrap_script}" 'mkdir -p "${migration_cover_dir}/raw"'
require_contains "${bootstrap_script}" 'test_migration_command=(env "GOCOVERDIR=${migration_cover_dir}/raw" go run -covermode=atomic -coverpkg=github.com/gemyago/sumweave/apps/sumweave/cmd/sumweave,github.com/gemyago/sumweave/finance/...)'
require_contains "${bootstrap_script}" '"${test_migration_command[@]}" ./cmd/sumweave db-migrate --env test'
require_contains "${bootstrap_script}" 'go tool covdata textfmt -i="${migration_cover_dir}/raw" -pkg=github.com/gemyago/sumweave/finance/... -o=/dev/null'
require_contains "${bootstrap_script}" 'touch "${migration_cover_dir}/ready"'
require_contains "${repository_root}/scripts/postgres/bootstrap-contract-test.sh" 'entry after validation is outside its trusted, non-concurrently-mutated'
if [[ "$(grep -Fc 'GOCOVERDIR' "${bootstrap_script}")" -ne 1 ]]; then
  printf '%s\n' 'bootstrap must set GOCOVERDIR only on the instrumented test migration command' >&2
  exit 1
fi
if [[ "$(grep -Fc 'db-migrate --env test' "${bootstrap_script}")" -ne 1 ]]; then
	printf '%s\n' 'bootstrap must retain exactly one executable test migration command' >&2
	exit 1
fi

readonly finance_makefile="${repository_root}/finance/Makefile"
require_contains "${finance_makefile}" 'Tagged coverage operates only in a trusted repository worktree with no'
require_contains "${finance_makefile}" 'cannot make later pathname operations safe against a concurrent replacement.'
require_contains "${finance_makefile}" 'finance_dir="$$(pwd -P)"'
require_contains "${finance_makefile}" 'postgres_cover_dir="$${finance_dir}/.cover"'
require_contains "${finance_makefile}" 'postgres_cover_profile="$${postgres_cover_dir}/postgres.out"'
require_contains "${finance_makefile}" 'expected_migration_cover_dir="$${postgres_cover_dir}/postgres-migration"'
require_contains "${finance_makefile}" 'migration_cover_dir="$${SUMWEAVE_FINANCE_MIGRATION_COVER_DIR:-}"'
require_contains "${finance_makefile}" 'migration_cover_raw_dir="$${migration_cover_dir}/raw"'
require_contains "${finance_makefile}" 'postgres_test_raw_dir="$${postgres_cover_dir}/postgres-test-raw"'
require_contains "${finance_makefile}" 'test "$${migration_cover_dir}" = "$${expected_migration_cover_dir}"'
require_contains "${finance_makefile}" 'test ! -L "$${postgres_cover_dir}"'
require_contains "${finance_makefile}" 'test "$$(cd "$${postgres_cover_dir}" && pwd -P)" = "$${postgres_cover_dir}"'
require_contains "${finance_makefile}" 'test ! -L "$${expected_migration_cover_dir}"'
require_contains "${finance_makefile}" 'test "$$(cd "$${expected_migration_cover_dir}" && pwd -P)" = "$${expected_migration_cover_dir}"'
require_contains "${finance_makefile}" 'test -n "$${migration_cover_dir}"'
require_contains "${finance_makefile}" 'test -f "$${migration_cover_dir}/ready"'
require_contains "${finance_makefile}" 'test -d "$${migration_cover_raw_dir}"'
require_contains "${finance_makefile}" 'find "$${migration_cover_raw_dir}" -type f -size +0c -print -quit'
require_contains "${finance_makefile}" 'test "$${migration_cover_dir}/ready" -nt "$${migration_cover_raw_dir}"'
require_contains "${finance_makefile}" 'rm -rf -- "$${postgres_test_raw_dir}"'
require_contains "${finance_makefile}" 'mkdir -p -- "$${postgres_test_raw_dir}"'
require_contains "${finance_makefile}" 'rm -f -- "$${postgres_cover_profile}"'
require_contains "${finance_makefile}" '-test.gocoverdir="$${postgres_test_raw_dir}"'
require_contains "${finance_makefile}" 'go tool covdata textfmt -i="$${migration_cover_raw_dir},$${postgres_test_raw_dir}" -pkg=github.com/gemyago/sumweave/finance/... -o="$${postgres_cover_profile}"'
if grep -Fq '$(CURDIR)' "${finance_makefile}" || grep -Fq '$(MAKEFILE_LIST)' "${finance_makefile}"; then
  printf '%s\n' 'finance PostgreSQL coverage paths must not derive from Make special variables' >&2
  exit 1
fi
if grep -Fq 'GOCOVERDIR' "${finance_makefile}"; then
	printf '%s\n' 'finance test binary must use -test.gocoverdir, not GOCOVERDIR' >&2
	exit 1
fi

readonly cover_dir_override_test_dir="$(mktemp -d "${repository_root}/tmp/postgres-target-contract.XXXXXX")"
cleanup_cover_dir_override_test() {
  if [[ -n "${finance_cover_dir:-}" ]] && [[ -e "${finance_cover_dir}" || -L "${finance_cover_dir}" ]]; then
    rm -rf -- "${finance_cover_dir}"
  fi
  if [[ "${finance_cover_dir_was_present:-0}" -eq 1 ]]; then
    mv -- "${saved_finance_cover_dir}" "${finance_cover_dir}"
  fi
  rm -rf -- "${cover_dir_override_test_dir}"
}
trap cleanup_cover_dir_override_test EXIT

# Use a valid fresh bootstrap fixture so each override attempt reaches the
# tagged cleanup boundary instead of being masked by readiness validation.
readonly finance_cover_dir="${repository_root}/finance/.cover"
readonly saved_finance_cover_dir="${cover_dir_override_test_dir}/saved-finance-cover"
finance_cover_dir_was_present=0
if [[ -e "${finance_cover_dir}" || -L "${finance_cover_dir}" ]]; then
  mv -- "${finance_cover_dir}" "${saved_finance_cover_dir}"
  finance_cover_dir_was_present=1
fi
mkdir -p -- "${finance_cover_dir}/postgres-migration/raw"
printf '%s\n' 'valid bootstrap coverage fixture' > "${finance_cover_dir}/postgres-migration/raw/fixture"
sleep 1
touch "${finance_cover_dir}/postgres-migration/ready"

make_fake_command() {
  local name="$1"
  local content="$2"

  printf '%s\n' "${content}" > "${cover_dir_override_test_dir}/${name}"
  chmod +x "${cover_dir_override_test_dir}/${name}"
}

make_fake_command go '#!/usr/bin/env bash
printf "%s\n" "$*" >> "${POSTGRES_GO_LOG}"
if [[ "$1" == "test" ]]; then
  exit 0
fi
if [[ "$1" == "tool" && "$2" == "covdata" ]]; then
  exit 17
fi
exit 17'

assert_pinned_directory_override() {
  local name="$1"
  local variable="$2"
  local sentinel="${cover_dir_override_test_dir}/${name}-sentinel"
  local output="${cover_dir_override_test_dir}/${name}.out"
  local go_log="${cover_dir_override_test_dir}/${name}.go.log"

  mkdir -- "${sentinel}"
  touch "${sentinel}/unchanged"
  if SUMWEAVE_FINANCE_MIGRATION_COVER_DIR="${finance_cover_dir}/postgres-migration" \
    PATH="${cover_dir_override_test_dir}:${PATH}" POSTGRES_GO_LOG="${go_log}" \
    make -C "${repository_root}/finance" test-postgres "${variable}=${sentinel}" \
    >"${output}" 2>&1; then
    printf 'expected pinned override case to fail at fake covdata: %s\n' "${name}" >&2
    exit 1
  fi
  test -f "${sentinel}/unchanged"
  test -f "${go_log}"
  if grep -Fq -- "${sentinel}" "${go_log}"; then
    printf 'override changed a finance coverage path: %s\n' "${name}" >&2
    exit 1
  fi
}

assert_pinned_file_override() {
  local name="$1"
  local variable="$2"
  local sentinel="${cover_dir_override_test_dir}/${name}-sentinel"
  local output="${cover_dir_override_test_dir}/${name}.out"
  local go_log="${cover_dir_override_test_dir}/${name}.go.log"

  touch "${sentinel}"
  if SUMWEAVE_FINANCE_MIGRATION_COVER_DIR="${finance_cover_dir}/postgres-migration" \
    PATH="${cover_dir_override_test_dir}:${PATH}" POSTGRES_GO_LOG="${go_log}" \
    make -C "${repository_root}/finance" test-postgres "${variable}=${sentinel}" \
    >"${output}" 2>&1; then
    printf 'expected pinned override case to fail at fake covdata: %s\n' "${name}" >&2
    exit 1
  fi
  test -f "${sentinel}"
  test -f "${go_log}"
  if grep -Fq -- "${sentinel}" "${go_log}"; then
    printf 'override changed a finance coverage path: %s\n' "${name}" >&2
    exit 1
  fi
}

assert_pinned_directory_override postgres-cover-dir postgres_cover_dir
assert_pinned_file_override postgres-cover-profile postgres_cover_profile
assert_pinned_directory_override migration-cover-dir migration_cover_dir
assert_pinned_directory_override migration-cover-raw-dir migration_cover_raw_dir
assert_pinned_directory_override expected-migration-cover-dir expected_migration_cover_dir
assert_pinned_directory_override postgres-test-raw-dir postgres_test_raw_dir
assert_pinned_directory_override cover-dir cover_dir

assert_special_variable_sentinel_is_ignored() {
  local name="$1"
  local variable="$2"
  local mode="$3"
  local sentinel="${cover_dir_override_test_dir}/${name}-external"
  local output="${cover_dir_override_test_dir}/${name}.out"
  local go_log="${cover_dir_override_test_dir}/${name}.go.log"

  mkdir -- "${sentinel}"
  touch "${sentinel}/unchanged"
  if [[ "${mode}" == 'command-line' ]]; then
    if SUMWEAVE_FINANCE_MIGRATION_COVER_DIR="${finance_cover_dir}/postgres-migration" \
      PATH="${cover_dir_override_test_dir}:${PATH}" POSTGRES_GO_LOG="${go_log}" \
      make -C "${repository_root}/finance" test-postgres "${variable}=${sentinel}/attacker.mk" \
      >"${output}" 2>&1; then
      printf 'expected fake covdata failure for special-variable case: %s\n' "${name}" >&2
      exit 1
    fi
  else
    if SUMWEAVE_FINANCE_MIGRATION_COVER_DIR="${finance_cover_dir}/postgres-migration" \
      PATH="${cover_dir_override_test_dir}:${PATH}" POSTGRES_GO_LOG="${go_log}" \
      env MAKEFLAGS=-e "${variable}=${sentinel}/attacker.mk" \
      make -C "${repository_root}/finance" test-postgres >"${output}" 2>&1; then
      printf 'expected fake covdata failure for special-variable case: %s\n' "${name}" >&2
      exit 1
    fi
  fi
  test -f "${sentinel}/unchanged"
  test ! -e "${sentinel}/.cover"
  test -f "${go_log}"
  if grep -Fq -- "${sentinel}" "${output}" || grep -Fq -- "${sentinel}" "${go_log}"; then
    printf 'special Make variable escaped into finance coverage commands: %s\n' "${name}" >&2
    exit 1
  fi
}

assert_special_variable_sentinel_is_ignored command-line-curdir CURDIR command-line
assert_special_variable_sentinel_is_ignored environment-curdir CURDIR environment
assert_special_variable_sentinel_is_ignored command-line-makefile-list MAKEFILE_LIST command-line
assert_special_variable_sentinel_is_ignored environment-makefile-list MAKEFILE_LIST environment

require_exact_exclusions "${repository_root}/runtime/.testcoverage.yaml" \
	'testing.go' 'mock_.*.go' 'mocks_.*.go' 'mock_*.go' 'mock.go' 'internal/agentapi/.*.gen.go'
require_exact_exclusions "${repository_root}/finance/.testcoverage.yaml" \
	'mock_.*.go' 'mock_*.go' 'mock.go'
require_exact_exclusions "${repository_root}/apps/sumweave/.testcoverage.yaml" \
	'testing.go' 'mock_.*.go' 'mock_*.go' 'mock.go' 'internal/telemetry/otel.go' 'internal/telemetry/otel_logger.go' 'internal/telemetry/otel_meter.go' 'internal/telemetry/otel_tracer.go' 'cmd/sumweave/engine_cmd.go' 'internal/runtime.go' 'internal/api/http/v1routes/.*' 'internal/app/models/.*'
require_exact_exclusions "${repository_root}/runtime/.testcoverage-routine.yaml" \
	'testing.go' 'mock_.*.go' 'mocks_.*.go' 'mock_*.go' 'mock.go' 'internal/agentapi/.*.gen.go' \
	'^agent/agent_profiles\.go$' '^agent/providers_config\.go$' '^internal/agentprofiles/db_agent_profiles_service\.go$' '^internal/llmproviders/db_providers_config_service\.go$' '^internal/sessions/database\.go$'
assert_finance_routine_contracts() {
	local name="$1"
	local output="${cover_dir_override_test_dir}/${name}.out"
	local fixture="${cover_dir_override_test_dir}/${name}.yaml"
	shift
	{
		printf '%s\n' 'exclude:' '  paths:'
		printf '    - %s\n' "${finance_routine_baseline_exclusions[@]}"
		printf '    - %s\n' "$@"
	} > "${fixture}"
	if (require_finance_routine_exclusions "${fixture}") >"${output}" 2>&1; then
		printf 'expected finance routine contract case to fail: %s\n' "${name}" >&2
		exit 1
	fi
}

assert_finance_routine_contract_accepts_baseline_only() {
	local fixture="${cover_dir_override_test_dir}/baseline-only.yaml"
	{
		printf '%s\n' 'exclude:' '  paths:'
		printf '    - %s\n' "${finance_routine_baseline_exclusions[@]}"
	} > "${fixture}"
	if ! require_finance_routine_exclusions "${fixture}" >"${fixture}.out" 2>&1; then
		printf '%s\n' 'expected finance routine baseline-only contract case to pass' >&2
		exit 1
	fi
}

assert_finance_routine_contract_accepts_baseline_only
assert_finance_routine_contracts duplicate-baseline 'mock.go'
assert_finance_routine_contracts duplicate-allowlisted '^finance.go$' '^finance.go$'
assert_finance_routine_contracts unknown-anchored '^unknown.go$'
assert_finance_routine_contracts broad-anchored '^internal/.*$'
assert_finance_routine_contracts unanchored 'finance.go'
assert_finance_routine_contracts directory '^persistence/$'
require_finance_routine_exclusions "${repository_root}/finance/.testcoverage-routine.yaml"
require_exact_exclusions "${repository_root}/apps/sumweave/.testcoverage-routine.yaml" \
	'testing.go' 'mock_.*.go' 'mock_*.go' 'mock.go' 'internal/telemetry/otel.go' 'internal/telemetry/otel_logger.go' 'internal/telemetry/otel_meter.go' 'internal/telemetry/otel_tracer.go' 'cmd/sumweave/engine_cmd.go' 'internal/runtime.go' 'internal/api/http/v1routes/.*' 'internal/app/models/.*' '^internal/application_database\.go$' '^internal/appdispatch/postgres_transport\.go$' '^internal/jobs/store\.go$' '^internal/auth/user_store\.go$' '^internal/auth/refresh_store\.go$' '^internal/appdispatch/migrator\.go$'

require_tagged_owner 'runtime/agent/agent_profiles.go' 'runtime/agent/database_services_postgres_test.go'
require_tagged_owner 'runtime/agent/providers_config.go' 'runtime/agent/database_services_postgres_test.go'
require_tagged_owner 'runtime/internal/agentprofiles/db_agent_profiles_service.go' 'runtime/internal/agentprofiles/db_agent_profiles_service_postgres_test.go'
require_tagged_owner 'runtime/internal/llmproviders/db_providers_config_service.go' 'runtime/internal/llmproviders/db_providers_config_service_postgres_test.go'
require_tagged_owner 'runtime/internal/sessions/database.go' 'runtime/internal/sessions/database_service_postgres_test.go'
require_tagged_owner 'apps/sumweave/internal/application_database.go' 'apps/sumweave/internal/application_composition_postgres_test.go'
require_tagged_owner 'apps/sumweave/internal/appdispatch/postgres_transport.go' 'apps/sumweave/internal/appdispatch/appdispatch_test.go'
require_tagged_owner 'apps/sumweave/internal/jobs/store.go' 'apps/sumweave/internal/jobs/store_postgres_test.go'
require_tagged_owner 'apps/sumweave/internal/auth/user_store.go' 'apps/sumweave/internal/auth/user_store_test.go'
require_tagged_owner 'apps/sumweave/internal/auth/refresh_store.go' 'apps/sumweave/internal/auth/refresh_store_test.go'
require_tagged_owner 'apps/sumweave/internal/appdispatch/migrator.go' 'apps/sumweave/internal/appdispatch/appdispatch_test.go'

for module in runtime finance apps/sumweave; do
  module_makefile="${repository_root}/${module}/Makefile"
  routine_config="${repository_root}/${module}/.testcoverage-routine.yaml"
  postgres_config="${repository_root}/${module}/.testcoverage.yaml"

  if [[ "${module}" == 'finance' ]]; then
    require_line "${module_makefile}" '.PHONY: lint test test-postgres'
  else
    require_contains "${module_makefile}" '.PHONY: lint test test-postgres'
  fi
  require_line "${module_makefile}" 'routine_cover_profile=$(cover_dir)/routine.out'
  if [[ "${module}" == 'finance' ]]; then
    require_contains "${module_makefile}" 'postgres_cover_profile="$${postgres_cover_dir}/postgres.out"'
  else
    require_line "${module_makefile}" 'postgres_cover_profile=$(cover_dir)/postgres.out'
  fi
  require_contains "${module_makefile}" 'go test -timeout=${DEFAULT_TESTS_TIMEOUT}'
  require_contains "${module_makefile}" '-coverprofile=$(routine_cover_profile)'
  require_contains "${module_makefile}" 'SUMWEAVE_POSTGRES_TEST_DSN="$(SUMWEAVE_POSTGRES_TEST_DSN)" go test -tags=postgres_test'
  require_contains "${module_makefile}" '$(go-test-coverage) --config .testcoverage-routine.yaml --profile $(routine_cover_profile)'
  if [[ "${module}" == 'finance' ]]; then
    require_contains "${module_makefile}" '$(go-test-coverage) --config .testcoverage.yaml --profile "$${postgres_cover_profile}"'
  else
    require_contains "${module_makefile}" '$(go-test-coverage) --config .testcoverage.yaml --profile $(postgres_cover_profile)'
  fi
  require_contains "${module_makefile}" 'go test -tags=postgres_test'
  if [[ "${module}" != 'finance' ]]; then
    require_contains "${module_makefile}" '-coverprofile=$(postgres_cover_profile)'
  fi
  require_line "${routine_config}" '  file: 90'
  require_line "${routine_config}" '  total: 90'
  require_line "${postgres_config}" '  file: 90'
  require_line "${postgres_config}" '  total: 90'
done
