include build/make/golangci-lint.mk

cover_dir=.cover
cover_profile=$(cover_dir)/profile.out

go-test-coverage=go run github.com/vladopajic/go-test-coverage/v2

POSTGRES_HOST ?= 127.0.0.1
POSTGRES_PORT ?= 55432
POSTGRES_MANAGED_EXTERNALLY ?=

ifeq ($(POSTGRES_MANAGED_EXTERNALLY),1)
POSTGRES_BOOTSTRAP_DSN ?=
else
POSTGRES_BOOTSTRAP_DSN ?= postgres://postgres:sumweave_postgres_local@127.0.0.1:55432/postgres?sslmode=disable
endif

postgres_test_dsn=postgres://sumweave_runtime:sumweave_runtime_local@$(POSTGRES_HOST):$(POSTGRES_PORT)/sumweave_test?sslmode=disable

ifneq ($(POSTGRES_HOST):$(POSTGRES_PORT),127.0.0.1:55432)
postgres_test_app_env=APP_APPLICATION_DATABASE_DSN="$(postgres_test_dsn)" APP_AGENTRUNTIME_DATABASE_DSN="$(postgres_test_dsn)"
endif

.PHONY: postgres-bootstrap postgres-bootstrap-contract-test postgres-target-contract-test postgres-workflow-contract-test postgres-test-runtime postgres-test-finance postgres-test-sumweave postgres-verify
.NOTPARALLEL: postgres-verify
postgres-bootstrap:
	@if [ "$(POSTGRES_MANAGED_EXTERNALLY)" != "1" ]; then \
		docker compose -f compose.yaml up --detach --wait postgres; \
	fi
	@POSTGRES_HOST="$(POSTGRES_HOST)" POSTGRES_PORT="$(POSTGRES_PORT)" \
		POSTGRES_MANAGED_EXTERNALLY="$(POSTGRES_MANAGED_EXTERNALLY)" \
		POSTGRES_BOOTSTRAP_DSN="$(POSTGRES_BOOTSTRAP_DSN)" \
		./scripts/postgres/bootstrap.sh

postgres-bootstrap-contract-test:
	./scripts/postgres/bootstrap-contract-test.sh

postgres-target-contract-test:
	bash ./scripts/postgres/targets-contract-test.sh

postgres-workflow-contract-test:
	bash ./scripts/postgres/workflow-contract-test.sh

postgres-test-runtime: postgres-bootstrap
	SUMWEAVE_POSTGRES_TEST_DSN="$(postgres_test_dsn)" $(MAKE) -C runtime test-postgres

postgres-test-finance: postgres-bootstrap
	SUMWEAVE_POSTGRES_TEST_DSN="$(postgres_test_dsn)" $(MAKE) -C finance test-postgres

postgres-test-sumweave: postgres-bootstrap
	SUMWEAVE_POSTGRES_TEST_DSN="$(postgres_test_dsn)" $(postgres_test_app_env) $(MAKE) -C apps/sumweave test-postgres

postgres-verify: postgres-test-runtime postgres-test-finance postgres-test-sumweave

$(cover_dir):
	mkdir -p $(cover_dir)

$(cover_dir)/repo-name-with-owner.txt:
	gh repo view --json nameWithOwner -q .nameWithOwner > $@

$(cover_dir)/coverage.%.blob-sha: $(cover_dir)/repo-name-with-owner.txt
	gh api \
		--method GET \
		-H "Accept: application/vnd.github+json" \
		-H "X-GitHub-Api-Version: 2022-11-28" \
		/repos/$(shell cat $(cover_dir)/repo-name-with-owner.txt)/contents/coverage/golang-coverage.$*?ref=test-artifacts \
		| jq -jr '.sha' > $@

$(cover_dir)/coverage.%.gh-cli-body.json: $(cover_dir)/coverage.% $(cover_dir)/coverage.%.blob-sha
	@echo "{" > $@
	@echo "\"branch\": \"test-artifacts\"," >> $@
	@printf "\"sha\": \"">> $@
	@cat $(cover_dir)/coverage.$*.blob-sha >> $@
	@printf "\",\n">> $@
	@echo "\"message\": \"Updating golang coverage.$*\",">> $@
	@printf "\"content\": \"">> $@
	@base64 -i $< | tr -d '\n' >> $@
	@printf "\"\n}">> $@

# Orphan branch will need to be created prior to running this
# git checkout --orphan test-artifacts
# git rm -rf .
# rm -f .gitignore
# echo '# Test Artifacts' > README.md
# git add README.md
# git commit -m 'init'
# git push origin test-artifacts
.PHONY: push-test-artifacts
push-test-artifacts: $(cover_dir)/coverage.svg.gh-cli-body.json $(cover_dir)/coverage.html.gh-cli-body.json $(cover_dir)/repo-name-with-owner.txt
	@gh api \
		--method PUT \
		-H "Accept: application/vnd.github+json" \
		-H "X-GitHub-Api-Version: 2022-11-28" \
		/repos/$(shell cat $(cover_dir)/repo-name-with-owner.txt)/contents/coverage/golang-coverage.svg \
		--input $(cover_dir)/coverage.svg.gh-cli-body.json
	@gh api \
		--method PUT \
		-H "Accept: application/vnd.github+json" \
		-H "X-GitHub-Api-Version: 2022-11-28" \
		/repos/$(shell cat $(cover_dir)/repo-name-with-owner.txt)/contents/coverage/golang-coverage.html \
		--input $(cover_dir)/coverage.html.gh-cli-body.json

.PHONY: affected-lint-test
ifeq ($(CI),true)
affected-lint-test:
	npx nx affected -t lint test --tuiAutoExit
else
affected-lint-test: $(GOLANGCI_LINT_BIN)
	npx nx affected -t lint test --tuiAutoExit
endif

.PHONY: lint
lint:
	$(MAKE) -C tools/firecrawl lint
	$(MAKE) -C runtime lint
	$(MAKE) -C finance lint
	$(MAKE) -C apps/sumweave lint
	$(MAKE) -C apps/sumweave-ui lint
	$(MAKE) -C tools/workspacefs lint
	$(MAKE) -C tools/skills lint

.PHONY: clean-lint-cache
clean-lint-cache:
	rm -r -f .cache/golangci-lint

.PHONY: test
test: $(cover_dir)
	rm -r -f $(cover_dir)/*
	$(MAKE) -C tools/firecrawl test
	$(MAKE) -C tools/skills test
	$(MAKE) -C tools/workspacefs test
	$(MAKE) -C runtime test
	$(MAKE) -C finance test
	$(MAKE) -C apps/sumweave test
	$(MAKE) -C apps/sumweave-ui test
	cat tools/firecrawl/.cover/profile.out > $(cover_profile)
	tail -n +2 tools/skills/.cover/profile.out >> $(cover_profile)
	tail -n +2 tools/workspacefs/.cover/profile.out >> $(cover_profile)
	tail -n +2 runtime/.cover/routine.out >> $(cover_profile)
	tail -n +2 finance/.cover/routine.out >> $(cover_profile)
	tail -n +2 apps/sumweave/.cover/routine.out >> $(cover_profile)
	go tool cover -html=$(cover_profile) -o $(cover_dir)/coverage.html
	$(go-test-coverage) --badge-file-name $(cover_dir)/coverage.svg --profile $(cover_profile)
