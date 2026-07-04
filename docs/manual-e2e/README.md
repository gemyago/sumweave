# Manual E2E

Manual guides:

- [finance-tenants-management-e2e.md](./finance-tenants-management-e2e.md) — create, list, get by id, archive, then verify archived tenants disappear from active list and get-by-id.
- [finance-account-balances-e2e.md](./finance-account-balances-e2e.md) — create a tenant and accounts, record mixed booked/pending transactions, then verify account balances.
- [finance-ui-shell-smoke-e2e.md](./finance-ui-shell-smoke-e2e.md) — smoke the finance shell, dashboard hierarchy, transactions workspace, responsive behavior, and implementation run/report/fix loop after the shell restructure lands.
- [bank-linking-e2e.md](./bank-linking-e2e.md) — bank-linking API e2e guide.
- [synthetic-provider-flow-e2e.md](./synthetic-provider-flow-e2e.md) — start synthetic redirect linking over HTTP, save pending configured accounts, finish the link, trigger sync, then verify linked accounts and provider transactions.
- [synthetic-provider-ui-e2e.md](./synthetic-provider-ui-e2e.md) — sign in to the UI, start synthetic setup from Finance connections, save duplicate configured accounts, reload pending state, finish the link, and confirm the new connection card appears.

## Setup

Commands assume the repo shell environment is loaded first. In a human shell, run `direnv allow` once and then run the commands below from the repo root unless noted otherwise.

1. Run `npm i` at the repo root.
2. Verify local CLI: `npx playwright-cli --version`.
3. Prepare backend tables before starting or restarting backend processes:
   - `cd apps/signal-foundry`
   - `go run ./cmd/signal-foundry db-migrate --env local`
4. Ensure backend is running and fresh:
   - `pm2 status`
   - `pm2 restart signal-foundry-api --update-env` - restart if process was already registered
   - `pm2 start -f ecosystem.config.js` - start if process was not running already
5. If backend startup reports `bind: address already in use`, stop the stray process already listening on `127.0.0.1:4501` before retrying. A common cause is an older direct `go run ./cmd/signal-foundry start ...` process.

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
cd apps/signal-foundry
go run ./cmd/signal-foundry --log-level WARN --env local user change-password --username 'e2e-manual-<yyyymmdd>-<suffix>' --password '<password>'
```

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
