---
type: Guide
title: "Development and verification"
description: "Module checks, Nx tasks, and reproducible finance verification journeys."
status: stable
sources:
  - resource: "repository AGENTS.md, module Makefiles, tests/README.md, and manual E2E guides"
---

# Development and verification

Read the nearest module `AGENTS.md` before working in it. Read
[tests/AGENTS.md](../../tests/AGENTS.md) before high-level tests. Backend work is
serialized. Mockery is the default for dependency mocks; hand-written test doubles
require user approval. Tests should assert meaningful behavior, not logger calls,
generated SQL strings, or schema internals. Migration coverage stays shallow.

## Local checks

Follow [local development](../operations/local-development.md) first. Run
`make postgres-bootstrap` before ordinary backend tests; tagged PostgreSQL tests
use the prepared `sumweave_test` schema. Core module `make test` uses its single
`.cover/profile.out` profile and configured coverage check. There is no separate
PostgreSQL verification lane.

Useful repo-root Nx commands:

```sh
npx nx test sumweave
npx nx test sumweave --skipNxCache
npx nx lint sumweave
npx nx run-many -t lint --skipNxCache
npx nx run-many -t test
npx nx show project sumweave --json
make affected-lint-test
```

Apply the root/module completion protocol for code and configuration changes,
fixing failures before completion. If golangci-lint reports unrelated stale
suppression findings, run `make clean-lint-cache` and repeat before fixing them.
Markdown-only work follows the non-coding protocol: inspect content and links,
state deliverables and checks; no application lint/test run is required.

For Go dependency maintenance, run from the owning module: `go mod download`,
`go get -u tool`, or `go install tool` as appropriate. Deliberate dependency
updates use `go get -u ./...` followed by `go mod tidy`; do not update dependencies
as a side effect of unrelated work. Read the [Go guide](../golang-coding-guide.md)
and [testing guide](../testing-best-practices.md).

## Finance verification journeys

The [manual E2E index](../manual-e2e/index.md) is the entry for explicit E2E work.
Preserve each guide's isolation, database, identity, provider-fixture, and evidence
requirements. In particular, lazy-job checks publish with an API-only process
before a bounded worker run; ordinary PM2 workers would close that observation
window. Clean-volume guides replace local data and require an isolated environment.

[tests/README.md](../../tests/README.md) owns automated finance smoke setup and
commands for API and headed Chromium projects. Tests retain reports, traces,
and screenshots under repository `tmp/playwright/`. Run one suite at a time
against a shared dev stack. The guide separately describes experimental agent
integration tests, which are not required finance acceptance coverage.

For login/Finance UI changes, follow the
[shell smoke journey](../manual-e2e/finance-ui-shell-smoke-e2e.md), the changed
feature journey, and the module visual-review protocol. Use `--headed` when
headed browser verification is requested. A runbook is a procedure, not evidence
that its checks have been performed.
