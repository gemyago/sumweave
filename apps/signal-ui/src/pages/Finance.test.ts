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
    mocks.listTenants.mockResolvedValue([
      { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now },
    ])
    mocks.getDashboard.mockResolvedValue({
      period: { preset: 'current_month', startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20), previous: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20) }, next: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20) } },
      settled: { displayCurrency: 'USD', incomeMinor: 120000, expenseMinor: 45000, netMinor: 75000, transactionCount: 12, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 5000, netMinor: -5000, transactionCount: 1, complete: true },
      categoryBreakdowns: [{ categoryId: 'cat-1', categoryName: 'Groceries', kind: 'expense', incomeMinor: 0, expenseMinor: 1000, transactionCount: 1 }],
      accountBalances: [{ accountId: 'acc-1', accountName: 'Checking', currency: 'USD', nativeBookedMinor: 50000, nativePendingMinor: 5000, displayBookedMinor: 50000, displayPendingMinor: 5000, missingFx: false }],
      alerts: [{ code: 'stale_connection', severity: 'warning', count: 1 }],
      missingFx: [{ source: 'provider', transactionId: 'tx-1', accountId: 'acc-1', baseCurrency: 'EUR', quoteCurrency: 'USD', rateDate: new Date(2026, 5, 20), provider: 'frankfurter' }],
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
        schedule: null,
      },
    ])
  })

  it('renders the canonical bootstrap dashboard with balance-first summaries and canonical finance links', async () => {
    render(Finance)

    expect(await screen.findByRole('heading', { name: 'Finance dashboard' })).toBeInTheDocument()
    expect(await screen.findByText('Booked balance story')).toBeInTheDocument()
    expect(screen.getByText('Income')).toBeInTheDocument()
    expect(screen.getByText('Expense')).toBeInTheDocument()
    expect(screen.getByText('Pending delta')).toBeInTheDocument()
    expect(screen.getByText('Cash-flow visual')).toBeInTheDocument()
    expect(screen.getByLabelText('Cash flow chart')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Top categories' })).toBeInTheDocument()
    expect(screen.getByLabelText('Category breakdown chart')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Largest balances' })).toBeInTheDocument()
    expect(screen.getByLabelText('Account balances chart')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Recent transactions' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Needs attention' })).toBeInTheDocument()
    expect(screen.getByText('Missing FX coverage')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open accounts' })).toHaveAttribute('href', '#/finance/accounts')
    expect(screen.getByRole('link', { name: 'Open transactions' })).toHaveAttribute('href', '#/finance/transactions')
    expect(screen.getByRole('link', { name: 'Review in admin FX diagnostics' })).toHaveAttribute('href', '#/admin/finance/fx')
    expect(screen.queryByText('Bootstrap pilot')).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Overview' })).not.toBeInTheDocument()
    expect(screen.getByLabelText('Custom start date')).not.toBeVisible()
    expect(screen.getByLabelText('Custom end date')).not.toBeVisible()
    expect(screen.queryByText('2026-06-20T12:00:00.000Z')).not.toBeInTheDocument()
    expect(screen.getByText('Jun 20, 2026 → Jun 20, 2026')).toBeInTheDocument()
  })

  it('renders compact needs-attention items for pending, missing FX, failed sync, and failed import signals', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getDashboard.mockResolvedValueOnce({
      period: { preset: 'current_month', startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20), previous: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20) }, next: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20) } },
      settled: { displayCurrency: 'USD', incomeMinor: 120000, expenseMinor: 45000, netMinor: 75000, transactionCount: 12, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 5000, netMinor: -5000, transactionCount: 3, complete: true },
      categoryBreakdowns: [{ categoryId: 'cat-1', categoryName: 'Groceries', kind: 'expense', incomeMinor: 0, expenseMinor: 1000, transactionCount: 1 }],
      accountBalances: [{ accountId: 'acc-1', accountName: 'Checking', currency: 'USD', nativeBookedMinor: 50000, nativePendingMinor: 5000, displayBookedMinor: 50000, displayPendingMinor: 5000, missingFx: false }],
      alerts: [{ code: 'failed_import', severity: 'error', count: 2 }],
      missingFx: [{ source: 'provider', transactionId: 'tx-1', accountId: 'acc-1', baseCurrency: 'EUR', quoteCurrency: 'USD', rateDate: '2026-06-20', provider: 'frankfurter' }],
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
    expect(screen.getByRole('link', { name: 'Review imports' })).toHaveAttribute('href', '#/finance/imports')
  })

  it('places an incomplete income and expense warning beside the totals with excluded count and FX diagnostics link', async () => {
    mocks.getDashboard.mockResolvedValueOnce({
      period: { preset: 'current_month', startDate: new Date(2026, 5, 1), endDate: new Date(2026, 5, 30), previous: { startDate: new Date(2026, 4, 1), endDate: new Date(2026, 4, 31) }, next: { startDate: new Date(2026, 6, 1), endDate: new Date(2026, 6, 31) } },
      settled: { displayCurrency: 'PLN', incomeMinor: 100, expenseMinor: 200, netMinor: -100, transactionCount: 2, complete: false },
      pending: { displayCurrency: 'PLN', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      categoryBreakdowns: [], accountBalances: [], alerts: [],
      missingFx: [
        { source: 'provider', transactionId: 'tx-1', accountId: 'acc-1', baseCurrency: 'EUR', quoteCurrency: 'PLN', rateDate: new Date(2026, 5, 20), provider: 'frankfurter' },
        { source: 'provider', transactionId: 'tx-2', accountId: 'acc-1', baseCurrency: 'USD', quoteCurrency: 'PLN', rateDate: new Date(2026, 5, 20), provider: 'frankfurter' },
      ],
      nativeSettledTotals: [],
    })

    render(Finance)

    const warning = (await screen.findByText('Income and expense totals are incomplete.')).parentElement!
    expect(warning).toHaveTextContent('Income and expense totals are incomplete.')
    expect(warning).toHaveTextContent('2 values were excluded')
    expect(screen.getByRole('link', { name: 'Open FX diagnostics' })).toHaveAttribute('href', '#/admin/finance/fx')
    expect(screen.getByText('Income').compareDocumentPosition(warning) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('shows the tenant-create prompt when no tenants exist', async () => {
    mocks.listTenants.mockResolvedValueOnce([])

    render(Finance)

    expect(await screen.findByText(/Create or join a tenant/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Finance tenants' })).toHaveAttribute('href', '#/finance/tenants')
  })

  it('requires explicit tenant selection when multiple tenants exist and none is active yet', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants.mockResolvedValueOnce([
      { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now },
      { id: 'tenant-2', name: 'Travel', displayCurrency: 'EUR', joinedAt: now, createdAt: now, updatedAt: now },
    ])

    render(Finance)

    expect(await screen.findByText('Select an active tenant to continue on this finance route.')).toBeInTheDocument()
  })

  it('calls the exact previous-month preset without deriving a custom window', async () => {
    const user = userEvent.setup()
    mocks.getDashboard
      .mockResolvedValueOnce({
        period: {
          preset: 'current_month',
          startDate: new Date(2026, 5, 20),
          endDate: new Date(2026, 5, 20),
          previous: { startDate: new Date(2026, 4, 1), endDate: new Date(2026, 4, 31) },
          next: { startDate: new Date(2026, 6, 1), endDate: new Date(2026, 6, 31) },
        },
        settled: { displayCurrency: 'USD', incomeMinor: 120000, expenseMinor: 45000, netMinor: 75000, transactionCount: 12, complete: true },
        pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 5000, netMinor: -5000, transactionCount: 1, complete: true },
        categoryBreakdowns: [],
        accountBalances: [],
        alerts: [],
        missingFx: [],
        nativeSettledTotals: [],
      })
      .mockResolvedValueOnce({
        period: {
          preset: '',
          startDate: new Date(2026, 4, 1),
          endDate: new Date(2026, 4, 31),
          previous: { startDate: new Date(2026, 3, 1), endDate: new Date(2026, 3, 30) },
          next: { startDate: new Date(2026, 5, 1), endDate: new Date(2026, 5, 30) },
        },
        settled: { displayCurrency: 'USD', incomeMinor: 120000, expenseMinor: 45000, netMinor: 75000, transactionCount: 12, complete: true },
        pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 5000, netMinor: -5000, transactionCount: 1, complete: true },
        categoryBreakdowns: [],
        accountBalances: [],
        alerts: [],
        missingFx: [],
        nativeSettledTotals: [],
      })
      .mockResolvedValueOnce({
        period: {
          preset: '',
          startDate: new Date(2026, 5, 1),
          endDate: new Date(2026, 5, 30),
          previous: { startDate: new Date(2026, 4, 1), endDate: new Date(2026, 4, 31) },
          next: { startDate: new Date(2026, 6, 1), endDate: new Date(2026, 6, 31) },
        },
        settled: { displayCurrency: 'USD', incomeMinor: 120000, expenseMinor: 45000, netMinor: 75000, transactionCount: 12, complete: true },
        pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 5000, netMinor: -5000, transactionCount: 1, complete: true },
        categoryBreakdowns: [],
        accountBalances: [],
        alerts: [],
        missingFx: [],
        nativeSettledTotals: [],
      })

    render(Finance)

    await user.click(await screen.findByRole('button', { name: 'Previous month' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(2))
    expect(mocks.getDashboard.mock.calls[1][0]).toEqual({ tenantId: 'tenant-1', preset: 'previous_month', startDate: undefined, endDate: undefined })

    await user.click(screen.getByText('Custom range'))
    expect(screen.getByLabelText('Custom start date')).toHaveValue('2026-05-01')
    expect(screen.getByLabelText('Custom end date')).toHaveValue('2026-05-31')

    await user.clear(screen.getByLabelText('Custom start date'))
    await user.type(screen.getByLabelText('Custom start date'), '2026-06-01')
    await user.clear(screen.getByLabelText('Custom end date'))
    await user.type(screen.getByLabelText('Custom end date'), '2026-06-30')
    await user.click(screen.getByRole('button', { name: 'Apply' }))

    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(3))
    expect(screen.getByLabelText('Custom start date')).toHaveValue('2026-06-01')
    expect(screen.getByLabelText('Custom end date')).toHaveValue('2026-06-30')
  })

  it('calls the exact current- and next-month presets', async () => {
    const user = userEvent.setup()
    render(Finance)

    await user.click(await screen.findByRole('button', { name: 'Current month' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(2))
    expect(mocks.getDashboard.mock.calls[1][0]).toEqual({ tenantId: 'tenant-1', preset: 'current_month', startDate: undefined, endDate: undefined })

    await user.click(screen.getByText('Custom range'))
    expect(screen.getByLabelText('Custom start date')).toHaveValue('2026-06-20')
    expect(screen.getByLabelText('Custom end date')).toHaveValue('2026-06-20')

    await user.click(screen.getByRole('button', { name: 'Next month' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(3))
    expect(mocks.getDashboard.mock.calls[2][0]).toEqual({ tenantId: 'tenant-1', preset: 'next_month', startDate: undefined, endDate: undefined })
  })

  it('keeps dashboard custom range instants until the date control changes them', async () => {
    const user = userEvent.setup()
    const startDate = new Date('2026-06-01T23:45:12.345-07:00')
    const endDate = new Date('2026-06-30T01:15:42.987+05:30')
    const dashboard = {
      period: { preset: 'custom', startDate, endDate, previous: { startDate, endDate }, next: { startDate, endDate } },
      settled: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      categoryBreakdowns: [], accountBalances: [], alerts: [], missingFx: [], nativeSettledTotals: [],
    }
    mocks.getDashboard.mockResolvedValueOnce(dashboard).mockResolvedValueOnce(dashboard)

    render(Finance)
    await user.click(await screen.findByText('Custom range'))
    await user.click(screen.getByRole('button', { name: 'Apply' }))

    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(2))
    const request = mocks.getDashboard.mock.calls[1][0]
    expect(request.startDate).toBeInstanceOf(Date)
    expect(request.endDate).toBeInstanceOf(Date)
    expect(request.startDate).toEqual(startDate)
    expect(request.endDate).toEqual(endDate)
  })

  it('uses inclusive local calendar bounds for selected custom dates so midnight imported rows are included', async () => {
    const user = userEvent.setup()
    const importedAtMidnight = new Date(2026, 4, 29, 0, 0, 0, 0)
    const initialDashboard = {
      period: {
        preset: 'current_month',
        startDate: new Date(2026, 4, 1, 9, 40),
        endDate: new Date(2026, 5, 3, 9, 40),
        previous: { startDate: new Date(2026, 3, 1), endDate: new Date(2026, 4, 1) },
        next: { startDate: new Date(2026, 5, 4), endDate: new Date(2026, 6, 4) },
      },
      settled: { displayCurrency: 'PLN', incomeMinor: 0, expenseMinor: 830000, netMinor: -830000, transactionCount: 1, complete: true },
      pending: { displayCurrency: 'PLN', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      categoryBreakdowns: [], accountBalances: [], alerts: [], missingFx: [], nativeSettledTotals: [{ currency: 'PLN', incomeMinor: 0, expenseMinor: 830000, netMinor: -830000 }],
    }
    mocks.getDashboard.mockResolvedValue(initialDashboard)

    render(Finance)
    await user.click(await screen.findByText('Custom range'))
    await user.clear(screen.getByLabelText('Custom start date'))
    await user.type(screen.getByLabelText('Custom start date'), '2026-05-29')
    await user.clear(screen.getByLabelText('Custom end date'))
    await user.type(screen.getByLabelText('Custom end date'), '2026-06-03')
    await user.click(screen.getByRole('button', { name: 'Apply' }))

    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(2))
    const request = mocks.getDashboard.mock.calls[1][0]
    expect(request.startDate).toEqual(new Date(2026, 4, 29, 0, 0, 0, 0))
    expect(request.endDate).toEqual(new Date(2026, 5, 3, 23, 59, 59, 999))
    expect(importedAtMidnight.getTime()).toBeGreaterThanOrEqual(request.startDate.getTime())
    expect(importedAtMidnight.getTime()).toBeLessThanOrEqual(request.endDate.getTime())
    expect(await screen.findByText('PLN')).toBeInTheDocument()
  })

  it('renders honest empty states when the dashboard has no activity', async () => {
    mocks.getDashboard.mockResolvedValueOnce({
      period: { preset: '', startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20), previous: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20) }, next: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20) } },
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
    expect(screen.getByText('No category activity to chart for this period.')).toBeInTheDocument()
    expect(screen.getByText('No account balances to chart yet.')).toBeInTheDocument()
    expect(screen.getByText('No recent transactions for this tenant yet.')).toBeInTheDocument()
    expect(screen.getByText('No active attention signals right now.')).toBeInTheDocument()
    expect(screen.getByText('No booked balances yet')).toBeInTheDocument()
  })

  it('routes native totals, sync issues, and import follow-up through the dashboard attention area', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getDashboard.mockResolvedValueOnce({
      period: { preset: '', startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20), previous: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20) }, next: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20) } },
      settled: { displayCurrency: 'USD', incomeMinor: 220000, expenseMinor: 60000, netMinor: 160000, transactionCount: 14, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 10000, expenseMinor: 4000, netMinor: 6000, transactionCount: 2, complete: true },
      categoryBreakdowns: [{ categoryId: 'cat-income', categoryName: 'Salary', kind: 'income', incomeMinor: 220000, expenseMinor: 0, transactionCount: 1 }],
      accountBalances: [],
      alerts: [{ code: 'failed_import', severity: 'error', count: 2 }],
      missingFx: [],
      nativeSettledTotals: [
        { currency: 'USD', incomeMinor: 220000, expenseMinor: 60000, netMinor: 160000 },
        { currency: 'EUR', incomeMinor: 8000, expenseMinor: 2000, netMinor: 6000 },
      ],
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

    expect(await screen.findByText('Native totals')).toBeInTheDocument()
    expect(screen.getByText('No booked balances yet')).toBeInTheDocument()
    expect(screen.getByText('Salary')).toBeInTheDocument()
    expect(screen.getByText('Failed sync')).toBeInTheDocument()
    expect(screen.getByText('Failed import')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Review imports' })).toHaveAttribute('href', '#/finance/imports')
    expect(screen.getByRole('link', { name: 'Review connections' })).toHaveAttribute('href', '#/finance/connections')
  })

  it('shows account-level missing FX badges and tolerates mixed connection timestamp fallbacks', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getDashboard.mockResolvedValueOnce({
      period: { preset: '', startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20), previous: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20) }, next: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20) } },
      settled: { displayCurrency: 'USD', incomeMinor: 5000, expenseMinor: 3000, netMinor: 2000, transactionCount: 2, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 1500, expenseMinor: 250, netMinor: 1250, transactionCount: 2, complete: true },
      categoryBreakdowns: [],
      accountBalances: [
        { accountId: 'acc-1', accountName: 'Operating', currency: 'USD', nativeBookedMinor: 5000, nativePendingMinor: 1250, displayBookedMinor: 5000, displayPendingMinor: 1250, missingFx: false },
        { accountId: 'acc-2', accountName: 'Travel wallet', currency: 'EUR', nativeBookedMinor: 1000, nativePendingMinor: 50, displayBookedMinor: 1100, displayPendingMinor: 55, missingFx: true },
      ],
      alerts: [
        { code: 'connection_backlog', severity: 'warning', count: 1 },
        { code: 'background_note', severity: 'info', count: 1 },
        { code: 'settled_ok', severity: 'success', count: 1 },
      ],
      missingFx: [],
      nativeSettledTotals: [],
    })
    mocks.listTransactions.mockResolvedValueOnce([
      {
        id: 'tx-review',
        tenantId: 'tenant-1',
        accountId: 'acc-2',
        source: 'provider',
        status: 'review',
        kind: 'expense',
        amountMinor: -3000,
        currency: 'EUR',
        description: 'Hotel',
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
        id: 'conn-ok',
        tenantId: 'tenant-1',
        provider: 'synthetic',
        displayName: 'Healthy sync',
        providerReference: 'ref-ok',
        externalId: 'ext-ok',
        state: 'ready',
        lastSyncJobId: 'job-ok',
        lastSyncStartedAt: now,
        lastSuccessfulSyncAt: now,
        lastSyncError: '',
        createdAt: now,
        updatedAt: now,
        schedule: null,
      },
      {
        id: 'conn-late',
        tenantId: 'tenant-1',
        provider: 'synthetic',
        displayName: 'Queued sync',
        providerReference: 'ref-late',
        externalId: 'ext-late',
        state: 'queued',
        lastSyncJobId: '',
        lastSyncStartedAt: null,
        lastSuccessfulSyncAt: null,
        lastSyncError: '',
        createdAt: now,
        updatedAt: new Date('2026-06-21T12:00:00Z'),
        schedule: null,
      },
    ])

    render(Finance)

    expect((await screen.findAllByText('Travel wallet')).length).toBeGreaterThan(0)
    expect(screen.getByText('Missing FX')).toBeInTheDocument()
    expect(screen.getByText('Connection backlog')).toBeInTheDocument()
    expect(screen.getByText('Background note')).toBeInTheDocument()
    expect(screen.getByText('Settled ok')).toBeInTheDocument()
    expect(screen.getByText('Hotel')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Review connections' })).toHaveAttribute('href', '#/finance/connections')
  })

  it('does not add native foreign minor values into an unavailable display balance', async () => {
    mocks.getDashboard.mockResolvedValueOnce({
      period: { preset: 'current_month', startDate: new Date(2026, 5, 1), endDate: new Date(2026, 5, 30), previous: { startDate: new Date(), endDate: new Date() }, next: { startDate: new Date(), endDate: new Date() } },
      settled: { displayCurrency: 'PLN', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      pending: { displayCurrency: 'PLN', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      categoryBreakdowns: [],
      accountBalances: [
        { accountId: 'pln', accountName: 'PLN', currency: 'PLN', nativeBookedMinor: 10000, nativePendingMinor: 0, displayBookedMinor: 10000, displayPendingMinor: 0, missingFx: false },
        { accountId: 'eur', accountName: 'EUR', currency: 'EUR', nativeBookedMinor: 20000, nativePendingMinor: 0, displayBookedMinor: null, displayPendingMinor: null, missingFx: true },
      ],
      alerts: [], missingFx: [{ source: 'account', transactionId: null, accountId: 'eur', baseCurrency: 'EUR', quoteCurrency: 'PLN', rateDate: new Date(), provider: 'frankfurter' }], currentFxRates: [], nativeSettledTotals: [],
    })

    render(Finance)

    expect(await screen.findByText('Booked total unavailable')).toBeInTheDocument()
    expect(screen.getAllByText('Unavailable')).not.toHaveLength(0)
    expect(screen.getByText('Native 200.00 EUR')).toBeInTheDocument()
    expect(screen.queryByText('300.00 PLN')).not.toBeInTheDocument()
  })

  it('explains that a prior period uses current FX valuation', async () => {
    mocks.getDashboard.mockResolvedValueOnce({
      period: { preset: 'custom', startDate: new Date(2026, 4, 1), endDate: new Date(2026, 4, 31), previous: { startDate: new Date(), endDate: new Date() }, next: { startDate: new Date(), endDate: new Date() } },
      settled: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true }, pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      categoryBreakdowns: [], accountBalances: [], alerts: [], missingFx: [], currentFxRates: [], nativeSettledTotals: [],
    })
    render(Finance)
    expect(await screen.findByText('Past activity is valued using today’s latest FX rates, not an end-of-period rate.')).toBeInTheDocument()
  })

  it('shows fresh rate metadata and a prominent stale-rate warning', async () => {
    mocks.getDashboard.mockResolvedValueOnce({
      period: { preset: 'current_month', startDate: new Date(), endDate: new Date(), previous: { startDate: new Date(), endDate: new Date() }, next: { startDate: new Date(), endDate: new Date() } },
      settled: { displayCurrency: 'PLN', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true }, pending: { displayCurrency: 'PLN', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      categoryBreakdowns: [], accountBalances: [], alerts: [], missingFx: [], nativeSettledTotals: [],
      currentFxRates: [
        { provider: 'frankfurter', baseCurrency: 'EUR', quoteCurrency: 'PLN', effectiveAt: new Date('2026-06-19T00:00:00Z'), lastSuccessfulRefreshAt: new Date('2026-06-20T12:00:00Z'), stale: false },
        { provider: 'frankfurter', baseCurrency: 'USD', quoteCurrency: 'PLN', effectiveAt: new Date('2026-06-18T00:00:00Z'), lastSuccessfulRefreshAt: new Date('2026-06-18T12:00:00Z'), stale: true },
      ],
    })
    render(Finance)
    expect(await screen.findByText('Current FX valuation may be stale.')).toBeInTheDocument()
    expect(screen.getByText('EUR → PLN')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Refresh current rates' })).toHaveAttribute('href', '#/admin/finance/fx')
  })

  it('caps account, category, and recent transaction sections to keep the dashboard scannable', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getDashboard.mockResolvedValueOnce({
      period: { preset: '', startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20), previous: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20) }, next: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20) } },
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
      { id: 'tx-1', tenantId: 'tenant-1', accountId: 'acc-1', source: 'provider', status: 'booked', kind: 'expense', amountMinor: -100, currency: 'USD', description: 'Transaction 1', effectiveAt: new Date('2026-06-20T12:00:00Z'), categoryId: null, transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now },
      { id: 'tx-2', tenantId: 'tenant-1', accountId: 'acc-1', source: 'provider', status: 'booked', kind: 'expense', amountMinor: -100, currency: 'USD', description: 'Transaction 2', effectiveAt: new Date('2026-06-19T12:00:00Z'), categoryId: null, transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now },
      { id: 'tx-3', tenantId: 'tenant-1', accountId: 'acc-1', source: 'provider', status: 'booked', kind: 'expense', amountMinor: -100, currency: 'USD', description: 'Transaction 3', effectiveAt: new Date('2026-06-18T12:00:00Z'), categoryId: null, transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now },
      { id: 'tx-4', tenantId: 'tenant-1', accountId: 'acc-1', source: 'provider', status: 'booked', kind: 'expense', amountMinor: -100, currency: 'USD', description: 'Transaction 4', effectiveAt: new Date('2026-06-17T12:00:00Z'), categoryId: null, transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now },
      { id: 'tx-5', tenantId: 'tenant-1', accountId: 'acc-1', source: 'provider', status: 'booked', kind: 'expense', amountMinor: -100, currency: 'USD', description: 'Transaction 5', effectiveAt: new Date('2026-06-16T12:00:00Z'), categoryId: null, transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now },
      { id: 'tx-6', tenantId: 'tenant-1', accountId: 'acc-1', source: 'provider', status: 'booked', kind: 'expense', amountMinor: -100, currency: 'USD', description: 'Transaction 6', effectiveAt: new Date('2026-06-15T12:00:00Z'), categoryId: null, transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now },
    ])

    render(Finance)

    expect((await screen.findAllByText('Account 1')).length).toBeGreaterThan(0)
    expect(screen.getAllByText('Account 4').length).toBeGreaterThan(0)
    expect(screen.queryByText('Account 5')).not.toBeInTheDocument()
    expect(screen.getAllByText('Category 4').length).toBeGreaterThan(0)
    expect(screen.queryByText('Category 5')).not.toBeInTheDocument()
    expect(screen.getByText('Transaction 5')).toBeInTheDocument()
    expect(screen.queryByText('Transaction 6')).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'View all accounts' })).toHaveAttribute('href', '#/finance/accounts')
    expect(screen.getByRole('link', { name: 'View all categories' })).toHaveAttribute('href', '#/finance/categories')
    expect(screen.getByRole('link', { name: 'View all transactions' })).toHaveAttribute('href', '#/finance/transactions')
  })

  it('renders a dashboard error after tenant selection', async () => {
    mocks.getDashboard.mockRejectedValueOnce(new Error('dashboard exploded'))

    render(Finance)

    expect(await screen.findByRole('alert')).toHaveTextContent('dashboard exploded')
  })

})
