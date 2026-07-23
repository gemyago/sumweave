import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceAccounts from './FinanceAccounts.svelte'

const mocks = vi.hoisted(() => ({ listTenants: vi.fn(), listAccounts: vi.fn() }))
vi.mock('../lib/finance/api', async (importOriginal) => ({ ...(await importOriginal<typeof import('../lib/finance/api')>()), createSignalFinanceApiForAuth: vi.fn(() => ({ ...mocks })) }))
vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

describe('Finance accounts page', () => {
  beforeEach(() => {
    window.localStorage.clear()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants.mockResolvedValue([{ id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now }])
    mocks.listAccounts.mockResolvedValue([{ id: 'account-1', tenantId: 'tenant-1', name: 'Checking', currency: 'USD', kind: 'manual', bookedBalanceMinor: 0, pendingBalanceMinor: -125, provider: '', providerAccountId: '', hiddenAt: null, createdAt: now, updatedAt: now }])
  })

  it('renders a compact account browse table and detail links', async () => {
    render(FinanceAccounts)
    expect(await screen.findByText('Checking')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open details' })).toHaveAttribute('href', '#/finance/accounts/account-1')
  })

  it('renders booked and pending balances without treating zero as absent', async () => {
    render(FinanceAccounts)

    expect(await screen.findByText(/Booked 0\.00 USD/)).toBeInTheDocument()
    expect(screen.getByText(/Pending -1\.25 USD/)).toBeInTheDocument()
  })

  it('keeps account creation on its dedicated route', async () => {
    render(FinanceAccounts)

    expect(await screen.findByRole('link', { name: 'Create account' })).toHaveAttribute('href', '#/finance/accounts/new')
    expect(screen.queryByLabelText('Account name')).not.toBeInTheDocument()
  })

  it('renders the empty state when no accounts exist', async () => {
    mocks.listAccounts.mockResolvedValueOnce([])
    render(FinanceAccounts)
    expect(await screen.findByText(/No accounts yet\./)).toBeInTheDocument()
  })

  it('renders a no-tenant state and keeps the dedicated create link available', async () => {
    mocks.listTenants.mockResolvedValueOnce([])
    render(FinanceAccounts)
    expect(await screen.findByRole('link', { name: 'Create account' })).toHaveAttribute('href', '#/finance/accounts/new')
  })

  it('reloads accounts when include-hidden is toggled', async () => {
    const user = userEvent.setup()
    render(FinanceAccounts)

    await user.click(await screen.findByRole('checkbox', { name: /Include hidden/i }))
    await waitFor(() => expect(mocks.listAccounts).toHaveBeenLastCalledWith({ tenantId: 'tenant-1', includeHidden: true }))
  })


  it('renders an error state when account loading fails', async () => {
    mocks.listTenants.mockRejectedValueOnce(new Error('accounts exploded'))
    render(FinanceAccounts)
    expect(await screen.findByRole('alert')).toHaveTextContent('accounts exploded')
  })

  it('falls back to a generic accounts error when workspace loading rejects without an Error', async () => {
    mocks.listTenants.mockRejectedValueOnce('boom')

    render(FinanceAccounts)

    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to load accounts')
  })
})
