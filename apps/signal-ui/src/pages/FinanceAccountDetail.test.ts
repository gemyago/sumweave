import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceAccountDetail from './FinanceAccountDetail.svelte'

const mocks = vi.hoisted(() => ({ listTenants: vi.fn(), listAccounts: vi.fn(), listTransactions: vi.fn(), listAccountProviderEvidence: vi.fn(), getAccountProviderEvidence: vi.fn() }))
vi.mock('../lib/finance/api', async (importOriginal) => ({ ...(await importOriginal<typeof import('../lib/finance/api')>()), createSignalFinanceApiForAuth: vi.fn(() => ({ ...mocks })) }))
vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

describe('Finance account detail page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.localStorage.clear()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants.mockResolvedValue([{ id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now }])
    mocks.listAccounts.mockResolvedValue([{ id: 'account-1', tenantId: 'tenant-1', name: 'Checking', currency: 'USD', kind: 'manual', bookedBalanceMinor: 2500, pendingBalanceMinor: 0, provider: '', providerAccountId: '', hiddenAt: null, createdAt: now, updatedAt: now }])
    mocks.listTransactions.mockResolvedValue([{ id: 'tx-1', tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'expense', amountMinor: 1200, currency: 'USD', description: 'Groceries', effectiveAt: now, categoryId: null, transferGroupId: null, transferMatchedAt: null, hiddenAt: null, createdAt: now, updatedAt: now }])
    mocks.listAccountProviderEvidence.mockResolvedValue([])
  })

  it('renders focused account detail route', async () => {
    render(FinanceAccountDetail, { params: { accountId: 'account-1' } })
    expect((await screen.findAllByRole('heading', { name: 'Checking' })).length).toBeGreaterThan(0)
    expect(screen.getByText('Groceries')).toBeInTheDocument()
  })

  it('renders booked and pending balances in the account summary', async () => {
    const { container } = render(FinanceAccountDetail, { params: { accountId: 'account-1' } })

    expect(await screen.findByText('Booked balance 25.00 USD')).toBeInTheDocument()
    expect(screen.getByText('Pending balance 0.00 USD')).toBeInTheDocument()
    expect(container.querySelector('.col-xl-5, .col-xl-7')).not.toBeInTheDocument()
  })

  it('loads account evidence metadata only when expanded and reveals sanitized details explicitly', async () => {
    const user = userEvent.setup()
    mocks.listAccountProviderEvidence.mockResolvedValueOnce([
      { id: 'evidence-1', scope: 'account', providerObjectId: 'provider-account', capturedAt: new Date('2026-06-20T12:00:00Z') },
    ])
    mocks.getAccountProviderEvidence.mockResolvedValueOnce({
      id: 'evidence-1', scope: 'account', providerObjectId: 'provider-account', capturedAt: new Date('2026-06-20T12:00:00Z'), payload: { name: 'sanitized' },
    })
    render(FinanceAccountDetail, { params: { accountId: 'account-1' } })

    await screen.findAllByRole('heading', { name: 'Checking' })
    expect(mocks.listAccountProviderEvidence).not.toHaveBeenCalled()
    await user.click(screen.getByText('Provider evidence'))
    await waitFor(() => expect(mocks.listAccountProviderEvidence).toHaveBeenCalledWith({ tenantId: 'tenant-1', accountId: 'account-1' }))
    expect(mocks.getAccountProviderEvidence).not.toHaveBeenCalled()
    await user.click(await screen.findByRole('button', { name: 'Reveal sanitized details' }))
    await waitFor(() => expect(mocks.getAccountProviderEvidence).toHaveBeenCalledWith({ tenantId: 'tenant-1', accountId: 'account-1', evidenceId: 'evidence-1' }))
    expect(screen.getByText('Sanitized provider evidence')).toBeInTheDocument()
  })

  it('renders an empty recent-transactions state', async () => {
    mocks.listTransactions.mockResolvedValueOnce([])
    render(FinanceAccountDetail, { params: { accountId: 'account-1' } })
    expect(await screen.findByText('No transactions yet.')).toBeInTheDocument()
  })

  it('uses the shared responsive transaction list for recent activity', async () => {
    render(FinanceAccountDetail, { params: { accountId: 'account-1' } })

    expect(await screen.findByLabelText('Recent transactions')).toBeInTheDocument()
    expect(screen.getByText('12.00 USD')).toBeInTheDocument()
    expect(screen.queryByText('booked')).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open full transaction details' })).toHaveAttribute('href', '#/finance/transactions/tx-1')
  })

  it('uses fixed ten-row offset paging for recent transactions', async () => {
    const user = userEvent.setup()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTransactions
      .mockResolvedValueOnce(Array.from({ length: 10 }, (_, index) => ({
        id: `tx-${index}`, tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'regular', amountMinor: -100,
        currency: 'USD', description: `Transaction ${index}`, effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, createdAt: now, updatedAt: now,
      })))
      .mockResolvedValueOnce([{ id: 'tx-11', tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'regular', amountMinor: -100, currency: 'USD', description: 'Transaction 11', effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, createdAt: now, updatedAt: now }])
    render(FinanceAccountDetail, { params: { accountId: 'account-1' } })

    expect(await screen.findByText('Transaction 0')).toBeInTheDocument()
    expect(mocks.listTransactions).toHaveBeenLastCalledWith({ tenantId: 'tenant-1', accountId: 'account-1', limit: 10, offset: 0 })
    await user.click(screen.getByRole('button', { name: 'Recent transaction pages: next page' }))
    expect(await screen.findByText('Transaction 11')).toBeInTheDocument()
    expect(mocks.listTransactions).toHaveBeenLastCalledWith({ tenantId: 'tenant-1', accountId: 'account-1', limit: 10, offset: 10 })
  })

  it('keeps recent activity visible and recoverable while paging', async () => {
    const user = userEvent.setup()
    const now = new Date('2026-06-20T12:00:00Z')
    let rejectNextPage!: (reason?: unknown) => void
    mocks.listTransactions
      .mockResolvedValueOnce(Array.from({ length: 10 }, (_, index) => ({
        id: `tx-${index}`, tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'regular', amountMinor: -100,
        currency: 'USD', description: `Transaction ${index}`, effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, createdAt: now, updatedAt: now,
      })))
      .mockImplementationOnce(() => new Promise((_, reject) => { rejectNextPage = reject }))
    render(FinanceAccountDetail, { params: { accountId: 'account-1' } })

    expect(await screen.findByText('Transaction 0')).toBeInTheDocument()
    const next = screen.getByRole('button', { name: 'Recent transaction pages: next page' })
    await user.click(next)

    await waitFor(() => expect(next).toBeDisabled())
    expect(screen.getByText('Transaction 0')).toBeInTheDocument()
    rejectNextPage(new Error('Recent page unavailable'))

    expect(await screen.findByText('Recent page unavailable')).toHaveAttribute('role', 'alert')
    expect(screen.getByText('Transaction 0')).toBeInTheDocument()
    expect(next).toBeEnabled()
  })

  it('resets recent paging when the account route changes', async () => {
    const user = userEvent.setup()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listAccounts.mockResolvedValue([
      { id: 'account-1', tenantId: 'tenant-1', name: 'Checking', currency: 'USD', kind: 'manual', bookedBalanceMinor: 0, pendingBalanceMinor: 0, provider: '', providerAccountId: '', hiddenAt: null, createdAt: now, updatedAt: now },
      { id: 'account-2', tenantId: 'tenant-1', name: 'Savings', currency: 'USD', kind: 'manual', bookedBalanceMinor: 0, pendingBalanceMinor: 0, provider: '', providerAccountId: '', hiddenAt: null, createdAt: now, updatedAt: now },
    ])
    mocks.listTransactions.mockImplementation(async ({ accountId, offset }) => {
      if (accountId === 'account-2') {
        return [{ id: 'savings-1', tenantId: 'tenant-1', accountId: 'account-2', source: 'manual', status: 'booked', kind: 'regular', amountMinor: -100, currency: 'USD', description: 'Savings activity', effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, createdAt: now, updatedAt: now }]
      }
      if (offset === 0) {
        return Array.from({ length: 10 }, (_, index) => ({
          id: `tx-${index}`, tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'regular', amountMinor: -100, currency: 'USD', description: `Checking ${index}`, effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, createdAt: now, updatedAt: now,
        }))
      }
      return []
    })
    const view = render(FinanceAccountDetail, { params: { accountId: 'account-1' } })

    await screen.findByText('Checking 0')
    await user.click(screen.getByRole('button', { name: 'Recent transaction pages: next page' }))
    await screen.findByText('No transactions yet.')
    await view.rerender({ params: { accountId: 'account-2' } })

    expect(await screen.findByText('Savings activity')).toBeInTheDocument()
    expect(mocks.listTransactions).toHaveBeenLastCalledWith({ tenantId: 'tenant-1', accountId: 'account-2', limit: 10, offset: 0 })
  })

  it('shows a not-found message when the account is missing', async () => {
    mocks.listAccounts.mockResolvedValueOnce([])

    render(FinanceAccountDetail, { params: { accountId: 'missing-account' } })

    expect(await screen.findByText('Account not found for the selected tenant.')).toBeInTheDocument()
  })

  it('shows the not-found state when no account id is provided', async () => {
    render(FinanceAccountDetail, { params: {} })
    expect(await screen.findByText('Account not found for the selected tenant.')).toBeInTheDocument()
  })

  it('shows the join-tenant route hint when no finance tenants are available', async () => {
    mocks.listTenants.mockResolvedValueOnce([])

    render(FinanceAccountDetail, { params: { accountId: 'account-1' } })

    expect(await screen.findByRole('link', { name: 'Finance tenants' })).toHaveAttribute(
      'href',
      '#/finance/tenants',
    )
    expect(screen.getByText(/before opening this account detail route\./)).toBeInTheDocument()
  })

  it('falls back to a generic account-detail error when workspace loading rejects without an Error', async () => {
    mocks.listTenants.mockRejectedValueOnce('boom')

    render(FinanceAccountDetail, { params: { accountId: 'account-1' } })

    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to load account detail')
  })

  it('requires an explicit tenant choice for deep links when multiple tenants are joined', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    const user = userEvent.setup()
    mocks.listTenants.mockResolvedValueOnce([
      { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now },
      { id: 'tenant-2', name: 'Travel', displayCurrency: 'EUR', joinedAt: now, createdAt: now, updatedAt: now },
    ])
    mocks.listAccounts.mockResolvedValueOnce([{ id: 'account-1', tenantId: 'tenant-2', name: 'Travel card', currency: 'EUR', kind: 'manual', provider: '', providerAccountId: '', hiddenAt: null, createdAt: now, updatedAt: now }])
    mocks.listTransactions.mockResolvedValueOnce([])

    render(FinanceAccountDetail, { params: { accountId: 'account-1' } })

    expect(await screen.findByText('Select an active tenant to continue on this finance route.')).toBeInTheDocument()
    expect(screen.queryByText('Account not found for the selected tenant.')).not.toBeInTheDocument()

    await user.selectOptions(screen.getByRole('combobox', { name: 'Tenant' }), 'tenant-2')

    await waitFor(() => expect(mocks.listAccounts).toHaveBeenCalledWith({ tenantId: 'tenant-2' }))
    expect(mocks.listTransactions).toHaveBeenLastCalledWith({ tenantId: 'tenant-2', accountId: 'account-1', limit: 10, offset: 0 })
    expect(await screen.findByText('Travel card')).toBeInTheDocument()
  })
})
