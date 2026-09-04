# Local PostgreSQL verification

PostgreSQL is the mandatory local backend database. The repository Compose
environment and `make postgres-bootstrap` are the supported setup path; this
guide explains its concrete contract for backend processes and ordinary tests.

## What this covers

- `db-migrate` runs once for `local` and once for `test` through the DDL-capable
  migrator role.
- Backend processes use the DML/query runtime role.
- Agent runtime, application auth/jobs/dispatch, and finance storage use the
  prepared PostgreSQL schemas.
- API-only manual E2E guides and ordinary backend tests reuse the same contract.

## Local roles, passwords, and DSNs

Fixed local-only credentials used by `compose.yaml` and the bootstrap target:

- owner role: `sumweave_owner`
- owner password: `sumweave_owner_local`
- migration role: `sumweave_migrator`
- migration password: `sumweave_migrator_local`
- runtime role: `sumweave_runtime`
- runtime password: `sumweave_runtime_local`
- databases: `sumweave_local` and `sumweave_test`
- host port: `55432`

## Start local Postgres

Run from the repo root:

```bash
make postgres-bootstrap
```

The target starts and waits for Compose, makes the owner, migrator, and runtime
roles idempotently, creates both databases, runs `sumweave db-migrate --env
local` and `sumweave db-migrate --env test` through the migrator role, then
applies runtime grants. Cluster bootstrap never defines application tables.

## Run the backend with the runtime role

From `apps/sumweave`:

```bash
go run ./cmd/sumweave start-all --env local
```

For normal local development, return to the repository root and run `pm2 start
ecosystem.config.js`. PM2 uses `start-all` only after bootstrap prepared the
schemas.

## Create or rotate the local app user

From `apps/sumweave`:

```bash
go run ./cmd/sumweave --log-level WARN --env local user add \
  --username 'sumweave-local-e2e' \
  --password 'sumweave-local-e2e'
```

If that user already exists after a partial run, use `user change-password` with the same env.

## Verification flows for the next step

After the backend is up, reuse the existing guides:

- API flow: [synthetic-provider-flow-e2e.md](./synthetic-provider-flow-e2e.md)
- UI flow: [synthetic-provider-ui-e2e.md](./synthetic-provider-ui-e2e.md)

Suggested login for that verification:

- username: `sumweave-local-e2e`
- password: `sumweave-local-e2e`

Useful spot checks:

```bash
curl -i http://127.0.0.1:4501/health
docker compose exec -T postgres \
  psql -p 55432 -U sumweave_owner -d sumweave_local \
  -c "SELECT table_name FROM information_schema.tables WHERE table_schema='public' ORDER BY table_name;"
```

Notes:

- PostgreSQL DSNs use `postgres://...`; SQLite DSNs are unsupported.
- `application.database.tablePrefix` remains a table-name prefix. Keep the default `sumweave_` for this path.
- If runtime gets permission errors after a successful migration, fix grants/default privileges instead of switching the runtime to the migration role.

## Stop and clean up

```bash
docker compose down -v
```

Run `make postgres-bootstrap` later to recreate the two databases and schemas.
Run ordinary backend `make test` or `make affected-lint-test` commands after
bootstrap; they select tagged PostgreSQL tests where applicable.
