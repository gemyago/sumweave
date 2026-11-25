import { expect, type APIRequestContext, type APIResponse } from '@playwright/test';

type TenantInput = { name: string; displayCurrency: string; seedDefaults: boolean };
export type Tenant = Pick<TenantInput, 'name' | 'displayCurrency'> & { id: string };

type AccountInput = { name: string; kind: string; currency: string };
export type Account = AccountInput & {
  id: string;
  bookedBalanceMinor: number;
  pendingBalanceMinor: number;
};

type TransactionInput = {
  accountId: string;
  source: string;
  kind: string;
  status: string;
  amountMinor: number;
  currency: string;
  description: string;
  effectiveAt: string;
};
type Transaction = TransactionInput & { id: string };

const tenantsPath = '/api/v1/finance/tenants';

async function readJSON<T>(response: APIResponse): Promise<T> {
  expect(response.status(), response.url()).toBe(200);
  return response.json();
}

export class FinanceClient {
  constructor(private readonly request: APIRequestContext) {}

  async listTenants(): Promise<Tenant[]> {
    const { items } = await readJSON<{ items: Tenant[] }>(await this.request.get(tenantsPath));
    return items;
  }

  createTenant(data: TenantInput): Promise<Tenant> {
    return this.request.post(tenantsPath, { data }).then(readJSON<Tenant>);
  }

  tenantResponse(tenantId: string): Promise<APIResponse> {
    return this.request.get(`${tenantsPath}/${tenantId}`);
  }

  async getTenant(tenantId: string): Promise<Tenant> {
    return readJSON<Tenant>(await this.tenantResponse(tenantId));
  }

  archiveTenant(tenantId: string): Promise<APIResponse> {
    return this.request.post(`${tenantsPath}/${tenantId}/archive`);
  }

  async listAccounts(tenantId: string): Promise<Account[]> {
    const response = await this.request.get(`${tenantsPath}/${tenantId}/accounts`);
    const { items } = await readJSON<{ items: Account[] }>(response);
    return items;
  }

  async createAccount(tenantId: string, data: AccountInput): Promise<Account> {
    return readJSON<Account>(await this.request.post(`${tenantsPath}/${tenantId}/accounts`, { data }));
  }

  async getAccount(tenantId: string, accountId: string): Promise<Account> {
    return readJSON<Account>(await this.request.get(`${tenantsPath}/${tenantId}/accounts/${accountId}`));
  }

  async createTransaction(tenantId: string, data: TransactionInput): Promise<Transaction> {
    return readJSON<Transaction>(await this.request.post(`${tenantsPath}/${tenantId}/transactions`, { data }));
  }

  async getTransaction(tenantId: string, transactionId: string): Promise<Transaction> {
    return readJSON<Transaction>(await this.request.get(`${tenantsPath}/${tenantId}/transactions/${transactionId}`));
  }
}
