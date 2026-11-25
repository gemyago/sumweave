# Finance smoke tests

The `api` project exercises the real HTTP API and PostgreSQL. The `chromium`
project signs in through the Vite UI in a fresh browser context for each test.

## Set up locally

Use the project shell and run these commands from the repository root. Follow
the [manual setup](../docs/manual-e2e/README.md#setup) to switch to HTTP if you
have enabled local HTTPS.

```sh
npm ci
npm ci --prefix apps/sumweave-ui
npm ci --prefix tests
make postgres-bootstrap

export PLAYWRIGHT_OWNER_USERNAME=playwright-smoke-owner
export PLAYWRIGHT_OWNER_PASSWORD=playwright-local-owner
export PLAYWRIGHT_OUTSIDER_USERNAME=playwright-smoke-outsider
export PLAYWRIGHT_OUTSIDER_PASSWORD=playwright-local-outsider

(
  cd apps/sumweave
  go run ./cmd/sumweave --env local user add --if-not-exists --username "$PLAYWRIGHT_OWNER_USERNAME" --password "$PLAYWRIGHT_OWNER_PASSWORD"
  go run ./cmd/sumweave --env local user add --if-not-exists --username "$PLAYWRIGHT_OUTSIDER_USERNAME" --password "$PLAYWRIGHT_OUTSIDER_PASSWORD"
)

export PATH="$PWD/tests/node_modules/.bin:$PWD/node_modules/.bin:$PATH"
pm2 start ecosystem.config.js
curl -fsS --retry 90 --retry-all-errors --retry-delay 1 --retry-max-time 90 --max-time 2 http://127.0.0.1:4501/health
curl -fsS --retry 90 --retry-all-errors --retry-delay 1 --retry-max-time 90 --max-time 2 -o /dev/null http://127.0.0.1:5173/
```

The example passwords are local-only. Provision the users once, then export
the same credentials in each test shell. Use dedicated accounts with no shared
tenant memberships. The tests create or reuse the owner's **Playwright Smoke**
tenant and its **Playwright Manual USD** account. They keep their uniquely
named transactions and archive the separate tenant created by the lifecycle
test. Run the suite once at a time against a given dev stack so another run
cannot change the balance between assertions.

## Run the tests

```sh
npm --prefix tests run typecheck
npm --prefix tests test -- --project=api

export PLAYWRIGHT_BROWSERS_PATH="$PWD/tmp/playwright/browser-cache"
npm exec --prefix tests -- playwright install chromium
npm --prefix tests test -- --project=chromium --headed
npm --prefix tests test
```

For the API project alone, skip the UI install and browser setup. Start only
the API with `pm2 start ecosystem.config.js --only api` after bootstrapping
PostgreSQL and provisioning the users. Run the API health check from the setup
above before running the tests.

Open `tmp/playwright/report/index.html` for the latest run. Failed tests retain
traces and browser screenshots under `tmp/playwright/results/`. The reusable
Tests workflow runs both projects after Nx tests. Its Playwright step reports
stack startup, test runtime, and total elapsed time. The target is five
minutes, with a ten-minute limit.
On failure, it uploads the report, available test artifacts, and PM2 logs.

## Add a test

Place API specs in `api/`. Use the Finance fixtures for test-scoped bearer
sessions, and add operations to the typed client as needed. Check request and
response fields against the [OpenAPI contract](../apps/sumweave/internal/api/http/v1routes.yaml).

```ts
// api/account-currency.spec.ts
import { expect } from '@playwright/test';
import { test } from '../fixtures/finance';

test('the manual account uses the tenant currency', async ({ tenant, account }) => {
  await test.step('Compare account and tenant currencies', async () => {
    expect(account.currency).toBe(tenant.displayCurrency);
  });
});
```

Run that file with
`npm --prefix tests test -- --project=api api/account-currency.spec.ts`.

Put browser specs in `e2e/`. Use the browser fixtures and keep role and label
locators in page objects. Use named steps for the business actions and a flow
when a journey needs several page objects or API assertions.

```ts
// e2e/finance-landing.spec.ts
import { test } from '../fixtures/browser';

test('sign-in opens Finance by default', async ({ page, loginPage, financePage, ownerCredentials }) => {
  await test.step('Sign in without a protected destination', async () => {
    await page.goto('/#/login');
    await loginPage.signIn(ownerCredentials);
    await financePage.expectDashboard();
  });
});
```

Run that file with
`npm --prefix tests test -- --project=chromium e2e/finance-landing.spec.ts`.

## Running agent integration tests

Agent integration tests require a local providers config file with real LLM credentials. This file is gitignored and must be created manually.

### Manual Setup

Copy the example and fill in your credentials:

```bash
cp tests/agent/integration-cli/data/providers/provider.example.yaml \
   tests/agent/integration-cli/data/providers/provider.yaml
```
Name the file against provider name.

Update `tests/agent/integration-cli/data/providers/provider.yaml` with provider specific details, like models, base URL e.t.c.

**NOTE for AI**: AI must not attempt to read actual provider files, never!!!

### Run tests

Run agent that supports sub-agents (do we have any that doesn't?) and run the prompt like this: "Please run agent integration tests".
