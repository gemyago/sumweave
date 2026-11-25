import { test } from '../fixtures/browser';

test('sign-in returns to the protected Accounts link', async ({
  page, loginPage, financePage, ownerCredentials, tenant, account,
}) => {
  await test.step('Open Accounts while signed out', async () => {
    await page.goto('/#/finance/accounts');
    await loginPage.expectSignIn();
  });

  await test.step('Sign in and verify Accounts is restored', async () => {
    await loginPage.signIn(ownerCredentials);
    await financePage.expectAccounts();
    await financePage.selectTenant(tenant);
    await financePage.expectAccounts();
    await financePage.expectAccount(account);
  });
});
