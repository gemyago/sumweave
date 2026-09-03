.PHONY: lint test

include ../build/make/golangci-lint.mk

cover_dir=.cover
cover_profile=$(cover_dir)/profile.out

SUMWEAVE_POSTGRES_TEST_DSN ?= postgres://sumweave_runtime:sumweave_runtime_local@127.0.0.1:55432/sumweave_test?sslmode=disable

DEFAULT_TESTS_TIMEOUT ?= 20s

.DEFAULT_GOAL := all

all: test

lint: $(GOLANGCI_LINT_PREREQS)
	$(GOLANGCI_LINT_RUN)

$(cover_dir):
	mkdir -p $(cover_dir)

go-test-coverage=go run github.com/vladopajic/go-test-coverage/v2

test: $(cover_dir)
	TZ=US/Alaska SUMWEAVE_POSTGRES_TEST_DSN="$(SUMWEAVE_POSTGRES_TEST_DSN)" go test -tags=postgres_test -timeout=${DEFAULT_TESTS_TIMEOUT} -shuffle=on -failfast -coverpkg=./... -coverprofile=$(cover_profile) -covermode=atomic ./...
	$(go-test-coverage) --config .testcoverage.yaml --profile $(cover_profile)
