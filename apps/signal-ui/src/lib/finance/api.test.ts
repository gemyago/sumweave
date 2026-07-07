import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createSignalFinanceApi, createSignalFinanceApiForAuth, FinanceApiError } from './api'

vi.mock('../auth/auth-fetch', () => ({
  createAuthFetch: vi.fn(() => vi.fn(async () => ({ ok: true, status: 200, statusText: 'OK', json: async () => ({ items: [] }) }) as Response)),
}))

describe('finance api', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('maps tenant, dashboard, connection, import, and fx responses', async () => {
    const responses = [
      { ok: true, json: { items: [{ id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: '2026-06-20T12:00:00Z', createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' }] } },
      { ok: true, json: { period: { preset: 'current_month', startDate: '2026-06-01T00:00:00Z', endDate: '2026-06-30T00:00:00Z', previous: { startDate: '2026-05-01T00:00:00Z', endDate: '2026-05-31T00:00:00Z' }, next: { startDate: '2026-07-01T00:00:00Z', endDate: '2026-07-31T00:00:00Z' } }, settled: { displayCurrency: 'USD', incomeMinor: 10, expenseMinor: 5, netMinor: 5, transactionCount: 1, complete: true }, pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 1, netMinor: -1, transactionCount: 1, complete: true }, categoryBreakdowns: [], accountBalances: [{ accountId: 'acc-1', accountName: 'Checking', currency: 'USD', nativeBookedMinor: 10, nativePendingMinor: 1, missingFx: false }], alerts: [], missingFx: [{ source: 'provider', transactionId: 'tx-1', accountId: 'acc-1', baseCurrency: 'EUR', quoteCurrency: 'USD', rateDate: '2026-06-20T00:00:00Z', provider: 'frankfurter' }], nativeSettledTotals: [] } },
      { ok: true, json: { items: [{ id: 'connection-1', tenantId: 'tenant-1', provider: 'monobank', displayName: 'Mono', providerReference: 'ref', externalId: 'ext', state: 'active', createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z', schedule: { connectionId: 'connection-1', intervalSeconds: 900, enabled: true, createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' } }] } },
      { ok: true, json: { importId: 'import-1', importType: 'transactions', headers: ['account'], mapping: { account: 'accountName' }, duplicateRows: [], rejectedRows: [], wouldCreateAccounts: ['Checking'], wouldCreateCategories: [], wouldCreateTags: [] } },
      { ok: true, json: { jobId: 'job-1', jobType: 'finance.fx_rates_sync', provider: 'frankfurter' } },
    ]
    const fetch = vi.fn(async () => {
      const next = responses.shift()
      return {
        ok: next?.ok ?? true,
        status: 200,
        statusText: 'OK',
        json: async () => next?.json,
      } as Response
    })

    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })
    const tenants = await api.listTenants()
    const dashboard = await api.getDashboard({ tenantId: 'tenant-1', preset: 'current_month' })
    const connections = await api.listConnections({ tenantId: 'tenant-1' })
    const preview = await api.previewCSVImport({ tenantId: 'tenant-1', importType: 'transactions', fileName: 'demo.csv', csv: 'account\nChecking' })
    const job = await api.triggerFXSync({ provider: 'frankfurter', baseCurrencies: ['USD'], quoteCurrency: 'PLN', startDate: new Date('2026-06-01T00:00:00Z'), endDate: new Date('2026-06-30T00:00:00Z') })

    expect(tenants[0].joinedAt).toBeInstanceOf(Date)
    expect(dashboard.accountBalances[0].displayBookedMinor).toBeNull()
    expect(dashboard.missingFx[0].rateDate).toBeInstanceOf(Date)
    expect(connections[0].schedule?.intervalSeconds).toBe(900)
    expect(preview.wouldCreateAccounts).toEqual(['Checking'])
    expect(job.jobId).toBe('job-1')
  })

  it('raises a typed error for failed responses', async () => {
    const fetch = vi.fn(async () => ({ ok: false, status: 400, statusText: 'Bad Request', json: async () => ({ message: 'bad finance request' }) }) as Response)
    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })
    await expect(api.listTenants()).rejects.toBeInstanceOf(FinanceApiError)
  })

  it('falls back to response status text or generic request failed message', async () => {
    const statusTextFetch = vi.fn(async () => ({ ok: false, status: 500, statusText: 'Server exploded', json: async () => ({}) }) as Response)
    const genericFetch = vi.fn(async () => ({ ok: false, status: 500, statusText: '', json: async () => undefined }) as Response)
    await expect(createSignalFinanceApi({ baseUrl: '/api/v1', fetch: statusTextFetch }).listTenants()).rejects.toThrow('Server exploded')
    await expect(createSignalFinanceApi({ baseUrl: '/api/v1', fetch: genericFetch }).listTenants()).rejects.toThrow('Request failed')
  })

  it('serializes dates and maps optional audit or connection fields', async () => {
    const calls: Array<{ input: RequestInfo | URL; init?: RequestInit }> = []
    const responses = [
      { ok: true, json: { id: 'tx-1', tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'expense', amountMinor: 100, currency: 'USD', description: '', effectiveAt: '2026-06-20T12:00:00Z', createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' } },
      { ok: true, json: { items: [{ id: 'connection-1', tenantId: 'tenant-1', provider: 'mono', displayName: 'Mono', providerReference: 'ref', externalId: 'ext', state: 'active', createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' }] } },
      { ok: true, json: { importId: 'import-1', tenantId: 'tenant-1', importType: 'transactions', status: 'completed', jobId: 'job-1', confirmedByUserId: 'user-1', importedCount: 1, createdAt: '2026-06-20T12:00:00Z' } },
    ]
    const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      calls.push({ input, init })
      const next = responses.shift()
      return { ok: next?.ok ?? true, status: 200, statusText: 'OK', json: async () => next?.json } as Response
    })
    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })
    await api.createTransaction({ tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'expense', amountMinor: 100, currency: 'USD', description: '', effectiveAt: new Date('2026-06-20T12:00:00Z') })
    const connections = await api.listConnections({ tenantId: 'tenant-1' })
    const audit = await api.getCSVImportAudit({ tenantId: 'tenant-1', importId: 'import-1' })

    expect(String(calls[0].init?.body)).toContain('2026-06-20T12:00:00.000Z')
    expect(connections[0].schedule).toBeNull()
    expect(audit.confirmedAt).toBeNull()
  })

  it('starts and finishes PKO redirect linking with camelCase payloads', async () => {
    const calls: Array<{ input: RequestInfo | URL; init?: RequestInit }> = []
    const responses = [
      {
        ok: true,
        json: {
          provider: 'pko',
          authorizationUrl: 'https://bank.example/authorize',
          state: 'state-1',
        },
      },
      {
        ok: true,
        json: {
          id: 'connection-1',
          tenantId: 'tenant-1',
          provider: 'pko',
          displayName: 'PKO',
          providerReference: 'ref',
          externalId: 'ext',
          state: 'active',
          lastSyncStartedAt: '2026-06-20T12:00:00Z',
          lastSuccessfulSyncAt: '2026-06-20T12:05:00Z',
          createdAt: '2026-06-20T12:00:00Z',
          updatedAt: '2026-06-20T12:06:00Z',
        },
      },
    ]
    const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      calls.push({ input, init })
      const next = responses.shift()
      return {
        ok: next?.ok ?? true,
        status: 200,
        statusText: 'OK',
        json: async () => next?.json,
      } as Response
    })

    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })
    const started = await api.startRedirectConnection({
      tenantId: 'tenant-1',
      provider: 'pko',
      callbackUrl: 'https://app.example.test/#/finance/connections',
    })
    const finished = await api.finishRedirectConnection({
      tenantId: 'tenant-1',
      provider: 'pko',
      code: 'code-1',
      state: 'state-1',
    })

    const startUrl = new URL(String(calls[0].input))
    const finishUrl = new URL(String(calls[1].input))
    expect(startUrl.pathname).toBe('/api/v1/finance/tenants/tenant-1/connections/link-redirect/start')
    expect(String(calls[0].init?.body)).toBe('{"provider":"pko","callbackUrl":"https://app.example.test/#/finance/connections"}')
    expect(started).toEqual({
      provider: 'pko',
      authorizationUrl: 'https://bank.example/authorize',
      state: 'state-1',
    })
    expect(finishUrl.pathname).toBe('/api/v1/finance/tenants/tenant-1/connections/link-redirect/finish')
    expect(String(calls[1].init?.body)).toBe('{"provider":"pko","code":"code-1","state":"state-1"}')
    expect(finished.lastSyncStartedAt).toBeInstanceOf(Date)
    expect(finished.lastSuccessfulSyncAt).toBeInstanceOf(Date)
  })

  it('propagates redirect start validation errors', async () => {
    const fetch = vi.fn(async () => ({
      ok: false,
      status: 400,
      statusText: 'Bad Request',
      json: async () => ({ message: 'callbackUrl must target /#/finance/connections' }),
    }) as Response)

    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })

    await expect(
      api.startRedirectConnection({
        tenantId: 'tenant-1',
        provider: 'pko',
        callbackUrl: 'https://app.example.test/#/finance/other',
      }),
    ).rejects.toThrow('callbackUrl must target /#/finance/connections')
  })

  it('loads and saves synthetic link state while preserving distinct duplicate configured accounts', async () => {
    const calls: Array<{ input: RequestInfo | URL; init?: RequestInit }> = []
    const responses = [
      {
        ok: true,
        json: {
          provider: 'synthetic',
          state: 'state-1',
          configuredAccounts: [
            { key: 'dup-1', name: 'Cash', currency: 'USD' },
            { key: 'dup-2', name: 'Cash', currency: 'USD' },
          ],
          canFinish: true,
        },
      },
      {
        ok: true,
        json: {
          provider: 'synthetic',
          state: 'state-1',
          configuredAccounts: [
            { key: 'dup-1', name: 'Cash', currency: 'USD' },
            { key: 'dup-3', name: 'Brokerage', currency: 'EUR' },
          ],
          canFinish: true,
        },
      },
    ]
    const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      calls.push({ input, init })
      const next = responses.shift()
      return {
        ok: next?.ok ?? true,
        status: 200,
        statusText: 'OK',
        json: async () => next?.json,
      } as Response
    })

    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })
    const loaded = await api.getSyntheticLinkState({ tenantId: 'tenant-1', state: 'state-1' })
    const saved = await api.saveSyntheticLinkState({
      tenantId: 'tenant-1',
      state: 'state-1',
      configuredAccounts: [
        { key: 'dup-1', name: 'Cash', currency: 'USD' },
        { name: 'Brokerage', currency: 'EUR' },
      ],
    })

    expect(loaded.configuredAccounts).toHaveLength(2)
    expect(loaded.configuredAccounts[0].key).toBe('dup-1')
    expect(loaded.configuredAccounts[1].key).toBe('dup-2')
    expect(saved.configuredAccounts[1].key).toBe('dup-3')
    expect(new URL(String(calls[0].input)).pathname).toBe('/api/v1/finance/tenants/tenant-1/connections/synthetic-link-states/state-1')
    expect(String(calls[1].init?.body)).toBe('{"configuredAccounts":[{"key":"dup-1","name":"Cash","currency":"USD"},{"name":"Brokerage","currency":"EUR"}]}')
  })

  it('maps provider-original detail responses and submits nullable transaction patch payloads', async () => {
    const calls: Array<{ input: RequestInfo | URL; init?: RequestInit }> = []
    const responses = [
      {
        ok: true,
        json: {
          id: 'tx-1',
          tenantId: 'tenant-1',
          accountId: 'account-1',
          source: 'provider',
          status: 'booked',
          kind: 'expense',
          amountMinor: 100,
          currency: 'USD',
          description: 'Coffee',
          effectiveAt: '2026-06-20T12:00:00Z',
          providerOriginal: {
            amountMinor: 120,
            currency: 'USD',
            description: 'Provider coffee',
            effectiveAt: '2026-06-19T12:00:00Z',
          },
          createdAt: '2026-06-20T12:00:00Z',
          updatedAt: '2026-06-20T12:00:00Z',
        },
      },
      {
        ok: true,
        json: {
          id: 'tx-1',
          tenantId: 'tenant-1',
          accountId: 'account-1',
          source: 'provider',
          status: 'booked',
          kind: 'expense',
          amountMinor: 95,
          currency: 'USD',
          description: 'Coffee updated',
          effectiveAt: '2026-06-21T12:00:00Z',
          createdAt: '2026-06-20T12:00:00Z',
          updatedAt: '2026-06-21T12:05:00Z',
        },
      },
    ]
    const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      calls.push({ input, init })
      const next = responses.shift()
      return {
        ok: next?.ok ?? true,
        status: 200,
        statusText: 'OK',
        json: async () => next?.json,
      } as Response
    })

    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })
    const transaction = await api.getTransaction({ tenantId: 'tenant-1', transactionId: 'tx-1' })
    const updated = await api.updateTransaction({
      tenantId: 'tenant-1',
      transactionId: 'tx-1',
      description: 'Coffee updated',
      amountMinor: 95,
      effectiveAt: new Date('2026-06-21T12:00:00Z'),
      categoryId: null,
    })

    expect(transaction.providerOriginal?.description).toBe('Provider coffee')
    expect(transaction.providerOriginal?.effectiveAt).toBeInstanceOf(Date)
    expect(updated.providerOriginal).toBeNull()
    expect(calls[1].init?.method).toBe('PATCH')
    expect(String(calls[1].init?.body)).toContain('"categoryId":null')
  })

  it('updates tenants with the patch endpoint and expects no response body', async () => {
    const calls: Array<{ input: RequestInfo | URL; init?: RequestInit }> = []
    const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      calls.push({ input, init })
      return {
        ok: true,
        status: 204,
        statusText: 'No Content',
        json: async () => {
          throw new Error('no content')
        },
      } as unknown as Response
    })

    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })
    await expect(
      api.updateTenant({ tenantId: 'tenant-1', name: 'Household Updated', displayCurrency: 'PLN' }),
    ).resolves.toBeUndefined()

    expect(new URL(String(calls[0].input)).pathname).toBe('/api/v1/finance/tenants/tenant-1')
    expect(calls[0].init?.method).toBe('PATCH')
    expect(String(calls[0].init?.body)).toBe('{"name":"Household Updated","displayCurrency":"PLN"}')
  })

  it('covers the remaining finance endpoints with defaulted optional fields', async () => {
    const responses = [
      { id: 'tenant-1', name: 'Created', displayCurrency: 'USD', joinedAt: '2026-06-20T12:00:00Z', createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' },
      { id: 'tenant-1', name: 'Created', displayCurrency: 'USD', joinedAt: '2026-06-20T12:00:00Z', createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' },
      { items: [{ tenantId: 'tenant-1', userId: 'user-1', joinedAt: '2026-06-20T12:00:00Z' }] },
      { items: [{ id: 'invite-1', tenantId: 'tenant-1', code: 'code-1', recipient: 'friend@example.com', createdByUserId: 'user-1', createdAt: '2026-06-20T12:00:00Z' }] },
      { id: 'invite-2', tenantId: 'tenant-1', code: 'code-2', recipient: 'team@example.com', createdByUserId: 'user-1', createdAt: '2026-06-20T12:00:00Z' },
      { tenantId: 'tenant-1', userId: 'user-2', joinedAt: '2026-06-20T12:00:00Z' },
      { items: [{ id: 'account-1', tenantId: 'tenant-1', name: 'Checking', currency: 'USD', kind: 'manual', createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' }] },
      { id: 'account-2', tenantId: 'tenant-1', name: 'Savings', currency: 'USD', kind: 'manual', createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' },
      { items: [{ id: 'cat-1', tenantId: 'tenant-1', name: 'Groceries', kind: 'expense', seededDefault: true, createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' }] },
      { id: 'cat-2', tenantId: 'tenant-1', name: 'Travel', kind: 'expense', seededDefault: false, createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' },
      { items: [{ id: 'tag-1', tenantId: 'tenant-1', name: 'Budget', createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' }] },
      { id: 'tag-2', tenantId: 'tenant-1', name: 'Holiday', createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' },
      { items: [{ id: 'tx-1', tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'expense', amountMinor: 100, currency: 'USD', description: 'Coffee', effectiveAt: '2026-06-20T12:00:00Z', createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' }] },
      { id: 'connection-1', tenantId: 'tenant-1', provider: 'mono', displayName: 'Mono', providerReference: 'ref', externalId: 'ext', state: 'active', createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' },
      undefined,
      { jobId: 'job-1', jobType: 'finance.bank_connection_sync' },
      { defaultProvider: 'frankfurter', storedRatesCount: 3 },
      { importId: 'import-2', jobId: 'job-2', jobType: 'finance.csv_import' },
    ]
    const fetch = vi.fn(async () => {
      const next = responses.shift()
      return { ok: true, status: 200, statusText: 'OK', json: async () => next } as Response
    })
    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })

    expect((await api.createTenant({ name: 'Created', displayCurrency: 'USD' })).name).toBe('Created')
    expect((await api.getTenant({ tenantId: 'tenant-1' })).id).toBe('tenant-1')
    expect((await api.listTenantMembers({ tenantId: 'tenant-1' }))[0].userId).toBe('user-1')
    expect((await api.listTenantInvites({ tenantId: 'tenant-1' }))[0].acceptedAt).toBeNull()
    expect((await api.createTenantInvite({ tenantId: 'tenant-1', recipient: 'team@example.com' })).recipient).toBe('team@example.com')
    expect((await api.acceptTenantInvite({ code: 'code-2' })).userId).toBe('user-2')
    expect((await api.listAccounts({ tenantId: 'tenant-1' }))[0].hiddenAt).toBeNull()
    expect((await api.createAccount({ tenantId: 'tenant-1', name: 'Savings', currency: 'USD', kind: 'manual' })).name).toBe('Savings')
    expect((await api.listCategories({ tenantId: 'tenant-1' }))[0].seededDefault).toBe(true)
    expect((await api.createCategory({ tenantId: 'tenant-1', name: 'Travel', kind: 'expense' })).name).toBe('Travel')
    expect((await api.listTags({ tenantId: 'tenant-1' }))[0].name).toBe('Budget')
    expect((await api.createTag({ tenantId: 'tenant-1', name: 'Holiday' })).name).toBe('Holiday')
    expect((await api.listTransactions({ tenantId: 'tenant-1' }))[0].description).toBe('Coffee')
    expect((await api.linkTokenConnection({ tenantId: 'tenant-1', provider: 'mono', token: 'secret' })).displayName).toBe('Mono')
    await expect(api.deleteConnection({ tenantId: 'tenant-1', connectionId: 'connection-1' })).resolves.toBeUndefined()
    expect((await api.triggerConnectionSync({ tenantId: 'tenant-1', connectionId: 'connection-1', reason: 'operator_ui' })).jobType).toBe('finance.bank_connection_sync')
    expect((await api.getFXDiagnostics()).providers).toEqual([])
    expect((await api.confirmCSVImport({ tenantId: 'tenant-1', importId: 'import-2', mapping: { account: 'accountName' } })).jobId).toBe('job-2')
  })

  it('builds an auth-backed finance api wrapper', async () => {
    const api = createSignalFinanceApiForAuth({ baseUrl: '/api/v1', authStore: { } as never })
    await expect(api.listTenants()).resolves.toEqual([])
  })

  it('defaults missing csv preview collections and mappings', async () => {
    const fetch = vi.fn(async () => ({ ok: true, status: 200, statusText: 'OK', json: async () => ({ importId: 'import-3', importType: 'transactions' }) }) as Response)
    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })
    const preview = await api.previewCSVImport({ tenantId: 'tenant-1', importType: 'transactions', fileName: 'demo.csv', csv: 'account\nChecking' })
    expect(preview.headers).toEqual([])
    expect(preview.mapping).toEqual({})
    expect(preview.duplicateRows).toEqual([])
    expect(preview.rejectedRows).toEqual([])
    expect(preview.wouldCreateAccounts).toEqual([])
    expect(preview.wouldCreateCategories).toEqual([])
    expect(preview.wouldCreateTags).toEqual([])
  })
})
