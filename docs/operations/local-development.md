---
type: Runbook
title: "Local development"
description: "Install dependencies, bootstrap PostgreSQL, and run the API, worker, and UI."
status: stable
sources:
  - resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/CONTRIBUTING.md"
  - resource: "https://github.com/gemyago/sumweave/blob/bc51bc8a59aed011589fbb6c32edfa3afc2511d7/ecosystem.config.js"
---

# Local development

## Prerequisites and dependencies

Use direnv, nvm, Go 1.26.x, Node.js 24.x (the repository `.nvmrc`), and Docker
Compose. In an interactive shell, run `direnv allow` at the repository root;
agents with ephemeral shells use `direnv exec <working_dir> <command>`.
The latter loads the environment without changing the command's working directory.
Documented commands assume the environment is already loaded.

From the repository root:

```sh
npm ci
npx nx run-many -t install-deps
```

The UI has its own lockfile; `npm ci --prefix apps/sumweave-ui` also installs its
dependencies directly. The root package supplies repo-scoped PM2 and Playwright,
not an npm workspace for the UI. Install `tests/` separately for its smoke suite.
All temporary files and artifacts belong under repository `tmp/`.

## Run the stack

The normal local workflow uses Compose PostgreSQL and PM2 from the repository
root. Bootstrap starts Compose, prepares the local and test databases, runs both
explicit migrations, and grants the runtime role before PM2 starts the backend:

```bash
make postgres-bootstrap
pm2 start ecosystem.config.js
```

Backend CLI paths, including the explicit `db-migrate` command, are relative to
`apps/sumweave`. Nx and PM2 set that working directory for the backend process;
PM2 commands themselves remain repo-root commands because `PM2_HOME` is
repo-scoped. Do not use direct `db-migrate` as a replacement for local bootstrap.

Frontend host/port: http://localhost:5173
Backend host/port: http://localhost:4501

PM2 process names:

- `api` and `worker` in the `backend` namespace
- `ui` in PM2's default namespace

Useful PM2 commands from the repo root:

```bash
pm2 status
pm2 restart backend
pm2 restart ui
pm2 stop backend
pm2 stop ui
pm2 delete backend
pm2 delete ui
```

Use `pm2 logs backend` for combined backend output and `pm2 logs` for all
output. PM2 accepts `backend` as the positional namespace target for start,
stop, restart, delete, and logs. If the backend ecosystem command or arguments
change, recreate it rather than restarting it:

```bash
pm2 delete backend
pm2 start ecosystem.config.js
```

For the optional HTTPS variant of this PM2 workflow, see
[Local HTTPS](../local-https.md).

## Direct-start diagnostics only

Use direct commands only to isolate a local startup problem; they are not the
normal development workflow. From separate terminals, run the backend from
`apps/sumweave` with `go run ./cmd/sumweave start-all --env local`
and the UI from `apps/sumweave-ui` with `npm run dev`.

If the data screen still shows a browse-first availability `404` after a PM2 restart, check `pm2 status`: the UI proxies `/api/v1/*` to port `4501`, and a stale non-PM2 `sumweave start` process on that port can keep PM2's backend stopped. Stop the stray listener before starting the PM2 API again.


## Scheduler, data, and verification

PM2 does not run a scheduler loop. From `apps/sumweave`, use
`go run ./cmd/sumweave jobs enqueue-due --env local` when a scheduler tick is
needed. The worker executes published commands; [background processing](../architecture/background-processing.md)
explains `--once` and lazy job observation.

Startup never substitutes for schema migration. `make postgres-bootstrap` starts
and waits for Compose, provisions owner/migrator/runtime roles and the local/test
databases, migrates each once, then grants runtime access. See
[persistence](../architecture/persistence.md) and the
[PostgreSQL verification runbook](../manual-e2e/postgres-local-verification.md)
for clean-volume procedures. Alpha data may be recreated; no SQLite migration
is supported.

For user setup and browser/API session acquisition, follow the
[shared manual setup](../manual-e2e/README.md#local-e2e-user). The backend
`user add --if-not-exists` option supports retry-safe bootstrap without resetting
an existing password. Run user CLI commands from `apps/sumweave`.

Use the first `.local-users` entry for seed/reseed requests unless specified.
Reseed means replace local seeded data, then reopen the live database. Reuse
existing credentials unless the guide requires an isolated identity. For live
system interaction use `swmd`; from `apps/sumweave`,
`go run ./cmd/swmd skill` prints its offline command guide. See
[direct access](../integrations/direct-access.md) for safe configuration.

For code changes, bootstrap PostgreSQL before ordinary backend tests, then run
`make affected-lint-test` from the root. Module instructions may define a narrower
completion scope. Nx examples and testing journeys are in
[development and verification](../development/verification.md).
