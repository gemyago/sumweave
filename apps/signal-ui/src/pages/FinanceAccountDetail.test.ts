import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceAccountDetail from './FinanceAccountDetail.svelte'

const mocks = vi.hoisted(() => ({ listTenants: vi.fn(), listAccounts: vi.fn(), listTransactions: vi.fn() }))
vi.mock('../lib/finance/api', async (importOriginal) => ({ ...(await importOriginal<typeof import('../lib/finance/api')>()), createSignalFinanceApiForAuth: vi.fn(() => ({ ...mocks })) }))
vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

describe('Finance account detail page', () => {
  beforeEach(() => {
    window.localStorage.clear()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants.mockResolvedValue([{ id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now }])
    mocks.listAccounts.mockResolvedValue([{ id: 'account-1', tenantId: 'tenant-1', name: 'Checking', currency: 'USD', kind: 'manual', provider: '', providerAccountId: '', hiddenAt: null, createdAt: now, updatedAt: now }])
    mocks.listTransactions.mockResolvedValue([{ id: 'tx-1', tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'expense', amountMinor: 1200, currency: 'USD', description: 'Groceries', effectiveAt: now, categoryId: null, transferGroupId: null, transferMatchedAt: null, hiddenAt: null, createdAt: now, updatedAt: now }])
  })

  it('renders focused account detail route', async () => {
    render(FinanceAccountDetail, { params: { accountId: 'account-1' } })
    expect(await screen.findByText('Checking')).toBeInTheDocument()
    expect(screen.getByText('Groceries')).toBeInTheDocument()
  })

  it('renders an empty recent-transactions state', async () => {
    mocks.listTransactions.mockResolvedValueOnce([])
    render(FinanceAccountDetail, { params: { accountId: 'account-1' } })
    expect(await screen.findByText('No transactions yet.')).toBeInTheDocument()
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
    expect(await screen.findByText('Travel card')).toBeInTheDocument()
  })
})
