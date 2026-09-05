<!-- AGENTS.md — README for machines. Nearest file in the tree wins (hierarchical precedence). Keep this concise, concrete, and executable. -->

## Overview

**Early alpha note** This project is early in development; breaking API changes, data loss due to migrations incompatibility or other breaking changes are not a concern. We should not optimize for backward compatibility or plan any work that accounts for it.

## Project Origin And Direction

This repository was created from template and foundation code. Treat inherited template structure, copied boilerplate, and generic support modules as reference material unless this file or a module `AGENTS.md` explicitly says they are part of the intended product path.

The intended product is a finance-only system with:
- `finance/` as the finance domain module
- `runtime/` as generic agent infrastructure
- one Go API/jobs application (`apps/sumweave/`)
- one UI (`apps/sumweave-ui/`)

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
│   ├── sumweave-ui/                 # Svelte/Vite SPA (js)
│   │   └── AGENTS.md
│   └── sumweave/                 # Bundled Sumweave backend (go)
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
- Never hardcode real secrets. Obvious local-only placeholders are allowed.
- Use env vars/secret stores. Authenticate to GHCR before push/pull when required.
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
- From the repository root, run `make postgres-bootstrap` before starting or restarting backend PM2 processes that rely on persisted tables.
- Standard local workflow is `make postgres-bootstrap`, then `pm2 start ecosystem.config.js` from the repository root.
- PM2 starts `api` (`start`), `worker` (`jobs worker`), and `ui`.
- Use direct `go run ./cmd/sumweave start-all --env local` only to diagnose a local startup problem.
- PM2 is invoked from the repository root, but both backend processes use `apps/sumweave` as their working directory.
- Run `pm2 start ecosystem.config.js` to create the PM2 apps from the current ecosystem file.
- The API and worker use the `backend` PM2 namespace; UI uses PM2's default namespace.
- Use `pm2 start|stop|restart|delete backend` or `pm2 logs backend` for backend lifecycle and combined logs; PM2 accepts a namespace as the positional target.
- If the ecosystem command/args changed or you need a guaranteed fresh backend shape, recreate it with `pm2 delete backend && pm2 start ecosystem.config.js`; PM2 can otherwise keep an older command definition.
- Run `pm2 status` to see the status of all processes
- Run `pm2 start|stop|restart id|name` to control specific processes
- Run `pm2 logs id|name` to see the logs of specific processes

For optional local HTTPS backend and Vite development, follow [docs/local-https.md](./docs/local-https.md). The documented workflow generates ignored local certificates and does not change the normal PM2 HTTP workflow.

Durable jobs workflow:
- Run `make postgres-bootstrap` from the repository root before any process that uses persisted application tables.
- Run `make postgres-bootstrap` before ordinary backend tests; each core
  module's `make test` runs normally selected tests.
- `sumweave start` is API-only: it publishes appdispatch messages but does not run a worker or execute finance work inline.
- PM2 runs `sumweave start` and `sumweave jobs worker` as separate local backend processes; run `sumweave jobs enqueue-due` separately when a scheduler tick is needed.
- `sumweave start-all` explicitly combines HTTP, the appdispatch worker, and the finance scheduler loop for diagnostics only.
- `sumweave jobs worker [--once]` is the split consumer; `--once` drains a bounded isolated/reseeded database and materializes observed jobs on delivery.
- `sumweave jobs enqueue-due` publishes due bank and FX semantic commands and advances finance-owned schedule state; it does not execute work or create job rows.
- Producers publish through appdispatch first and receive a message ID; observed job rows are created lazily with that same ID on first delivery.
- A known future job ID may return `404` before delivery; only the initiating UI flow treats that response as pending.

## Nx (monorepo tasks)

