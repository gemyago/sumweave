import { test as financeTest } from './finance';
import { RecordExpenseFlow } from '../flows/record-expense';
import { FinancePage } from '../pages/finance';
import { LoginPage } from '../pages/login';
import { TransactionsPage } from '../pages/transactions';

export const test = financeTest.extend<{
  loginPage: LoginPage;
  financePage: FinancePage;
  recordExpense: RecordExpenseFlow;
}>({
  loginPage: async ({ page }, use) => {
    await use(new LoginPage(page));
  },
  financePage: async ({ page }, use) => {
    await use(new FinancePage(page));
  },
  recordExpense: async ({ page, financePage, ownerFinance }, use) => {
    await use(new RecordExpenseFlow(financePage, new TransactionsPage(page), ownerFinance));
  },
});
