# Signal Foundry backend architecture (brief)

This file describes the current backend foundation in this repository. For product direction and naming, use the repository-level [../../../docs/ARCHITECTURE.md](../../../docs/ARCHITECTURE.md) as the source of truth.

HTTP server and CLI entrypoint for Signal Foundry under `apps/signal-foundry`: a single **`signal-foundry`** binary built from `cmd/signal-foundry`. Local commands run with `apps/signal-foundry` as CWD (`go run ./cmd/signal-foundry db-migrate --env local`, `go run ./cmd/signal-foundry start-all --env local`, or `go run ./cmd/signal-foundry start --env local`). The app wires configuration, logging, OpenTelemetry, health, finance APIs, auth, durable jobs, and the generic **runtime** agent HTTP API. The long-term shape is a deployable backend that can serve or embed **`apps/signal-ui`** as one unit.

## Stack

| Area | Choice |
| --- | --- |
| Language | **Go** (see repo root / module **`go.mod`**, currently **1.26.x**) |
| CLI | **Cobra** — root command **`signal-foundry`**, subcommands e.g. **`db-migrate`**, **`start-all`**, **`start`** (HTTP), **`jobs`** |
| DI | **`go.uber.org/dig`** — wiring in `internal.Setup`, `internal/di`, `*Register` packages |
| HTTP | **`net/http`** + **`http.ServeMux`** — route patterns `METHOD path` on `internal/api/http/server.HTTPRouter`; SPA/static UI served from `/` with backend-path-aware fallback |
| Config | **Viper** — embedded YAML under `internal/config/`, env prefix **`APP_`** (see `internal/config/load.go`) |
| Logging | **`log/slog`** — root logger options in `internal/telemetry`, optional JSON / file / OTEL bridge |
| Observability | **OpenTelemetry** — traces/metrics/logs hooks in `internal/telemetry` (off by default in `default.yaml`) |
| Generated REST (app-owned) | **apigen** — `//go:generate` on `internal/api/http/register.go` reads **`internal/api/http/v1routes.yaml`** |
| Agent HTTP | **`runtime/httpapi`** — handler built in `internal/agent_runtime.go`, mounted under **`/api/v1/runtime/`** |

## Root package (engine.go)

- **`Engine`** (`engine.go`, package **`signal-foundry`**) holds the **`dig.Container`** and **`viper.Viper`** produced by **`internal.Setup`**. It is the embeddable entrypoint when you do not run **`cmd/signal-foundry`** (tests, other binaries, library-style use).
- **`NewEngine`** applies optional **`EngineOpt`** (JSON logs, log file path, **`env`**, default log level), then calls **`internal.Setup`** to register the full graph (config, telemetry, runtime, auth, app, HTTP constructors). Failures surface as **`failed to setup engine`**.
- **`StartHTTPServer`** is blocking until shutdown: it **`Register`s** **`server`** and **`http`** on the container (route wiring), then invokes **`StartupGroupFactory`** to start **`HTTPServer`**. Intended to be called **once** per process; **`EngineStartServerOpt`** supports **`noop`** for tests.
- **Typed accessors** on **`Engine`** resolve concrete types via **`container.Invoke`**: **`GetToolsRegistry`**, **`GetAgentRunner`**. Use these when you need the same instances as the HTTP server without starting it.
- **CLI user subcommands** (`cmd/signal-foundry`, e.g. **`user add` / `list` / `change-password`**) obtain **`auth.UserStore`** and **`auth.Argon2idHasher`** by calling **`NewEngine`** with the same injected **`dig.Container`** as the command (`internal.WithEngineContainer`), then resolving those types from that container—no extra **`Engine`** getters.
- **Rule of thumb:** keep **`engine.go`** free of business logic—only engine configuration, HTTP lifecycle, and narrow **`Get*`** exports.

## Layout (conceptual)

- **`main.go`** / **`cli.go`** — Cobra commands, process lifecycle, **`internal.Setup`** before subcommands run.
- **`db-migrate`** — explicit backend schema setup for the finance application database (auth, dispatch, jobs, and finance) and database-backed agent runtime persistence; standard local backend workflow runs this before **`start-all`**.
- **`start-all`** — standard local backend workflow entrypoint; runs the HTTP server, durable jobs consumer, and non-overlapping scheduler loop in one process using the same components as the split commands.
- **`start`** — API-only HTTP server mode for split or production-like environments.
- **`signal-foundry jobs worker`** / **`signal-foundry jobs enqueue-due`** — dedicated split-environment consumer and one-shot scheduler commands.
- **`internal/wireup.go`** — loads config and registers DI for identity, shutdown hooks, agent runtime, configuration providers, telemetry, application services, and infrastructure.
- **`internal/agent_runtime.go`** — constructs **`agent.Runner`** (LLM provider, **`workspacefs`** tools, filesystem storage under configurable data dir, and a required persisted agent profile service for runner-owned profile execution) and exposes **`httpapi`** as **`HTTPHandler`**.
- **`internal/api/http/`** — HTTP composition: **`server/`** (router, HTTPServer, middleware chain), **`v1routes/`** (generated routes + handlers, e.g. health), **`v1controllers/`**, **`middleware/`**, plus embedded UI staging under **`embeddedui/`** (tracked placeholder + generated ignored `dist/`).
- **`internal/config/`** — **`default.yaml`** plus per-env **`local.yaml`**, **`test.yaml`**, **`production.yaml`**; optional **`*-user.yaml`** overrides.
- **`internal/telemetry/`** — slog, OTEL resource, HTTP middleware, pprof helper.
- **`internal/infrastructure/`** — outbound HTTP client factory and middleware (correlation, logging).

