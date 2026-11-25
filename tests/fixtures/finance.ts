import { expect, test as base } from '@playwright/test';
import { anonymousSession, signedInSession, type Credentials } from '../clients/auth';
import { FinanceClient, type Account, type Tenant } from '../clients/finance';
import { smokeAccount, smokeTenant } from '../data/finance';

type Fixtures = {
  ownerCredentials: Credentials;
  ownerFinance: FinanceClient;
  outsiderFinance: FinanceClient;
  anonymousFinance: FinanceClient;
  tenant: Tenant;
  account: Account;
};

function credentials(role: 'OWNER' | 'OUTSIDER'): Credentials {
  const usernameKey = `PLAYWRIGHT_${role}_USERNAME`;
  const passwordKey = `PLAYWRIGHT_${role}_PASSWORD`;
  const username = process.env[usernameKey];
  const password = process.env[passwordKey];
  if (!username || !password) {
    throw new Error(`Set ${usernameKey} and ${passwordKey} for a dedicated smoke-test user. See tests/README.md.`);
  }
  return { username, password };
}

export const test = base.extend<Fixtures>({
  ownerCredentials: async ({}, use) => {
    await use(credentials('OWNER'));
  },
  ownerFinance: async ({ ownerCredentials }, use) => {
    const session = await signedInSession(ownerCredentials);
    try {
      await use(new FinanceClient(session));
    } finally {
      await session.dispose();
    }
  },
  outsiderFinance: async ({ ownerCredentials }, use) => {
    const outsider = credentials('OUTSIDER');
    expect(outsider.username, 'smoke-test users are distinct').not.toBe(ownerCredentials.username);
    const session = await signedInSession(outsider);
    try {
      await use(new FinanceClient(session));
    } finally {
      await session.dispose();
    }
  },
  anonymousFinance: async ({}, use) => {
    const session = await anonymousSession();
    try {
      await use(new FinanceClient(session));
    } finally {
      await session.dispose();
    }
  },
  tenant: async ({ ownerFinance }, use) => {
    const existing = (await ownerFinance.listTenants()).find(({ name }) => name === smokeTenant.name);
    const tenant = existing ?? await ownerFinance.createTenant(smokeTenant);
    expect(tenant.displayCurrency).toBe(smokeTenant.displayCurrency);
    await use(tenant);
  },
  account: async ({ ownerFinance, tenant }, use) => {
    const existing = (await ownerFinance.listAccounts(tenant.id)).find(({ name }) => name === smokeAccount.name);
    const account = existing ?? await ownerFinance.createAccount(tenant.id, smokeAccount);
    expect(account).toMatchObject(smokeAccount);
    await use(account);
  },
});
