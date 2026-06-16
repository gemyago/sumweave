<!-- AGENTS.md — README for machines. Nearest file in the tree wins (hierarchical precedence). -->

## Status

**Lean module.** This process currently exposes the HTTP server, config, logging, OpenTelemetry, health routes, and inherited runtime HTTP surface. Product direction is still governed by the repository-level [../../docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md).

## Template Origin And Boundary

This module is part of the intended long-term product path. Treat `apps/signal-foundry/` as the current foundation for the real Go API/jobs application, even if the final package or binary naming changes later.

This module was originally bootstrapped from backend boilerplate and then trimmed. Template-origin material that remains here is foundation only, not product scope by itself:
- Cobra CLI / process skeleton under `cmd/signal-foundry/`
- shared config and DI wiring under `internal/config/` and `internal/`
- generic HTTP/server/telemetry scaffolding

Use retained boilerplate patterns as reference and a starting point. Do not reintroduce removed sample domains, demo persistence, MCP, or other old template features unless the user explicitly asks for them.

## Layout (module root)

Notable layout parts of `apps/signal-foundry`:

```
.
├── cmd/signal-foundry/     # `package main`: Cobra CLI (`start`, user commands, …)
├── doc/              # Architecture notes for the module
├── internal/         # Config, DI, HTTP API, auth, app layer, infrastructure, telemetry, …
├── engine.go         # Root package: thin embed/test surface (`NewEngine`, `StartHTTPServer`, typed getters)
└── project.json      # Nx project
```

## Run

From the repo root:

`pm2 start signal-foundry-api` (PM2 process name is `signal-foundry-api`).

This will start the HTTP server on port 4501.

## Lint / test

- **This module:** `make lint`, `make test` from `apps/signal-foundry` (uses repo-root pinned `golangci-lint` from `bin/` unless `CI=true`).
- **Whole repo:** from the repository root, `make lint` and `make test` include this module via `$(MAKE) -C apps/signal-foundry …`.

## Module Rules and Conventions

This section defines module-specific rules and conventions. Project-level rules and conventions must also be followed.

Use gopher skill as your primary source of golang coding conventions and best practices.

The rules are:
- Update module rules and conventions when user corrects the behavior of AI.
- OpenAPI JSON uses camelCase for property names or any other identifiers or keys; regenerate after spec edits.

## Purpose (directional)

- **Architecture overview:** [doc/architecture.md](doc/architecture.md) (module boundaries and current implementation notes).
- Consumer-facing entrypoint is `package main` under `cmd/signal-foundry/` (standard Go `cmd/<binary>` layout); application code lives under `internal/`.
- May depend on `runtime/` and coordinate with `apps/signal-ui` for delivery when that work lands.
- `engine.go` is a thin surface for embedding and tests: it wraps the DI container after `internal.Setup` and exposes a small API (`NewEngine`, `StartHTTPServer`, typed `Get*` resolvers). Details: [doc/architecture.md](doc/architecture.md#root-package-enginego).

## Task Completion Protocol

Repository level task completion protocol **MUST ALWAYS** be followed. If you didn't follow it, this means task is not complete.
