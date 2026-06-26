import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import Finance from './Finance.svelte'

const mocks = vi.hoisted(() => ({
  listTenants: vi.fn(),
  getDashboard: vi.fn(),
}))

vi.mock('../lib/finance/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/finance/api')>()
  return {
    ...actual,
    createSignalFinanceApiForAuth: vi.fn(() => ({
      listTenants: mocks.listTenants,
      getDashboard: mocks.getDashboard,
    })),
  }
})

vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

describe('Finance dashboard page', () => {
  beforeEach(() => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants.mockReset()
    mocks.getDashboard.mockReset()
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
  })

  it('loads the tenant-aware dashboard and admin FX deep link', async () => {
    render(Finance)

    expect(await screen.findByRole('heading', { name: 'Finance' })).toBeInTheDocument()
    expect(screen.getByText('Tenant: Household')).toBeInTheDocument()
    expect(await screen.findByText('stale_connection')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open admin FX diagnostics' })).toHaveAttribute('href', '#/admin/finance/fx')
    expect(screen.getByLabelText('Custom start date')).toHaveValue('2026-06-20')
    expect(screen.getByLabelText('Custom end date')).toHaveValue('2026-06-20')
    expect(screen.queryByText('2026-06-20T12:00:00.000Z')).not.toBeInTheDocument()
  })

  it('shows the tenant-create prompt when no tenants exist', async () => {
    mocks.listTenants.mockResolvedValueOnce([])

    render(Finance)

    expect(await screen.findByText(/Create or join a tenant/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Finance tenants' })).toHaveAttribute('href', '#/finance/tenants')
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

    render(Finance)

    expect(await screen.findByText('No active dashboard alerts.')).toBeInTheDocument()
    expect(screen.getByText('No accounts yet for this tenant.')).toBeInTheDocument()
    expect(screen.getByText('No category activity for this period.')).toBeInTheDocument()
  })

  it('renders a dashboard error after tenant selection', async () => {
    mocks.getDashboard.mockRejectedValueOnce(new Error('dashboard exploded'))

    render(Finance)

    expect(await screen.findByRole('alert')).toHaveTextContent('dashboard exploded')
  })

  it('returns to the no-tenant prompt when the selected tenant is cleared', async () => {
    render(Finance)

    const tenantSelect = (await screen.findByRole('combobox', { name: 'Tenant' })) as HTMLSelectElement
    await fireEvent.change(tenantSelect, { target: { value: '' } })
    expect(await screen.findByText(/Create or join a tenant/)).toBeInTheDocument()
  })
})
