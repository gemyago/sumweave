# Contributing to Sumweave

Sumweave is a personal finance tracking app. Start with the
[documentation index](docs/index.md), [product direction](docs/ARCHITECTURE.md),
and [domain vocabulary](docs/domain-terminology.md).

- [Local development](docs/operations/local-development.md) owns installation,
  PostgreSQL bootstrap, PM2, process modes, and startup troubleshooting.
- [Development and verification](docs/development/verification.md) covers Nx,
  module checks, finance smoke tests, and manual journeys.
- [Go coding guide](docs/golang-coding-guide.md) and the nearest `AGENTS.md`
  define implementation conventions; load the gopher skill before Go work.
- [Release and deployment](docs/operations/release-deployment.md) links the
  maintained host-build and Helm runbooks.

## OpenSpec workflow

Keep the existing OpenSpec workflow for proposed behavior changes and granular
acceptance scenarios. Initialization, when setting up the tooling, remains:

```sh
npm install -D @fission-ai/openspec@latest
openspec init
```

Use `openspec propose` and `openspec apply` with the repository's manager prompts
and standard artifact locations. Natural-language approval completes review by
default. Current capability scenarios live in `openspec/specs/`; active changes
live in `openspec/changes/`, and completed changes remain in its `archive/`.
Do not replace scenarios with prose summaries or rewrite archived change records.

Update the [OKF knowledge bundle](docs/development/documentation.md) alongside
behavior changes. It explains current behavior and connects each topic to formal
requirements, implementation, and verification. Generic agent and Chat work is
[experimental](docs/experiments/agent-runtime.md), not the product roadmap.
