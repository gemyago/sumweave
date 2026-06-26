# Manual Browser E2E

Use direct commands only.

## Setup

Commands assume the repo shell environment is loaded first. In a human shell, run `direnv allow` once and then run the commands below from the repo root unless noted otherwise.

1. Run `npm i` at the repo root.
2. Verify local CLI: `npx playwright-cli --version`.
3. Prepare backend tables before starting or restarting backend processes:
   - `cd apps/signal-foundry`
   - `go run ./cmd/signal-foundry db-migrate --env local`
4. Recreate the backend PM2 app from the current ecosystem config so it really uses the documented `start-all` command:
   - `pm2 delete signal-foundry-api`
   - `pm2 start ecosystem.config.js`
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

## Finance seed shorthand

For repo-local manual work and AI-assisted local ops, use this shorthand:

- `Seed the database` means seed the local finance/dev database for the first user listed in repo-root `.local-users`.
- `Reseed the database` means replace existing local seeded data with a fresh seed, again defaulting to the first user listed in repo-root `.local-users`.

This is a workflow convention, not a separate product command. Keep using the existing documented app commands and local data paths.

When reseeding local SQLite data, restart the backend afterwards so the live API reopens the current database files instead of continuing to serve an older open file handle.

## Playwright CLI flow

```bash
npx playwright-cli open http://127.0.0.1:5173/#/login
npx playwright-cli fill "getByLabel('Username')" "e2e-manual-<yyyymmdd>-<suffix>"
npx playwright-cli fill "getByLabel('Password')" "<password>"
npx playwright-cli click "getByRole('button', { name: 'Sign in' })"
```

Optional debug snapshot after login:

```bash
npx playwright-cli snapshot
```

Success check:

```bash
npx playwright-cli --raw eval "window.location.hash"
npx playwright-cli close
```

Expected value: `"#/data"`

## Notes

- Durable historical jobs smoke flow after login:
  1. Open `#/data`, choose a bounded Hyperliquid futures scope, and use **Start historical backfill**.
  2. Follow the created job link to `#/jobs/<jobId>` or open `#/jobs` and wait for `succeeded`.
  3. Return to `#/data`, reload the same scope, and confirm candles are now visible.
  4. Open `#/evaluations` and run the existing synchronous evaluation on the backfilled range.
- Finance/admin smoke flow after login:
  1. Open `#/finance/tenants`; if needed, create a tenant and confirm it becomes selectable.
  2. Visit `#/finance`, verify the tenant picker works, and confirm the dashboard renders period controls plus KPI/alert sections.
  3. Visit `#/finance/accounts`, create a manual account, then open its detail route.
  4. Visit `#/finance/imports`, run a small CSV preview/confirm flow, and follow the finance job deep link if one is returned.
  5. Visit `#/finance/connections`, confirm the route exposes only the documented bank-link panels: monobank token linking and PKO bank login via Enable Banking. Do not enter arbitrary provider names.
  6. If safe local test credentials are available, exercise the matching flow only: submit a monobank token through the monobank panel, or start PKO bank login and confirm the provider redirect uses the backend callback endpoint (`/enable-banking/callback`, locally `http://localhost:6060/enable-banking/callback` or the active backend origin equivalent) before the browser is handed back to `/?code=...&state=...#/finance/connections`. If the registered callback origin differs from the active backend host, set `ENABLE_BANKING_CALLBACK_BASE_URL` for the backend before the run. For the current shared sandbox, set `ENABLE_BANKING_ASPSP_NAME="Mock ASPSP"` on the backend before testing; the product flow still starts from the PKO panel, but that sandbox app currently exposes Mock ASPSP instead of PKO. If the sandbox inventory changes, run `finance-poc enable-banking aspsps --country PL --json` and update `ENABLE_BANKING_ASPSP_NAME` to one of the currently exposed entries before retrying. After the flow finishes, confirm the app clears the consumed query string while preserving the `#/finance/connections` hash route.
  7. Confirm existing or newly linked connection cards render schedule/last-sync visibility, and trigger a bounded sync only if safe test data is available.
  8. Visit `#/admin/finance/fx` and `#/admin/finance/providers`; confirm sanitized diagnostics render and no secrets/raw payloads are shown.
- Use `snapshot` when you need locators or page state. For a quick pass, the hash check above is the cleaner success signal.
- Use `console` and `requests` for browser-side debugging.
- Use `state-save auth.json` if you want to reuse a logged-in session.
- Use `close` when done.