This monorepo is managed by Nx. Most typical tasks are:
- Run tests of specific module: `npx nx test sumweave` (cached)
- Run lint of specific module: `npx nx lint sumweave` (cached)
- Run test of specific module without caching: `npx nx test sumweave --skipNxCache`
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
- **DO NOT** over engineer or over-complicate. Address problems present now or explicitly requested.
- Main rule to follow - the simpler the better.
- Never go outside of the project root
- Store temp files in a project scoped tmp directory (e.g ${PWD}/tmp/...)
- Update project rules and conventions when user corrects the behavior of AI.
- Each rule must aim to be a simple and clear one line (50-80 characters)
- Backend tasks must be serialized; only unrelated frontend tasks run in parallel
- `docs/ARCHITECTURE.md` is the source of truth for product direction.
- Keep platform-internal skills under `.platform-agents/skills`.
- Keep harness agnostic repo-local skills under `.agents/skills`.
- Platform runtime loads only `.platform-agents/skills`.
- Treat template-derived code as reference unless adopted.
- Prefer core Go, Go app, and UI as real product scope.
- Keep package/release pipeline code removed unless explicitly revived.
- Natural-language approval completes OpenSpec review by default.
- Seed/reseed requests default to the first `.local-users` entry.
- Reseed means replace local seeded data, then reopen the live DB.
- Launch local backend CLI commands from `apps/sumweave`.
- Avoid markdown tables, prefer lists or other formatting. Tables are hard to read by humans. Use tables only when user explicitly requests it.
- Do not run `git diff --check` as a routine verification step.
- Bootstrap PostgreSQL before ordinary backend test commands.
- Do not test generated ORM queries by matching SQL strings.
- Do not explicitly normalize dates or timestamps to UTC.
- PostgreSQL is the sole supported core product database in every environment.
- Keep API responses single-purpose by default (e.g operating on a single entity); Composition must have a good justification.
- Keep migration tests shallow; allow one smoke test, no detailed schema checks.
- Build frontend/backend release artifacts on the host; Docker only packages them.
- Build release artifacts with `make -C build dist`; Docker never compiles source.
- Render the Helm chart with `make -C deploy lint` before deployment changes.
- Allow obvious local-only placeholder keys in committed local config.
- Use environment-specific config values, not runtime environment labels.
- Log ordinary provider errors; avoid generic provider-error redaction.
- Appdispatch carries generic durable commands and domain events.
- Jobs add opt-in product visibility to background processing.
- Do not create job records for invisible background processing.
- Jobs never replace appdispatch as the execution transport.
- Phase 0 classification loads rules into memory per attempt.
- Phase 0 classification uses jobs for feedback and logs counts.
- Treat concurrent classification updates as an accepted Phase 0 risk.
- Describe chosen designs; omit rejected alternatives and their history.
- Use plain ASCII diagrams in system design documentation.
- Classification range APIs accept full timestamps with offsets.
- Jobs and sync-completion events use the same range classifier.
- PM2 local apps are `api`, `worker`, and `ui`; backend operations target `backend`.
- Use direct PM2 commands rather than root npm PM2 wrapper scripts.

Gopher skill must be used prior to **writing** any Go code, or **planning** go code changes.

## Golang

- **Always** load and use gopher skill when working with Go code.
- viper should only be used for config wireup, it should never leak into the codebase outside of the entrypoints or wireup paths.
- components should not be doing nil checks to ensure if dependencies are initialized, this is a job of the DI container or the caller. This may only be justified if the dependency is optional.
- unless explicitly documented, internal logic do not need to trim or otherwise normalize identifiers. Upper orchestration layer may chose to do it if needed.
- system must have reasonable logging that allows to troubleshoot problems and understand the flow of the system.
- when logging attributes, use camelCase for keys
- avoid logging helpers, inline the logging statements instead
- required component dependencies must be enforced in constructor, not in methods that use them

### Testing and mocking

- Mockery is the default for dependency mocks in tests.
- Hand-written stubs/fakes/spies are forbidden without user approval.
- Avoid unit testing logger statements unless it's part of a business logic (which is rare). Logs are not business logic.

## Manual E2E Testing

When user asks to e2e test something, usually this means following the steps in the relevant [manual e2e testing guide](./docs/manual-e2e/README.md).

Also if user is asking you to run browser in headed mode, usually this means adding `--headed` flag if you're using playwright cli.

When user is asking you to add a new skill, do this:
- Create the skill in the `.agents/skills` directory. Name it properly
- Define standard frontmatter for the skill (at least name and description)
- Keep description short and to the point, avoid fluff. Make it minimalistic and include enough for LLM model to understand what the skill is about.
- Define skill instruction as per user request.
- Link the skill to the harnesses using `.agents/skills/link-vendor-harnesses.sh` script

## Harness Agent Skills

Harness agnostic repo-local skills live under `.agents/skills`. After creating a new skill, make sure to link it using `.agents/skills/link-vendor-harnesses.sh`

## Platform Agent Skills

Platform-internal skills live under `.platform-agents/skills`.
The in-product Sumweave agent loads only this platform-agent root.

The platform-agent skill root is currently empty and stageable. Add only
finance-relevant skills when they are explicitly requested.

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
