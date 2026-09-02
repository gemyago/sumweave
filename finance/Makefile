.PHONY: lint test test-postgres

include ../build/make/golangci-lint.mk

cover_dir=.cover
routine_cover_profile=$(cover_dir)/routine.out

# Tagged coverage operates only in a trusted repository worktree with no
# concurrent writer. Static validation rejects pre-existing unsafe paths; it
# cannot make later pathname operations safe against a concurrent replacement.

DEFAULT_TESTS_TIMEOUT ?= 20s

.DEFAULT_GOAL := all

all: test

lint: $(GOLANGCI_LINT_PREREQS)
	$(GOLANGCI_LINT_RUN)

$(cover_dir):
	mkdir -p $(cover_dir)

go-test-coverage=go run github.com/vladopajic/go-test-coverage/v2

test: $(cover_dir)
	TZ=US/Alaska go test -timeout=${DEFAULT_TESTS_TIMEOUT} -shuffle=on -failfast -coverpkg=./... -coverprofile=$(routine_cover_profile) -covermode=atomic ./...
	$(go-test-coverage) --config .testcoverage-routine.yaml --profile $(routine_cover_profile)

test-postgres:
	@set -eu; \
	finance_dir="$$(pwd -P)"; \
	postgres_cover_dir="$${finance_dir}/.cover"; \
	postgres_cover_profile="$${postgres_cover_dir}/postgres.out"; \
	expected_migration_cover_dir="$${postgres_cover_dir}/postgres-migration"; \
	migration_cover_dir="$${SUMWEAVE_FINANCE_MIGRATION_COVER_DIR:-}"; \
	migration_cover_raw_dir="$${migration_cover_dir}/raw"; \
	postgres_test_raw_dir="$${postgres_cover_dir}/postgres-test-raw"; \
	test -n "$${migration_cover_dir}" || (printf '%s\n' 'SUMWEAVE_FINANCE_MIGRATION_COVER_DIR is required for finance PostgreSQL coverage' >&2; exit 1); \
	test "$${migration_cover_dir}" = "$${expected_migration_cover_dir}" || (printf '%s\n' 'SUMWEAVE_FINANCE_MIGRATION_COVER_DIR must equal the repository finance coverage directory' >&2; exit 1); \
	test ! -L "$${postgres_cover_dir}" || (printf '%s\n' 'finance PostgreSQL coverage parent must not use symlinks' >&2; exit 1); \
	test ! -e "$${postgres_cover_dir}" || test -d "$${postgres_cover_dir}" || (printf '%s\n' 'finance PostgreSQL coverage parent must be a directory' >&2; exit 1); \
	mkdir -p -- "$${postgres_cover_dir}"; \
	test ! -L "$${postgres_cover_dir}" || (printf '%s\n' 'finance PostgreSQL coverage parent must not use symlinks' >&2; exit 1); \
	test "$$(cd "$${postgres_cover_dir}" && pwd -P)" = "$${postgres_cover_dir}" || (printf '%s\n' 'finance PostgreSQL coverage parent must be repository-owned' >&2; exit 1); \
	test ! -L "$${expected_migration_cover_dir}" || (printf '%s\n' 'finance migration coverage directory must not use symlinks' >&2; exit 1); \
	test -d "$${expected_migration_cover_dir}" || (printf '%s\n' 'finance migration coverage raw data is missing' >&2; exit 1); \
	test "$$(cd "$${expected_migration_cover_dir}" && pwd -P)" = "$${expected_migration_cover_dir}" || (printf '%s\n' 'finance migration coverage directory must be repository-owned' >&2; exit 1); \
	test -f "$${migration_cover_dir}/ready" || (printf '%s\n' 'finance migration coverage readiness marker is missing' >&2; exit 1); \
	test -d "$${migration_cover_raw_dir}" || (printf '%s\n' 'finance migration coverage raw data is missing' >&2; exit 1); \
	test -n "$$(find "$${migration_cover_raw_dir}" -type f -size +0c -print -quit)" || (printf '%s\n' 'finance migration coverage raw data is empty' >&2; exit 1); \
	test "$${migration_cover_dir}/ready" -nt "$${migration_cover_raw_dir}" || (printf '%s\n' 'finance migration coverage raw data is stale' >&2; exit 1); \
	rm -rf -- "$${postgres_test_raw_dir}"; \
	mkdir -p -- "$${postgres_test_raw_dir}"; \
	rm -f -- "$${postgres_cover_profile}"; \
	TZ=US/Alaska SUMWEAVE_POSTGRES_TEST_DSN="$(SUMWEAVE_POSTGRES_TEST_DSN)" go test -tags=postgres_test -timeout=${DEFAULT_TESTS_TIMEOUT} -shuffle=on -failfast -coverpkg=./... -covermode=atomic ./... -args -test.gocoverdir="$${postgres_test_raw_dir}"; \
	go tool covdata textfmt -i="$${migration_cover_raw_dir},$${postgres_test_raw_dir}" -pkg=github.com/gemyago/sumweave/finance/... -o="$${postgres_cover_profile}"; \
	$(go-test-coverage) --config .testcoverage.yaml --profile "$${postgres_cover_profile}"
