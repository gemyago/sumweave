<!-- AGENTS.md — README for machines. Nearest file in the tree wins (hierarchical precedence). -->

## Status

**Finance application.** This process exposes finance APIs, auth, durable jobs, and operational infrastructure. Generic agent HTTP routes and model-provider configuration are experimental extensions, not finance product commitments. Product direction is governed by [../../docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md).

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

Follow [local development](../../docs/operations/local-development.md) for
installation, PostgreSQL bootstrap, PM2, logs, HTTPS, and startup diagnostics.
Run PM2 at the repository root; backend CLI paths are relative to this module.
Standard startup is root `make postgres-bootstrap`, then `pm2 start ecosystem.config.js`.
PM2 runs the backend processes in the `backend` namespace as `api` and `worker`;
UI is `ui`. Use `pm2 start|stop|restart|delete backend` for backend lifecycle
and `pm2 logs backend` for combined backend logs.

Follow [release and deployment](../../docs/operations/release-deployment.md) for
the host build, embedded UI, and container/Helm runbooks.

Durable jobs workflow:
- `make postgres-bootstrap` is the standard schema setup path for local/dev and PM2-backed environments.
- `sumweave user add --if-not-exists` supports retry-safe bootstrap hooks
  without resetting an existing password.
- PM2 runs `sumweave start` and `sumweave jobs worker` as the standard local backend mode; run `sumweave jobs enqueue-due` separately when a scheduler tick is required.
- `sumweave start-all` combines the HTTP server, durable consumer, and scheduler loop for startup diagnostics only.
- `sumweave start` starts only the API/server path; it must not execute durable jobs inline.
- `sumweave jobs worker` runs the observed-job consumer plus independent `finance.classification.v1` and `finance.transfer-matching.v1` ordinary event routers. Each committed bank-sync window can trigger both enrichments for its exact range independently; automatic handling creates no job projection and does not alter the bank-sync job outcome.
- `sumweave jobs worker --once` drains observed commands first, then attempts both ordinary enrichment drains after each router's configured subscription attempts return and through its existing two idle polls; it is not a generic-broker barrier; use a reseeded or isolated local DB for a bounded E2E step.
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
- Worker recovery runs at startup and between polls for claims older than `jobs.worker.staleRunningAge` (30 minutes by default); active handlers renew their claims before recovery.
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
- `swmd skill` renders offline agent instructions and never resolves credentials or calls the API.
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
