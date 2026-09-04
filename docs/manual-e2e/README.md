# Manual E2E

Manual guides:

- [finance-tenants-management-e2e.md](./finance-tenants-management-e2e.md) — create, list, get by id, archive, then verify archived tenants disappear from active list and get-by-id.
- [finance-account-balances-e2e.md](./finance-account-balances-e2e.md) — create a tenant and accounts, record mixed booked/pending transactions, then verify account balances.
- [finance-report-transaction-ui-e2e.md](./finance-report-transaction-ui-e2e.md) — create a tenant and manual account in the UI, report a transaction through the dedicated editor, then verify the new ledger row.
- [finance-transaction-csv-import-e2e.md](./finance-transaction-csv-import-e2e.md) — isolated API-only preview, confirm, expected `404`, bounded worker, terminal audit, and repeat-safe fixed-contract transaction import.
- [finance-account-csv-import-e2e.md](./finance-account-csv-import-e2e.md) — isolated API-only preview, confirm, expected `404`, bounded worker, terminal audit, and repeat-safe account CSV import.
- [finance-fx-refresh-e2e.md](./finance-fx-refresh-e2e.md) — local/static-provider manual and scheduled FX refresh, controlled provider failure, due-state checks, and lazy job observation.
- [finance-ui-shell-smoke-e2e.md](./finance-ui-shell-smoke-e2e.md) — smoke canonical Bootstrap login, default Finance landing, Finance shell/dashboard and route groups, responsive behavior, and a quick non-finance regression path.
- [bank-linking-e2e.md](./bank-linking-e2e.md) — bank-linking API e2e guide.
- [enable-banking-mock-aspsp-ui-e2e.md](./enable-banking-mock-aspsp-ui-e2e.md) — headed-browser PKO linking through Enable Banking Mock ASPSP, including mock accounts, transactions, authorization, and sync.
- [synthetic-provider-flow-e2e.md](./synthetic-provider-flow-e2e.md) — isolated synthetic redirect linking, stable pending-account keys, API-only sync publication, expected `404`, bounded worker, terminal job, and provider transaction assertions.
- [synthetic-provider-ui-e2e.md](./synthetic-provider-ui-e2e.md) — sign in to the UI, start synthetic setup from Finance connections, save duplicate configured accounts, reload pending state, finish the link, and confirm the new connection card appears.
- [finance-scheduled-sync-lifecycle-e2e.md](./finance-scheduled-sync-lifecycle-e2e.md) — isolated scheduled bank/FX publication with local Monobank and static FX fixtures, due-state checks, expected `404`s, bounded worker-once, terminal jobs, and repeat no-op assertions.

For job-observation checks, stop the normal PM2 worker and use an API-only
`start` process before publishing, then query the returned ID before starting
`jobs worker --once`. A `404` is expected in that window because publication
creates only the appdispatch message; the worker creates the job projection on
first delivery.

Local database setup:

- [postgres-local-verification.md](./postgres-local-verification.md) — local
  Compose PostgreSQL bootstrap, ordinary backend-test, and clean-volume runbook.

## Setup

Commands assume the repo shell environment is loaded first. In a human shell,
run `direnv allow` once at the repo root. Run repo-root npm, Playwright, and
PM2 commands from the repo root; run UI npm commands from `apps/sumweave-ui`
and backend CLI commands from `apps/sumweave`.

1. Install the separate project-scoped npm packages:
   - at the repo root, run `npm ci` for PM2 and Playwright;
   - from `apps/sumweave-ui`, run `npm ci` for Vite and the UI dependencies.
   The root package is not an npm workspace for the UI, so installing its
   dependencies does not install the UI package's Vite executable.
