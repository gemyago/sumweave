import { expect, type Page } from '@playwright/test';
import type { Account, Tenant } from '../clients/finance';

export class FinancePage {
  constructor(private readonly page: Page) {}

  async expectDashboard(): Promise<void> {
    await expect(this.page).toHaveURL(/#\/finance(?:\?|$)/);
    await expect(this.page.getByRole('heading', { name: 'Finance dashboard' })).toBeVisible();
  }

  async expectAccounts(): Promise<void> {
    await expect(this.page).toHaveURL(/#\/finance\/accounts$/);
    await expect(this.page.getByRole('heading', { name: 'Finance accounts' })).toBeVisible();
  }

  async selectTenant(tenant: Tenant): Promise<void> {
    const menu = this.page.getByRole('button', { name: 'User menu' });
    await menu.click();
    const preferences = this.page.getByLabel('Workspace and preferences', { exact: true });
    const picker = preferences.getByRole('combobox', { name: 'Active tenant' });
    const soleTenant = preferences.getByText(tenant.name, { exact: true });

    // Finance selects a sole tenant automatically and only shows a picker for multiple tenants.
    await expect(picker.or(soleTenant).first()).toBeVisible();
    if (await picker.isVisible()) {
      await picker.selectOption(tenant.id);
      await expect(picker).toHaveValue(tenant.id);
    }
    await menu.click();
  }

  async expectAccount(account: Account): Promise<void> {
    const accounts = this.page.getByRole('table', { name: 'Accounts' });
    await expect(accounts.getByText(account.name, { exact: true })).toBeVisible();
  }

  async openTransactions(): Promise<void> {
    const navigation = this.page.getByRole('navigation', { name: 'Finance navigation' });
    await navigation.getByRole('link', { name: 'Transactions', exact: true }).click();
    await expect(this.page).toHaveURL(/#\/finance\/transactions$/);
  }
}
