import { expect } from '@playwright/test';
import type { Account, FinanceClient, Tenant } from '../clients/finance';
import type { FinancePage } from '../pages/finance';
import type { TransactionsPage } from '../pages/transactions';

export class RecordExpenseFlow {
  constructor(
    private readonly financePage: FinancePage,
    private readonly transactionsPage: TransactionsPage,
    private readonly financeClient: FinanceClient,
  ) {}

  async recordExpense(tenant: Tenant, account: Account, description: string): Promise<string> {
    await this.financePage.openTransactions();
    await this.transactionsPage.fillBookedExpense(account, description);
    const response = await this.transactionsPage.save((response) =>
      response.request().method() === 'POST' &&
      new URL(response.url()).pathname === `/api/v1/finance/tenants/${tenant.id}/transactions`,
    );
    const { id }: { id: string } = await response.json();
    expect(id, 'recorded transaction ID').toBeTruthy();
    return id;
  }

  async expectPersistedExpense(
    fixture: { tenant: Tenant; account: Account },
    transaction: { id: string; description: string },
  ): Promise<void> {
    await this.financePage.openTransactions();
    await this.transactionsPage.expectExpenseAfterReload(fixture.account, transaction.description);
    expect(await this.financeClient.getTransaction(fixture.tenant.id, transaction.id)).toMatchObject({
      id: transaction.id,
      accountId: fixture.account.id,
      description: transaction.description,
      amountMinor: -1234,
      currency: 'USD',
      status: 'booked',
      kind: 'expense',
      source: 'manual',
    });
  }
}
