.PHONY: lint test test-postgres

include ../build/make/golangci-lint.mk

cover_dir=.cover
routine_cover_profile=$(cover_dir)/routine.out
postgres_cover_profile=$(cover_dir)/postgres.out
cover_html=$(cover_dir)/coverage.html

# Tests should not be taking that long
DEFAULT_TESTS_TIMEOUT ?= 10s
LIVE_TESTS_TIMEOUT ?= 45s

.DEFAULT_GOAL := all

all: test

lint: $(GOLANGCI_LINT_PREREQS)
	$(GOLANGCI_LINT_RUN)

$(cover_dir):
	mkdir -p $(cover_dir)

go-test-coverage=go run github.com/vladopajic/go-test-coverage/v2

test: $(cover_dir)
	TZ=US/Alaska go test -timeout=${DEFAULT_TESTS_TIMEOUT} -shuffle=on -failfast -coverpkg=./internal/...,./httpapi/...,./agent/... -coverprofile=$(routine_cover_profile) -covermode=atomic ./...
	$(go-test-coverage) --config .testcoverage-routine.yaml --profile $(routine_cover_profile)

test-postgres: $(cover_dir)
	TZ=US/Alaska SUMWEAVE_POSTGRES_TEST_DSN="$(SUMWEAVE_POSTGRES_TEST_DSN)" go test -tags=postgres_test -timeout=${DEFAULT_TESTS_TIMEOUT} -shuffle=on -failfast -coverpkg=./internal/...,./httpapi/...,./agent/... -coverprofile=$(postgres_cover_profile) -covermode=atomic ./...
	$(go-test-coverage) --config .testcoverage.yaml --profile $(postgres_cover_profile)
