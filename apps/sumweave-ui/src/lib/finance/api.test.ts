import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createSignalFinanceApi, createSignalFinanceApiForAuth, FinanceResponseError } from './api'

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
      { ok: true, json: { period: { preset: 'current_month', startDate: '2026-06-01T00:00:00-07:00', endDate: '2026-06-30T00:00:00-07:00', previous: { startDate: '2026-05-01T00:00:00-07:00', endDate: '2026-05-31T00:00:00-07:00' }, next: { startDate: '2026-07-01T00:00:00-07:00', endDate: '2026-07-31T00:00:00-07:00' } }, settled: { displayCurrency: 'USD', incomeMinor: 10, expenseMinor: 5, netMinor: 5, transactionCount: 1, complete: true }, pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 1, netMinor: -1, transactionCount: 1, complete: true }, categoryBreakdowns: [], accountBalances: [{ accountId: 'acc-1', accountName: 'Checking', currency: 'USD', nativeBookedMinor: 10, nativePendingMinor: 1, displayBookedMinor: null, displayPendingMinor: 0, missingFx: false }], alerts: [], fxCoverage: [{ provider: 'frankfurter', baseCurrency: 'EUR', quoteCurrency: 'USD', affectedTransactionCount: 3, affectedAccountCount: 2 }], currentFxRates: [{ provider: 'frankfurter', baseCurrency: 'EUR', quoteCurrency: 'USD', effectiveAt: '2026-06-20T00:00:00Z', lastSuccessfulRefreshAt: '2026-06-20T12:00:00Z', stale: false }], nativeSettledTotals: [] } },
      { ok: true, json: { items: [{ id: 'connection-1', tenantId: 'tenant-1', provider: 'monobank', displayName: 'Mono', providerReference: 'ref', state: 'active', createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z', schedule: { connectionId: 'connection-1', intervalSeconds: 900, enabled: true, createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' } }] } },
      { ok: true, json: { importId: 'import-1', importableCount: 1, headers: ['Date'], duplicateRows: [], rejectedRows: [], wouldCreateAccounts: ['Checking'], wouldCreateCategories: [], wouldCreateTags: [], accountOptions: [{ name: 'Checking', sourceRowCount: 1, selected: true }] } },
      { ok: true, json: { jobId: 'job-1', jobType: 'finance.fx_rates_refresh', provider: 'frankfurter' } },
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
    const preview = await api.previewCSVImport({ tenantId: 'tenant-1', fileName: 'demo.csv', csv: 'Date\n29.05.26' })
    const job = await api.triggerFXSync({ provider: 'frankfurter' })

    expect(tenants[0].joinedAt).toBeInstanceOf(Date)
    expect(dashboard.period.startDate).toEqual(new Date('2026-06-01T00:00:00-07:00'))
    expect(dashboard.accountBalances[0].displayBookedMinor).toBeNull()
    expect(dashboard.accountBalances[0].displayPendingMinor).toBe(0)
    expect(dashboard.accountBalances[0].missingFx).toBe(false)
    expect(dashboard.fxCoverage[0]).toMatchObject({ baseCurrency: 'EUR', quoteCurrency: 'USD', affectedTransactionCount: 3, affectedAccountCount: 2 })
    expect(dashboard.currentFxRates[0].lastSuccessfulRefreshAt).toEqual(new Date('2026-06-20T12:00:00Z'))
    expect(connections[0].schedule?.intervalSeconds).toBe(900)
    expect(preview.wouldCreateAccounts).toEqual(['Checking'])
    expect(preview.accountOptions).toEqual([{ name: 'Checking', sourceRowCount: 1, selected: true }])
    expect(preview.importableCount).toBe(1)
    expect(job.jobId).toBe('job-1')
  })

  it('preserves an explicit empty transaction-import account selection', async () => {
    let requestInit: RequestInit | undefined
    const fetch = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      requestInit = init
      return {
      ok: true,
      status: 200,
      statusText: 'OK',
      json: async () => ({ importId: 'import-none', importableCount: 0, headers: [], duplicateRows: [], rejectedRows: [], wouldCreateAccounts: [], wouldCreateCategories: [], wouldCreateTags: [], accountOptions: [{ name: 'Checking', sourceRowCount: 1, selected: false }] }),
      } as Response
    })
    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })

    const preview = await api.previewCSVImport({ tenantId: 'tenant-1', fileName: 'demo.csv', csv: 'Date', selectedAccountNames: [] })

    expect(preview.importableCount).toBe(0)
    expect(JSON.parse(requestInit?.body as string)).toMatchObject({ selectedAccountNames: [] })
  })

  it('submits a latest-only manual FX refresh without date-range fields', async () => {
    let requestInit: RequestInit | undefined
    const fetch = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      requestInit = init
    return { ok: true, status: 200, statusText: 'OK', json: async () => ({ jobId: 'job-fx', jobType: 'finance.fx_rates_refresh' }) } as Response
    })
    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })

    await api.triggerFXSync({ provider: 'frankfurter' })

    expect(JSON.parse(String(requestInit?.body))).toEqual({ provider: 'frankfurter' })
  })

  it('loads provider evidence metadata separately from an explicit sanitized detail reveal', async () => {
    const calls: Array<{ input: RequestInfo | URL; init?: RequestInit }> = []
    const responses = [
      { items: [{ id: 'evidence-1', scope: 'transaction', providerObjectId: 'provider-tx', capturedAt: '2026-06-20T12:00:00Z' }] },
      { id: 'evidence-1', scope: 'transaction', providerObjectId: 'provider-tx', capturedAt: '2026-06-20T12:00:00Z', payload: { amount: 'sanitized' } },
    ]
    const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      calls.push({ input, init })
      return { ok: true, status: 200, statusText: 'OK', json: async () => responses.shift() } as Response
    })
    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })

    const metadata = await api.listTransactionProviderEvidence({ tenantId: 'tenant-1', transactionId: 'tx-1' })
    expect(metadata[0].capturedAt).toEqual(new Date('2026-06-20T12:00:00Z'))
    expect(String(calls[0].input)).toContain('/transactions/tx-1/evidence')
    expect(String(calls[0].input)).not.toContain('evidence-1')

    const evidence = await api.getTransactionProviderEvidence({ tenantId: 'tenant-1', transactionId: 'tx-1', evidenceId: 'evidence-1' })
    expect(evidence.payload).toEqual({ amount: 'sanitized' })
    expect(String(calls[1].input)).toContain('/transactions/tx-1/evidence/evidence-1')
  })

  it('calls the transfer pair link and unlink APIs with the selected records', async () => {
    const calls: Array<{ input: RequestInfo | URL; init?: RequestInit }> = []
    const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      calls.push({ input, init })
      return { ok: true, status: 204, statusText: 'No Content', json: async () => undefined } as Response
    })
    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })
    const params = { tenantId: 'tenant-1', firstTransactionId: 'tx-out', secondTransactionId: 'tx-in' }

    await api.linkTransferPair(params)
    await api.unlinkTransferPair(params)

    expect(calls.map((call) => call.init?.method)).toEqual(['POST', 'DELETE'])
    expect(calls.map((call) => String(call.input))).toEqual([
      expect.stringContaining('/finance/tenants/tenant-1/transactions/transfer-links'),
      expect.stringContaining('/finance/tenants/tenant-1/transactions/transfer-links'),
    ])
    expect(JSON.parse(String(calls[0].init?.body))).toEqual({ firstTransactionId: 'tx-out', secondTransactionId: 'tx-in' })
  })

  it('loads transfer candidates and the exact matched partner with local-range instants', async () => {
    const calls: Array<{ input: RequestInfo | URL; init?: RequestInit }> = []
    const transaction = {
      id: 'tx-in', tenantId: 'tenant-1', accountId: 'account-2', source: 'provider', status: 'booked', kind: 'regular', amountMinor: 100,
      currency: 'USD', description: 'Transfer in', effectiveAt: '2026-06-20T12:00:00-07:00', tagIds: [], createdAt: '2026-06-20T12:00:00-07:00', updatedAt: '2026-06-20T12:00:00-07:00',
    }
    const responses = [{ items: [transaction] }, { ...transaction, kind: 'transfer', transferGroupId: 'group-1', transferMatchedAt: '2026-06-20T12:05:00-07:00' }]
    const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      calls.push({ input, init })
      return { ok: true, status: 200, statusText: 'OK', json: async () => responses.shift() } as Response
    })
    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })

    const candidates = await api.listTransferCandidates({
      tenantId: 'tenant-1', transactionId: 'tx-out', effectiveFrom: new Date('2026-06-18T00:00:00-07:00'), effectiveBefore: new Date('2026-06-23T00:00:00-07:00'), limit: 20, offset: 20,
    })
    const partner = await api.getTransferPartner({ tenantId: 'tenant-1', transactionId: 'tx-out' })

    const candidateUrl = new URL(String(calls[0].input))
    expect(candidateUrl.pathname).toBe('/api/v1/finance/tenants/tenant-1/transactions/tx-out/transfer-candidates')
    expect(candidateUrl.searchParams.get('limit')).toBe('20')
    expect(candidateUrl.searchParams.get('offset')).toBe('20')
    expect(candidateUrl.searchParams.get('effectiveFrom')).toBe('2026-06-18T07:00:00.000Z')
    expect(candidates.items[0].effectiveAt).toEqual(new Date('2026-06-20T12:00:00-07:00'))
    expect(new URL(String(calls[1].input)).pathname).toBe('/api/v1/finance/tenants/tenant-1/transactions/tx-out/transfer-partner')
    expect(partner.transferMatchedAt).toEqual(new Date('2026-06-20T12:05:00-07:00'))
  })

  it('calls the account evidence metadata and sanitized-detail APIs', async () => {
    const calls: Array<RequestInfo | URL> = []
    const responses = [
      { items: [{ id: 'evidence-1', scope: 'account', providerObjectId: 'provider-account', capturedAt: '2026-06-20T12:00:00Z' }] },
      { id: 'evidence-1', scope: 'account', providerObjectId: 'provider-account', capturedAt: '2026-06-20T12:00:00Z', payload: { balance: 'sanitized' } },
    ]
    const fetch = vi.fn(async (input: RequestInfo | URL) => {
      calls.push(input)
      return { ok: true, status: 200, statusText: 'OK', json: async () => responses.shift() } as Response
    })
    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })

    await api.listAccountProviderEvidence({ tenantId: 'tenant-1', accountId: 'account-1' })
    await api.getAccountProviderEvidence({ tenantId: 'tenant-1', accountId: 'account-1', evidenceId: 'evidence-1' })

    expect(String(calls[0])).toContain('/accounts/account-1/evidence')
    expect(String(calls[1])).toContain('/accounts/account-1/evidence/evidence-1')
  })

  it('gets and manages an account with the dedicated lifecycle APIs', async () => {
    const calls: Array<{ input: RequestInfo | URL; init?: RequestInit }> = []
    const account = { id: 'account-1', tenantId: 'tenant-1', name: 'Checking', currency: 'USD', kind: 'manual', bookedBalanceMinor: 0, pendingBalanceMinor: 0, hiddenAt: '2026-06-20T12:00:00Z', createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' }
    const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      calls.push({ input, init })
      return { ok: true, status: init?.method === 'GET' ? 200 : 204, statusText: 'OK', json: async () => init?.method === 'GET' ? account : undefined } as Response
    })
    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })

    const loaded = await api.getAccount({ tenantId: 'tenant 1', accountId: 'account 1' })
    await api.renameAccount({ tenantId: 'tenant 1', accountId: 'account 1', name: 'Everyday' })
    await api.hideAccount({ tenantId: 'tenant 1', accountId: 'account 1' })
    await api.restoreAccount({ tenantId: 'tenant 1', accountId: 'account 1' })

    expect(loaded.hiddenAt).toEqual(new Date('2026-06-20T12:00:00Z'))
    expect(calls.map((call) => [call.init?.method, new URL(String(call.input)).pathname])).toEqual([
      ['GET', '/api/v1/finance/tenants/tenant%201/accounts/account%201'],
      ['PATCH', '/api/v1/finance/tenants/tenant%201/accounts/account%201'],
      ['POST', '/api/v1/finance/tenants/tenant%201/accounts/account%201/hide'],
      ['POST', '/api/v1/finance/tenants/tenant%201/accounts/account%201/unhide'],
    ])
    expect(JSON.parse(String(calls[1].init?.body))).toEqual({ name: 'Everyday' })
  })

  it('updates categories and renames tags with their catalog-specific endpoints', async () => {
    const calls: Array<{ input: RequestInfo | URL; init?: RequestInit }> = []
    const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      calls.push({ input, init })
      return { ok: true, status: 204, statusText: 'No Content', json: async () => undefined } as Response
    })
    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })

    await api.updateCategory({ tenantId: 'tenant 1', categoryId: 'category 1', name: 'Food', kind: 'expense' })
    await api.renameTag({ tenantId: 'tenant 1', tagId: 'tag 1', name: 'Essential' })

    expect(calls.map((call) => [call.init?.method, new URL(String(call.input)).pathname])).toEqual([
      ['PATCH', '/api/v1/finance/tenants/tenant%201/categories/category%201'],
      ['PATCH', '/api/v1/finance/tenants/tenant%201/tags/tag%201'],
    ])
    expect(calls.map((call) => JSON.parse(String(call.init?.body)))).toEqual([
      { name: 'Food', kind: 'expense' },
      { name: 'Essential' },
    ])
  })

  it('renames a connection with its tenant-scoped endpoint', async () => {
    const calls: Array<{ input: RequestInfo | URL; init?: RequestInit }> = []
    const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      calls.push({ input, init })
      return { ok: true, status: 204, statusText: 'No Content', json: async () => undefined } as Response
    })
    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })

    await api.renameConnection({ tenantId: 'tenant 1', connectionId: 'connection 1', name: 'Joint checking' })

    expect(calls).toHaveLength(1)
    expect(calls[0].init?.method).toBe('PATCH')
    expect(new URL(String(calls[0].input)).pathname).toBe('/api/v1/finance/tenants/tenant%201/connections/connection%201')
    expect(JSON.parse(String(calls[0].init?.body))).toEqual({ name: 'Joint checking' })
  })

  it('rejects malformed or missing required dashboard timestamps and nested fields', async () => {
    const dashboard = {
      period: { preset: 'current_month', startDate: '2026-03-29T00:00:00+14:00', endDate: '2026-03-31', previous: { startDate: '2026-02-01', endDate: '2026-02-28' }, next: { startDate: '2026-04-01', endDate: '2026-04-30' } },
      settled: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: false },
      pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: false },
      categoryBreakdowns: [], accountBalances: [], alerts: [], fxCoverage: [], currentFxRates: [], nativeSettledTotals: [],
    }
    const malformedDateFetch = vi.fn(async () => ({ ok: true, status: 200, statusText: 'OK', json: async () => dashboard }) as Response)
    await expect(createSignalFinanceApi({ baseUrl: '/api/v1', fetch: malformedDateFetch }).getDashboard({ tenantId: 'tenant-1' })).rejects.toBeInstanceOf(FinanceResponseError)

    const incompleteSettled: Partial<typeof dashboard.settled> = { ...dashboard.settled }
    delete incompleteSettled.complete
    const missingNestedFieldFetch = vi.fn(async () => ({ ok: true, status: 200, statusText: 'OK', json: async () => ({ ...dashboard, period: { ...dashboard.period, startDate: '2026-03-29' }, settled: incompleteSettled }) }) as Response)
    await expect(createSignalFinanceApi({ baseUrl: '/api/v1', fetch: missingNestedFieldFetch }).getDashboard({ tenantId: 'tenant-1' })).rejects.toBeInstanceOf(FinanceResponseError)
  })

  it('raises a typed error from status metadata without parsing an error body', async () => {
    const fetch = vi.fn(async () => ({ ok: false, status: 400, statusText: 'Bad Request', json: async () => ({ message: 'bad finance request' }) }) as Response)
    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })
    await expect(api.listTenants()).rejects.toThrow('Bad Request')
    await expect(api.listTenants()).rejects.not.toThrow('bad finance request')
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
      { ok: true, json: { id: 'tx-1', tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'expense', amountMinor: 100, currency: 'USD', description: '', effectiveAt: '2026-06-20T12:00:00Z', tagIds: ['tag-1'], createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' } },
      { ok: true, json: { items: [{ id: 'connection-1', tenantId: 'tenant-1', provider: 'mono', displayName: 'Mono', providerReference: 'ref', state: 'active', createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' }] } },
      { ok: true, json: { importId: 'import-1', tenantId: 'tenant-1', status: 'completed', jobId: 'job-1', confirmedByUserId: 'user-1', importedCount: 1, rejectedRows: [], rowOutcomes: [], createdAt: '2026-06-20T12:00:00Z' } },
      { ok: true, json: { items: [{ importId: 'import-1', tenantId: 'tenant-1', status: 'completed', jobId: 'job-1', confirmedByUserId: 'user-1', importedCount: 1, rejectedRows: [], rowOutcomes: [], createdAt: '2026-06-20T12:00:00Z' }] } },
    ]
    const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      calls.push({ input, init })
      const next = responses.shift()
      return { ok: next?.ok ?? true, status: 200, statusText: 'OK', json: async () => next?.json } as Response
    })
    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })
    const created = await api.createTransaction({ tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'expense', amountMinor: 100, currency: 'USD', description: '', effectiveAt: new Date('2026-06-20T12:00:00Z'), tagIds: ['tag-1'] })
    const connections = await api.listConnections({ tenantId: 'tenant-1' })
    const audit = await api.getCSVImportAudit({ tenantId: 'tenant-1', importId: 'import-1' })
    const recentAudits = await api.listRecentCSVImportAudits({ tenantId: 'tenant-1' })

    expect(String(calls[0].init?.body)).toContain('2026-06-20T12:00:00.000Z')
    expect(created.tagIds).toEqual(['tag-1'])
    expect(String(calls[0].init?.body)).toContain('"tagIds":["tag-1"]')
    expect(connections[0].schedule).toBeUndefined()
    expect(audit.confirmedAt).toBeUndefined()
    expect(recentAudits).toHaveLength(1)
    expect(String(calls[3].input)).toContain('/finance/tenants/tenant-1/imports')
  })

  it('submits a create-transaction payload with accountId and exact minor amount', async () => {
    const calls: Array<{ input: RequestInfo | URL; init?: RequestInit }> = []
    const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      calls.push({ input, init })
      return {
        ok: true,
        status: 200,
        statusText: 'OK',
        json: async () => ({
          id: 'tx-1',
          tenantId: 'tenant-1',
          accountId: 'account-2',
          source: 'manual',
          status: 'booked',
          kind: 'expense',
          amountMinor: -55300,
          currency: 'USD',
          description: 'Coffee',
          effectiveAt: '2026-06-20T12:00:00Z',
          categoryId: 'cat-2',
          tagIds: ['tag-2'],
          createdAt: '2026-06-20T12:00:00Z',
          updatedAt: '2026-06-20T12:00:00Z',
        }),
      } as Response
    })

    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })

    await api.createTransaction({
      tenantId: 'tenant-1',
      accountId: 'account-2',
      source: 'manual',
      status: 'booked',
      kind: 'expense',
      amountMinor: -55300,
      currency: 'USD',
      description: 'Coffee',
      effectiveAt: new Date('2026-06-20T12:00:00Z'),
      categoryId: 'cat-2',
      tagIds: ['tag-2'],
    })

    expect(calls).toHaveLength(1)
    expect(String(calls[0].input)).toContain('/finance/tenants/tenant-1/transactions')
    expect(JSON.parse(String(calls[0].init?.body))).toMatchObject({
      accountId: 'account-2',
      amountMinor: -55300,
      source: 'manual',
      status: 'booked',
      kind: 'expense',
      currency: 'USD',
      description: 'Coffee',
      effectiveAt: '2026-06-20T12:00:00.000Z',
      categoryId: 'cat-2',
      tagIds: ['tag-2'],
    })
  })

  it('omits undefined connection sync windows and preserves supplied instants', async () => {
    const calls: RequestInit[] = []
    const fetch = vi.fn(async (...args: [RequestInfo | URL, RequestInit?]) => {
      calls.push(args[1] ?? {})
      return {
        ok: true,
        status: 200,
        statusText: 'OK',
        json: async () => ({ jobId: 'job-1', jobType: 'finance.bank_connection_sync' }),
      } as Response
    })
    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })
    const windowStart = new Date('2026-11-01T06:30:00Z')

    await api.triggerConnectionSync({ tenantId: 'tenant-1', connectionId: 'connection-1', reason: 'operator_ui' })
    await api.triggerConnectionSync({ tenantId: 'tenant-1', connectionId: 'connection-1', reason: 'operator_ui', windowStart })

    expect(JSON.parse(String(calls[0].body))).toEqual({ reason: 'operator_ui' })
    expect(JSON.parse(String(calls[1].body))).toEqual({
      reason: 'operator_ui',
      windowStart: '2026-11-01T06:30:00.000Z',
    })
    expect(fetch).toHaveBeenCalledTimes(2)
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

  it('uses status metadata for redirect start validation errors', async () => {
    const fetch = vi.fn(async () => ({
      ok: false,
      status: 400,
      statusText: 'Bad Request',
      json: async () => { throw new Error('error bodies must not be parsed') },
    }) as unknown as Response)

    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })

    await expect(
      api.startRedirectConnection({
        tenantId: 'tenant-1',
        provider: 'pko',
        callbackUrl: 'https://app.example.test/#/finance/other',
      }),
    ).rejects.toThrow('Bad Request')
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
    expect(new URL(String(calls[0].input)).pathname).toBe('/api/v1/finance/tenants/tenant-1/connections/synthetic-link-states/state/state-1')
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
          tagIds: ['tag-1'],
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
          tagIds: [],
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
      tagIds: [],
    })

    expect(transaction.providerOriginal?.description).toBe('Provider coffee')
    expect(transaction.providerOriginal?.effectiveAt).toBeInstanceOf(Date)
    expect(updated.providerOriginal).toBeUndefined()
    expect(calls[1].init?.method).toBe('PATCH')
		expect(String(calls[1].init?.body)).toContain('"clearCategory":true')
  })

	it('encodes unchanged, clear, and set category operations across transaction patches', async () => {
    const calls: Array<{ input: RequestInfo | URL; init?: RequestInit }> = []
    const transaction = {
      id: 'tx-1', tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'expense',
      amountMinor: 100, currency: 'USD', description: 'Coffee', effectiveAt: '2026-06-20T12:00:00Z',
      tagIds: [],
      createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z',
    }
    const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      calls.push({ input, init })
      return { ok: true, status: 200, statusText: 'OK', json: async () => transaction } as Response
    })
    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })
    const params = {
      tenantId: 'tenant-1', transactionId: 'tx-1', description: 'Coffee', amountMinor: 100,
      effectiveAt: new Date('2026-06-20T12:00:00Z'), tagIds: [],
    }

    await api.updateTransaction(params)
    await api.updateTransaction({ ...params, categoryId: null, tagIds: [] })
    await api.updateTransaction({ ...params, categoryId: 'category-1', tagIds: [] })

    expect(JSON.parse(String(calls[0].init?.body))).not.toHaveProperty('categoryId')
		expect(JSON.parse(String(calls[1].init?.body))).toMatchObject({ clearCategory: true })
    expect(JSON.parse(String(calls[2].init?.body))).toMatchObject({ categoryId: 'category-1' })
  })

  it('omits an optional transaction effective-at value and preserves a supplied instant', async () => {
    const calls: Array<{ input: RequestInfo | URL; init?: RequestInit }> = []
    const transaction = {
      id: 'tx-1', tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'expense',
      amountMinor: 100, currency: 'USD', description: 'Coffee', effectiveAt: '2026-06-20T12:00:00Z',
      tagIds: [],
      createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z',
    }
    const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      calls.push({ input, init })
      return { ok: true, status: 200, statusText: 'OK', json: async () => transaction } as Response
    })
    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })

    await api.updateTransaction({ tenantId: 'tenant-1', transactionId: 'tx-1', description: 'Coffee', amountMinor: 100, tagIds: [] })
    await api.updateTransaction({
      tenantId: 'tenant-1', transactionId: 'tx-1', description: 'Coffee', amountMinor: 100,
      effectiveAt: new Date('2026-11-01T06:30:00Z'), tagIds: [],
    })

    expect(JSON.parse(String(calls[0].init?.body))).not.toHaveProperty('effectiveAt')
    expect(JSON.parse(String(calls[1].init?.body))).toMatchObject({ effectiveAt: '2026-11-01T06:30:00.000Z' })
  })

  it('preserves zero balances and distinguishes omitted from nullable response timestamps', async () => {
    const fetch = vi.fn(async () => ({
      ok: true,
      status: 200,
      statusText: 'OK',
      json: async () => ({
        items: [
          {
            id: 'account-null', tenantId: 'tenant-1', name: 'Checking', currency: 'USD', kind: 'manual',
            bookedBalanceMinor: 0, pendingBalanceMinor: -125, hiddenAt: null,
            createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z',
          },
          {
            id: 'account-absent', tenantId: 'tenant-1', name: 'Savings', currency: 'EUR', kind: 'manual',
            bookedBalanceMinor: 25, pendingBalanceMinor: 0,
            createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z',
          },
        ],
      }),
    }) as Response)

    const accounts = await createSignalFinanceApi({ baseUrl: '/api/v1', fetch }).listAccounts({ tenantId: 'tenant-1' })

    expect(accounts[0]).toMatchObject({ bookedBalanceMinor: 0, pendingBalanceMinor: -125, hiddenAt: null })
    expect(accounts[1]).toMatchObject({ bookedBalanceMinor: 25, pendingBalanceMinor: 0 })
    expect(accounts[1].hiddenAt).toBeUndefined()
  })

  it('preserves a disabled schedule instead of defaulting false to an enabled state', async () => {
    const fetch = vi.fn(async () => ({
      ok: true,
      status: 200,
      statusText: 'OK',
      json: async () => ({
        items: [{
          id: 'connection-1', tenantId: 'tenant-1', provider: 'monobank', displayName: 'Mono',
          providerReference: 'ref', state: 'active',
          createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z',
          schedule: {
            connectionId: 'connection-1', intervalSeconds: 900, enabled: false,
            createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z',
          },
        }],
      }),
    }) as Response)

    const [connection] = await createSignalFinanceApi({ baseUrl: '/api/v1', fetch }).listConnections({ tenantId: 'tenant-1' })

    expect(connection.schedule?.enabled).toBe(false)
  })

  it('rejects missing required response collections instead of inventing an empty product state', async () => {
    const fetch = vi.fn(async () => ({
      ok: true,
      status: 200,
      statusText: 'OK',
      json: async () => ({}),
    }) as Response)

    await expect(createSignalFinanceApi({ baseUrl: '/api/v1', fetch }).listTenants()).rejects.toThrow(
      'Finance API response contract violation',
    )
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

  it('sends the required starter-catalog choice when creating a tenant', async () => {
    let requestInit: RequestInit | undefined
    const fetch = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      requestInit = init
      return {
        ok: true,
        status: 200,
        statusText: 'OK',
        json: async () => ({ id: 'tenant-1', name: 'Created', displayCurrency: 'USD', joinedAt: '2026-06-20T12:00:00Z', createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' }),
      } as Response
    })
    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })

    await api.createTenant({ name: 'Created', displayCurrency: 'USD', seedDefaults: false })

    expect(JSON.parse(String(requestInit?.body))).toEqual({ name: 'Created', displayCurrency: 'USD', seedDefaults: false })
  })

  it('archives a tenant with the no-content archive endpoint', async () => {
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

    await expect(api.archiveTenant({ tenantId: 'tenant 1' })).resolves.toBeUndefined()

    expect(new URL(String(calls[0].input)).pathname).toBe('/api/v1/finance/tenants/tenant%201/archive')
    expect(calls[0].init?.method).toBe('POST')
    expect(calls[0].init?.body).toBeUndefined()
  })

  it('covers the remaining finance endpoints while preserving omitted optional fields', async () => {
    const responses = [
      { id: 'tenant-1', name: 'Created', displayCurrency: 'USD', joinedAt: '2026-06-20T12:00:00Z', createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' },
      { id: 'tenant-1', name: 'Created', displayCurrency: 'USD', joinedAt: '2026-06-20T12:00:00Z', createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' },
      { items: [{ tenantId: 'tenant-1', userId: 'user-1', username: 'casey', joinedAt: '2026-06-20T12:00:00Z' }] },
      { items: [{ id: 'invite-1', tenantId: 'tenant-1', code: 'code-1', recipient: 'friend@example.com', createdByUserId: 'user-1', createdAt: '2026-06-20T12:00:00Z' }] },
      { id: 'invite-2', tenantId: 'tenant-1', code: 'code-2', recipient: 'team@example.com', createdByUserId: 'user-1', createdAt: '2026-06-20T12:00:00Z' },
      { tenantId: 'tenant-1', userId: 'user-2', joinedAt: '2026-06-20T12:00:00Z' },
      { items: [{ id: 'account-1', tenantId: 'tenant-1', name: 'Checking', currency: 'USD', kind: 'manual', bookedBalanceMinor: 0, pendingBalanceMinor: 0, createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' }] },
      { id: 'account-2', tenantId: 'tenant-1', name: 'Savings', currency: 'USD', kind: 'manual', bookedBalanceMinor: 0, pendingBalanceMinor: 0, createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' },
      { items: [{ id: 'cat-1', tenantId: 'tenant-1', name: 'Groceries', kind: 'expense', seededDefault: true, createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' }] },
      { id: 'cat-2', tenantId: 'tenant-1', name: 'Travel', kind: 'expense', seededDefault: false, createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' },
      { items: [{ id: 'tag-1', tenantId: 'tenant-1', name: 'Budget', createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' }] },
      { id: 'tag-2', tenantId: 'tenant-1', name: 'Holiday', createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' },
      { items: [{ id: 'tx-1', tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'expense', amountMinor: 100, currency: 'USD', description: 'Coffee', effectiveAt: '2026-06-20T12:00:00Z', tagIds: [], createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' }] },
      { id: 'connection-1', tenantId: 'tenant-1', provider: 'mono', displayName: 'Mono', providerReference: 'ref', state: 'active', createdAt: '2026-06-20T12:00:00Z', updatedAt: '2026-06-20T12:00:00Z' },
      undefined,
      { jobId: 'job-1', jobType: 'finance.bank_connection_sync' },
      { defaultProvider: 'frankfurter', storedRatesCount: 3, providers: [] },
      { importId: 'import-2', jobId: 'job-2', jobType: 'finance.csv_import' },
    ]
    const fetch = vi.fn(async () => {
      const next = responses.shift()
      return { ok: true, status: 200, statusText: 'OK', json: async () => next } as Response
    })
    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })

    expect((await api.createTenant({ name: 'Created', displayCurrency: 'USD', seedDefaults: true })).name).toBe('Created')
    expect((await api.getTenant({ tenantId: 'tenant-1' })).id).toBe('tenant-1')
    expect((await api.listTenantMembers({ tenantId: 'tenant-1' }))[0]).toMatchObject({ userId: 'user-1', username: 'casey' })
    expect((await api.listTenantInvites({ tenantId: 'tenant-1' }))[0].acceptedAt).toBeUndefined()
    expect((await api.createTenantInvite({ tenantId: 'tenant-1', recipient: 'team@example.com' })).recipient).toBe('team@example.com')
    expect((await api.acceptTenantInvite({ code: 'code-2' })).userId).toBe('user-2')
    expect((await api.listAccounts({ tenantId: 'tenant-1' }))[0].hiddenAt).toBeUndefined()
    expect((await api.createAccount({ tenantId: 'tenant-1', name: 'Savings', currency: 'USD', kind: 'manual' })).name).toBe('Savings')
    expect((await api.listCategories({ tenantId: 'tenant-1' }))[0].seededDefault).toBe(true)
    expect((await api.createCategory({ tenantId: 'tenant-1', name: 'Travel', kind: 'expense' })).name).toBe('Travel')
    expect((await api.listTags({ tenantId: 'tenant-1' }))[0].name).toBe('Budget')
    expect((await api.createTag({ tenantId: 'tenant-1', name: 'Holiday' })).name).toBe('Holiday')
    expect((await api.listTransactions({ tenantId: 'tenant-1', limit: 20 }))[0].description).toBe('Coffee')
    expect((await api.linkTokenConnection({ tenantId: 'tenant-1', provider: 'mono', token: 'secret' })).displayName).toBe('Mono')
    await expect(api.deleteConnection({ tenantId: 'tenant-1', connectionId: 'connection-1' })).resolves.toBeUndefined()
    expect((await api.triggerConnectionSync({ tenantId: 'tenant-1', connectionId: 'connection-1', reason: 'operator_ui' })).jobType).toBe('finance.bank_connection_sync')
    expect((await api.getFXDiagnostics()).providers).toEqual([])
    expect((await api.confirmCSVImport({ tenantId: 'tenant-1', importId: 'import-2' })).jobId).toBe('job-2')
  })

  it('builds an auth-backed finance api wrapper', async () => {
    const api = createSignalFinanceApiForAuth({ baseUrl: '/api/v1', authStore: { } as never })
    await expect(api.listTenants()).resolves.toEqual([])
  })

  it('rejects missing required csv preview collections', async () => {
    const fetch = vi.fn(async () => ({ ok: true, status: 200, statusText: 'OK', json: async () => ({ importId: 'import-3' }) }) as Response)
    const api = createSignalFinanceApi({ baseUrl: '/api/v1', fetch })
    await expect(api.previewCSVImport({ tenantId: 'tenant-1', fileName: 'demo.csv', csv: 'Date\n29.05.26' })).rejects.toBeInstanceOf(FinanceResponseError)
  })
})
