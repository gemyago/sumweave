# Manual Browser E2E

Use direct commands only.

## Setup

Commands assume the repo shell environment is loaded first. In a human shell, run `direnv allow` once and then run the commands below from the repo root unless noted otherwise.

1. Run `npm i` at the repo root.
2. Verify local CLI: `npx playwright-cli --version`.
3. Restart the stack to make sure all processes are fresh:
   - `pm2 restart all`

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
- Use `snapshot` when you need locators or page state. For a quick pass, the hash check above is the cleaner success signal.
- Use `console` and `requests` for browser-side debugging.
- Use `state-save auth.json` if you want to reuse a logged-in session.
- Use `close` when done.
