import { expect } from '@playwright/test';
import { smokeTenant, uniqueLabel } from '../data/finance';
import { test } from '../fixtures/finance';

test('a USD tenant is visible until its owner archives it', async ({ ownerFinance }) => {
  const name = uniqueLabel('Playwright lifecycle');
  const created = await test.step('Create the tenant without defaults', () =>
    ownerFinance.createTenant({ ...smokeTenant, name }),
  );

  try {
    await test.step('Find the active tenant in the list and by ID', async () => {
      expect(created.id).toBeTruthy();
      expect(created).toMatchObject({ name, displayCurrency: 'USD' });
      expect(await ownerFinance.listTenants()).toContainEqual(expect.objectContaining({ id: created.id }));
      expect(await ownerFinance.getTenant(created.id)).toMatchObject({
        id: created.id,
        name,
        displayCurrency: 'USD',
      });
    });
  } finally {
    await test.step('Archive the tenant', async () => {
      const response = await ownerFinance.archiveTenant(created.id);
      expect(response.status()).toBe(204);
      expect(await response.text()).toBe('');
    });
  }

  await test.step('Verify the archived tenant disappears', async () => {
    expect(await ownerFinance.listTenants()).not.toContainEqual(expect.objectContaining({ id: created.id }));
    expect((await ownerFinance.tenantResponse(created.id)).status()).toBe(404);
  });
});

test('only the owner can see an unshared tenant', async ({ ownerFinance, anonymousFinance, outsiderFinance, tenant }) => {
  await test.step('Read the tenant as its owner', async () => {
    expect(await ownerFinance.getTenant(tenant.id)).toMatchObject({ id: tenant.id });
  });

  await test.step('Check access as an anonymous caller and as another user', async () => {
    expect((await anonymousFinance.tenantResponse(tenant.id)).status()).toBe(401);
    expect((await outsiderFinance.tenantResponse(tenant.id)).status()).toBe(404);
    expect(await outsiderFinance.listTenants()).not.toContainEqual(expect.objectContaining({ id: tenant.id }));
  });
});
