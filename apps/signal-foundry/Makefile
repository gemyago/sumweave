.PHONY: lint test

include ../../build/make/golangci-lint.mk

cover_dir=.cover
cover_profile=$(cover_dir)/profile.out
cover_html=$(cover_dir)/coverage.html

# Package-level coverage runs need some CI headroom.
DEFAULT_TESTS_TIMEOUT ?= 20s

.DEFAULT_GOAL := all

all: test

lint: $(GOLANGCI_LINT_PREREQS)
	$(GOLANGCI_LINT_RUN)

$(cover_dir):
	mkdir -p $(cover_dir)

dist/bin:
	go build \
		-tags=release \
		-o dist/bin/signal-foundry \
		./cmd/signal-foundry;

go-test-coverage=go run github.com/vladopajic/go-test-coverage/v2

test: $(cover_dir)
	TZ=US/Alaska go test -timeout=${DEFAULT_TESTS_TIMEOUT} -shuffle=on -failfast -coverpkg=./cmd/signal-foundry,./internal/... -coverprofile=$(cover_profile) -covermode=atomic ./...
	$(go-test-coverage) --config .testcoverage.yaml --profile $(cover_profile)
