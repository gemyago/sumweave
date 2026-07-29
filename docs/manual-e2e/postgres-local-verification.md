# Optional local Postgres verification

This is an optional local-only verification path.

- Default local workflow stays SQLite.
- Do not change `default.yaml`, `local.yaml`, or `ecosystem.config.js` for this.
- Use env overrides only.

## What this covers

Use this when you want to verify backend flows against local Postgres without making Postgres the default developer path.

- `db-migrate` uses a DDL-capable role.
- backend runtime uses a DML/query role.
- agent runtime, application auth/jobs/dispatch, and finance storage use PostgreSQL.
- synthetic-provider verification can then reuse the existing manual e2e guides.

## Local roles, passwords, and DSNs

Fixed local-only credentials used by `postgres-local.compose.yml`:

- owner role: `sumweave_owner`
- owner password: `sumweave_owner_local`
- migration role: `sumweave_migrator`
- migration password: `sumweave_migrator_local`
- runtime role: `sumweave_runtime`
- runtime password: `sumweave_runtime_local`
- database: `sumweave_local`
- host port: `55432`

Suggested local env:

```bash
export PG_VERIFY_PROJECT=sumweave-pg-verify
export PG_VERIFY_APP_DATA_DIR="$PWD/tmp/postgres-verify-appdata"
export PG_VERIFY_OWNER_DSN='postgres://sumweave_owner:sumweave_owner_local@127.0.0.1:55432/sumweave_local?sslmode=disable'
export PG_VERIFY_MIGRATE_DSN='postgres://sumweave_migrator:sumweave_migrator_local@127.0.0.1:55432/sumweave_local?sslmode=disable'
export PG_VERIFY_RUNTIME_DSN='postgres://sumweave_runtime:sumweave_runtime_local@127.0.0.1:55432/sumweave_local?sslmode=disable'
```

Backend env mapping for this path:

- `APP_DATADIR="$PG_VERIFY_APP_DATA_DIR"` is only an ephemeral workspace path.
- `APP_APPLICATION_DATABASE_DSN="$PG_VERIFY_MIGRATE_DSN"` for `db-migrate`
- `APP_APPLICATION_DATABASE_DSN="$PG_VERIFY_RUNTIME_DSN"` for runtime and user commands
- `APP_AGENTRUNTIME_DATABASE_DSN="$PG_VERIFY_MIGRATE_DSN"` for `db-migrate`
- `APP_AGENTRUNTIME_DATABASE_DSN="$PG_VERIFY_RUNTIME_DSN"` for runtime and user commands

## Start local Postgres

Run from the repo root:

```bash
docker compose -f docs/manual-e2e/postgres-local.compose.yml -p "$PG_VERIFY_PROJECT" down -v
rm -rf "$PG_VERIFY_APP_DATA_DIR"
mkdir -p "$PG_VERIFY_APP_DATA_DIR"
docker compose -f docs/manual-e2e/postgres-local.compose.yml -p "$PG_VERIFY_PROJECT" up -d
docker compose -f docs/manual-e2e/postgres-local.compose.yml -p "$PG_VERIFY_PROJECT" ps
```

## Run migrations with the DDL role

Change to the backend app root once:

```bash
cd apps/sumweave
APP_DATADIR="$PG_VERIFY_APP_DATA_DIR" \
APP_APPLICATION_DATABASE_DSN="$PG_VERIFY_MIGRATE_DSN" \
APP_AGENTRUNTIME_DATABASE_DSN="$PG_VERIFY_MIGRATE_DSN" \
go run ./cmd/sumweave db-migrate --env local
```

Then run an explicit grant pass as a guard:

```bash
docker compose -f ../../docs/manual-e2e/postgres-local.compose.yml -p "$PG_VERIFY_PROJECT" exec postgres \
  psql -U sumweave_owner -d sumweave_local \
  -c "GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO sumweave_runtime; GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO sumweave_runtime;"
```

## Run the backend with the runtime role

From `apps/sumweave`:

```bash
APP_DATADIR="$PG_VERIFY_APP_DATA_DIR" \
APP_APPLICATION_DATABASE_DSN="$PG_VERIFY_RUNTIME_DSN" \
APP_AGENTRUNTIME_DATABASE_DSN="$PG_VERIFY_RUNTIME_DSN" \
go run ./cmd/sumweave start-all --env local
```

For longer manual sessions, you can run the same command under PM2 with a separate process name instead of changing the default ecosystem config.

## Create or rotate the local app user

From `apps/sumweave`:

```bash
APP_DATADIR="$PG_VERIFY_APP_DATA_DIR" \
APP_APPLICATION_DATABASE_DSN="$PG_VERIFY_RUNTIME_DSN" \
APP_AGENTRUNTIME_DATABASE_DSN="$PG_VERIFY_RUNTIME_DSN" \
go run ./cmd/sumweave --log-level WARN --env local user add \
  --username 'postgres-verify-e2e' \
  --password 'postgres-verify-e2e-local'
```

If that user already exists after a partial run, use `user change-password` with the same env.

## Verification flows for the next step

After the backend is up, reuse the existing guides:

- API flow: [synthetic-provider-flow-e2e.md](./synthetic-provider-flow-e2e.md)
- UI flow: [synthetic-provider-ui-e2e.md](./synthetic-provider-ui-e2e.md)

Suggested login for that verification:

- username: `postgres-verify-e2e`
- password: `postgres-verify-e2e-local`

Useful spot checks:

```bash
curl -i http://127.0.0.1:4501/health
docker compose -f ../../docs/manual-e2e/postgres-local.compose.yml -p "$PG_VERIFY_PROJECT" exec postgres \
  psql -U sumweave_owner -d sumweave_local \
  -c "SELECT table_name FROM information_schema.tables WHERE table_schema='public' ORDER BY table_name;"
```

Notes:

- Any non-SQLite DSN is treated as Postgres; use `postgres://...` and avoid `.db` or `.sqlite` in the DSN.
- `application.database.tablePrefix` remains a table-name prefix. Keep the default `sumweave_` for this path.
- If runtime gets permission errors after a successful migration, fix grants/default privileges instead of switching the runtime to the migration role.

## Stop and clean up

```bash
docker compose -f ../../docs/manual-e2e/postgres-local.compose.yml -p "$PG_VERIFY_PROJECT" down -v
```
