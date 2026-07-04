import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceAccounts from './FinanceAccounts.svelte'

const mocks = vi.hoisted(() => ({ listTenants: vi.fn(), listAccounts: vi.fn(), createAccount: vi.fn() }))
vi.mock('../lib/finance/api', async (importOriginal) => ({ ...(await importOriginal<typeof import('../lib/finance/api')>()), createSignalFinanceApiForAuth: vi.fn(() => ({ ...mocks })) }))
vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

describe('Finance accounts page', () => {
  beforeEach(() => {
    window.localStorage.clear()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants.mockResolvedValue([{ id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now }])
    mocks.listAccounts.mockResolvedValue([{ id: 'account-1', tenantId: 'tenant-1', name: 'Checking', currency: 'USD', kind: 'manual', provider: '', providerAccountId: '', hiddenAt: null, createdAt: now, updatedAt: now }])
    mocks.createAccount.mockResolvedValue({})
  })

  it('renders account cards and detail links', async () => {
    render(FinanceAccounts)
    expect(await screen.findByText('Checking')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open account detail' })).toHaveAttribute('href', '#/finance/accounts/account-1')
  })

  it('submits the create-account form', async () => {
    const user = userEvent.setup()
    render(FinanceAccounts)

    await user.type(await screen.findByLabelText('Account name'), 'Savings')
    await user.click(screen.getByRole('button', { name: 'Create account' }))
    await waitFor(() => expect(mocks.createAccount).toHaveBeenCalled())
  })

  it('renders the empty state when no accounts exist', async () => {
    mocks.listAccounts.mockResolvedValueOnce([])
    render(FinanceAccounts)
    expect(await screen.findByText('No accounts yet.')).toBeInTheDocument()
  })

  it('renders a no-tenant state and keeps create disabled', async () => {
    mocks.listTenants.mockResolvedValueOnce([])
    render(FinanceAccounts)
    expect(await screen.findByRole('button', { name: 'Create account' })).toBeDisabled()
  })

  it('reloads accounts when include-hidden is toggled', async () => {
    const user = userEvent.setup()
    render(FinanceAccounts)

    await user.click(await screen.findByRole('checkbox', { name: /Include hidden/i }))
    await waitFor(() => expect(mocks.listAccounts).toHaveBeenLastCalledWith({ tenantId: 'tenant-1', includeHidden: true }))
  })

  it('reloads accounts when the selected tenant changes', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants.mockResolvedValueOnce([
      { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now },
      { id: 'tenant-2', name: 'Travel', displayCurrency: 'EUR', joinedAt: now, createdAt: now, updatedAt: now },
    ])
    const user = userEvent.setup()
    render(FinanceAccounts)

    await user.selectOptions(await screen.findByRole('combobox', { name: 'Tenant' }), 'tenant-2')
    await waitFor(() => expect(mocks.listAccounts).toHaveBeenLastCalledWith({ tenantId: 'tenant-2', includeHidden: false }))
  })

  it('returns to the no-tenant state when the standalone tenant selection is cleared', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants.mockResolvedValueOnce([
      { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now },
      { id: 'tenant-2', name: 'Travel', displayCurrency: 'EUR', joinedAt: now, createdAt: now, updatedAt: now },
    ])
    const user = userEvent.setup()
    render(FinanceAccounts)

    await user.selectOptions(await screen.findByRole('combobox', { name: 'Tenant' }), 'tenant-2')
    await waitFor(() => expect(mocks.listAccounts).toHaveBeenLastCalledWith({ tenantId: 'tenant-2', includeHidden: false }))

    await user.selectOptions(screen.getByRole('combobox', { name: 'Tenant' }), '')

    expect(await screen.findByRole('button', { name: 'Create account' })).toBeDisabled()
    expect(screen.getByText('No accounts yet.')).toBeInTheDocument()
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