## Configuration and env

- **Layers:** embedded **`default.yaml`**, then **`internal/config/<env>.yaml`** (from **`--env` / `-e`**, default **`local`**), then optional **`internal/config/<env>-user.yaml`** for local secrets.
- **Env:** keys map to **`APP_…`** (Viper `AutomaticEnv()`); nested keys use underscores (e.g. **`APP_OPENAI_APIKEY`** for OpenAI). See **`internal/config/default.yaml`** and **`internal/config/provide.go`** for injected **`name:"config.…"`** bindings.
- **Database setup:** startup commands no longer auto-migrate app-owned schemas; run **`signal-foundry db-migrate`** before **`start-all`** as the standard local backend workflow, and also before **`start`**, **`jobs worker`**, or **`jobs enqueue-due`** when the environment uses persisted tables.
- **HTTP defaults:** e.g. **`httpServer.port`** **4501**, **`writeTimeout`** aligned with long SSE/agent runs (see comments in **`default.yaml`**). Set both `httpServer.tls.certFile` and `keyFile` (or their `APP_` equivalents) for local HTTPS; see [../../../docs/local-https.md](../../../docs/local-https.md). No secrets in repo.

## Repository integration

- **Module:** `github.com/gemyago/signal-foundry/apps/signal-foundry` with **`replace github.com/gemyago/signal-foundry/runtime => ../../runtime`** for local **`runtime/`** development.
- **`apps/signal-foundry/Makefile`:** **`make lint`** (**`golangci-lint`**), **`make test`** (coverage profile + **`go-test-coverage`** vs **`.testcoverage.yaml`**), **`make dist/bin`** (rebuilds UI embed assets, validates `embeddedui/dist/index.html`, then `go build`).
- **Root `Makefile`** runs **`$(MAKE) -C apps/signal-foundry lint|test`** before **`apps/signal-ui`**; Go coverage from this module is merged into the root merged HTML report (see root Makefile **`tail`** of **`apps/signal-foundry/.cover/profile.out`**).

## API integration

- **Signal Foundry-owned OpenAPI:** **`internal/api/http/v1routes.yaml`** — health, **`/api/v1/auth/*`** (login, refresh, me), and related schemas; **camelCase** in JSON per module conventions. Codegen updates **`internal/api/http/v1routes/`** via apigen (**`go generate`** on **`register.go`**).
- **Runtime agent API:** authoritative OpenAPI for the agent HTTP surface lives under **`runtime/`** (e.g. **`runtime/internal/agentapi/openapi.yaml`**). This process **strips** the prefix and forwards **`/api/v1/runtime/*`** to the runtime **`httpapi`** handler (see **`internal/api/http/register.go`** **`SetupV1Routes`**). Standard run endpoints are profile-based (`profileName`), agent profiles carry mode-specific **`executionSettings`** (`regular` by default or `acp-stdio` for ACP subprocess execution), regular profile execution stays runner-owned, and the public runtime surface no longer exposes `/opencode-*` endpoints. The **`apps/signal-ui`** client generates TypeScript types from that runtime spec.
- **Browser SPA delivery:** backend builds can embed **`apps/signal-ui`** output into the same binary via Go embed. The same-origin deployment shape serves the SPA from `/` and backend routes under `/api/*`; the root UI handler serves real files first, falls back to **`index.html`** for SPA routes, and keeps `/api/*` plus `/enable-banking/*` misses as backend 404s instead of masking them with the SPA shell.

## Decisions (why not X)

- **`go.uber.org/dig` instead of a larger framework:** explicit constructor graphs, small surface, fits a single-binary service.
- **`net/http` + `ServeMux` instead of Chi/Echo:** standard library, route patterns with method prefix; middleware wrapped per route in **`HTTPRouter`**.
- **apigen for `v1routes` only:** keeps generated HTTP glue for the thin app API separate from the heavier **`runtime/httpapi`** implementation.

## Technical notes (one-liners)

- **Local persistence:** app-root launches use **`data`** for filesystem-backed agent state and **`data/application.db`** as the finance application database for auth, finance, durable transport, and jobs. Agent runtime database persistence is configured separately. On disk these are under **`apps/signal-foundry/data`**.
- **LLM HTTP client timeout** in **`internal/agent_runtime.go`** should stay consistent with **`httpServer.writeTimeout`** for streaming runs.
- **Health** and **auth** routes use the generated v1 stack (**`RegisterHealthRoutes`**, **`RegisterAuthRoutes`**); **`GET /api/v1/auth/me`** is wrapped with **`AuthMiddleware`** in controller. **Agent** traffic is a separate subtree under **`/api/v1/runtime/`**.

For the browser client stack and env (**`VITE_*`**), see **`apps/signal-ui/doc/architecture.md`**.
