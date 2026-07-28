# Manual E2E

Manual guides:

- [finance-tenants-management-e2e.md](./finance-tenants-management-e2e.md) — create, list, get by id, archive, then verify archived tenants disappear from active list and get-by-id.
- [finance-account-balances-e2e.md](./finance-account-balances-e2e.md) — create a tenant and accounts, record mixed booked/pending transactions, then verify account balances.
- [finance-report-transaction-ui-e2e.md](./finance-report-transaction-ui-e2e.md) — create a tenant and manual account in the UI, report a transaction through the dedicated editor, then verify the new ledger row.
- [finance-transaction-csv-import-e2e.md](./finance-transaction-csv-import-e2e.md) — preview, confirm, and observe a fixed-contract Finance transaction CSV import.
- [finance-ui-shell-smoke-e2e.md](./finance-ui-shell-smoke-e2e.md) — smoke canonical Bootstrap login, default Finance landing, Finance shell/dashboard and route groups, responsive behavior, and a quick non-finance regression path.
- [bank-linking-e2e.md](./bank-linking-e2e.md) — bank-linking API e2e guide.
- [enable-banking-mock-aspsp-ui-e2e.md](./enable-banking-mock-aspsp-ui-e2e.md) — headed-browser PKO linking through Enable Banking Mock ASPSP, including mock accounts, transactions, authorization, and sync.
- [synthetic-provider-flow-e2e.md](./synthetic-provider-flow-e2e.md) — start synthetic redirect linking over HTTP, save pending configured accounts, finish the link, trigger sync, then verify linked accounts and provider transactions.
- [synthetic-provider-ui-e2e.md](./synthetic-provider-ui-e2e.md) — sign in to the UI, start synthetic setup from Finance connections, save duplicate configured accounts, reload pending state, finish the link, and confirm the new connection card appears.
- [finance-scheduled-sync-lifecycle-e2e.md](./finance-scheduled-sync-lifecycle-e2e.md) — scheduled sync lifecycle runbook with app-root backend commands, isolated local persistence, local Monobank mock, enqueue-due, bounded worker-once, public connection/jobs assertions, and PM2 restore.

Optional local verification helpers:

- [postgres-local-verification.md](./postgres-local-verification.md) — optional local-only Postgres compose/runbook for backend verification without changing the default SQLite local workflow.

## Setup

Commands assume the repo shell environment is loaded first. In a human shell,
run `direnv allow` once at the repo root. Run npm, Playwright, and PM2 commands
from the repo root; run backend CLI commands from `apps/signal-foundry`.

1. Run `npm i` at the repo root.
2. Verify local CLI: `npx playwright-cli --version`.
3. For the standard HTTP workflow, make sure optional local TLS is disabled:
   - remove `APP_HTTPSERVER_TLS_CERTFILE` and `APP_HTTPSERVER_TLS_KEYFILE`
     from ignored root `.envrc.local`
   - remove `VITE_LOCAL_HTTPS` and its certificate-path variables from ignored
     `apps/signal-ui/.env.local`; retain `VITE_AGENT_API_BASE_URL=/api/v1/runtime/`
   - see [local HTTPS](../local-https.md#return-to-the-standard-local-http-workflow)
     for the full switch-back procedure
4. Prepare backend tables before starting or restarting backend processes:
    - `cd apps/signal-foundry`
    - `go run ./cmd/signal-foundry db-migrate --env local`
    - `cd ../..`
5. From the repo root, recreate both PM2 services when switching protocols or
   when a fresh ecosystem shape is required:
   - `pm2 status`
   - `pm2 delete signal-foundry-api`
   - `pm2 delete signal-foundry-ui`
   - `pm2 start ecosystem.config.js`
   - `pm2 status`
6. Verify both HTTP services before the browser run:
   - `curl -i http://127.0.0.1:4501/health`
   - `curl -I http://127.0.0.1:5173/`
7. If backend startup reports `bind: address already in use`, stop the stray process already listening on `127.0.0.1:4501` before retrying. A common cause is an older direct `go run ./cmd/signal-foundry start ...` process launched from the app root.

**Note**: Always use repo scoped data/users and other dirs as if you just started all services using documented pm2 instruction. Do not try to use other dirs (like system temp or similar). If some data feels incorrect or missing - it's dev env, not production so you can drop local sqlite DB and recreate it using standard approach.

## Local e2e user

If needed, create the backend user from `apps/signal-foundry` so it uses the same local data dir as the PM2 API process. If you don't need a fresh user, just reuse the existing one if it exists.

This is usually one-time setup only. Reuse the same user across runs unless the test specifically needs a fresh identity.

If you do need a fresh user, use a truly unique username such as `e2e-manual-<yyyymmdd>-<suffix>`:

```bash
cd apps/signal-foundry
go run ./cmd/signal-foundry --log-level WARN --env local user add --username 'e2e-manual-<yyyymmdd>-<suffix>' --password '<password>'
```

Deleting repo-root `.local-users` does not remove the backend user from local app data. If that username already exists, the add command will fail with `username already exists`. In that case, rotate the password instead:

```bash
go run ./cmd/signal-foundry --log-level WARN --env local user change-password --username 'e2e-manual-<yyyymmdd>-<suffix>' --password '<password>'
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
