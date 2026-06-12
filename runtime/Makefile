.PHONY: lint test test-live-compile test-live

include ../build/make/golangci-lint.mk

cover_dir=.cover
cover_profile=$(cover_dir)/profile.out
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

test: test-live-compile $(cover_dir)
	TZ=US/Alaska go test -timeout=${DEFAULT_TESTS_TIMEOUT} -shuffle=on -failfast -coverpkg=./internal/...,./httpapi/...,./agent/... -coverprofile=$(cover_profile) -covermode=atomic ./...
	$(go-test-coverage) --config .testcoverage.yaml --profile $(cover_profile)

test-live-compile:
	go test -tags=live -run '^$$' ./venueedge/live

test-live:
	TZ=UTC go test -tags=live -count=1 -timeout=${LIVE_TESTS_TIMEOUT} ./venueedge/live
