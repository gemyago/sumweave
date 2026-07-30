# Sumweave backend architecture (brief)

This file describes the current backend foundation in this repository. For product direction and naming, use the repository-level [../../../docs/ARCHITECTURE.md](../../../docs/ARCHITECTURE.md) as the source of truth.

HTTP server and CLI entrypoint for Sumweave under `apps/sumweave`: a single **`sumweave`** binary built from `cmd/sumweave`. Local commands run with `apps/sumweave` as CWD (`go run ./cmd/sumweave db-migrate --env local`, `go run ./cmd/sumweave start-all --env local`, or `go run ./cmd/sumweave start --env local`). The app wires configuration, logging, OpenTelemetry, health, finance APIs, auth, durable jobs, and the generic **runtime** agent HTTP API. The long-term shape is a deployable backend that can serve or embed **`apps/sumweave-ui`** as one unit.

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
- **`NewEngine`** applies optional **`EngineOpt`** then eagerly builds the validated HTTP root: telemetry, runtime, auth, jobs (without execution), finance registration, routes, and server.
- **`Close(context.Context)`** releases resources owned by eager construction when the embedder does not start the server. It is idempotent and is also safe after **`StartHTTPServer`** completes, whose lifecycle already closes the root.
- **`StartHTTPServer`** is blocking until shutdown and delegates to the root lifecycle. **`EngineStartServerOpt`** supports **`noop`**, which performs normal composition and shutdown without listening.
- **Typed accessors** return the root's existing **`GetToolsRegistry`** and **`GetAgentRunner`** instances directly.
- **CLI user and fixture subcommands** use their own narrow explicit capabilities; they do not use `Engine`.
- **Rule of thumb:** keep **`engine.go`** free of business logic—only engine configuration, HTTP lifecycle, and narrow **`Get*`** exports.

## Layout (conceptual)

- **`main.go`** / **`cli.go`** — Cobra commands, process lifecycle, and command-local explicit root resolution.
- **`db-migrate`** — loads typed configuration and eagerly builds only logging/lifecycle/telemetry, SQL/auth stores, runtime migration inputs, and `DatabaseMigrator`; it does not build finance services, jobs worker/service, JWT, routes, or HTTP. It migrates the finance schema itself and remains the standard local backend workflow before **`start-all`**.
- **`start-all`** — standard local backend workflow entrypoint; runs the HTTP server, durable jobs consumer, and non-overlapping scheduler loop in one process using the same components as the split commands.
- **`start`** — API-only HTTP server mode for split or production-like environments.
- **`sumweave jobs worker`** / **`sumweave jobs enqueue-due`** — dedicated split-environment consumer and one-shot scheduler commands. Each uses the typed jobs root, which constructs jobs and completes finance schedule/handler registration before command orchestration starts a worker or scheduler operation; it builds no HTTP routes or server.
- **`internal/wireup/`** — command-specific eager roots and explicit application wiring. HTTP, `start-all`, `db-migrate`, split jobs, user administration, and finance fixtures use direct construction.
- **`internal/agent_runtime.go`** — constructs **`agent.Runner`** (LLM provider, **`workspacefs`** tools, filesystem storage under configurable data dir, and a required persisted agent profile service for runner-owned profile execution) and exposes **`httpapi`** as **`HTTPHandler`**.
- **`internal/api/http/`** — HTTP composition: **`server/`** (router, HTTPServer, middleware chain), **`v1routes/`** (generated routes + handlers, e.g. health), **`v1controllers/`**, **`middleware/`**, plus embedded UI staging under **`embeddedui/`** (tracked placeholder + generated ignored `dist/`).
- **`internal/config/`** — typed app config loader with **`default.yaml`** plus per-env **`local.yaml`**, **`test.yaml`**, **`production.yaml`**; optional **`*-user.yaml`** overrides. It is intentionally visible to app-internal packages, but only wireup should consume it; components receive native inputs or collaborators.
- **`internal/telemetry/`** — slog, OTEL resource, HTTP middleware, pprof helper.
- **`internal/infrastructure/`** — outbound HTTP client factory and middleware (correlation, logging).

## Configuration and env

