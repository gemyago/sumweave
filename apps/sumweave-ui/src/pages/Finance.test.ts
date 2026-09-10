import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import Finance from './Finance.svelte'
import { FinanceShellState } from '../lib/finance/shell-state.svelte'
import { dateInputValue } from '../lib/date-range'
import { formatFinanceDate } from '../lib/finance/format'

const mocks = vi.hoisted(() => ({
  listTenants: vi.fn(),
  getDashboard: vi.fn(),
  getCashFlowSeries: vi.fn(),
  listAccounts: vi.fn(),
  listTransactions: vi.fn(),
  listConnections: vi.fn(),
  chartSetOption: vi.fn(),
  shellState: null as FinanceShellState | null,
}))

function monthlyCashFlowSeries(year: number, month: number, monthCount: number) {
  return {
    period: { startDate: new Date(year, month, 1), endDate: new Date(year, month + monthCount, 1) },
    groupBy: 'month' as const,
    displayCurrency: 'USD',
    complete: true,
    missingFx: [],
    buckets: Array.from({ length: monthCount }, (_, index) => {
      const startDate = index === 2
        ? new Date(year, month + index - 1, 31)
        : new Date(year, month + index, 1)
      return {
        startDate,
        endDate: new Date(year, month + index + 1, 1),
        incomeMinor: 100,
        expenseMinor: 50,
      }
    }),
  }
}

vi.mock('../lib/finance/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/finance/api')>()
  return {
    ...actual,
    createSignalFinanceApiForAuth: vi.fn(() => ({
      listTenants: mocks.listTenants,
      getDashboard: mocks.getDashboard,
      getCashFlowSeries: mocks.getCashFlowSeries,
      listAccounts: mocks.listAccounts,
      listTransactions: mocks.listTransactions,
      listConnections: mocks.listConnections,
    })),
  }
})

vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

vi.mock('../lib/finance/shell-state.svelte', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/finance/shell-state.svelte')>()),
  useFinanceShellState: vi.fn(() => mocks.shellState!),
}))

vi.mock('echarts/core', () => ({
  init: vi.fn((element: HTMLElement) => ({
    dispose: vi.fn(),
    resize: vi.fn(),
    setOption: (option: { xAxis?: { data?: string[] } }) => {
      mocks.chartSetOption(option)
      element.replaceChildren(...(option.xAxis?.data ?? []).map((label) => {
        const tick = document.createElement('span')
        tick.textContent = label
        return tick
      }))
    },
  })),
  use: vi.fn(),
}))

vi.mock('echarts/charts', () => ({ BarChart: class BarChart {} }))
vi.mock('echarts/components', () => ({
  AriaComponent: class AriaComponent {},
  GridComponent: class GridComponent {},
  LegendComponent: class LegendComponent {},
  TooltipComponent: class TooltipComponent {},
}))
vi.mock('echarts/renderers', () => ({ SVGRenderer: class SVGRenderer {} }))