2. Verify local CLI: `npx playwright-cli --version`.
3. For the standard HTTP workflow, make sure optional local TLS is disabled:
   - remove `APP_HTTPSERVER_TLS_CERTFILE` and `APP_HTTPSERVER_TLS_KEYFILE`
     from ignored root `.envrc.local`
   - remove `VITE_LOCAL_HTTPS` and its certificate-path variables from ignored
     `apps/sumweave-ui/.env.local`; retain `VITE_AGENT_API_BASE_URL=/api/v1/runtime/`
   - see [local HTTPS](../local-https.md#return-to-the-standard-local-http-workflow)
     for the full switch-back procedure
4. Prepare the mandatory local PostgreSQL environment before starting or
   restarting backend processes: `make postgres-bootstrap`. This starts the
   repository Compose service, provisions `sumweave_local` and `sumweave_test`,
   runs the two explicit migrations through `sumweave_migrator`, and grants the
   runtime role access to the prepared schemas.
5. From the repo root, recreate both PM2 services when switching protocols or
   when a fresh ecosystem shape is required:
    - `pm2 status`
    - `pm2 delete backend`
    - `pm2 delete ui`
   - `pm2 start ecosystem.config.js`
   - `pm2 status`
6. Verify both HTTP services before the browser run:
   - `curl -i http://127.0.0.1:4501/health`
   - `curl -I http://127.0.0.1:5173/`
7. If backend startup reports `bind: address already in use`, stop the stray process already listening on `127.0.0.1:4501` before retrying. A common cause is an older direct `go run ./cmd/sumweave start ...` process launched from the app root.

The API-only gate guides deliberately replace the repo-scoped Compose volume,
then use the freshly prepared local PostgreSQL database and write non-database
evidence below `tmp/jobs-system-simplification-028-e2e/`. Do not run one while
another developer needs the local database. Each guide starts an API-only process
before its bounded worker step. The FX guides set
`APP_FINANCE_PROVIDERS_FRANKFURTER_BASEURL` to a local static
Frankfurter-compatible fixture; they do not call public FX endpoints.

**Note**: Always use repo-scoped data/users and other directories as if you just
started services through the documented PM2 workflow. If local data is incorrect
or a clean environment is required, recreate the repository Compose volume and
run `make postgres-bootstrap`; no SQLite data migration or compatibility copy is
supported.

## Local e2e user

If needed, create the backend user from `apps/sumweave` so it uses the same local data dir as the PM2 API process. If you don't need a fresh user, just reuse the existing one if it exists.

This is usually one-time setup only. Reuse the same user across runs unless the test specifically needs a fresh identity.

If you do need a fresh user, use a truly unique username such as `e2e-manual-<yyyymmdd>-<suffix>`:

```bash
cd apps/sumweave
go run ./cmd/sumweave --log-level WARN --env local user add --username 'e2e-manual-<yyyymmdd>-<suffix>' --password '<password>'
```

Deleting repo-root `.local-users` does not remove the backend user from local app data. If that username already exists, the add command will fail with `username already exists`. In that case, rotate the password instead:

```bash
go run ./cmd/sumweave --log-level WARN --env local user change-password --username 'e2e-manual-<yyyymmdd>-<suffix>' --password '<password>'
```

Return to the repo root before continuing with PM2, UI, or `.local-users`
commands.

Save credentials in repo-root `.local-users` for reuse across runs, unless the user is truly temporary. If the file does not exist yet, create it and keep one `username:password` entry per line. Use simple plain-text notes, for example:

```text
e2e-manual-<yyyymmdd>-<suffix>:<password>
```

That file is gitignored.

## Getting API token

Follow steps in this section to get API token for API level testing.

Preflight:
```bash
curl -i "http://127.0.0.1:4501/health"
```

Get the token:
```bash
IFS=: read -r USER PASS < ".local-users" && ACCESS_TOKEN=$(curl -sS -X POST "http://127.0.0.1:4501/api/v1/auth/login" -H "Content-Type: application/json" --data "{\"username\":\"${USER}\",\"password\":\"${PASS}\"}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["accessToken"])')
```
