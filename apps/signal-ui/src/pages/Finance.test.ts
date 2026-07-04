import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import Finance from './Finance.svelte'

const mocks = vi.hoisted(() => ({
  listTenants: vi.fn(),
  getDashboard: vi.fn(),
  listTransactions: vi.fn(),
  listConnections: vi.fn(),
}))

vi.mock('../lib/finance/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/finance/api')>()
    return {
      ...actual,
      createSignalFinanceApiForAuth: vi.fn(() => ({
        listTenants: mocks.listTenants,
        getDashboard: mocks.getDashboard,
        listTransactions: mocks.listTransactions,
        listConnections: mocks.listConnections,
      })),
    }
  })

vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

describe('Finance dashboard page', () => {
  beforeEach(() => {
    window.localStorage.clear()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants.mockReset()
    mocks.getDashboard.mockReset()
    mocks.listTransactions.mockReset()
    mocks.listConnections.mockReset()
    mocks.listTenants.mockResolvedValue([{ id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now }])
    mocks.getDashboard.mockResolvedValue({
      period: { preset: 'current_month', startDate: now, endDate: now, previous: { startDate: now, endDate: now }, next: { startDate: now, endDate: now } },
      settled: { displayCurrency: 'USD', incomeMinor: 120000, expenseMinor: 45000, netMinor: 75000, transactionCount: 12, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 5000, netMinor: -5000, transactionCount: 1, complete: true },
      categoryBreakdowns: [{ categoryId: 'cat-1', categoryName: 'Groceries', kind: 'expense', incomeMinor: 0, expenseMinor: 1000, transactionCount: 1 }],
      accountBalances: [{ accountId: 'acc-1', accountName: 'Checking', currency: 'USD', nativeBookedMinor: 50000, nativePendingMinor: 5000, displayBookedMinor: 50000, displayPendingMinor: 5000, missingFx: false }],
      alerts: [{ code: 'stale_connection', severity: 'warning', count: 1 }],
      missingFx: [{ source: 'provider', transactionId: 'tx-1', accountId: 'acc-1', baseCurrency: 'EUR', quoteCurrency: 'USD', rateDate: now, provider: 'frankfurter' }],
      nativeSettledTotals: [],
    })
    mocks.listTransactions.mockResolvedValue([
      {
        id: 'tx-1',
        tenantId: 'tenant-1',
        accountId: 'acc-1',
        source: 'provider',
        status: 'booked',
        kind: 'expense',
        amountMinor: -4500,
        currency: 'USD',
        description: 'Groceries',
        effectiveAt: now,
        categoryId: 'cat-1',
        transferGroupId: null,
        transferMatchedAt: null,
        hiddenAt: null,
        providerOriginal: null,
        createdAt: now,
        updatedAt: now,
      },
    ])
    mocks.listConnections.mockResolvedValue([
      {
        id: 'conn-1',
        tenantId: 'tenant-1',
        provider: 'synthetic',
        displayName: 'Primary sync',
        providerReference: 'ref-1',
        externalId: 'ext-1',
        state: 'ready',
        lastSyncJobId: 'job-1',
        lastSyncStartedAt: now,
        lastSuccessfulSyncAt: now,
        lastSyncError: '',
        createdAt: now,
        updatedAt: now,
        schedule: {
          connectionId: 'conn-1',
          intervalSeconds: 3600,
          nextRunAt: now,
          lastScheduledAt: now,
          lastStartedAt: now,
          lastCompletedAt: now,
          lastJobId: 'job-1',
          enabled: true,
          createdAt: now,
          updatedAt: now,
        },
      },
    ])
  })

  it('loads the tenant-aware dashboard with secondary attention links', async () => {
    render(Finance)

    expect(await screen.findByRole('heading', { name: 'Finance' })).toBeInTheDocument()
    expect(await screen.findByRole('heading', { name: 'Booked balance' })).toBeInTheDocument()
    expect(screen.getByText('Income')).toBeInTheDocument()
    expect(screen.getByText('Expense')).toBeInTheDocument()
    expect(screen.getByText('Pending delta')).toBeInTheDocument()
    expect(await screen.findByText('Preset: current_month')).toBeInTheDocument()
    expect(await screen.findByText('Stale connection')).toBeInTheDocument()
    expect(screen.getByText('Cash flow chart')).toBeInTheDocument()
    expect(screen.getByText('Account balances chart')).toBeInTheDocument()
    expect(screen.getByText('Category breakdown chart')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Recent transactions' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Needs attention' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Connections' })).toBeInTheDocument()
    expect(screen.getAllByText('Groceries').length).toBeGreaterThan(0)
    expect(screen.getByText('Primary sync')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Review in admin FX diagnostics' })).toHaveAttribute('href', '#/admin/finance/fx')
    expect(screen.queryByRole('link', { name: 'Open FX diagnostics' })).not.toBeInTheDocument()
    expect(screen.getByLabelText('Custom start date')).not.toBeVisible()
    expect(screen.getByLabelText('Custom end date')).not.toBeVisible()
    expect(screen.queryByText('2026-06-20T12:00:00.000Z')).not.toBeInTheDocument()
  })

  it('renders compact needs-attention items for pending, missing FX, failed sync, and failed import signals', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getDashboard.mockResolvedValueOnce({
      period: { preset: 'current_month', startDate: now, endDate: now, previous: { startDate: now, endDate: now }, next: { startDate: now, endDate: now } },
      settled: { displayCurrency: 'USD', incomeMinor: 120000, expenseMinor: 45000, netMinor: 75000, transactionCount: 12, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 5000, netMinor: -5000, transactionCount: 3, complete: true },
      categoryBreakdowns: [{ categoryId: 'cat-1', categoryName: 'Groceries', kind: 'expense', incomeMinor: 0, expenseMinor: 1000, transactionCount: 1 }],
      accountBalances: [{ accountId: 'acc-1', accountName: 'Checking', currency: 'USD', nativeBookedMinor: 50000, nativePendingMinor: 5000, displayBookedMinor: 50000, displayPendingMinor: 5000, missingFx: false }],
      alerts: [{ code: 'failed_import', severity: 'error', count: 2 }],
      missingFx: [{ source: 'provider', transactionId: 'tx-1', accountId: 'acc-1', baseCurrency: 'EUR', quoteCurrency: 'USD', rateDate: now, provider: 'frankfurter' }],
      nativeSettledTotals: [],
    })
    mocks.listConnections.mockResolvedValueOnce([
      {
        id: 'conn-2',
        tenantId: 'tenant-1',
        provider: 'synthetic',
        displayName: 'Broken sync',
        providerReference: 'ref-2',
        externalId: 'ext-2',
        state: 'attention',
        lastSyncJobId: 'job-2',
        lastSyncStartedAt: now,
        lastSuccessfulSyncAt: null,
        lastSyncError: 'token expired',
        createdAt: now,
        updatedAt: now,
        schedule: null,
      },
    ])

    render(Finance)

    expect(await screen.findByRole('heading', { name: 'Needs attention' })).toBeInTheDocument()
    expect(screen.getByText('Pending transactions')).toBeInTheDocument()
    expect(screen.getByText('Missing FX coverage')).toBeInTheDocument()
    expect(screen.getByText('Failed sync')).toBeInTheDocument()
    expect(screen.getByText('Failed import')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Review transactions' })).toHaveAttribute('href', '#/finance/transactions')
    expect(screen.getByRole('link', { name: 'Review connections' })).toHaveAttribute('href', '#/finance/connections')
    expect(screen.getByRole('link', { name: 'Review in admin FX diagnostics' })).toHaveAttribute('href', '#/admin/finance/fx')
  })

  it('shows the tenant-create prompt when no tenants exist', async () => {
    mocks.listTenants.mockResolvedValueOnce([])

    render(Finance)

    expect(await screen.findByText(/Create or join a tenant/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Finance tenants' })).toHaveAttribute('href', '#/finance/tenants')
  })

  it('requires an explicit tenant choice when multiple tenants exist and none is active yet', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants.mockResolvedValueOnce([
      { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now },
      { id: 'tenant-2', name: 'Travel', displayCurrency: 'EUR', joinedAt: now, createdAt: now, updatedAt: now },
    ])

    render(Finance)

    expect(
      await screen.findByText('Select an active tenant to continue on this finance route.'),
    ).toBeInTheDocument()
  })

  it('supports previous and custom period reload actions', async () => {
    const user = userEvent.setup()
    mocks.getDashboard
      .mockResolvedValueOnce({
        period: {
          preset: 'current_month',
          startDate: new Date('2026-06-20T12:00:00Z'),
          endDate: new Date('2026-06-20T12:00:00Z'),
          previous: { startDate: new Date('2026-06-20T12:00:00Z'), endDate: new Date('2026-06-20T12:00:00Z') },
          next: { startDate: new Date('2026-06-20T12:00:00Z'), endDate: new Date('2026-06-20T12:00:00Z') },
        },
        settled: { displayCurrency: 'USD', incomeMinor: 120000, expenseMinor: 45000, netMinor: 75000, transactionCount: 12, complete: true },
        pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 5000, netMinor: -5000, transactionCount: 1, complete: true },
        categoryBreakdowns: [{ categoryId: 'cat-1', categoryName: 'Groceries', kind: 'expense', incomeMinor: 0, expenseMinor: 1000, transactionCount: 1 }],
        accountBalances: [{ accountId: 'acc-1', accountName: 'Checking', currency: 'USD', nativeBookedMinor: 50000, nativePendingMinor: 5000, displayBookedMinor: 50000, displayPendingMinor: 5000, missingFx: false }],
        alerts: [{ code: 'stale_connection', severity: 'warning', count: 1 }],
        missingFx: [{ source: 'provider', transactionId: 'tx-1', accountId: 'acc-1', baseCurrency: 'EUR', quoteCurrency: 'USD', rateDate: new Date('2026-06-20T12:00:00Z'), provider: 'frankfurter' }],
        nativeSettledTotals: [],
      })
      .mockResolvedValueOnce({
        period: {
          preset: '',
          startDate: new Date('2026-05-01T00:00:00Z'),
          endDate: new Date('2026-05-31T00:00:00Z'),
          previous: { startDate: new Date('2026-04-01T00:00:00Z'), endDate: new Date('2026-04-30T00:00:00Z') },
          next: { startDate: new Date('2026-06-01T00:00:00Z'), endDate: new Date('2026-06-30T00:00:00Z') },
        },
        settled: { displayCurrency: 'USD', incomeMinor: 120000, expenseMinor: 45000, netMinor: 75000, transactionCount: 12, complete: true },
        pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 5000, netMinor: -5000, transactionCount: 1, complete: true },
        categoryBreakdowns: [{ categoryId: 'cat-1', categoryName: 'Groceries', kind: 'expense', incomeMinor: 0, expenseMinor: 1000, transactionCount: 1 }],
        accountBalances: [{ accountId: 'acc-1', accountName: 'Checking', currency: 'USD', nativeBookedMinor: 50000, nativePendingMinor: 5000, displayBookedMinor: 50000, displayPendingMinor: 5000, missingFx: false }],
        alerts: [{ code: 'stale_connection', severity: 'warning', count: 1 }],
        missingFx: [{ source: 'provider', transactionId: 'tx-1', accountId: 'acc-1', baseCurrency: 'EUR', quoteCurrency: 'USD', rateDate: new Date('2026-06-20T12:00:00Z'), provider: 'frankfurter' }],
        nativeSettledTotals: [],
      })
      .mockResolvedValueOnce({
        period: {
          preset: '',
          startDate: new Date('2026-06-01T00:00:00Z'),
          endDate: new Date('2026-06-30T00:00:00Z'),
          previous: { startDate: new Date('2026-05-01T00:00:00Z'), endDate: new Date('2026-05-31T00:00:00Z') },
          next: { startDate: new Date('2026-07-01T00:00:00Z'), endDate: new Date('2026-07-31T00:00:00Z') },
        },
        settled: { displayCurrency: 'USD', incomeMinor: 120000, expenseMinor: 45000, netMinor: 75000, transactionCount: 12, complete: true },
        pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 5000, netMinor: -5000, transactionCount: 1, complete: true },
        categoryBreakdowns: [{ categoryId: 'cat-1', categoryName: 'Groceries', kind: 'expense', incomeMinor: 0, expenseMinor: 1000, transactionCount: 1 }],
        accountBalances: [{ accountId: 'acc-1', accountName: 'Checking', currency: 'USD', nativeBookedMinor: 50000, nativePendingMinor: 5000, displayBookedMinor: 50000, displayPendingMinor: 5000, missingFx: false }],
        alerts: [{ code: 'stale_connection', severity: 'warning', count: 1 }],
        missingFx: [{ source: 'provider', transactionId: 'tx-1', accountId: 'acc-1', baseCurrency: 'EUR', quoteCurrency: 'USD', rateDate: new Date('2026-06-20T12:00:00Z'), provider: 'frankfurter' }],
        nativeSettledTotals: [],
      })
    render(Finance)

    await user.click(await screen.findByRole('button', { name: 'Previous period' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(2))

    await user.click(screen.getByText('Custom range'))
    expect(screen.getByLabelText('Custom start date')).toHaveValue('2026-05-01')
    expect(screen.getByLabelText('Custom end date')).toHaveValue('2026-05-31')

    await user.clear(screen.getByLabelText('Custom start date'))
    await user.type(screen.getByLabelText('Custom start date'), '2026-06-01')
    await user.clear(screen.getByLabelText('Custom end date'))
    await user.type(screen.getByLabelText('Custom end date'), '2026-06-30')
    await user.click(screen.getByRole('button', { name: 'Apply custom range' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(3))
    expect(screen.getByLabelText('Custom start date')).toHaveValue('2026-06-01')
    expect(screen.getByLabelText('Custom end date')).toHaveValue('2026-06-30')
  })

  it('supports current-month and next-period actions', async () => {
    const user = userEvent.setup()
    render(Finance)

    await user.click(await screen.findByRole('button', { name: 'Current month' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(2))

    await user.click(screen.getByText('Custom range'))
    expect(screen.getByLabelText('Custom start date')).toHaveValue('2026-06-20')
    expect(screen.getByLabelText('Custom end date')).toHaveValue('2026-06-20')

    await user.click(screen.getByRole('button', { name: 'Next period' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(3))
    expect(screen.getByLabelText('Custom start date')).toHaveValue('2026-06-20')
    expect(screen.getByLabelText('Custom end date')).toHaveValue('2026-06-20')
  })

  it('renders empty dashboard sections when no alerts or balances exist', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getDashboard.mockResolvedValueOnce({
      period: { preset: '', startDate: now, endDate: now, previous: { startDate: now, endDate: now }, next: { startDate: now, endDate: now } },
      settled: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      categoryBreakdowns: [],
      accountBalances: [],
      alerts: [],
      missingFx: [],
      nativeSettledTotals: [],
    })
    mocks.listTransactions.mockResolvedValueOnce([])
    mocks.listConnections.mockResolvedValueOnce([])

    render(Finance)

    expect(await screen.findByText('No settled or pending cash flow to chart for this period.')).toBeInTheDocument()
    expect(screen.getByText('No account balances to chart yet.')).toBeInTheDocument()
    expect(screen.getByText('No category activity to chart for this period.')).toBeInTheDocument()
    expect(screen.getByText('No accounts yet for this tenant.')).toBeInTheDocument()
    expect(screen.getByText('No category activity for this period.')).toBeInTheDocument()
    expect(screen.getByText('No recent transactions for this tenant yet.')).toBeInTheDocument()
    expect(screen.getByText('No linked connections yet.')).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Needs attention' })).not.toBeInTheDocument()
  })

  it('renders native settled totals when the dashboard provides multi-currency cash flow summaries', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getDashboard.mockResolvedValueOnce({
      period: { preset: '', startDate: now, endDate: now, previous: { startDate: now, endDate: now }, next: { startDate: now, endDate: now } },
      settled: { displayCurrency: 'USD', incomeMinor: 120000, expenseMinor: 45000, netMinor: 75000, transactionCount: 12, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 5000, netMinor: -5000, transactionCount: 1, complete: true },
      categoryBreakdowns: [],
      accountBalances: [],
      alerts: [],
      missingFx: [],
      nativeSettledTotals: [
        { currency: 'USD', incomeMinor: 120000, expenseMinor: 45000, netMinor: 75000 },
        { currency: 'EUR', incomeMinor: 8000, expenseMinor: 2000, netMinor: 6000 },
      ],
    })

    render(Finance)

    expect(await screen.findByText('EUR')).toBeInTheDocument()
    expect(screen.getAllByText('Settled total').length).toBeGreaterThan(0)
  })

  it('renders recent activity fallbacks for connection review and unsynced states', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTransactions.mockResolvedValueOnce([
      {
        id: 'tx-2',
        tenantId: 'tenant-1',
        accountId: 'acc-1',
        source: 'manual',
        status: 'pending',
        kind: 'refund',
        amountMinor: 500,
        currency: 'USD',
        description: '',
        effectiveAt: now,
        categoryId: null,
        transferGroupId: null,
        transferMatchedAt: null,
        hiddenAt: null,
        providerOriginal: null,
        createdAt: now,
        updatedAt: now,
      },
    ])
    mocks.listConnections.mockResolvedValueOnce([
      {
        id: 'conn-2',
        tenantId: 'tenant-1',
        provider: 'synthetic',
        displayName: 'Broken sync',
        providerReference: 'ref-2',
        externalId: 'ext-2',
        state: 'attention',
        lastSyncJobId: 'job-2',
        lastSyncStartedAt: now,
        lastSuccessfulSyncAt: null,
        lastSyncError: 'token expired',
        createdAt: now,
        updatedAt: now,
        schedule: null,
      },
      {
        id: 'conn-3',
        tenantId: 'tenant-1',
        provider: 'synthetic',
        displayName: 'Manual sync',
        providerReference: 'ref-3',
        externalId: 'ext-3',
        state: 'new',
        lastSyncJobId: '',
        lastSyncStartedAt: null,
        lastSuccessfulSyncAt: null,
        lastSyncError: '',
        createdAt: now,
        updatedAt: now,
        schedule: null,
      },
      {
        id: 'conn-4',
        tenantId: 'tenant-1',
        provider: 'synthetic',
        displayName: 'Completed sync',
        providerReference: 'ref-4',
        externalId: 'ext-4',
        state: 'ready',
        lastSyncJobId: 'job-4',
        lastSyncStartedAt: now,
        lastSuccessfulSyncAt: now,
        lastSyncError: '',
        createdAt: now,
        updatedAt: now,
        schedule: null,
      },
    ])

    render(Finance)

    expect(await screen.findByText('refund')).toBeInTheDocument()
    expect(screen.getByText('Broken sync')).toBeInTheDocument()
    expect(screen.getByText('Needs review')).toBeInTheDocument()
    expect(screen.getByText('token expired')).toBeInTheDocument()
    expect(screen.getByText('Manual sync')).toBeInTheDocument()
    expect(screen.getByText('No sync history yet')).toBeInTheDocument()
    expect(screen.getByText(/Last success/)).toBeInTheDocument()
  })

  it('renders chart data for zero balances, income categories, and mixed alert severities', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getDashboard.mockResolvedValueOnce({
      period: { preset: '', startDate: now, endDate: now, previous: { startDate: now, endDate: now }, next: { startDate: now, endDate: now } },
      settled: { displayCurrency: 'USD', incomeMinor: 220000, expenseMinor: 60000, netMinor: 160000, transactionCount: 14, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 10000, expenseMinor: 4000, netMinor: 6000, transactionCount: 2, complete: true },
      categoryBreakdowns: [{ categoryId: 'cat-income', categoryName: 'Salary', kind: 'income', incomeMinor: 220000, expenseMinor: 0, transactionCount: 1 }],
      accountBalances: [
        { accountId: 'acc-zero', accountName: 'Cash reserve', currency: 'USD', nativeBookedMinor: 0, nativePendingMinor: 0, displayBookedMinor: 0, displayPendingMinor: 0, missingFx: false },
        { accountId: 'acc-fx', accountName: 'Travel wallet', currency: 'EUR', nativeBookedMinor: 1500, nativePendingMinor: 100, displayBookedMinor: 1600, displayPendingMinor: 100, missingFx: true },
      ],
      alerts: [
        { code: 'needs_review', severity: 'error', count: 2 },
        { code: 'background_note', severity: 'info' as const, count: 1 },
      ],
      missingFx: [],
      nativeSettledTotals: [],
    })

    render(Finance)

    expect((await screen.findAllByText('Salary')).length).toBeGreaterThan(0)
    expect(screen.getAllByText('Cash reserve').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Travel wallet').length).toBeGreaterThan(0)
    expect(screen.getByText('Needs review')).toBeInTheDocument()
    expect(screen.getByText('Background note')).toBeInTheDocument()
    expect(screen.queryByText('No settled or pending cash flow to chart for this period.')).not.toBeInTheDocument()
  })

  it('caps account, category, and recent transaction sections to keep the dashboard scannable', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getDashboard.mockResolvedValueOnce({
      period: { preset: '', startDate: now, endDate: now, previous: { startDate: now, endDate: now }, next: { startDate: now, endDate: now } },
      settled: { displayCurrency: 'USD', incomeMinor: 220000, expenseMinor: 60000, netMinor: 160000, transactionCount: 14, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 10000, expenseMinor: 4000, netMinor: 6000, transactionCount: 2, complete: true },
      categoryBreakdowns: [
        { categoryId: 'cat-1', categoryName: 'Category 1', kind: 'expense', incomeMinor: 0, expenseMinor: 8000, transactionCount: 1 },
        { categoryId: 'cat-2', categoryName: 'Category 2', kind: 'expense', incomeMinor: 0, expenseMinor: 7000, transactionCount: 1 },
        { categoryId: 'cat-3', categoryName: 'Category 3', kind: 'expense', incomeMinor: 0, expenseMinor: 6000, transactionCount: 1 },
        { categoryId: 'cat-4', categoryName: 'Category 4', kind: 'expense', incomeMinor: 0, expenseMinor: 5000, transactionCount: 1 },
        { categoryId: 'cat-5', categoryName: 'Category 5', kind: 'expense', incomeMinor: 0, expenseMinor: 4000, transactionCount: 1 },
      ],
      accountBalances: [
        { accountId: 'acc-1', accountName: 'Account 1', currency: 'USD', nativeBookedMinor: 9000, nativePendingMinor: 0, displayBookedMinor: 9000, displayPendingMinor: 0, missingFx: false },
        { accountId: 'acc-2', accountName: 'Account 2', currency: 'USD', nativeBookedMinor: 8000, nativePendingMinor: 0, displayBookedMinor: 8000, displayPendingMinor: 0, missingFx: false },
        { accountId: 'acc-3', accountName: 'Account 3', currency: 'USD', nativeBookedMinor: 7000, nativePendingMinor: 0, displayBookedMinor: 7000, displayPendingMinor: 0, missingFx: false },
        { accountId: 'acc-4', accountName: 'Account 4', currency: 'USD', nativeBookedMinor: 6000, nativePendingMinor: 0, displayBookedMinor: 6000, displayPendingMinor: 0, missingFx: false },
        { accountId: 'acc-5', accountName: 'Account 5', currency: 'USD', nativeBookedMinor: 5000, nativePendingMinor: 0, displayBookedMinor: 5000, displayPendingMinor: 0, missingFx: false },
      ],
      alerts: [],
      missingFx: [],
      nativeSettledTotals: [],
    })
    mocks.listTransactions.mockResolvedValueOnce([
      {
        id: 'tx-1', tenantId: 'tenant-1', accountId: 'acc-1', source: 'provider', status: 'booked', kind: 'expense', amountMinor: -100, currency: 'USD', description: 'Transaction 1', effectiveAt: new Date('2026-06-20T12:00:00Z'), categoryId: null, transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now,
      },
      {
        id: 'tx-2', tenantId: 'tenant-1', accountId: 'acc-1', source: 'provider', status: 'booked', kind: 'expense', amountMinor: -100, currency: 'USD', description: 'Transaction 2', effectiveAt: new Date('2026-06-19T12:00:00Z'), categoryId: null, transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now,
      },
      {
        id: 'tx-3', tenantId: 'tenant-1', accountId: 'acc-1', source: 'provider', status: 'booked', kind: 'expense', amountMinor: -100, currency: 'USD', description: 'Transaction 3', effectiveAt: new Date('2026-06-18T12:00:00Z'), categoryId: null, transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now,
      },
      {
        id: 'tx-4', tenantId: 'tenant-1', accountId: 'acc-1', source: 'provider', status: 'booked', kind: 'expense', amountMinor: -100, currency: 'USD', description: 'Transaction 4', effectiveAt: new Date('2026-06-17T12:00:00Z'), categoryId: null, transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now,
      },
      {
        id: 'tx-5', tenantId: 'tenant-1', accountId: 'acc-1', source: 'provider', status: 'booked', kind: 'expense', amountMinor: -100, currency: 'USD', description: 'Transaction 5', effectiveAt: new Date('2026-06-16T12:00:00Z'), categoryId: null, transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now,
      },
      {
        id: 'tx-6', tenantId: 'tenant-1', accountId: 'acc-1', source: 'provider', status: 'booked', kind: 'expense', amountMinor: -100, currency: 'USD', description: 'Transaction 6', effectiveAt: new Date('2026-06-15T12:00:00Z'), categoryId: null, transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now,
      },
    ])

    render(Finance)

    expect((await screen.findAllByText('Account 1')).length).toBeGreaterThan(0)
    expect(screen.getAllByText('Account 4').length).toBeGreaterThan(0)
    expect(screen.queryByText('Account 5')).not.toBeInTheDocument()
    expect(screen.getAllByText('Category 4').length).toBeGreaterThan(0)
    expect(screen.queryByText('Category 5')).not.toBeInTheDocument()
    expect(screen.getByText('Transaction 5')).toBeInTheDocument()
    expect(screen.queryByText('Transaction 6')).not.toBeInTheDocument()
    expect(screen.getAllByRole('link', { name: 'View all accounts' })[0]).toHaveAttribute('href', '#/finance/accounts')
    expect(screen.getAllByRole('link', { name: 'View all categories' })[0]).toHaveAttribute('href', '#/finance/categories')
    expect(screen.getAllByRole('link', { name: 'View all transactions' })[0]).toHaveAttribute('href', '#/finance/transactions')
  })

  it('renders a dashboard error after tenant selection', async () => {
    mocks.getDashboard.mockRejectedValueOnce(new Error('dashboard exploded'))

    render(Finance)

    expect(await screen.findByRole('alert')).toHaveTextContent('dashboard exploded')
  })

})