- **Layers:** embedded **`default.yaml`**, then **`internal/config/<env>.yaml`** (from **`--env` / `-e`**, default **`local`**), then optional **`<env>-user.yaml`** for local secrets.
- **Env:** keys map to **`APP_…`** (Viper `AutomaticEnv()`); nested keys use underscores (e.g. **`APP_OPENAI_APIKEY`** for OpenAI). The loader exact-decodes the layered values before roots validate and translate them to native component inputs.
- **Database setup:** startup commands no longer auto-migrate app-owned schemas; run **`sumweave db-migrate`** before **`start-all`** as the standard local backend workflow, and also before **`start`**, **`jobs worker`**, or **`jobs enqueue-due`** when the environment uses persisted tables.
- **HTTP defaults:** e.g. **`httpServer.port`** **4501**, **`writeTimeout`** aligned with long SSE/agent runs (see comments in **`default.yaml`**). Set both `httpServer.tls.certFile` and `keyFile` (or their `APP_` equivalents) for local HTTPS; see [../../../docs/local-https.md](../../../docs/local-https.md). No secrets in repo.

## Repository integration

- **Module:** `github.com/gemyago/sumweave/apps/sumweave` with **`replace github.com/gemyago/sumweave/runtime => ../../runtime`** for local **`runtime/`** development.
- **`apps/sumweave/Makefile`:** **`make lint`** (**`golangci-lint`**), **`make test`** (coverage profile + **`go-test-coverage`** vs **`.testcoverage.yaml`**), **`make dist/bin`** (rebuilds UI embed assets, validates `embeddedui/dist/index.html`, then `go build`).
- **Root `Makefile`** runs **`$(MAKE) -C apps/sumweave lint|test`** before **`apps/sumweave-ui`**; Go coverage from this module is merged into the root merged HTML report (see root Makefile **`tail`** of **`apps/sumweave/.cover/profile.out`**).

## API integration

- **Sumweave-owned OpenAPI:** **`internal/api/http/v1routes.yaml`** — health, **`/api/v1/auth/*`** (login, refresh, me), and related schemas; **camelCase** in JSON per module conventions. Codegen updates **`internal/api/http/v1routes/`** via apigen (**`go generate`** on **`register.go`**).
- **Runtime agent API:** authoritative OpenAPI for the agent HTTP surface lives under **`runtime/`** (e.g. **`runtime/internal/agentapi/openapi.yaml`**). This process **strips** the prefix and forwards **`/api/v1/runtime/*`** to the runtime **`httpapi`** handler (see **`internal/api/http/register.go`** **`SetupV1Routes`**). Standard run endpoints are profile-based (`profileName`), agent profiles carry mode-specific **`executionSettings`** (`regular` by default or `acp-stdio` for ACP subprocess execution), regular profile execution stays runner-owned, and the public runtime surface no longer exposes `/opencode-*` endpoints. The **`apps/sumweave-ui`** client generates TypeScript types from that runtime spec.
- **Browser SPA delivery:** backend builds can embed **`apps/sumweave-ui`** output into the same binary via Go embed. The same-origin deployment shape serves the SPA from `/` and backend routes under `/api/*`; the root UI handler serves real files first, falls back to **`index.html`** for SPA routes, and keeps `/api/*` plus `/enable-banking/*` misses as backend 404s instead of masking them with the SPA shell.

## Decisions (why not X)

- **Explicit composition:** the HTTP root builds runtime before handing its `net/http.Handler` to app route composition, and constructs jobs and finance before routes are exposed. `start` remains API-only; every command root is direct and command-appropriate.
- **`net/http` + `ServeMux` instead of Chi/Echo:** standard library, route patterns with method prefix; middleware wrapped per route in **`HTTPRouter`**.
- **apigen for `v1routes` only:** keeps generated HTTP glue for the thin app API separate from the heavier **`runtime/httpapi`** implementation.

## Technical notes (one-liners)

- **Local persistence:** app-root launches use **`data`** for filesystem-backed agent state and **`data/application.db`** as the finance application database for auth, finance, durable transport, and jobs. Agent runtime database persistence is configured separately. On disk these are under **`apps/sumweave/data`**.
- **LLM HTTP client timeout** in **`internal/agent_runtime.go`** should stay consistent with **`httpServer.writeTimeout`** for streaming runs.
- **Health** and **auth** routes use the generated v1 stack (**`RegisterHealthRoutes`**, **`RegisterAuthRoutes`**); **`GET /api/v1/auth/me`** is wrapped with **`AuthMiddleware`** in controller. **Agent** traffic is a separate subtree under **`/api/v1/runtime/`**.

For the browser client stack and env (**`VITE_*`**), see **`apps/sumweave-ui/doc/architecture.md`**.
