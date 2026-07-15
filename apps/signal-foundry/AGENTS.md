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

Rules for some files:
- internal/config/load_test.go - testing generic config loading, not specific variables

## API Routes

API Routes are generated using [apigen](https://github.com/gemyago/apigen) which follows openapi first approach.Steps to add new routes:
- Add the new route to the `v1routes.yaml` file.
- Run `go generate ./internal/api/http/register.go` to generate the new routes.
- Implement new controller/new methods in the `v1controllers` package.

Key rules:
- Controllers are defined with tags in the openapi spec
- Controller actions (e.g openapi operations) are implemented using 
  `builder.HandleWith` - provides standard params parsing, validation and response serialization
  `builder.HandleWithHTTP` - allows direct access to http objects if needed for advanced scenarios
- Standard error handling can be tuned with `error_handler.go` middleware.

## Run

Invoke repo-scoped PM2 commands from the repo root:

`pm2 start signal-foundry-api` (PM2 process name is `signal-foundry-api`).

Before starting or restarting backend processes that rely on persisted tables, change to `apps/signal-foundry` and run `go run ./cmd/signal-foundry db-migrate --env local`.

Standard local backend workflow uses `apps/signal-foundry` as the working directory: run `go run ./cmd/signal-foundry db-migrate --env local` and then `go run ./cmd/signal-foundry start-all --env local`. Local filesystem paths are app-root-relative; arbitrary working directories are unsupported.

Release build workflow from `apps/signal-foundry` is `make dist/bin`; it rebuilds the SPA into the backend embed directory, validates `dist/index.html`, and then produces the backend binary with embedded UI assets. Runtime UI serving is embedded-only: if embedded `dist/index.html` is absent, the backend stays API-only.

PM2 startup runs the same all-in-one local backend shape on port 4501.
PM2 remains repo-scoped, but `ecosystem.config.js` sets the backend process working directory to `apps/signal-foundry`.

If the PM2 command shape changed (for example from `start` to `start-all`) or you need to guarantee the current ecosystem config is applied, recreate the backend app with `pm2 delete signal-foundry-api && pm2 start ecosystem.config.js` from the repo root; PM2 can otherwise keep an older command definition.

Durable jobs workflow:
- `signal-foundry db-migrate` is the standard schema setup path for local/dev and PM2-backed environments.
- `signal-foundry start-all` is the standard local backend mode; it runs the HTTP server, durable consumer, and scheduler loop together after schemas are prepared.
- `signal-foundry start` starts only the API/server path; it must not execute durable jobs inline.
- `signal-foundry jobs worker [--once]` is the dedicated split-mode consumer path for production-like or supervised environments. `--once` consumes until two poll intervals pass idle, so it can drain a reused DB backlog; use a reseeded or isolated local DB for a bounded E2E step.
- `signal-foundry jobs enqueue-due` performs one scheduler tick and enqueues due scheduled jobs without running them; keep it for split or externally scheduled environments.

## Lint / test

- **This module:** `make lint`, `make test` from `apps/signal-foundry` (uses repo-root pinned `golangci-lint` from `bin/` unless `CI=true`).
- **Release build:** `make dist/bin` from `apps/signal-foundry` produces `dist/bin/signal-foundry` with embedded UI assets when generated `embeddedui/dist/index.html` is present.
- **Whole repo:** from the repository root, `make lint` and `make test` include this module via `$(MAKE) -C apps/signal-foundry …`.

## Module Rules and Conventions

This section defines module-specific rules and conventions. Project-level rules and conventions must also be followed.

Use gopher skill as your primary source of golang coding conventions and best practices.

The rules are:
- Update module rules and conventions when user corrects the behavior of AI.
- Mutating APIs (such as POST, PUT, PATCH, DELETE) **should not** return entity data unless backend generates needed data. In this case just the minimal required response data must be returned.
- OpenAPI JSON uses camelCase for property names or any other identifiers or keys; regenerate after spec edits.
- HTTP controller tests use registered routes, not custom builders.
- Config env overrides use standard APP_ AutomaticEnv mapping only.
- Config load tests cover app logic, not Viper env binding.
- Put required test defaults in test.yaml, not per-test env.
- Keep files referenced by test.yaml as committed test fixtures, never use real secrets or ssh keys, generate fake random values instead.
- Run local backend CLI commands with `apps/signal-foundry` as CWD.
- Error responses stay empty unless a documented endpoint contract justifies a safe body.

## Purpose (directional)

- **Architecture overview:** [doc/architecture.md](doc/architecture.md) (module boundaries and current implementation notes).
- Consumer-facing entrypoint is `package main` under `cmd/signal-foundry/` (standard Go `cmd/<binary>` layout); application code lives under `internal/`.
- May depend on `runtime/` and coordinate with `apps/signal-ui` for delivery when that work lands.
- `engine.go` is a thin surface for embedding and tests: it wraps the DI container after `internal.Setup` and exposes a small API (`NewEngine`, `StartHTTPServer`, typed `Get*` resolvers). Details: [doc/architecture.md](doc/architecture.md#root-package-enginego).

## Task Completion Protocol

Repository level task completion protocol **MUST ALWAYS** be followed. If you didn't follow it, this means task is not complete.
