# Sumweave backend architecture (brief)

This file describes the current backend foundation in this repository. For product direction and naming, use the repository-level [../../../docs/ARCHITECTURE.md](../../../docs/ARCHITECTURE.md) as the source of truth.

HTTP server and CLI entrypoint for Sumweave under `apps/sumweave`: a single **`sumweave`** binary built from `cmd/sumweave`. Local commands run with `apps/sumweave` as CWD (`go run ./cmd/sumweave db-migrate --env local`, `go run ./cmd/sumweave start-all --env local`, or `go run ./cmd/sumweave start --env local`). The app wires configuration, logging, OpenTelemetry, health, finance APIs, auth, durable jobs, and experimental generic agent HTTP routes. Release builds already embed the SPA for same-origin delivery.

`cmd/swmd` is a separate HTTP-only client binary in the same Go module. It does
not compose server, finance persistence, runtime, appdispatch, or jobs stores;
it calls the existing app API with a personal access token.

`swmd skill` is an offline, stdout-only projection of the live `swmd` Cobra
tree. It writes an installable agent `SKILL.md` with the safe finance/jobs
command surface, flags, usage, and examples without resolving credentials,
reading stdin, writing files, or calling the API. Credential-mutating auth
commands are excluded while the redacted auth-status command is included.

## Stack

| Area | Choice |
| --- | --- |
| Language | **Go** (see repo root / module **`go.mod`**, currently **1.26.x**) |
| CLI | **Cobra** — root command **`sumweave`**, subcommands e.g. **`db-migrate`**, **`start-all`**, **`start`** (HTTP), **`jobs`** |
| Wireup | Hand-written command-appropriate roots in `internal/wireup` |
| HTTP | **`net/http`** + **`http.ServeMux`** — route patterns `METHOD path` on `internal/api/http/server.HTTPRouter`; SPA/static UI served from `/` with backend-path-aware fallback |
| Config | **Viper** — embedded YAML and typed loading under `internal/config/`, env prefix **`APP_`** |
| Logging | **`log/slog`** — root logger options in `internal/telemetry`, optional JSON / file / OTEL bridge |
| Observability | **OpenTelemetry** — traces/metrics/logs hooks in `internal/telemetry` (off by default in `default.yaml`) |
| Generated REST (app-owned) | **apigen** — `//go:generate` on `internal/api/http/register.go` reads **`internal/api/http/v1routes.yaml`** |
| Agent HTTP | **`runtime/httpapi`** — handler built in `internal/agent_runtime.go`, mounted under **`/api/v1/runtime/`** |

## Root package (engine.go)

- **`Engine`** (`engine.go`, package **`sumweave`**) owns a typed `wireup.HTTPRoot`; it has no container or Viper state. It is the embeddable API-only entrypoint when you do not run **`cmd/sumweave`**.
- **`NewEngine`** applies optional **`EngineOpt`** then eagerly builds the validated HTTP root: telemetry, runtime, auth, jobs reads, finance registration, routes, and server. It does not construct or start a worker or scheduler.
- **`Close(context.Context)`** releases resources owned by eager construction when the embedder does not start the server. It is idempotent and is also safe after **`StartHTTPServer`** completes, whose lifecycle already closes the root.
- **`StartHTTPServer`** is blocking until shutdown and delegates to the root lifecycle. **`EngineStartServerOpt`** supports **`noop`**, which performs normal composition and shutdown without listening.
- **Typed accessors** return the root's existing **`GetToolsRegistry`** and **`GetAgentRunner`** instances directly.
- **CLI user and fixture subcommands** use their own narrow explicit capabilities; they do not use `Engine`.
- **Rule of thumb:** keep **`engine.go`** free of business logic—only engine configuration, HTTP lifecycle, and narrow **`Get*`** exports.

## Layout (conceptual)

