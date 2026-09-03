<!-- AGENTS.md — README for machines. Nearest file in the tree wins (hierarchical precedence). -->

## Status

**Finance application.** This process exposes finance APIs, auth, durable jobs, generic agent HTTP routes, and operational infrastructure. Product direction is governed by [../../docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md).

## Template Origin And Boundary

This module is part of the intended long-term product path. Treat `apps/sumweave/` as the current foundation for the real Go API/jobs application, even if the final package or binary naming changes later.

This module was originally bootstrapped from backend boilerplate and then trimmed. Template-origin material that remains here is foundation only, not product scope by itself:
- Cobra CLI / process skeleton under `cmd/sumweave/`
- typed app config loading under `internal/config/`
- generic HTTP/server/telemetry scaffolding

Use retained boilerplate patterns as reference and a starting point. Do not reintroduce removed sample domains, demo persistence, MCP, or other old template features unless the user explicitly asks for them.

## Layout (module root)

Notable layout parts of `apps/sumweave`:

```
.
├── cmd/sumweave/     # `package main`: Cobra CLI (`start`, user commands, …)
├── doc/              # Architecture notes for the module
├── internal/         # Wireup, HTTP API, auth, app layer, infrastructure, telemetry, …
├── engine.go         # Root package: thin embed/test surface (`NewEngine`, `StartHTTPServer`, typed getters)
└── project.json      # Nx project
```

Rules for some files:
- internal/config/load_test.go - generic config loading only

## API Routes

