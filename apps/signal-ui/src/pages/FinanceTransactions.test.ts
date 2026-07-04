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

  it('renders the ledger table, inspector, and editor navigation links', async () => {
    render(FinanceTransactions)
    expect(await screen.findByRole('heading', { name: 'Refund' })).toBeInTheDocument()
    expect(screen.getByRole('table', { name: 'Transactions ledger' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Create transaction' })).toHaveAttribute('href', '#/finance/transactions/new')
    expect(screen.getByRole('link', { name: 'Open transaction' })).toHaveAttribute('href', '#/finance/transactions/tx-1')
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
    const { container } = render(FinanceTransactions)
    await user.selectOptions(await screen.findByRole('combobox', { name: 'Sort order' }), 'asc')
    await waitFor(() => {
      const rows = Array.from(container.querySelectorAll('tbody tr'))
      expect(rows[0]?.textContent).toContain('Earlier')
    })
    expect(screen.getByText('clear')).toBeInTheDocument()
    expect(screen.getAllByText('reconciliation').length).toBeGreaterThan(0)
  })

  it('renders provider-original details in the inspector when they are available', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTransactions.mockResolvedValueOnce([
      {
        id: 'tx-1',
        tenantId: 'tenant-1',
        accountId: 'account-1',
        source: 'provider',
        status: 'booked',
        kind: 'expense',
        amountMinor: 300,
        currency: 'USD',
        description: 'Card charge',
        effectiveAt: now,
        categoryId: null,
        transferGroupId: null,
        transferMatchedAt: null,
        hiddenAt: null,
        providerOriginal: {
          amountMinor: 350,
          currency: 'USD',
          description: 'Original provider memo',
          effectiveAt: now,
        },
        createdAt: now,
        updatedAt: now,
      },
    ])

    render(FinanceTransactions)

    expect(await screen.findByText('Provider original')).toBeInTheDocument()
    expect(screen.getByText('Original provider memo')).toBeInTheDocument()
    expect(screen.getByText('3.50 USD')).toBeInTheDocument()
  })

  it('clears the inspector selection when a later filter result is empty', async () => {
    const user = userEvent.setup()
    mocks.listTransactions
      .mockResolvedValueOnce([
        {
          id: 'tx-1',
          tenantId: 'tenant-1',
          accountId: 'account-1',
          source: 'manual',
          status: 'pending',
          kind: 'expense',
          amountMinor: 900,
          currency: 'USD',
          description: 'Refund',
          effectiveAt: new Date('2026-06-20T12:00:00Z'),
          categoryId: 'cat-1',
          transferGroupId: null,
          transferMatchedAt: null,
          hiddenAt: null,
          providerOriginal: null,
          createdAt: new Date('2026-06-20T12:00:00Z'),
          updatedAt: new Date('2026-06-20T12:00:00Z'),
        },
      ])
      .mockResolvedValueOnce([])

    render(FinanceTransactions)

    await screen.findByRole('heading', { name: 'Refund' })
    await user.selectOptions(screen.getByRole('combobox', { name: 'Transaction status filter' }), 'booked')

    expect(await screen.findByText('No transactions matched the current filters.')).toBeInTheDocument()
    expect(screen.getByText('Select a transaction row to inspect it here.')).toBeInTheDocument()
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

  it('requires an explicit tenant choice before loading standalone multi-tenant transaction routes', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    const user = userEvent.setup()
    mocks.listTenants.mockResolvedValueOnce([
      { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now },
      { id: 'tenant-2', name: 'Travel', displayCurrency: 'EUR', joinedAt: now, createdAt: now, updatedAt: now },
    ])
    mocks.listAccounts.mockResolvedValueOnce([
      { id: 'account-2', tenantId: 'tenant-2', name: 'Travel card', currency: 'EUR', kind: 'manual', provider: '', providerAccountId: '', hiddenAt: null, createdAt: now, updatedAt: now },
    ])
    mocks.listTransactions.mockResolvedValueOnce([
      {
        id: 'tx-2',
        tenantId: 'tenant-2',
        accountId: 'account-2',
        source: 'manual',
        status: 'booked',
        kind: 'expense',
        amountMinor: 1200,
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

    render(FinanceTransactions)

    expect(await screen.findByText('Select an active tenant to continue on this finance route.')).toBeInTheDocument()
    await user.selectOptions(screen.getByRole('combobox', { name: 'Tenant' }), 'tenant-2')

    await waitFor(() => expect(mocks.listTransactions).toHaveBeenLastCalledWith({ tenantId: 'tenant-2', accountId: '', status: '', source: '' }))
    expect(await screen.findByRole('heading', { name: 'Hotel' })).toBeInTheDocument()
  })

  it('returns to the no-tenant route state when the standalone tenant selection is cleared', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    const user = userEvent.setup()
    mocks.listTenants.mockResolvedValueOnce([
      { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now },
      { id: 'tenant-2', name: 'Travel', displayCurrency: 'EUR', joinedAt: now, createdAt: now, updatedAt: now },
    ])
    mocks.listAccounts.mockResolvedValueOnce([
      { id: 'account-2', tenantId: 'tenant-2', name: 'Travel card', currency: 'EUR', kind: 'manual', provider: '', providerAccountId: '', hiddenAt: null, createdAt: now, updatedAt: now },
    ])
    mocks.listTransactions.mockResolvedValueOnce([
      {
        id: 'tx-2',
        tenantId: 'tenant-2',
        accountId: 'account-2',
        source: 'manual',
        status: 'booked',
        kind: 'expense',
        amountMinor: 1200,
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

    render(FinanceTransactions)

    await user.selectOptions(await screen.findByRole('combobox', { name: 'Tenant' }), 'tenant-2')
    expect(await screen.findByRole('heading', { name: 'Hotel' })).toBeInTheDocument()

    await user.selectOptions(screen.getByRole('combobox', { name: 'Tenant' }), '')

    expect(
      await screen.findByText('Select an active tenant to continue on this finance route.'),
    ).toBeInTheDocument()
    expect(screen.queryByText('Hotel')).not.toBeInTheDocument()
  })

  it('renders an error state when transaction loading fails', async () => {
    mocks.listTenants.mockRejectedValueOnce(new Error('transactions exploded'))
    render(FinanceTransactions)
    expect(await screen.findByRole('alert')).toHaveTextContent('transactions exploded')
  })

  it('falls back to a generic transactions error when workspace loading rejects without an Error', async () => {
    mocks.listTenants.mockRejectedValueOnce('boom')

    render(FinanceTransactions)

    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to load transactions')
  })
})
