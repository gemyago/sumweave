<!-- AGENTS.md — README for machines. Nearest file in the tree wins (hierarchical precedence). Keep this concise, concrete, and executable. -->

## Overview

**Early alpha note** This project is early in development; breaking API changes, data loss due to migrations incompatibility or other breaking changes are not a concern. We should not optimize for backward compatibility or plan any work that accounts for it.

## Project Origin And Direction

This repository was created from template and foundation code. Treat inherited template structure, copied boilerplate, and generic support modules as reference material unless this file or a module `AGENTS.md` explicitly says they are part of the intended product path.

The intended long-term system shape is:
- one core Go package/module (`runtime/` is the current foundation; naming may change)
- one Go API/jobs application (`apps/signal-foundry/` is the current foundation; naming may change)
- one UI (`apps/signal-ui/`)

Unless explicitly promoted by the user, everything outside that core package, Go app, and UI should be treated as template-origin reference code and not as a product requirement.

## ⚠️ IMPORTANT: Read Module-Specific AGENTS.md Files First

**Before performing ANY action** (running tests, editing files, debugging, etc.) in a specific module, **ALWAYS read the relevant module-specific AGENTS.md file first**. These files contain critical context, conventions, and requirements specific to each module. For high-level tests (integration/e2e, agent harness), read [tests/AGENTS.md](./tests/AGENTS.md) first.

```
<project layout>/
├── AGENTS.md                     # root AGENTS.md
├── .platform-agents/
│   └── skills/                   # bundled platform-internal agent skills
├── .agents/
│   └── skills/                   # generic repo-local agent skills
├── runtime/                      # core runtime and related infrastructure (go)
│   └── AGENTS.md
├── apps/
│   ├── signal-ui/                 # Svelte/Vite SPA (js)
│   │   └── AGENTS.md
│   └── signal-foundry/                 # Bundled Signal Foundry backend (go)
│       └── AGENTS.md
├── build/
│   ├── AGENTS.md
│   ├── make/                     # shared make fragments (repo-root pinned golangci-lint, etc.)
│   └── npm/                      # pipeline sources — details in build/AGENTS.md
├── tests/                        # high-level integration/e2e tests — [tests/AGENTS.md](./tests/AGENTS.md)
│   └── AGENTS.md
└── tools/
    ├── firecrawl/                 # web scraping tool (go)
    │   └── AGENTS.md
    ├── workspacefs/              # workspace filesystem tools (go)
    │   └── AGENTS.md
    └── skills/                  # skills toolset (go)
        └── AGENTS.md
```

## Security
- NEVER hardcode secrets. Use env vars/secret stores. Authenticate to GHCR before push/pull when required.
- Validate/sanitize all external inputs. Do not disable security linters without explicit justification.

## Dev Environment

Tools/Frameworks:
- Go 1.26.x
- Node.js 24.x
- Svelte 5
- PM2 for repo-scoped local process management

**Shell usage notes**
- If shell is persistent (e.g human controlled terminal), then `direnv allow` and then run all commands directly in the shell.
- If shell is ephemeral (most typical in AI agents), then run most commands via `direnv exec <working_dir> <command>`.
- This mostly applies to project specific commands (like `make`, `go` e.t.c). Regular exploration related commands (like `ls`, `cd`, `pwd` e.t.c) can be run directly in the shell.
- Keep in mind that `<working_dir>` doesn't change cwd, just loads the env from the specified working dir.
- When documenting commands, do not add `direnv exec` prefix. This should be assumed after reading this section.

Go and Node.js are managed by direnv (in .envrc) and nvm respectively. All dependencies are project scoped (e.g no global node_modules e.t.c).
PM2 is repo scoped too: `.envrc` exports `PM2_HOME=$PWD/.pm2`, so run `pm2` from the repo root.

**PM2 usage notes**
- Run `pm2 start ecosystem.config.js` to start all processes and add them to the PM2 process manager. Usually do it just once.
- Run `pm2 status` to see the status of all processes
- Run `pm2 start|stop|restart id|name` to control specific processes
- Run `pm2 logs id|name` to see the logs of specific processes

## Nx (monorepo tasks)

