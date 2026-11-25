import { expect, type Page, type Response } from '@playwright/test';
import type { Account } from '../clients/finance';

export class TransactionsPage {
  constructor(private readonly page: Page) {}

  async fillBookedExpense(account: Account, description: string): Promise<void> {
    await this.page.getByRole('link', { name: 'Create transaction' }).click();
    await expect(this.page).toHaveURL(/#\/finance\/transactions\/new$/);

    await this.page.getByLabel('Transaction account').selectOption(account.id);
    await this.page.getByLabel('Transaction kind').selectOption('expense');
    await this.page.getByLabel('Transaction status').selectOption('booked');
    await this.page.getByLabel('Transaction source').selectOption('manual');
    await expect(this.page.getByLabel('Transaction currency')).toHaveValue('USD');
    await this.page.getByLabel('Amount', { exact: true }).fill('-12.34');
    await this.page.getByLabel('Transaction description').fill(description);
  }

  async save(awaitResponse: (response: Response) => boolean): Promise<Response> {
    const [response] = await Promise.all([
      this.page.waitForResponse(awaitResponse),
      this.page.getByRole('button', { name: 'Save transaction' }).click(),
    ]);
    expect(response.status(), 'save transaction request').toBe(200);
    await expect(this.page.getByRole('status').filter({ hasText: 'Transaction recorded.' })).toBeVisible();
    return response;
  }

  async expectExpenseAfterReload(account: Account, description: string): Promise<void> {
    // Pending charges from previous runs must not push this expense off the first page.
    await this.page.getByLabel('Transaction type filter').selectOption('expense');
    const entry = this.page.getByRole('article').filter({
      has: this.page.getByText(description, { exact: true }),
    });
    await expect(entry).toBeVisible();
    await this.page.reload();
    await expect(entry).toHaveCount(1);
    await expect(entry.getByText(account.name, { exact: true })).toBeVisible();
    await expect(entry.getByText('-12.34 USD', { exact: true })).toBeVisible();
    await expect(entry.getByText('expense', { exact: true })).toBeVisible();
  }
}