- **`main.go`** / **`cli.go`** — Cobra commands, process lifecycle, and command-local explicit root resolution.
- **`db-migrate`** — loads typed configuration and eagerly builds only logging/lifecycle/telemetry, SQL/auth stores, runtime migration inputs, and `DatabaseMigrator`; it does not build finance services, jobs worker/service, JWT, routes, or HTTP. It migrates the finance schema itself. Local setup invokes it exactly once for each prepared PostgreSQL environment through `make postgres-bootstrap` before **`start-all`**.
- **`start-all`** — diagnostic entrypoint that runs the HTTP server, appdispatch worker, and non-overlapping scheduler loop in one process using the same components as the split commands.
- **`start`** — API-only HTTP server mode used by local PM2 and production-like environments.
- **`sumweave jobs worker`** / **`sumweave jobs enqueue-due`** — dedicated split-environment appdispatch consumer and one-shot scheduler commands. The worker registers ordinary and job-observed finance consumers; the scheduler reads finance-owned bank and FX schedules, publishes due semantic commands, advances schedule state atomically with the publication, and does not run finance work or create job rows. Neither builds HTTP routes or a server.
- **`internal/wireup/`** — command-specific eager roots and explicit application wiring. HTTP, `start-all`, `db-migrate`, split jobs, user administration, and finance fixtures use direct construction.
- **`internal/agent_runtime.go`** — constructs **`agent.Runner`** (LLM provider, **`workspacefs`** tools, database-backed state in the configured app environments, with data dir for execution artifacts, and a required persisted agent profile service for runner-owned profile execution) and exposes **`httpapi`** as **`HTTPHandler`**.
- **`internal/api/http/`** — HTTP composition: **`server/`** (router, HTTPServer, middleware chain), **`v1routes/`** (generated routes + handlers, e.g. health), **`v1controllers/`**, **`middleware/`**, plus embedded UI staging under **`embeddedui/`** (tracked placeholder + generated ignored `dist/`).
- **`internal/config/`** — typed app config loader with **`default.yaml`** plus per-env **`local.yaml`**, **`test.yaml`**, **`production.yaml`**; optional **`*-user.yaml`** overrides. It is intentionally visible to app-internal packages, but only wireup should consume it; components receive native inputs or collaborators.
- **`internal/telemetry/`** — slog, OTEL resource, HTTP middleware, pprof helper.
- **`internal/infrastructure/`** — outbound HTTP client factory and middleware (correlation, logging).

## Shared architecture and operating contracts

Use the maintained repository references for cross-module behavior:

- [Product architecture](../../../docs/ARCHITECTURE.md) owns boundaries and scope.
- [Background processing](../../../docs/architecture/background-processing.md)
  owns appdispatch, typed events, job visibility, schedules, attempts, stale
  claims, retention policy, lazy job `404`, and shutdown ordering.
- [Persistence](../../../docs/architecture/persistence.md) owns DSNs, roles,
  explicit migrations, fixed prefixes, secrets, and volume boundaries.
- [Direct access](../../../docs/integrations/direct-access.md) owns personal
  tokens, route policies, requester scoping, idempotency, and `swmd` contracts.
- [Local development](../../../docs/operations/local-development.md) owns
  bootstrap, PM2, process commands, logs, and startup troubleshooting.
- [Release/deployment](../../../docs/operations/release-deployment.md) links
  host-build and chart runbooks.

## Configuration and env

- **Layers:** embedded **`default.yaml`**, then **`internal/config/<env>.yaml`** (from **`--env` / `-e`**, default **`local`**), then optional **`<env>-user.yaml`** for local secrets.
- **Env:** keys map to **`APP_…`** (Viper `AutomaticEnv()`); nested keys use underscores (e.g. **`APP_OPENAI_APIKEY`** for OpenAI). The loader exact-decodes the layered values before roots validate and translate them to native component inputs.
- **Database setup:** PostgreSQL is the only supported database. Startup commands never migrate app-owned schemas; run **`make postgres-bootstrap`** from the repository root before **`start-all`**, **`start`**, **`jobs worker`**, or **`jobs enqueue-due`**. It provisions local/test databases and roles, runs the two explicit `db-migrate` commands through the migrator role, then grants the runtime role access to the prepared schemas.
- **Worker claims:** **`jobs.worker.staleRunningAge`** defaults to 30 minutes,
  above the expected finance execution window. Active handlers renew their
  claims before recovery; a canceled handler conditionally requeues its own
  claim while leaving its dispatch message unacknowledged for retry.
