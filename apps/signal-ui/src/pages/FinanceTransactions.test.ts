import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceTransactions from './FinanceTransactions.svelte'

const mocks = vi.hoisted(() => ({
  listTenants: vi.fn(),
  listAccounts: vi.fn(),
  listTransactions: vi.fn(),
}))

vi.mock('../lib/finance/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/finance/api')>()),
  createSignalFinanceApiForAuth: vi.fn(() => ({ ...mocks })),
}))

vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

describe('Finance transactions page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.localStorage.clear()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants.mockResolvedValue([
      { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now },
    ])
    mocks.listAccounts.mockResolvedValue([
      { id: 'account-1', tenantId: 'tenant-1', name: 'Checking', currency: 'USD', kind: 'manual', provider: '', providerAccountId: '', hiddenAt: null, createdAt: now, updatedAt: now },
    ])
    mocks.listTransactions.mockResolvedValue([
      {
        id: 'tx-1',
        tenantId: 'tenant-1',
        accountId: 'account-1',
        source: 'manual',
        status: 'pending',
        kind: 'refund',
        amountMinor: 900,
        currency: 'USD',
        description: 'Refund',
        effectiveAt: now,
        categoryId: 'cat-1',
        transferGroupId: 'transfer-1',
        transferMatchedAt: null,
        hiddenAt: now,
        providerOriginal: null,
        createdAt: now,
        updatedAt: now,
      },
    ])
  })

  it('renders transaction state badges and editor navigation links', async () => {
    render(FinanceTransactions)
    expect(await screen.findByText('Refund')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Create transaction' })).toHaveAttribute('href', '#/finance/transactions/new')
    expect(screen.getByRole('link', { name: 'Open transaction' })).toHaveAttribute('href', '#/finance/transactions/tx-1')
    expect(screen.queryByRole('heading', { name: 'Record transaction' })).not.toBeInTheDocument()
    expect(screen.getAllByText('pending').length).toBeGreaterThan(0)
    expect(screen.getByText('hidden')).toBeInTheDocument()
    expect(screen.getAllByText('refund').length).toBeGreaterThan(0)
  })

  it('renders the empty state when filters return no transactions', async () => {
    mocks.listTransactions.mockResolvedValueOnce([])
    render(FinanceTransactions)
    expect(await screen.findByText('No transactions matched the current filters.')).toBeInTheDocument()
  })

  it('renders reconciliation state badges and oldest-first sorting', async () => {
    const earlier = new Date('2026-06-19T12:00:00Z')
    const later = new Date('2026-06-20T12:00:00Z')
    mocks.listTransactions.mockResolvedValueOnce([
      { id: 'tx-1', tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'expense', amountMinor: 300, currency: 'USD', description: 'Later', effectiveAt: later, categoryId: null, transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: later, updatedAt: later },
      { id: 'tx-2', tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'reconciliation', amountMinor: 100, currency: 'USD', description: 'Earlier', effectiveAt: earlier, categoryId: null, transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: earlier, updatedAt: earlier },
    ])
    const user = userEvent.setup()
    render(FinanceTransactions)
    await user.selectOptions(await screen.findByRole('combobox', { name: 'Sort order' }), 'asc')
    const headings = await screen.findAllByRole('heading', { level: 2 })
    expect(headings.some((item) => item.textContent === 'Earlier')).toBe(true)
    expect(screen.getAllByText('reconciliation').length).toBeGreaterThan(0)
  })

  it('reloads transaction filters when selectors change', async () => {
    const user = userEvent.setup()
    render(FinanceTransactions)
    await user.selectOptions(await screen.findByRole('combobox', { name: 'Account filter' }), 'account-1')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Transaction status filter' }), 'pending')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Transaction source filter' }), 'manual')
    await waitFor(() =>
      expect(mocks.listTransactions).toHaveBeenLastCalledWith({
        tenantId: 'tenant-1',
        accountId: 'account-1',
        status: 'pending',
        source: 'manual',
      }),
    )
  })

  it('renders a no-tenant state and keeps the create link available', async () => {
    mocks.listTenants.mockResolvedValueOnce([])
    render(FinanceTransactions)
    expect(await screen.findByText('Select a finance tenant to load transaction history and editor links.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Create transaction' })).toHaveAttribute('href', '#/finance/transactions/new')
  })

  it('renders an error state when transaction loading fails', async () => {
    mocks.listTenants.mockRejectedValueOnce(new Error('transactions exploded'))
    render(FinanceTransactions)
    expect(await screen.findByRole('alert')).toHaveTextContent('transactions exploded')
  })
})
