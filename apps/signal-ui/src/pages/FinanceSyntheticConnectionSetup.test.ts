import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceSyntheticConnectionSetup from './FinanceSyntheticConnectionSetup.svelte'

const mocks = vi.hoisted(() => ({
  listTenants: vi.fn(),
  getSyntheticLinkState: vi.fn(),
  saveSyntheticLinkState: vi.fn(),
  finishRedirectConnection: vi.fn(),
}))

vi.mock('../lib/finance/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/finance/api')>()),
  createSignalFinanceApiForAuth: vi.fn(() => ({ ...mocks })),
}))

vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

function createTenantFixture() {
  const now = new Date('2026-06-20T12:00:00Z')
  return { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now }
}

function renderPage() {
  return render(FinanceSyntheticConnectionSetup)
}

describe('finance synthetic setup page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.localStorage.clear()
    window.history.replaceState({}, '', '/#/finance/connections/synthetic?state=state-1')
    mocks.listTenants.mockResolvedValue([createTenantFixture()])
    mocks.getSyntheticLinkState.mockResolvedValue({ provider: 'synthetic', state: 'state-1', configuredAccounts: [], canFinish: false })
    mocks.saveSyntheticLinkState.mockResolvedValue({ provider: 'synthetic', state: 'state-1', configuredAccounts: [], canFinish: false })
    mocks.finishRedirectConnection.mockResolvedValue({})
  })

  it('loads pending synthetic configuration from the returned setup state', async () => {
    mocks.getSyntheticLinkState.mockResolvedValueOnce({
      provider: 'synthetic',
      state: 'state-1',
      configuredAccounts: [{ key: 'account-1', name: 'Cash', currency: 'USD' }],
      canFinish: true,
    })

    renderPage()

    expect(await screen.findByDisplayValue('Cash')).toBeInTheDocument()
    expect(screen.getByDisplayValue('USD')).toBeInTheDocument()
  })

  it('adds and removes configured account rows locally', async () => {
    const user = userEvent.setup()
    renderPage()

    await screen.findByLabelText('Account name 1')
    await user.click(screen.getByRole('button', { name: 'Add account' }))

    expect(screen.getByLabelText('Account name 2')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Remove configured account 2' }))

    expect(screen.queryByLabelText('Account name 2')).not.toBeInTheDocument()
  })

  it('preserves duplicate configured accounts as distinct rows across save and reload', async () => {
    const user = userEvent.setup()
    const duplicateState = {
      provider: 'synthetic',
      state: 'state-1',
      configuredAccounts: [
        { key: 'dup-1', name: 'Cash', currency: 'USD' },
        { key: 'dup-2', name: 'Cash', currency: 'USD' },
      ],
      canFinish: true,
    }
    mocks.getSyntheticLinkState
      .mockResolvedValueOnce(duplicateState)
      .mockResolvedValueOnce(duplicateState)
    mocks.saveSyntheticLinkState.mockResolvedValueOnce(duplicateState)

    renderPage()

    await waitFor(() => expect(screen.getAllByDisplayValue('Cash')).toHaveLength(2))
    await user.click(screen.getByRole('button', { name: 'Save configuration' }))

    await waitFor(() =>
      expect(mocks.saveSyntheticLinkState).toHaveBeenCalledWith({
        tenantId: 'tenant-1',
        state: 'state-1',
        configuredAccounts: [
          { key: 'dup-1', name: 'Cash', currency: 'USD' },
          { key: 'dup-2', name: 'Cash', currency: 'USD' },
        ],
      }),
    )

    await user.click(screen.getByRole('button', { name: 'Reload pending setup' }))

    await waitFor(() => expect(mocks.getSyntheticLinkState).toHaveBeenCalledTimes(2))
    expect(screen.getAllByDisplayValue('Cash')).toHaveLength(2)
    expect(screen.getAllByDisplayValue('USD')).toHaveLength(2)
  })

  it('finishes the link and returns to finance connections', async () => {
    const user = userEvent.setup()
    mocks.saveSyntheticLinkState.mockResolvedValueOnce({
      provider: 'synthetic',
      state: 'state-1',
      configuredAccounts: [{ key: 'account-1', name: 'Savings', currency: 'EUR' }],
      canFinish: true,
    })

    renderPage()

    await user.type(await screen.findByLabelText('Account name 1'), 'Savings')
    await user.type(screen.getByLabelText('Account currency 1'), 'EUR')
    await user.click(screen.getByRole('button', { name: 'Finish link' }))

    await waitFor(() =>
      expect(mocks.finishRedirectConnection).toHaveBeenCalledWith({
        tenantId: 'tenant-1',
        provider: 'synthetic',
        state: 'state-1',
      }),
    )
    await waitFor(() => expect(window.location.hash).toBe('#/finance/connections'))
  })

  it('shows guidance when the setup route is opened without a pending state', async () => {
    window.history.replaceState({}, '', '/#/finance/connections/synthetic')

    renderPage()

    expect(await screen.findByText(/No pending synthetic setup state is present/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Finance connections' })).toHaveAttribute('href', '#/finance/connections')
  })

  it('requires explicit tenant selection when multiple tenants are available', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants.mockResolvedValueOnce([
      createTenantFixture(),
      { id: 'tenant-2', name: 'Travel', displayCurrency: 'EUR', joinedAt: now, createdAt: now, updatedAt: now },
    ])

    renderPage()

    expect(await screen.findByText('Select an active tenant to continue on this finance route.')).toBeInTheDocument()
    expect(mocks.getSyntheticLinkState).not.toHaveBeenCalled()
  })

  it('keeps the operator on setup when validation fails before save or finish', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.type(await screen.findByLabelText('Account name 1'), 'Savings')
    await user.click(screen.getByRole('button', { name: 'Save configuration' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Configured account 1 requires both name and currency.')
    expect(mocks.saveSyntheticLinkState).not.toHaveBeenCalled()
    expect(window.location.hash).toBe('#/finance/connections/synthetic?state=state-1')
  })

  it('surfaces save failures without leaving the setup route', async () => {
    const user = userEvent.setup()
    mocks.saveSyntheticLinkState.mockRejectedValueOnce(new Error('Save failed'))

    renderPage()

    await user.type(await screen.findByLabelText('Account name 1'), 'Savings')
    await user.type(screen.getByLabelText('Account currency 1'), 'EUR')
    await user.click(screen.getByRole('button', { name: 'Save configuration' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Save failed')
    expect(window.location.hash).toBe('#/finance/connections/synthetic?state=state-1')
  })

  it('surfaces finish failures without leaving the setup route', async () => {
    const user = userEvent.setup()
    mocks.saveSyntheticLinkState.mockResolvedValueOnce({
      provider: 'synthetic',
      state: 'state-1',
      configuredAccounts: [{ key: 'account-1', name: 'Savings', currency: 'EUR' }],
      canFinish: true,
    })
    mocks.finishRedirectConnection.mockRejectedValueOnce(new Error('Finish failed'))

    renderPage()

    await user.type(await screen.findByLabelText('Account name 1'), 'Savings')
    await user.type(screen.getByLabelText('Account currency 1'), 'EUR')
    await user.click(screen.getByRole('button', { name: 'Finish link' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Finish failed')
    expect(window.location.hash).toBe('#/finance/connections/synthetic?state=state-1')
  })
})
