import { uniqueLabel } from '../data/finance';
import { test } from '../fixtures/browser';

test('a booked expense appears in the ledger after reload', async ({
  page, loginPage, financePage, recordExpense, ownerCredentials, tenant, account,
}) => {
  const description = uniqueLabel('Playwright booked expense');

  await test.step('Sign in and select the smoke-test workspace', async () => {
    await page.goto('/#/login');
    await loginPage.signIn(ownerCredentials);
    await financePage.expectDashboard();
    await financePage.selectTenant(tenant);
  });

  const id = await test.step('Record a booked expense of -12.34 USD', () =>
    recordExpense.recordExpense(tenant, account, description),
  );

  await test.step('Verify the saved expense in the reloaded ledger and API', () =>
    recordExpense.expectPersistedExpense({ tenant, account }, { id, description }),
  );
});
