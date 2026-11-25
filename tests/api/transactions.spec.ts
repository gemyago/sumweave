import { expect } from '@playwright/test';
import { uniqueLabel } from '../data/finance';
import { test } from '../fixtures/finance';

test('a pending card charge reduces only the pending balance', async ({ ownerFinance, tenant, account }) => {
  const charge = {
    accountId: account.id,
    source: 'manual',
    kind: 'regular',
    status: 'pending',
    amountMinor: -1200,
    currency: 'USD',
    description: uniqueLabel('Pending card charge'),
    effectiveAt: '2026-01-15T12:00:00+01:00',
  };

  const baseline = await test.step('Read the account balances before the charge', async () => {
    const balances = await ownerFinance.getAccount(tenant.id, account.id);
    expect(Number.isInteger(balances.bookedBalanceMinor)).toBe(true);
    return balances;
  });

  const created = await test.step('Record a pending manual card charge', () =>
    ownerFinance.createTransaction(tenant.id, charge),
  );

  await test.step('Read the charge back by its returned ID', async () => {
    expect(created.id).toBeTruthy();
    expect(await ownerFinance.getTransaction(tenant.id, created.id)).toMatchObject({
      id: created.id,
      accountId: account.id,
      description: charge.description,
      currency: charge.currency,
      status: charge.status,
      amountMinor: charge.amountMinor,
    });
  });

  await test.step('Compare the reported balances with the baseline', async () => {
    const accounts = await ownerFinance.listAccounts(tenant.id);
    expect(accounts.find(({ id }) => id === account.id)).toMatchObject({
      pendingBalanceMinor: baseline.pendingBalanceMinor + charge.amountMinor,
      bookedBalanceMinor: baseline.bookedBalanceMinor,
    });
  });
});
