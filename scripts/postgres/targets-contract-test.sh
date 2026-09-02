#!/usr/bin/env bash

set -euo pipefail

readonly repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

require_line() {
  local file="$1"
  local expected="$2"

  if ! grep -Fqx "${expected}" "${file}"; then
    printf 'missing expected target contract line in %s: %s\n' "${file}" "${expected}" >&2
    exit 1
  fi
}

require_equal_files() {
  local first="$1"
  local second="$2"

  if ! cmp -s "${first}" "${second}"; then
    printf 'expected equivalent coverage configurations: %s and %s\n' "${first}" "${second}" >&2
    exit 1
  fi
}

require_contains() {
  local file="$1"
  local expected="$2"

  if ! grep -Fq "${expected}" "${file}"; then
    printf 'missing expected target contract text in %s: %s\n' "${file}" "${expected}" >&2
    exit 1
  fi
}

readonly root_makefile="${repository_root}/Makefile"
require_line "${root_makefile}" '.NOTPARALLEL: postgres-verify'
require_line "${root_makefile}" 'postgres_test_dsn=postgres://sumweave_runtime:sumweave_runtime_local@$(POSTGRES_HOST):$(POSTGRES_PORT)/sumweave_test?sslmode=disable'
require_line "${root_makefile}" 'ifneq ($(POSTGRES_HOST):$(POSTGRES_PORT),127.0.0.1:55432)'
require_line "${root_makefile}" 'postgres_test_app_env=APP_APPLICATION_DATABASE_DSN="$(postgres_test_dsn)" APP_AGENTRUNTIME_DATABASE_DSN="$(postgres_test_dsn)"'
require_line "${root_makefile}" 'postgres-test-runtime: postgres-bootstrap'
require_line "${root_makefile}" 'postgres-test-finance: postgres-bootstrap'
require_line "${root_makefile}" 'postgres-test-sumweave: postgres-bootstrap'
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

for module in runtime finance apps/sumweave; do
  module_makefile="${repository_root}/${module}/Makefile"
  routine_config="${repository_root}/${module}/.testcoverage-routine.yaml"
  postgres_config="${repository_root}/${module}/.testcoverage.yaml"

  require_contains "${module_makefile}" '.PHONY: lint test test-postgres'
  require_line "${module_makefile}" 'routine_cover_profile=$(cover_dir)/routine.out'
  require_line "${module_makefile}" 'postgres_cover_profile=$(cover_dir)/postgres.out'
  require_contains "${module_makefile}" 'go test -timeout=${DEFAULT_TESTS_TIMEOUT}'
  require_contains "${module_makefile}" '-coverprofile=$(routine_cover_profile)'
  require_contains "${module_makefile}" 'SUMWEAVE_POSTGRES_TEST_DSN="$(SUMWEAVE_POSTGRES_TEST_DSN)" go test -tags=postgres_test'
  require_contains "${module_makefile}" '-coverprofile=$(postgres_cover_profile)'
  require_contains "${module_makefile}" '$(go-test-coverage) --config .testcoverage-routine.yaml --profile $(routine_cover_profile)'
  require_contains "${module_makefile}" '$(go-test-coverage) --config .testcoverage.yaml --profile $(postgres_cover_profile)'
  require_contains "${module_makefile}" 'go test -tags=postgres_test'
  require_line "${routine_config}" '  file: 90'
  require_line "${routine_config}" '  total: 90'
  require_line "${postgres_config}" '  file: 90'
  require_line "${postgres_config}" '  total: 90'
  require_equal_files "${routine_config}" "${postgres_config}"
done
