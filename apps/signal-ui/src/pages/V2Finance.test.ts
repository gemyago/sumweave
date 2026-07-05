import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import V2Finance from './V2Finance.svelte'
import V2FinanceSource from './V2Finance.svelte?raw'

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

describe('V2Finance', () => {
  beforeEach(() => {
    window.localStorage.clear()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants.mockReset()
    mocks.getDashboard.mockReset()
    mocks.listTransactions.mockReset()
    mocks.listConnections.mockReset()
    mocks.listTenants.mockResolvedValue([
      {
        id: 'tenant-1',
        name: 'Household',
        displayCurrency: 'USD',
        joinedAt: now,
        createdAt: now,
        updatedAt: now,
      },
    ])
    mocks.getDashboard.mockResolvedValue({
      period: {
        preset: 'current_month',
        startDate: now,
        endDate: now,
        previous: { startDate: now, endDate: now },
        next: { startDate: now, endDate: now },
      },
      settled: {
        displayCurrency: 'USD',
        incomeMinor: 120000,
        expenseMinor: 45000,
        netMinor: 75000,
        transactionCount: 12,
        complete: true,
      },
      pending: {
        displayCurrency: 'USD',
        incomeMinor: 0,
        expenseMinor: 5000,
        netMinor: -5000,
        transactionCount: 1,
        complete: true,
      },
      categoryBreakdowns: [
        {
          categoryId: 'cat-1',
          categoryName: 'Groceries',
          kind: 'expense',
          incomeMinor: 0,
          expenseMinor: 1000,
          transactionCount: 1,
        },
      ],
      accountBalances: [
        {
          accountId: 'acc-1',
          accountName: 'Checking',
          currency: 'USD',
          nativeBookedMinor: 50000,
          nativePendingMinor: 5000,
          displayBookedMinor: 50000,
          displayPendingMinor: 5000,
          missingFx: false,
        },
      ],
      alerts: [{ code: 'stale_connection', severity: 'warning', count: 1 }],
      missingFx: [
        {
          source: 'provider',
          transactionId: 'tx-1',
          accountId: 'acc-1',
          baseCurrency: 'EUR',
          quoteCurrency: 'USD',
          rateDate: now,
          provider: 'frankfurter',
        },
      ],
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

  it('renders the finance dashboard sections with finance navigation links', async () => {
    render(V2Finance)

    expect(await screen.findByRole('heading', { name: 'Finance dashboard' })).toBeInTheDocument()
    expect(await screen.findByText('Booked balance story')).toBeInTheDocument()
    expect(screen.getByText('Period context')).toBeInTheDocument()
    expect(screen.getByText('Income')).toBeInTheDocument()
    expect(screen.getByText('Expense')).toBeInTheDocument()
    expect(screen.getAllByText('Pending').length).toBeGreaterThan(0)
    expect(screen.getByText('Period flow')).toBeInTheDocument()
    expect(screen.getByLabelText('Cash flow chart')).toBeInTheDocument()
    expect(screen.getByText('Category breakdown')).toBeInTheDocument()
    expect(screen.getByLabelText('Category breakdown chart')).toBeInTheDocument()
    expect(screen.getByText('Largest balances')).toBeInTheDocument()
    expect(screen.getByLabelText('Account balances chart')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Recent transactions' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'What still needs review' })).toBeInTheDocument()
    expect(screen.getByText('Missing FX coverage')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open accounts' })).toHaveAttribute('href', '#/finance/accounts')
    expect(screen.getByRole('link', { name: 'Open transactions' })).toHaveAttribute('href', '#/finance/transactions')
    expect(screen.getByRole('link', { name: 'Open FX diagnostics' })).toHaveAttribute('href', '#/admin/finance/fx')
  })

  it('supports previous and custom period actions through the existing dashboard contract', async () => {
    const user = userEvent.setup()
    mocks.getDashboard
      .mockResolvedValueOnce({
        period: {
          preset: 'current_month',
          startDate: new Date('2026-06-20T12:00:00Z'),
          endDate: new Date('2026-06-20T12:00:00Z'),
          previous: {
            startDate: new Date('2026-05-01T00:00:00Z'),
            endDate: new Date('2026-05-31T00:00:00Z'),
          },
          next: {
            startDate: new Date('2026-07-01T00:00:00Z'),
            endDate: new Date('2026-07-31T00:00:00Z'),
          },
        },
        settled: {
          displayCurrency: 'USD',
          incomeMinor: 120000,
          expenseMinor: 45000,
          netMinor: 75000,
          transactionCount: 12,
          complete: true,
        },
        pending: {
          displayCurrency: 'USD',
          incomeMinor: 0,
          expenseMinor: 5000,
          netMinor: -5000,
          transactionCount: 1,
          complete: true,
        },
        categoryBreakdowns: [],
        accountBalances: [],
        alerts: [],
        missingFx: [],
        nativeSettledTotals: [],
      })
      .mockResolvedValueOnce({
        period: {
          preset: '',
          startDate: new Date('2026-05-01T00:00:00Z'),
          endDate: new Date('2026-05-31T00:00:00Z'),
          previous: {
            startDate: new Date('2026-04-01T00:00:00Z'),
            endDate: new Date('2026-04-30T00:00:00Z'),
          },
          next: {
            startDate: new Date('2026-06-01T00:00:00Z'),
            endDate: new Date('2026-06-30T00:00:00Z'),
          },
        },
        settled: {
          displayCurrency: 'USD',
          incomeMinor: 120000,
          expenseMinor: 45000,
          netMinor: 75000,
          transactionCount: 12,
          complete: true,
        },
        pending: {
          displayCurrency: 'USD',
          incomeMinor: 0,
          expenseMinor: 5000,
          netMinor: -5000,
          transactionCount: 1,
          complete: true,
        },
        categoryBreakdowns: [],
        accountBalances: [],
        alerts: [],
        missingFx: [],
        nativeSettledTotals: [],
      })
      .mockResolvedValueOnce({
        period: {
          preset: '',
          startDate: new Date('2026-06-01T00:00:00Z'),
          endDate: new Date('2026-06-30T00:00:00Z'),
          previous: {
            startDate: new Date('2026-05-01T00:00:00Z'),
            endDate: new Date('2026-05-31T00:00:00Z'),
          },
          next: {
            startDate: new Date('2026-07-01T00:00:00Z'),
            endDate: new Date('2026-07-31T00:00:00Z'),
          },
        },
        settled: {
          displayCurrency: 'USD',
          incomeMinor: 120000,
          expenseMinor: 45000,
          netMinor: 75000,
          transactionCount: 12,
          complete: true,
        },
        pending: {
          displayCurrency: 'USD',
          incomeMinor: 0,
          expenseMinor: 5000,
          netMinor: -5000,
          transactionCount: 1,
          complete: true,
        },
        categoryBreakdowns: [],
        accountBalances: [],
        alerts: [],
        missingFx: [],
        nativeSettledTotals: [],
      })

    render(V2Finance)

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
  })

  it('supports current-month and next-period actions', async () => {
    const user = userEvent.setup()
    render(V2Finance)

    await user.click(await screen.findByRole('button', { name: 'Current month' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(2))

    await user.click(screen.getByRole('button', { name: 'Next period' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(3))
  })

  it('shows the tenant-create prompt when no tenants exist', async () => {
    mocks.listTenants.mockResolvedValueOnce([])

    render(V2Finance)

    expect(await screen.findByText(/Create or join a tenant/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Finance tenants' })).toHaveAttribute('href', '#/finance/tenants')
  })

  it('requires explicit tenant selection when multiple tenants exist and none is active yet', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants.mockResolvedValueOnce([
      {
        id: 'tenant-1',
        name: 'Household',
        displayCurrency: 'USD',
        joinedAt: now,
        createdAt: now,
        updatedAt: now,
      },
      {
        id: 'tenant-2',
        name: 'Travel',
        displayCurrency: 'EUR',
        joinedAt: now,
        createdAt: now,
        updatedAt: now,
      },
    ])

    render(V2Finance)

    expect(
      await screen.findByText('Select an active tenant to continue on this finance route.'),
    ).toBeInTheDocument()
  })

  it('renders empty-state and no-attention fallbacks when the dashboard has no activity', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getDashboard.mockResolvedValueOnce({
      period: {
        preset: '',
        startDate: now,
        endDate: now,
        previous: { startDate: now, endDate: now },
        next: { startDate: now, endDate: now },
      },
      settled: {
        displayCurrency: 'USD',
        incomeMinor: 0,
        expenseMinor: 0,
        netMinor: 0,
        transactionCount: 0,
        complete: true,
      },
      pending: {
        displayCurrency: 'USD',
        incomeMinor: 0,
        expenseMinor: 0,
        netMinor: 0,
        transactionCount: 0,
        complete: true,
      },
      categoryBreakdowns: [],
      accountBalances: [],
      alerts: [],
      missingFx: [],
      nativeSettledTotals: [],
    })
    mocks.listTransactions.mockResolvedValueOnce([])
    mocks.listConnections.mockResolvedValueOnce([])

    render(V2Finance)

    expect(await screen.findByText('No settled or pending cash flow to chart for this period.')).toBeInTheDocument()
    expect(screen.getByText('No category activity to chart for this period.')).toBeInTheDocument()
    expect(screen.getByText('No account balances to chart yet.')).toBeInTheDocument()
    expect(screen.getByText('No recent transactions for this tenant yet.')).toBeInTheDocument()
    expect(screen.getByText('No active attention signals right now.')).toBeInTheDocument()
  })

  it('renders native totals, balance-empty copy, and failed-sync attention links when provided', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getDashboard.mockResolvedValueOnce({
      period: {
        preset: '',
        startDate: now,
        endDate: now,
        previous: { startDate: now, endDate: now },
        next: { startDate: now, endDate: now },
      },
      settled: {
        displayCurrency: 'USD',
        incomeMinor: 220000,
        expenseMinor: 60000,
        netMinor: 160000,
        transactionCount: 14,
        complete: true,
      },
      pending: {
        displayCurrency: 'USD',
        incomeMinor: 10000,
        expenseMinor: 4000,
        netMinor: 6000,
        transactionCount: 2,
        complete: true,
      },
      categoryBreakdowns: [
        {
          categoryId: 'cat-income',
          categoryName: 'Salary',
          kind: 'income',
          incomeMinor: 220000,
          expenseMinor: 0,
          transactionCount: 1,
        },
      ],
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

    render(V2Finance)

    expect(await screen.findByText('Native totals')).toBeInTheDocument()
    expect(screen.getByText('No booked balances yet')).toBeInTheDocument()
    expect(screen.getByText('Salary')).toBeInTheDocument()
    expect(screen.getByText('Failed sync')).toBeInTheDocument()
    expect(screen.getByText('Failed import')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Review imports' })).toHaveAttribute('href', '#/finance/imports')
    expect(screen.getByRole('link', { name: 'Review connections' })).toHaveAttribute('href', '#/finance/connections')
  })

  it('routes generic connection alerts and mixed transaction statuses through the dashboard sections', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getDashboard.mockResolvedValueOnce({
      period: {
        preset: '',
        startDate: now,
        endDate: now,
        previous: { startDate: now, endDate: now },
        next: { startDate: now, endDate: now },
      },
      settled: {
        displayCurrency: 'USD',
        incomeMinor: 5000,
        expenseMinor: 3000,
        netMinor: 2000,
        transactionCount: 2,
        complete: true,
      },
      pending: {
        displayCurrency: 'USD',
        incomeMinor: 1500,
        expenseMinor: 250,
        netMinor: 1250,
        transactionCount: 2,
        complete: true,
      },
      categoryBreakdowns: [
        {
          categoryId: 'cat-income',
          categoryName: 'Consulting',
          kind: 'income',
          incomeMinor: 5000,
          expenseMinor: 0,
          transactionCount: 1,
        },
        {
          categoryId: 'cat-expense',
          categoryName: 'Travel',
          kind: 'expense',
          incomeMinor: 0,
          expenseMinor: 3000,
          transactionCount: 1,
        },
      ],
      accountBalances: [
        {
          accountId: 'acc-1',
          accountName: 'Operating',
          currency: 'USD',
          nativeBookedMinor: 5000,
          nativePendingMinor: 1250,
          displayBookedMinor: 5000,
          displayPendingMinor: 1250,
          missingFx: false,
        },
        {
          accountId: 'acc-2',
          accountName: 'Travel wallet',
          currency: 'EUR',
          nativeBookedMinor: 1000,
          nativePendingMinor: 50,
          displayBookedMinor: 1100,
          displayPendingMinor: 55,
          missingFx: true,
        },
      ],
      alerts: [
        { code: 'connection_backlog', severity: 'warning', count: 1 },
        { code: 'background_note', severity: 'info', count: 1 },
      ],
      missingFx: [],
      nativeSettledTotals: [],
    })
    mocks.listTransactions.mockResolvedValueOnce([
      {
        id: 'tx-pending',
        tenantId: 'tenant-1',
        accountId: 'acc-1',
        source: 'manual',
        status: 'pending',
        kind: 'income',
        amountMinor: 1500,
        currency: 'USD',
        description: 'Incoming transfer',
        effectiveAt: now,
        categoryId: null,
        transferGroupId: null,
        transferMatchedAt: null,
        hiddenAt: null,
        providerOriginal: null,
        createdAt: now,
        updatedAt: now,
      },
      {
        id: 'tx-other',
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
    ])

    render(V2Finance)

    expect(await screen.findByText('Connection backlog')).toBeInTheDocument()
    expect(screen.getByText('Background note')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Review connections' })).toHaveAttribute('href', '#/finance/connections')
    expect(screen.getAllByText('Travel wallet').length).toBeGreaterThan(0)
    expect(screen.getByText('Missing FX')).toBeInTheDocument()
    expect(screen.getByText('Incoming transfer')).toBeInTheDocument()
    expect(screen.getByText('Hotel')).toBeInTheDocument()
  })

  it('renders a dashboard error after tenant selection', async () => {
    mocks.getDashboard.mockRejectedValueOnce(new Error('dashboard exploded'))

    render(V2Finance)

    expect(await screen.findByRole('alert')).toHaveTextContent('dashboard exploded')
  })

  it('does not define route-local styles or style attributes', () => {
    expect(V2FinanceSource).not.toMatch(/<style[\s>]/)
    expect(V2FinanceSource).not.toMatch(/\sstyle=/)
  })
})
