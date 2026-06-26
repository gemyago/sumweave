import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceTransactionEditor from './FinanceTransactionEditor.svelte'

const mocks = vi.hoisted(() => ({
  listTenants: vi.fn(),
  listAccounts: vi.fn(),
  listCategories: vi.fn(),
  getTransaction: vi.fn(),
  createTransaction: vi.fn(),
  updateTransaction: vi.fn(),
}))

vi.mock('../lib/finance/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/finance/api')>()),
  createSignalFinanceApiForAuth: vi.fn(() => ({ ...mocks })),
}))

vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

describe('Finance transaction editor page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.localStorage.clear()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants.mockResolvedValue([
      { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now },
    ])
    mocks.listAccounts.mockResolvedValue([
      { id: 'account-1', tenantId: 'tenant-1', name: 'Checking', currency: 'USD', kind: 'manual', provider: '', providerAccountId: '', hiddenAt: null, createdAt: now, updatedAt: now },
      { id: 'account-2', tenantId: 'tenant-1', name: 'Savings', currency: 'EUR', kind: 'manual', provider: '', providerAccountId: '', hiddenAt: null, createdAt: now, updatedAt: now },
    ])
    mocks.listCategories.mockResolvedValue([
      { id: 'cat-1', tenantId: 'tenant-1', name: 'Groceries', kind: 'expense', seededDefault: true, hiddenAt: null, createdAt: now, updatedAt: now },
      { id: 'cat-2', tenantId: 'tenant-1', name: 'Travel', kind: 'expense', seededDefault: false, hiddenAt: null, createdAt: now, updatedAt: now },
    ])
    mocks.getTransaction.mockResolvedValue({
      id: 'tx-1',
      tenantId: 'tenant-1',
      accountId: 'account-1',
      source: 'provider',
      status: 'pending',
      kind: 'refund',
      amountMinor: 900,
      currency: 'USD',
      description: 'Refund',
      effectiveAt: now,
      categoryId: 'cat-1',
      transferGroupId: 'transfer-1',
      transferMatchedAt: null,
      hiddenAt: null,
      providerOriginal: {
        amountMinor: 950,
        currency: 'USD',
        description: 'Provider refund',
        effectiveAt: new Date('2026-06-19T12:00:00Z'),
      },
      createdAt: now,
      updatedAt: now,
    })
    mocks.createTransaction.mockResolvedValue({
      id: 'tx-new',
      tenantId: 'tenant-1',
      accountId: 'account-1',
      source: 'manual',
      status: 'booked',
      kind: 'expense',
      amountMinor: 1200,
      currency: 'USD',
      description: 'Coffee',
      effectiveAt: now,
      categoryId: 'cat-1',
      transferGroupId: null,
      transferMatchedAt: null,
      hiddenAt: null,
      providerOriginal: null,
      createdAt: now,
      updatedAt: now,
    })
    mocks.updateTransaction.mockResolvedValue({
      id: 'tx-1',
      tenantId: 'tenant-1',
      accountId: 'account-1',
      source: 'provider',
      status: 'pending',
      kind: 'refund',
      amountMinor: 800,
      currency: 'USD',
      description: 'Refund updated',
      effectiveAt: new Date('2026-06-21T12:00:00Z'),
      categoryId: null,
      transferGroupId: 'transfer-1',
      transferMatchedAt: null,
      hiddenAt: null,
      providerOriginal: {
        amountMinor: 950,
        currency: 'USD',
        description: 'Provider refund',
        effectiveAt: new Date('2026-06-19T12:00:00Z'),
      },
      createdAt: now,
      updatedAt: now,
    })
  })

  it('initializes create mode with a blank editable record and submits through the shared editor', async () => {
    const user = userEvent.setup()
    render(FinanceTransactionEditor, { params: {} })

    expect(await screen.findByRole('heading', { name: 'Record transaction' })).toBeInTheDocument()
    expect(mocks.getTransaction).not.toHaveBeenCalled()
    await screen.findByLabelText('Amount minor')

    await user.clear(screen.getByLabelText('Amount minor'))
    await user.type(screen.getByLabelText('Amount minor'), '1200')
    await user.clear(screen.getByLabelText('Transaction description'))
    await user.type(screen.getByLabelText('Transaction description'), 'Coffee')
    await user.type(screen.getByLabelText('Transaction effective at'), '2026-06-20T12:00')
    await user.selectOptions(screen.getByLabelText('Transaction category'), 'cat-1')
    await user.click(screen.getByRole('button', { name: 'Save transaction' }))

    await waitFor(() => expect(mocks.createTransaction).toHaveBeenCalled())
    expect(await screen.findByText('Transaction recorded.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Cancel' })).toHaveAttribute('href', '#/finance/transactions')
  })

  it('updates create-mode state flags and currency from the selected account', async () => {
    const user = userEvent.setup()
    render(FinanceTransactionEditor, { params: {} })

    await screen.findByRole('heading', { name: 'Record transaction' })
    await screen.findByLabelText('Transaction account')
    await user.selectOptions(screen.getByLabelText('Transaction account'), 'account-2')
    await user.selectOptions(screen.getByLabelText('Transaction status'), 'pending')
    await user.selectOptions(screen.getByLabelText('Transaction kind'), 'refund')
    await user.type(screen.getByLabelText('Transfer group'), 'transfer-1')

    expect(screen.getByLabelText('Transaction currency')).toHaveValue('EUR')
    expect(screen.getAllByText('pending').length).toBeGreaterThan(0)
    expect(screen.getAllByText('refund').length).toBeGreaterThan(0)
    expect(screen.getAllByText('transfer').length).toBeGreaterThan(0)
  })

  it('loads edit mode through the detail endpoint, shows provider-original context, and saves nullable category updates', async () => {
    const user = userEvent.setup()
    render(FinanceTransactionEditor, { params: { transactionId: 'tx-1' } })

    expect(await screen.findByRole('heading', { name: 'Edit transaction' })).toBeInTheDocument()
    await waitFor(() =>
      expect(mocks.getTransaction).toHaveBeenCalledWith({
        tenantId: 'tenant-1',
        transactionId: 'tx-1',
      }),
    )
    await screen.findByText('Description Provider refund')
    expect(screen.getAllByText('pending').length).toBeGreaterThan(0)
    expect(screen.getAllByText('refund').length).toBeGreaterThan(0)

    await user.clear(screen.getByLabelText('Amount minor'))
    await user.type(screen.getByLabelText('Amount minor'), '800')
    await user.clear(screen.getByLabelText('Transaction description'))
    await user.type(screen.getByLabelText('Transaction description'), 'Refund updated')
    await user.selectOptions(screen.getByLabelText('Transaction category'), '')
    await user.clear(screen.getByLabelText('Transaction effective at'))
    await user.type(screen.getByLabelText('Transaction effective at'), '2026-06-21T12:00')
    await user.click(screen.getByRole('button', { name: 'Save transaction' }))

    await waitFor(() =>
      expect(mocks.updateTransaction).toHaveBeenCalledWith({
        tenantId: 'tenant-1',
        transactionId: 'tx-1',
        description: 'Refund updated',
        amountMinor: 800,
        effectiveAt: new Date('2026-06-21T12:00'),
        categoryId: null,
      }),
    )
    expect(await screen.findByRole('status')).toHaveTextContent('Transaction updated.')
  })

  it('requires an explicit tenant choice for deep-link edit routes when multiple tenants are joined', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    const user = userEvent.setup()
    mocks.listTenants.mockResolvedValueOnce([
      { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now },
      { id: 'tenant-2', name: 'Travel', displayCurrency: 'EUR', joinedAt: now, createdAt: now, updatedAt: now },
    ])

    render(FinanceTransactionEditor, { params: { transactionId: 'tx-1' } })

    expect(await screen.findByText('Select an active tenant to continue on this finance route.')).toBeInTheDocument()
    expect(mocks.getTransaction).not.toHaveBeenCalled()

    await user.selectOptions(screen.getByRole('combobox', { name: 'Tenant' }), 'tenant-2')
    await waitFor(() => expect(mocks.getTransaction).toHaveBeenCalledWith({ tenantId: 'tenant-2', transactionId: 'tx-1' }))
  })

  it('shows the finance-tenant empty state when no joined tenant is available', async () => {
    mocks.listTenants.mockResolvedValueOnce([])

    render(FinanceTransactionEditor, { params: { transactionId: 'tx-1' } })

    const tenantsLink = await screen.findByRole('link', { name: 'Finance tenants' })
    expect(tenantsLink.closest('p')).toHaveTextContent(
      'Create or join a tenant from Finance tenants before editing transactions.',
    )
    expect(mocks.listAccounts).not.toHaveBeenCalled()
    expect(mocks.getTransaction).not.toHaveBeenCalled()
  })

  it('shows save errors without leaving the shared editor flow', async () => {
    const user = userEvent.setup()
    mocks.updateTransaction.mockRejectedValueOnce(new Error('save exploded'))

    render(FinanceTransactionEditor, { params: { transactionId: 'tx-1' } })

    await screen.findByRole('heading', { name: 'Edit transaction' })
    await screen.findByRole('button', { name: 'Save transaction' })
    await user.click(screen.getByRole('button', { name: 'Save transaction' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('save exploded')
  })
})