This monorepo is managed by Nx. Most typical tasks are:
- Run tests of specific module: `npx nx test signal-foundry` (cached)
- Run lint of specific module: `npx nx lint signal-foundry` (cached)
- Run test of specific module without caching: `npx nx test signal-foundry --skipNxCache`
- Run lint of all affected modules without caching: `npx nx run-many -t lint --skipNxCache`

To run all affected lint and tests, use `make affected-lint-test`

Any weird issues from golangci-linter (like invalid suppression directives or similar) maybe caused by caching issues. Try to clean the cache with `make clean-lint-cache` from repo root (this will only remove the cache) and run the linter again.

> ⚠️ If golangci-lint reports findings that seem unrelated to your changes (e.g. stale suppression directives in untouched files), clean the cache first: `make clean-lint-cache`, then re-run

## Distribution

This repo does not maintain an npm/package distribution pipeline.

## Coding Guide
- Always read [golang-coding-guide.md](./docs/golang-coding-guide.md) if planning to write golang code.

## Product Docs

- Active docs index: [docs/README.md](./docs/README.md)
- Canonical domain vocabulary: [docs/domain-terminology.md](./docs/domain-terminology.md)
- High-level product and runtime shape: [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md)

## Project Rules and Conventions

AI must always follow the rules and conventions defined in this section. This section defines a project specific rules and conventions. Module level rules and conventions must also be followed.

The rules are:
- Update project rules and conventions when user corrects the behavior of AI.
- Each rule must aim to be a simple and clear one line (50-80 characters)
- `docs/ARCHITECTURE.md` is the source of truth for product direction.
- Keep platform-internal skills under `.platform-agents/skills`.
- Keep generic repo-local skills under `.agents/skills`.
- Platform runtime loads only `.platform-agents/skills`.
- Treat template-derived code as reference unless adopted.
- Prefer core Go, Go app, and UI as real product scope.
- Keep package/release pipeline code removed unless explicitly revived.
- Natural-language approval completes OpenSpec review by default.
- OpenSpec gates pass only with a clean relevant git status.
- Preserve outside-flow additions; never revert as contamination.
- Archive OpenSpec changes before any final submission step.
- Use `make test-live-compile` for regular live-lane build coverage.
- OpenSpec manager work must be done by the requested sub-agent.

Gopher skill must be used prior to **writing** any Go code, or **planning** go code changes.

## Platform Agent Skills

Platform-internal skills live under `.platform-agents/skills`.
The in-product Signal Foundry agent loads only this platform-agent root.

Current platform-agent skills:
- `platform-info` — vendor/platform behavior, constraints, and bug-vs-vendor triage
- `historical-data-jobs` — bounded historical backfill workflow for missing candles
- `strategy-research-loop` — browse-first strategy research and evaluation loop
- `strategy-dsl-v0` — persisted Strategy DSL v0 shape, rules, and crossover semantics
- `strategy-iteration` — safe saved-strategy revision and re-evaluation
- `backtest-critique` — evidence-first backtest review and failure analysis

Generic repo-local skills may still live under `.agents/skills`, but they are not loaded
by the in-product platform agent unless the app config is explicitly changed.

## Task Completion Protocol

AI must always follow this protocol when completing tasks. The protocol varies by task type.

Keep in mind that modules may have different task completion protocols, if defined, it must be prioritized over the general protocol.
If code changed is module scoped, run lint and test for that module only.

### Coding Task Completion Protocol

Apply this when any code files were changed (Go, YAML, config files, etc.).

Note: Documentation files (e.g., .md) are not considered code files and should follow the Non-Coding Protocol.

**Always** perform these steps before reporting completion:
1. Run `make affected-lint-test`
   - If findings appear unrelated to your changes, run `make clean-lint-cache` and re-run before fixing
2. Verify AGENTS.md is updated if commands, workflows, or architecture changed

Report task completion status:
- Lint/test: ✓ no errors
- AGENTS.md: ✓ updated / no changes needed

**Note:** Failing tests or lint errors mean the task is NOT complete. All failures must be resolved before completion.

### Non-Coding Task Completion Protocol

For tasks not involving code changes (investigation, documentation review or updates, committing, etc.):
- Summarize findings or actions taken
- Confirm any deliverables were produced
- No lint/test protocol required