describe('Finance dashboard page', () => {
  beforeEach(() => {
    window.localStorage.clear()
    window.location.hash = '#/finance'
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants.mockReset()
    mocks.getDashboard.mockReset()
    mocks.getCashFlowSeries.mockReset()
    mocks.listAccounts.mockReset()
    mocks.listTransactions.mockReset()
    mocks.listConnections.mockReset()
    mocks.chartSetOption.mockReset()
    mocks.shellState = new FinanceShellState()
    mocks.listTenants.mockResolvedValue([
      { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now },
    ])
    mocks.getDashboard.mockResolvedValue({
      period: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 21) },
      settled: { displayCurrency: 'USD', incomeMinor: 120000, expenseMinor: 45000, netMinor: 75000, transactionCount: 12, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 5000, netMinor: -5000, transactionCount: 1, complete: true },
      categoryBreakdowns: [{ categoryId: 'cat-1', categoryName: 'Groceries', kind: 'expense', incomeMinor: 0, expenseMinor: 1000, transactionCount: 1 }],
      accountBalances: [{ accountId: 'acc-1', accountName: 'Checking', currency: 'USD', nativeBookedMinor: 50000, nativePendingMinor: 5000, displayBookedMinor: 50000, displayPendingMinor: 5000, missingFx: false }],
      alerts: [{ code: 'stale_connection', severity: 'warning', count: 1 }],
      fxCoverage: [{ provider: 'frankfurter', baseCurrency: 'EUR', quoteCurrency: 'USD', affectedTransactionCount: 1, affectedAccountCount: 0 }],
      nativeSettledTotals: [],
    })
    mocks.listAccounts.mockResolvedValue([
      { id: 'acc-1', tenantId: 'tenant-1', name: 'Checking', currency: 'USD', kind: 'manual', bookedBalanceMinor: 50000, pendingBalanceMinor: 5000, hiddenAt: null, createdAt: now, updatedAt: now },
    ])
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
    mocks.getCashFlowSeries.mockResolvedValue({
      period: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 21) },
      groupBy: 'day',
      displayCurrency: 'USD',
      complete: true,
      missingFx: [],
      buckets: [{ startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 21), incomeMinor: 120000, expenseMinor: 45000 }],
    })
  })

  it('renders the canonical bootstrap dashboard with period-net summaries and canonical finance links', async () => {
    render(Finance)

    expect(await screen.findByRole('heading', { name: 'Finance dashboard' })).toBeInTheDocument()
    expect(await screen.findByText('Period net')).toBeInTheDocument()
    expect(screen.getByText('Income minus expenses for Jun 20, 2026 → Jun 20, 2026.')).toBeInTheDocument()
    expect(screen.getByText('Booked balance total')).toBeInTheDocument()
    expect(screen.getAllByText('Income').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Expense').length).toBeGreaterThan(0)
    expect(screen.getByText('Pending net')).toBeInTheDocument()
    expect(screen.getByText('Cash-flow visual')).toBeInTheDocument()
    expect(screen.getByLabelText('Cash flow chart')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Top categories' })).toBeInTheDocument()
    expect(screen.getByLabelText('Category breakdown chart')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Largest balances' })).toBeInTheDocument()
    expect(screen.getByLabelText('Account balances chart')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Transactions' })).toBeInTheDocument()
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

  it('renders the independently loaded cash-flow series with its complete textual alternative', async () => {
    window.location.hash = '#/finance?startDate=2026-06-20&endDate=2026-06-20'
    render(Finance)

    expect(await screen.findByRole('heading', { name: 'Cash flow over time' })).toBeInTheDocument()
    await waitFor(() => expect(mocks.getCashFlowSeries).toHaveBeenCalledOnce())
    expect(mocks.getCashFlowSeries).toHaveBeenCalledWith({
      tenantId: 'tenant-1',
      startDate: new Date(2026, 5, 20),
      endDate: new Date(2026, 5, 21),
      groupBy: 'day',
    })
    expect(screen.getByRole('img', { name: 'Cash flow chart' })).toBeInTheDocument()
    expect(screen.getByText('Jun 20, 2026 → Jun 20, 2026: Income 1200.00 USD · Expense 450.00 USD')).toBeInTheDocument()
  })

  it('uses the full dashboard row for period performance and prevents ECharts emphasis blur from fading either cash-flow series', async () => {
    render(Finance)

    const periodPerformance = await screen.findByText('Period performance')
    expect(periodPerformance.closest('.col-12')).toHaveClass('col-12')
    expect(periodPerformance.closest('.col-12')).not.toHaveClass('col-xxl-7')

    await waitFor(() => expect(mocks.chartSetOption).toHaveBeenCalled())
    const option = mocks.chartSetOption.mock.calls.at(-1)?.[0] as {
      tooltip: { confine?: boolean }
      series: Array<{
        emphasis?: { focus?: string; itemStyle?: { color?: string; opacity?: number } }
        blur?: { itemStyle?: { color?: string; opacity?: number } }
      }>
    }
    expect(option.tooltip.confine).toBe(true)
    expect(option.series).toEqual([
      expect.objectContaining({
        emphasis: { focus: 'none', itemStyle: { color: 'var(--color-success)', opacity: 1 } },
        blur: { itemStyle: { color: 'var(--color-success)', opacity: 1 } },
      }),
      expect.objectContaining({
        emphasis: { focus: 'none', itemStyle: { color: 'var(--color-danger)', opacity: 1 } },
        blur: { itemStyle: { color: 'var(--color-danger)', opacity: 1 } },
      }),
    ])
  })

  it('requests monthly series for aligned longer reporting periods and keeps their bounds in the URL', async () => {
    const user = userEvent.setup()
    render(Finance)

    await user.click(await screen.findByRole('button', { name: 'Last 6 months' }))
    await waitFor(() => expect(mocks.getCashFlowSeries).toHaveBeenCalledTimes(2))
    expect(mocks.getCashFlowSeries.mock.calls[1][0]).toMatchObject({ groupBy: 'month' })
    expect(window.location.hash).toContain('startDate=')
    expect(window.location.hash).toContain('endDate=')

    await user.click(screen.getByRole('button', { name: 'Last 12 months' }))
    await waitFor(() => expect(mocks.getCashFlowSeries).toHaveBeenCalledTimes(3))
    expect(mocks.getCashFlowSeries.mock.calls[2][0]).toMatchObject({ groupBy: 'month' })
  })

  it('labels six- and twelve-month charts with their included months rather than bucket dates', async () => {
    const user = userEvent.setup()
    mocks.getCashFlowSeries
      .mockResolvedValueOnce({
        period: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 21) },
        groupBy: 'day', displayCurrency: 'USD', complete: true, missingFx: [],
        buckets: [{ startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 21), incomeMinor: 120000, expenseMinor: 45000 }],
      })
      .mockResolvedValueOnce(monthlyCashFlowSeries(2026, 3, 6))
      .mockResolvedValueOnce(monthlyCashFlowSeries(2025, 9, 12))

    render(Finance)

    await user.click(await screen.findByRole('button', { name: 'Last 6 months' }))
    expect(await screen.findByText('Apr 2026')).toBeInTheDocument()
    expect(screen.getByText('May 31, 2026 → Jun 30, 2026: Income 1.00 USD · Expense 0.50 USD')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Last 12 months' }))
    expect(await screen.findByText('Oct 2025')).toBeInTheDocument()
  })

  it('keeps a series failure local to the chart and retries it without reloading the dashboard', async () => {
    const user = userEvent.setup()
    mocks.getCashFlowSeries.mockRejectedValueOnce(new Error('series exploded'))

    render(Finance)

    expect(await screen.findByText('series exploded')).toBeInTheDocument()
    expect(screen.getByText('Period net')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Retry cash-flow chart' }))
    await waitFor(() => expect(mocks.getCashFlowSeries).toHaveBeenCalledTimes(2))
    expect(mocks.getDashboard).toHaveBeenCalledOnce()
  })

  it('shows grouped missing-FX diagnostics in the chart-local partial-data warning', async () => {
    mocks.getCashFlowSeries.mockResolvedValueOnce({
      period: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 21) },
      groupBy: 'day', displayCurrency: 'USD', complete: false,
      missingFx: [{ provider: 'frankfurter', baseCurrency: 'EUR', quoteCurrency: 'USD', affectedTransactionCount: 2 }],
      buckets: [{ startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 21), incomeMinor: 120000, expenseMinor: 45000 }],
    })

    render(Finance)

    const warning = (await screen.findByText('Cash-flow chart data is incomplete.')).parentElement!
    expect(warning).toHaveTextContent('EUR → USD (frankfurter, 2 transaction values)')
  })

  it('shows incomplete all-zero cash-flow diagnostics before zero activity and retains the textual alternative', async () => {
    mocks.getCashFlowSeries.mockResolvedValueOnce({
      period: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 21) },
      groupBy: 'day', displayCurrency: 'USD', complete: false,
      missingFx: [{ provider: 'frankfurter', baseCurrency: 'EUR', quoteCurrency: 'USD', affectedTransactionCount: 2 }],
      buckets: [{ startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 21), incomeMinor: 0, expenseMinor: 0 }],
    })

    render(Finance)

    const warning = await screen.findByText('Cash-flow chart data is incomplete.')
    const zeroActivity = screen.getByText('No settled cash flow to chart for this period.')
    expect(warning.compareDocumentPosition(zeroActivity) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(warning.parentElement).toHaveTextContent('EUR → USD (frankfurter, 2 transaction values)')
    expect(screen.getByText('Jun 20, 2026 → Jun 20, 2026: Income 0.00 USD · Expense 0.00 USD')).toBeInTheDocument()
  })

  it('does not let a stale series response replace a newer reporting period', async () => {
    const user = userEvent.setup()
    let resolveInitialSeries: (value: unknown) => void
    const initialSeries = new Promise((resolve) => {
      resolveInitialSeries = resolve
    })
    mocks.getCashFlowSeries.mockReturnValueOnce(initialSeries).mockResolvedValueOnce({
      period: { startDate: new Date(2026, 0, 1), endDate: new Date(2026, 6, 1) },
      groupBy: 'month', displayCurrency: 'USD', complete: true, missingFx: [],
      buckets: [{ startDate: new Date(2026, 0, 1), endDate: new Date(2026, 1, 1), incomeMinor: 60000, expenseMinor: 20000 }],
    })

    render(Finance)
    await user.click(await screen.findByRole('button', { name: 'Last 6 months' }))
    await waitFor(() => expect(mocks.getCashFlowSeries).toHaveBeenCalledTimes(2))
    expect(await screen.findByText('Jan 1, 2026 → Jan 31, 2026: Income 600.00 USD · Expense 200.00 USD')).toBeInTheDocument()

    resolveInitialSeries!({
      period: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 21) },
      groupBy: 'day', displayCurrency: 'USD', complete: true, missingFx: [],
      buckets: [{ startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 21), incomeMinor: 99900, expenseMinor: 0 }],
    })

    await waitFor(() => expect(screen.queryByText('Jun 20, 2026 → Jun 20, 2026: Income 999.00 USD · Expense 0.00 USD')).not.toBeInTheDocument())
  })

  it('keeps hidden-account history named and labeled without restoring it to current balances', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listAccounts.mockResolvedValueOnce([
      { id: 'acc-1', tenantId: 'tenant-1', name: 'Checking', currency: 'USD', kind: 'manual', bookedBalanceMinor: 50000, pendingBalanceMinor: 5000, hiddenAt: null, createdAt: now, updatedAt: now },
      { id: 'acc-hidden', tenantId: 'tenant-1', name: 'Old checking', currency: 'USD', kind: 'linked', bookedBalanceMinor: 10000, pendingBalanceMinor: 0, hiddenAt: now, createdAt: now, updatedAt: now },
    ])
    mocks.listTransactions.mockResolvedValueOnce([{
      id: 'tx-hidden', tenantId: 'tenant-1', accountId: 'acc-hidden', source: 'provider', status: 'booked', kind: 'expense', amountMinor: -1200, currency: 'USD', description: 'Historical purchase', effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, createdAt: now, updatedAt: now,
    }])

    render(Finance)

    expect(await screen.findByText('Old checking')).toBeInTheDocument()
    expect(screen.getByText('Hidden account')).toBeInTheDocument()
    expect(mocks.listAccounts).toHaveBeenCalledWith({ tenantId: 'tenant-1', includeHidden: true })
    expect(screen.queryByText('Unknown account')).not.toBeInTheDocument()
  })

  it('renders compact needs-attention items for pending, missing FX, failed sync, and failed import signals', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getDashboard.mockResolvedValue({
      period: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20) },
      settled: { displayCurrency: 'USD', incomeMinor: 120000, expenseMinor: 45000, netMinor: 75000, transactionCount: 12, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 5000, netMinor: -5000, transactionCount: 3, complete: true },
      categoryBreakdowns: [{ categoryId: 'cat-1', categoryName: 'Groceries', kind: 'expense', incomeMinor: 0, expenseMinor: 1000, transactionCount: 1 }],
      accountBalances: [{ accountId: 'acc-1', accountName: 'Checking', currency: 'USD', nativeBookedMinor: 50000, nativePendingMinor: 5000, displayBookedMinor: 50000, displayPendingMinor: 5000, missingFx: false }],
      alerts: [{ code: 'failed_import', severity: 'error', count: 2 }],
      fxCoverage: [{ provider: 'frankfurter', baseCurrency: 'EUR', quoteCurrency: 'USD', affectedTransactionCount: 1, affectedAccountCount: 0 }],
      nativeSettledTotals: [],
    })
    mocks.listConnections.mockResolvedValueOnce([
      {
        id: 'conn-2',
        tenantId: 'tenant-1',
        provider: 'synthetic',
        displayName: 'Broken sync',
        providerReference: 'ref-2',
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
      period: { startDate: new Date(2026, 5, 1), endDate: new Date(2026, 6, 1) },
      settled: { displayCurrency: 'PLN', incomeMinor: 100, expenseMinor: 200, netMinor: -100, transactionCount: 2, complete: false },
      pending: { displayCurrency: 'PLN', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      categoryBreakdowns: [], accountBalances: [], alerts: [],
      fxCoverage: [
        { provider: 'frankfurter', baseCurrency: 'EUR', quoteCurrency: 'PLN', affectedTransactionCount: 2, affectedAccountCount: 1 },
        { provider: 'frankfurter', baseCurrency: 'USD', quoteCurrency: 'PLN', affectedTransactionCount: 1, affectedAccountCount: 0 },
      ],
      nativeSettledTotals: [],
    })

    render(Finance)

    const warning = (await screen.findByText('Income and expense totals are incomplete.')).parentElement!
    expect(warning).toHaveTextContent('Income and expense totals are incomplete.')
    expect(warning).toHaveTextContent('FX coverage missing for 2 pairs (EUR → PLN, USD → PLN), affecting 4 values.')
    expect(screen.getByText('EUR → PLN · frankfurter · 2 transaction values · 1 account value')).toBeInTheDocument()
    expect(screen.getByText('USD → PLN · frankfurter · 1 transaction value · 0 account values')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open FX diagnostics' })).toHaveAttribute('href', '#/admin/finance/fx')
    expect(screen.getAllByText('Income')[0].compareDocumentPosition(warning) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
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

  it('navigates backward one local calendar month per click', async () => {
    const user = userEvent.setup()
    const dashboardForMonth = (month: number) => ({
      period: {
        startDate: new Date(2026, month, 1),
        endDate: new Date(2026, month + 1, 1),
      },
      settled: { displayCurrency: 'USD', incomeMinor: 120000, expenseMinor: 45000, netMinor: 75000, transactionCount: 12, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 5000, netMinor: -5000, transactionCount: 1, complete: true },
      categoryBreakdowns: [], accountBalances: [], alerts: [], fxCoverage: [], nativeSettledTotals: [],
    })
    mocks.getDashboard
      .mockResolvedValueOnce(dashboardForMonth(5))
      .mockResolvedValueOnce(dashboardForMonth(4))
      .mockResolvedValueOnce(dashboardForMonth(3))

    render(Finance)

    await user.click(await screen.findByRole('button', { name: 'Previous month' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(2))
    expect(mocks.getDashboard.mock.calls[1][0]).toEqual({ tenantId: 'tenant-1', startDate: new Date(2026, 4, 1), endDate: new Date(2026, 5, 1) })

    await user.click(screen.getByRole('button', { name: 'Previous month' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(3))
    expect(mocks.getDashboard.mock.calls[2][0]).toEqual({ tenantId: 'tenant-1', startDate: new Date(2026, 3, 1), endDate: new Date(2026, 4, 1) })

    await user.click(screen.getByText('Custom range'))
    expect(screen.getByLabelText('Custom start date')).toHaveValue('2026-04-01')
    expect(screen.getByLabelText('Custom end date')).toHaveValue('2026-04-30')
  })

  it('restores a direct dashboard date-range URL and requests its exclusive end boundary', async () => {
    window.location.hash = '#/finance?startDate=2026-05-10&endDate=2026-05-12'

    render(Finance)

    await screen.findByRole('heading', { name: 'Finance dashboard' })
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledOnce())
    const request = mocks.getDashboard.mock.calls[0][0]
    expect(request.tenantId).toBe('tenant-1')
    expect(dateInputValue(request.startDate)).toBe('2026-05-10')
    expect(dateInputValue(request.endDate)).toBe('2026-05-13')
  })

  it('pages dashboard transactions inside the selected range and carries that range to the ledger', async () => {
    const user = userEvent.setup()
    window.location.hash = '#/finance?startDate=2026-06-01&endDate=2026-06-30'
    const transactions = (offset: number, count = 6) => Array.from({ length: count }, (_, index) => ({
      id: `tx-${offset + index}`,
      tenantId: 'tenant-1', accountId: 'acc-1', source: 'manual', status: 'booked', kind: 'expense', amountMinor: -100,
      currency: 'USD', description: `Transaction ${offset + index}`, effectiveAt: new Date(2026, 5, 30 - offset - index), categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: new Date(), updatedAt: new Date(),
    }))
    mocks.listTransactions.mockResolvedValueOnce(transactions(0)).mockResolvedValueOnce(transactions(5, 5)).mockResolvedValueOnce(transactions(0))
    mocks.getDashboard.mockResolvedValueOnce({
      period: { startDate: new Date(2026, 5, 1), endDate: new Date(2026, 6, 1) },
      settled: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      categoryBreakdowns: [], accountBalances: [], alerts: [], fxCoverage: [], nativeSettledTotals: [],
    })

    render(Finance)
    expect(await screen.findByText('Transaction 0')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'View all transactions' })).toHaveAttribute('href', '#/finance/transactions?startDate=2026-06-01&endDate=2026-06-30')

    await user.click(screen.getByRole('button', { name: 'Dashboard transaction pages: older page' }))
    expect(await screen.findByText('Transaction 5')).toBeInTheDocument()
    expect(mocks.listTransactions).toHaveBeenLastCalledWith(expect.objectContaining({
      startDate: new Date(2026, 5, 1), endDate: new Date(2026, 6, 1), limit: 6, offset: 5,
    }))

    await user.click(screen.getByRole('button', { name: 'Dashboard transaction pages: newer page' }))
    expect(await screen.findByText('Transaction 0')).toBeInTheDocument()
    expect(mocks.listTransactions).toHaveBeenLastCalledWith(expect.objectContaining({ offset: 0 }))
  })

  it('keeps newer navigation available on a full final dashboard page', async () => {
    const user = userEvent.setup()
    const transactions = (offset: number, count: number) => Array.from({ length: count }, (_, index) => ({
      id: `tx-${offset + index}`,
      tenantId: 'tenant-1', accountId: 'acc-1', source: 'manual', status: 'booked', kind: 'expense', amountMinor: -100,
      currency: 'USD', description: `Transaction ${offset + index}`, effectiveAt: new Date(2026, 5, 30 - offset - index), categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: new Date(), updatedAt: new Date(),
    }))
    mocks.listTransactions.mockResolvedValueOnce(transactions(0, 6)).mockResolvedValueOnce(transactions(5, 5))

    render(Finance)

    await user.click(await screen.findByRole('button', { name: 'Dashboard transaction pages: older page' }))

    expect(await screen.findByText('Transaction 5')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Dashboard transaction pages: older page' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Dashboard transaction pages: newer page' })).toBeEnabled()
    expect(mocks.listTransactions).toHaveBeenCalledTimes(2)
  })

  it('navigates forward one local calendar month per click', async () => {
    const user = userEvent.setup()
    const dashboardForMonth = (month: number) => ({
      period: {
        startDate: new Date(2026, month, 1),
        endDate: new Date(2026, month + 1, 1),
      },
      settled: { displayCurrency: 'USD', incomeMinor: 120000, expenseMinor: 45000, netMinor: 75000, transactionCount: 12, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 5000, netMinor: -5000, transactionCount: 1, complete: true },
      categoryBreakdowns: [], accountBalances: [], alerts: [], fxCoverage: [], nativeSettledTotals: [],
    })
    mocks.getDashboard
      .mockResolvedValueOnce(dashboardForMonth(5))
      .mockResolvedValueOnce(dashboardForMonth(6))
      .mockResolvedValueOnce(dashboardForMonth(7))

    render(Finance)

    await user.click(await screen.findByRole('button', { name: 'Next month' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(2))
    expect(mocks.getDashboard.mock.calls[1][0]).toEqual({ tenantId: 'tenant-1', startDate: new Date(2026, 6, 1), endDate: new Date(2026, 7, 1) })

    await user.click(screen.getByRole('button', { name: 'Next month' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(3))
    expect(mocks.getDashboard.mock.calls[2][0]).toEqual({ tenantId: 'tenant-1', startDate: new Date(2026, 7, 1), endDate: new Date(2026, 8, 1) })

    await user.click(screen.getByText('Custom range'))
    expect(screen.getByLabelText('Custom start date')).toHaveValue('2026-08-01')
    expect(screen.getByLabelText('Custom end date')).toHaveValue('2026-08-31')
  })

  it('disables period actions until a delayed previous-month request completes', async () => {
    const user = userEvent.setup()
    const dashboardForMonth = (month: number) => ({
      period: {
        startDate: new Date(2026, month, 1),
        endDate: new Date(2026, month + 1, 1),
      },
      settled: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      categoryBreakdowns: [], accountBalances: [], alerts: [], fxCoverage: [], nativeSettledTotals: [],
    })
    let resolveDelayedDashboard: (dashboard: ReturnType<typeof dashboardForMonth>) => void
    const delayedDashboard = new Promise<ReturnType<typeof dashboardForMonth>>((resolve) => {
      resolveDelayedDashboard = resolve
    })
    mocks.getDashboard
      .mockResolvedValueOnce(dashboardForMonth(5))
      .mockReturnValueOnce(delayedDashboard)

    render(Finance)

    const previousButton = await screen.findByRole('button', { name: 'Previous month' })
    await user.click(previousButton)
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(2))
    expect(previousButton).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Current month' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Next month' })).toBeDisabled()

    await user.click(previousButton)
    expect(mocks.getDashboard).toHaveBeenCalledTimes(2)
    expect(mocks.getDashboard.mock.calls[1][0]).toMatchObject({ tenantId: 'tenant-1' })

    resolveDelayedDashboard!(dashboardForMonth(4))
    await waitFor(() => expect(previousButton).toBeEnabled())
  })

  it('keeps tenant-local month navigation successive and ignores a stale dashboard response', async () => {
    const user = userEvent.setup()
    const now = new Date('2026-06-20T12:00:00Z')
    const dashboardForMonth = (month: number) => ({
      period: {
        startDate: new Date(2026, month, 1),
        endDate: new Date(2026, month + 1, 1),
      },
      settled: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      categoryBreakdowns: [], accountBalances: [], alerts: [], fxCoverage: [], nativeSettledTotals: [],
    })
    window.localStorage.setItem('sumweave-ui-finance-tenant-id', 'tenant-1')
    mocks.listTenants.mockResolvedValueOnce([
      { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now },
      { id: 'tenant-2', name: 'Travel', displayCurrency: 'EUR', joinedAt: now, createdAt: now, updatedAt: now },
    ])
    let resolveStaleDashboard: (dashboard: ReturnType<typeof dashboardForMonth>) => void
    const staleDashboard = new Promise<ReturnType<typeof dashboardForMonth>>((resolve) => {
      resolveStaleDashboard = resolve
    })
    mocks.getDashboard
      .mockResolvedValueOnce(dashboardForMonth(5))
      .mockResolvedValueOnce(dashboardForMonth(4))
      .mockReturnValueOnce(staleDashboard)
      .mockResolvedValueOnce(dashboardForMonth(4))
      .mockResolvedValueOnce(dashboardForMonth(3))
      .mockResolvedValueOnce(dashboardForMonth(2))

    render(Finance)

    await user.click(await screen.findByRole('button', { name: 'Previous month' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(2))
    expect(mocks.getDashboard.mock.calls[1][0]).toMatchObject({ tenantId: 'tenant-1' })

    await user.click(screen.getByRole('button', { name: 'Previous month' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(3))

    mocks.shellState!.selectTenant('tenant-2')
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(4))
    expect(mocks.getDashboard.mock.calls[3][0]).toEqual({
      tenantId: 'tenant-2',
      startDate: new Date(2026, 4, 1),
      endDate: new Date(2026, 5, 1),
    })

    await user.click(screen.getByText('Custom range'))
    expect(screen.getByLabelText('Custom start date')).toHaveValue('2026-05-01')
    expect(screen.getByLabelText('Custom end date')).toHaveValue('2026-05-31')

    await user.click(screen.getByRole('button', { name: 'Previous month' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(5))
    expect(mocks.getDashboard.mock.calls[4][0]).toEqual({ tenantId: 'tenant-2', startDate: new Date(2026, 3, 1), endDate: new Date(2026, 4, 1) })
    expect(screen.getByLabelText('Custom start date')).toHaveValue('2026-04-01')
    expect(screen.getByLabelText('Custom end date')).toHaveValue('2026-04-30')

    resolveStaleDashboard!(dashboardForMonth(1))
    await waitFor(() => expect(screen.getByLabelText('Custom start date')).toHaveValue('2026-04-01'))

    await user.click(screen.getByRole('button', { name: 'Previous month' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(6))
    expect(mocks.getDashboard.mock.calls[5][0]).toEqual({ tenantId: 'tenant-2', startDate: new Date(2026, 2, 1), endDate: new Date(2026, 3, 1) })
    expect(screen.getByLabelText('Custom start date')).toHaveValue('2026-03-01')
    expect(screen.getByLabelText('Custom end date')).toHaveValue('2026-03-31')
  })

  it('requests explicit ranges for current and next month', async () => {
    const user = userEvent.setup()
    render(Finance)

    await user.click(await screen.findByRole('button', { name: 'Current month' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(2))
    expect(mocks.getDashboard.mock.calls[1][0]).toMatchObject({ tenantId: 'tenant-1', startDate: expect.any(Date), endDate: expect.any(Date) })

    await user.click(screen.getByText('Custom range'))
    expect(screen.getByLabelText('Custom start date')).toHaveValue('2026-06-20')
    expect(screen.getByLabelText('Custom end date')).toHaveValue('2026-06-20')

    await user.click(screen.getByRole('button', { name: 'Next month' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(3))
    expect(mocks.getDashboard.mock.calls[2][0]).toMatchObject({ tenantId: 'tenant-1', startDate: expect.any(Date), endDate: expect.any(Date) })
  })

  it('returns to month navigation after applying a custom range', async () => {
    const user = userEvent.setup()
    render(Finance)

    await user.click(await screen.findByRole('button', { name: 'Previous month' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(2))

    await user.click(screen.getByText('Custom range'))
    await user.click(screen.getByRole('button', { name: 'Apply' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(3))

    await user.click(screen.getByRole('button', { name: 'Next month' }))
    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(4))
    expect(mocks.getDashboard.mock.calls[3][0]).toMatchObject({ tenantId: 'tenant-1', startDate: expect.any(Date), endDate: expect.any(Date) })
  })

  it('shows the calendar day before an exclusive local-midnight dashboard end', async () => {
    mocks.getDashboard.mockResolvedValueOnce({
      period: { startDate: new Date(2026, 5, 1), endDate: new Date(2026, 6, 1) },
      settled: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      categoryBreakdowns: [], accountBalances: [], alerts: [], fxCoverage: [], nativeSettledTotals: [],
    })

    render(Finance)

    expect(await screen.findByText('Jun 1, 2026 → Jun 30, 2026')).toBeInTheDocument()
  })

  it('keeps a non-midnight dashboard end date visible and unchanged until custom input changes it', async () => {
    const user = userEvent.setup()
    const startDate = new Date('2026-06-01T09:45:12.345Z')
    const endDate = new Date('2026-06-30T13:15:42.987Z')
    const dashboard = {
      period: { startDate, endDate },
      settled: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      categoryBreakdowns: [], accountBalances: [], alerts: [], fxCoverage: [], nativeSettledTotals: [],
    }
    mocks.getDashboard.mockResolvedValueOnce(dashboard).mockResolvedValueOnce(dashboard)

    render(Finance)
    expect(await screen.findByText(`${formatFinanceDate(startDate)} → ${formatFinanceDate(endDate)}`)).toBeInTheDocument()
    await user.click(await screen.findByText('Custom range'))
    expect(screen.getByLabelText('Custom end date')).toHaveValue(dateInputValue(endDate))
    await user.click(screen.getByRole('button', { name: 'Apply' }))

    await waitFor(() => expect(mocks.getDashboard).toHaveBeenCalledTimes(2))
    const request = mocks.getDashboard.mock.calls[1][0]
    expect(request.startDate).toBeInstanceOf(Date)
    expect(request.endDate).toBeInstanceOf(Date)
    expect(request.startDate).toEqual(startDate)
    expect(request.endDate).toEqual(endDate)
  })

  it('uses exclusive next-day bounds for selected inclusive custom dates', async () => {
    const user = userEvent.setup()
    const importedAtMidnight = new Date(2026, 4, 29, 0, 0, 0, 0)
    const initialDashboard = {
      period: {
        startDate: new Date(2026, 4, 1, 9, 40),
        endDate: new Date(2026, 5, 3, 9, 40),
      },
      settled: { displayCurrency: 'PLN', incomeMinor: 0, expenseMinor: 830000, netMinor: -830000, transactionCount: 1, complete: true },
      pending: { displayCurrency: 'PLN', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      categoryBreakdowns: [], accountBalances: [], alerts: [], fxCoverage: [], nativeSettledTotals: [{ currency: 'PLN', incomeMinor: 0, expenseMinor: 830000, netMinor: -830000 }],
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
    expect(request.endDate).toEqual(new Date(2026, 5, 4, 0, 0, 0, 0))
    expect(importedAtMidnight.getTime()).toBeGreaterThanOrEqual(request.startDate.getTime())
    expect(importedAtMidnight.getTime()).toBeLessThan(request.endDate.getTime())
    expect(await screen.findByText('PLN')).toBeInTheDocument()
  })

  it('renders honest empty states when the dashboard has no activity', async () => {
    mocks.getDashboard.mockResolvedValueOnce({
      period: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20) },
      settled: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      categoryBreakdowns: [],
      accountBalances: [],
      alerts: [],
      fxCoverage: [],
      nativeSettledTotals: [],
    })
    mocks.listTransactions.mockResolvedValueOnce([])
    mocks.listConnections.mockResolvedValueOnce([])
    mocks.getCashFlowSeries.mockResolvedValueOnce({
      period: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 21) },
      groupBy: 'day', displayCurrency: 'USD', complete: true, missingFx: [],
      buckets: [{ startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 21), incomeMinor: 0, expenseMinor: 0 }],
    })

    render(Finance)

    expect(await screen.findByText('No settled cash flow to chart for this period.')).toBeInTheDocument()
    expect(screen.getByText('No category activity to chart for this period.')).toBeInTheDocument()
    expect(screen.getByText('No account balances to chart yet.')).toBeInTheDocument()
    expect(screen.getByText('No transactions in this reporting period.')).toBeInTheDocument()
    expect(screen.getByText('No active attention signals right now.')).toBeInTheDocument()
    expect(screen.getByText('No booked account balances yet. Connect or create accounts to start tracking balances here.')).toBeInTheDocument()
  })

  it('routes native totals, sync issues, and import follow-up through the dashboard attention area', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getDashboard.mockResolvedValueOnce({
      period: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20) },
      settled: { displayCurrency: 'USD', incomeMinor: 220000, expenseMinor: 60000, netMinor: 160000, transactionCount: 14, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 10000, expenseMinor: 4000, netMinor: 6000, transactionCount: 2, complete: true },
      categoryBreakdowns: [{ categoryId: 'cat-income', categoryName: 'Salary', kind: 'income', incomeMinor: 220000, expenseMinor: 0, transactionCount: 1 }],
      accountBalances: [],
      alerts: [{ code: 'failed_import', severity: 'error', count: 2 }],
      fxCoverage: [],
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
    expect(screen.getByText('No booked account balances yet. Connect or create accounts to start tracking balances here.')).toBeInTheDocument()
    expect(screen.getByText('Salary')).toBeInTheDocument()
    expect(screen.getByText('Failed sync')).toBeInTheDocument()
    expect(screen.getByText('Failed import')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Review imports' })).toHaveAttribute('href', '#/finance/imports')
    expect(screen.getByRole('link', { name: 'Review connections' })).toHaveAttribute('href', '#/finance/connections')
  })

  it('shows account-level missing FX badges and tolerates mixed connection timestamp fallbacks', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getDashboard.mockResolvedValueOnce({
      period: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20) },
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
      fxCoverage: [],
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
      period: { startDate: new Date(2026, 5, 1), endDate: new Date(2026, 6, 1) },
      settled: { displayCurrency: 'PLN', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      pending: { displayCurrency: 'PLN', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      categoryBreakdowns: [],
      accountBalances: [
        { accountId: 'pln', accountName: 'PLN', currency: 'PLN', nativeBookedMinor: 10000, nativePendingMinor: 0, displayBookedMinor: 10000, displayPendingMinor: 0, missingFx: false },
        { accountId: 'eur', accountName: 'EUR', currency: 'EUR', nativeBookedMinor: 20000, nativePendingMinor: 0, displayBookedMinor: null, displayPendingMinor: null, missingFx: true },
      ],
      alerts: [], fxCoverage: [{ provider: 'frankfurter', baseCurrency: 'EUR', quoteCurrency: 'PLN', affectedTransactionCount: 0, affectedAccountCount: 2 }], currentFxRates: [], nativeSettledTotals: [],
    })

    render(Finance)

    expect(await screen.findByText('Booked balance total')).toBeInTheDocument()
    expect(screen.getAllByText('Unavailable')).not.toHaveLength(0)
    expect(screen.getByText('Native 200.00 EUR')).toBeInTheDocument()
    expect(screen.queryByText('300.00 PLN')).not.toBeInTheDocument()
  })

  it('explains that a prior period uses current FX valuation', async () => {
    mocks.getDashboard.mockResolvedValueOnce({
      period: { startDate: new Date(2026, 4, 1), endDate: new Date(2026, 5, 1) },
      settled: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true }, pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      categoryBreakdowns: [], accountBalances: [], alerts: [], fxCoverage: [], currentFxRates: [], nativeSettledTotals: [],
    })
    render(Finance)
    expect(await screen.findByText('Display-currency balances, flows, categories, and pending values use current FX valuation and can change after a rate refresh.')).toBeInTheDocument()
  })

  it('shows fresh rate metadata and a prominent stale-rate warning', async () => {
    const user = userEvent.setup()
    mocks.getDashboard.mockResolvedValue({
      period: { startDate: new Date(), endDate: new Date() },
      settled: { displayCurrency: 'PLN', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true }, pending: { displayCurrency: 'PLN', incomeMinor: 0, expenseMinor: 0, netMinor: 0, transactionCount: 0, complete: true },
      categoryBreakdowns: [], accountBalances: [], alerts: [], fxCoverage: [], nativeSettledTotals: [],
      currentFxRates: [
        { provider: 'frankfurter', baseCurrency: 'EUR', quoteCurrency: 'PLN', effectiveAt: new Date('2026-06-19T00:00:00Z'), lastSuccessfulRefreshAt: new Date('2026-06-20T12:00:00Z'), stale: false },
        { provider: 'frankfurter', baseCurrency: 'USD', quoteCurrency: 'PLN', effectiveAt: new Date('2026-06-18T00:00:00Z'), lastSuccessfulRefreshAt: new Date('2026-06-18T12:00:00Z'), stale: true },
      ],
    })
    render(Finance)
    expect(await screen.findByText('Current FX valuation may be stale.')).toBeInTheDocument()
    expect(screen.getByText('FX coverage')).toBeInTheDocument()
    await user.click(screen.getByText('FX coverage'))
    expect(screen.getByText(/EUR → PLN · frankfurter · effective/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Refresh current rates' })).toHaveAttribute('href', '#/admin/finance/fx')
    expect(screen.queryByText('Current FX valuation')).not.toBeInTheDocument()
  })

  it('caps account, category, and recent transaction sections to keep the dashboard scannable', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getDashboard.mockResolvedValueOnce({
      period: { startDate: new Date(2026, 5, 20), endDate: new Date(2026, 5, 20) },
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
      fxCoverage: [],
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
    expect(screen.getByRole('link', { name: 'View all transactions' })).toHaveAttribute('href', '#/finance/transactions?startDate=2026-06-20&endDate=2026-06-19')
  })

  it('renders a dashboard error after tenant selection', async () => {
    mocks.getDashboard.mockRejectedValueOnce(new Error('dashboard exploded'))

    render(Finance)

    expect(await screen.findByRole('alert')).toHaveTextContent('dashboard exploded')
  })

})