API Routes are generated using [apigen](https://github.com/gemyago/apigen) which follows openapi first approach.Steps to add new routes:
- Add the new route to the `v1routes.yaml` file.
- Run `go generate ./internal/api/http/register.go` to generate the new routes.
- Implement new controller/new methods in the `v1controllers` package.
- No generated-route post-processing workaround unless explicitly justified.
- apigen issues/improvements discovered, need to be presented to the user and submitted to apigen after user approval. Such submissions must include: high level overview, steps to reproduce, suggested fix (if applicable) and why the issue matters.

Key rules:
- Controllers are defined with tags in the openapi spec
- Controller actions (e.g openapi operations) are implemented using 
  `builder.HandleWith` - provides standard params parsing, validation and response serialization
  `builder.HandleWithHTTP` - allows direct access to http objects if needed for advanced scenarios
- Standard error handling can be tuned with `error_handler.go` middleware.

## Run

Invoke repo-scoped PM2 commands from the repo root:

`pm2 start ecosystem.config.js` creates the API and UI development processes.

Before starting or restarting backend processes that rely on persisted tables, run `make postgres-bootstrap` from the repository root.

The standard local workflow runs `make postgres-bootstrap`, then `pm2 start ecosystem.config.js` from the repository root. Use direct `go run ./cmd/sumweave start-all --env local` only to diagnose a local startup problem. Local filesystem paths are app-root-relative; arbitrary working directories are unsupported.

For the optional HTTPS backend and Vite development workflow, follow [../../docs/local-https.md](../../docs/local-https.md). It uses ignored local certificate files and `APP_HTTPSERVER_TLS_CERTFILE` / `APP_HTTPSERVER_TLS_KEYFILE`.

Release build workflow is `make -C build dist` from the repository root. It builds the SPA once, embeds it before CGO-disabled Linux amd64/arm64 Go cross-compilation, and stages platform-agent skills. Runtime UI serving is embedded-only: if embedded `dist/index.html` is absent, the backend stays API-only.

PM2 startup runs the same all-in-one local backend shape on port 4501.
PM2 remains repo-scoped, but `ecosystem.config.js` sets the backend process working directory to `apps/sumweave`.

If the PM2 command shape changed (for example from `start` to `start-all`) or you need to guarantee the current ecosystem config is applied, recreate the backend app with `pm2 delete sumweave-api && pm2 start ecosystem.config.js` from the repo root; PM2 can otherwise keep an older command definition.

Durable jobs workflow:
- `make postgres-bootstrap` is the standard schema setup path for local/dev and PM2-backed environments.
- `sumweave user add --if-not-exists` supports retry-safe bootstrap hooks
  without resetting an existing password.
- `sumweave start-all` is the standard local backend mode; it runs the HTTP server, durable consumer, and scheduler loop together after schemas are prepared.
- `sumweave start` starts only the API/server path; it must not execute durable jobs inline.
- `sumweave jobs worker [--once]` is the dedicated split-mode consumer path for production-like or supervised environments. `--once` consumes until two poll intervals pass idle, so it can drain a reused DB backlog; use a reseeded or isolated local DB for a bounded E2E step.
- `sumweave jobs enqueue-due` performs one scheduler tick for finance-owned bank and FX schedules. It publishes semantic appdispatch commands and advances schedule state without running finance work or creating job rows; keep it for split or externally scheduled environments.
- `APP_FINANCE_PROVIDERS_FRANKFURTER_BASEURL` overrides the Frankfurter provider endpoint for deterministic local fixtures; manual FX E2E must not use the public network.
- Appdispatch is generic durable pub/sub for commands and domain events.
- Jobs add API/user visibility only when a product feature requires it.
- Background processing does not require a job record by default.
- Observable jobs still use appdispatch as their execution transport.
- Publication returns the immutable dispatch message ID before consumption; an observed job row is materialized lazily on first delivery with that same ID.
- A known future job ID may return `404` before delivery; only the initiating UI flow treats that response as pending.
- Finance bank and FX schedule state is authoritative in `finance/`; publication, schedule advance, and stored future reference commit together.
- Explicit finance terminal failures become sanitized failed observed jobs and are acknowledged; unclassified service, payload, materialization, claim, panic, and terminal-write failures remain dispatch failures.
- Message routers use at-least-once delivery and durable dead letters.
- Worker recovery runs at startup and between polls for claims older than `jobs.worker.staleRunningAge` (five minutes by default); active handlers renew their claims before recovery.
- Recreate the repo-scoped Compose PostgreSQL volume only when a clean local
  environment is required; no SQLite data migration or compatibility path exists.

## Lint / test

- **This module:** run root `make postgres-bootstrap`, then `make lint` and
  `make test`; ordinary tests use one coverage profile.
- **Release build:** `make -C build dist` from the repository root produces `build/dist/linux/{amd64,arm64}/sumweave` with embedded UI assets and stages `build/dist/platform-agents/skills`.
- **Whole repo:** from the repository root, `make lint` and `make test` include this module via `$(MAKE) -C apps/sumweave …`.

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
- Run local backend CLI commands with `apps/sumweave` as CWD.
- Error responses stay empty unless a documented endpoint contract justifies a safe body.
- Only wireup should consume app config; components use native inputs.
- Explicit roots stop message routers before the shared SQL database.
- Keep appdispatch transport separate from optional job observability.

## Purpose (directional)

- **Architecture overview:** [doc/architecture.md](doc/architecture.md) (module boundaries and current implementation notes).
- Consumer-facing entrypoint is `package main` under `cmd/sumweave/` (standard Go `cmd/<binary>` layout); application code lives under `internal/`.
- May depend on `runtime/` and coordinate with `apps/sumweave-ui` for delivery when that work lands.
- `engine.go` is a thin explicit HTTP-root surface for embedding and tests. It exposes `NewEngine`, `Close`, `StartHTTPServer`, and typed `Get*` accessors. Call `Close` for every eagerly constructed Engine that is not already closed by `StartHTTPServer`; it is safe to call more than once. Details: [doc/architecture.md](doc/architecture.md#root-package-enginego).

## Task Completion Protocol

Repository level task completion protocol **MUST ALWAYS** be followed. If you didn't follow it, this means task is not complete.
