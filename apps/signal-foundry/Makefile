.PHONY: lint test dist/bin

include ../../build/make/golangci-lint.mk

cover_dir=.cover
cover_profile=$(cover_dir)/profile.out
cover_html=$(cover_dir)/coverage.html
ui_embed_dist_dir=$(CURDIR)/internal/api/http/embeddedui/dist

# Package-level coverage runs need some CI headroom.
DEFAULT_TESTS_TIMEOUT ?= 30s

.DEFAULT_GOAL := all

all: test

lint: $(GOLANGCI_LINT_PREREQS)
	$(GOLANGCI_LINT_RUN)

$(cover_dir):
	mkdir -p $(cover_dir)

dist/bin:
	mkdir -p dist/bin
	$(MAKE) -C ../signal-ui build-embed-dist EMBED_OUT_DIR="$(ui_embed_dist_dir)"
	test -f "$(ui_embed_dist_dir)/index.html"
	go build \
		-tags=release \
		-o dist/bin/signal-foundry \
		./cmd/signal-foundry;

go-test-coverage=go run github.com/vladopajic/go-test-coverage/v2

test: $(cover_dir)
	TZ=US/Alaska go test -timeout=${DEFAULT_TESTS_TIMEOUT} -shuffle=on -failfast -coverpkg=./cmd/signal-foundry,./internal/... -coverprofile=$(cover_profile) -covermode=atomic ./...
	$(go-test-coverage) --config .testcoverage.yaml --profile $(cover_profile)