- **HTTP defaults:** e.g. **`httpServer.port`** **4501**, **`writeTimeout`** aligned with long SSE/agent runs (see comments in **`default.yaml`**). Set both `httpServer.tls.certFile` and `keyFile` (or their `APP_` equivalents) for local HTTPS; see [../../../docs/local-https.md](../../../docs/local-https.md). No secrets in repo.

## Repository integration

- **Module:** `github.com/gemyago/sumweave/apps/sumweave` with **`replace github.com/gemyago/sumweave/runtime => ../../runtime`** for local **`runtime/`** development.
- **`apps/sumweave/Makefile`:** after root **`make postgres-bootstrap`**, **`make lint`** and ordinary **`make test`** (the single **`.cover/profile.out`** checked by **`.testcoverage.yaml`**), plus **`make dist/bin`** (rebuilds UI embed assets, validates `embeddedui/dist/index.html`, then `go build`).
- **Root `Makefile`** runs module targets before **`apps/sumweave-ui`**; Go coverage from this module's `.cover/profile.out` is merged into the root HTML report.

## API integration

- **Sumweave-owned OpenAPI:** **`internal/api/http/v1routes.yaml`** — health, **`/api/v1/auth/*`** (login, refresh, me), and related schemas; **camelCase** in JSON per module conventions. Codegen updates **`internal/api/http/v1routes/`** via apigen (**`go generate`** on **`register.go`**).
- **Runtime agent API:** authoritative OpenAPI for the agent HTTP surface lives under **`runtime/`** (e.g. **`runtime/internal/agentapi/openapi.yaml`**). This process **strips** the prefix and forwards **`/api/v1/runtime/*`** to the runtime **`httpapi`** handler (see **`internal/api/http/register.go`** **`SetupV1Routes`**). Standard run endpoints are profile-based (`profileName`), agent profiles carry mode-specific **`executionSettings`** (`regular` by default or `acp-stdio` for ACP subprocess execution), regular profile execution stays runner-owned, and the public runtime surface no longer exposes `/opencode-*` endpoints. The **`apps/sumweave-ui`** client generates TypeScript types from that runtime spec.
- **Browser SPA delivery:** backend builds can embed **`apps/sumweave-ui`** output into the same binary via Go embed. The same-origin deployment shape serves the SPA from `/` and backend routes under `/api/*`; the root UI handler serves real files first, falls back to **`index.html`** for SPA routes, and keeps `/api/*` plus `/enable-banking/*` misses as backend 404s instead of masking them with the SPA shell.

## Composition choices

- **Explicit composition:** the HTTP root builds runtime before handing its `net/http.Handler` to app route composition, and constructs jobs and finance before routes are exposed. `start` remains API-only; every command root is direct and command-appropriate.
- **`net/http` + `ServeMux` instead of Chi/Echo:** standard library, route patterns with method prefix; middleware wrapped per route in **`HTTPRouter`**.
- **apigen for `v1routes` only:** keeps generated HTTP glue for the thin app API separate from the heavier **`runtime/httpapi`** implementation.

## Technical notes (one-liners)

- **Local artifacts:** `data` holds experimental execution files, not durable auth, finance, job, or runtime state; those use configured PostgreSQL persistence.
- **LLM HTTP client timeout** in **`internal/agent_runtime.go`** should stay consistent with **`httpServer.writeTimeout`** for streaming runs.
- **Health** and **auth** routes use the generated v1 stack (**`RegisterHealthRoutes`**, **`RegisterAuthRoutes`**); **`GET /api/v1/auth/me`** uses the token-read policy in controller. **Agent** traffic is a separate subtree under **`/api/v1/runtime/`**.

For the browser client stack and env (`VITE_*`), see the
[UI architecture](../../sumweave-ui/doc/architecture.md). Experimental runtime
contracts remain in [runtime instructions](../../../runtime/AGENTS.md).
