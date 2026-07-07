import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceTenants from './FinanceTenants.svelte'

const supportedCurrencyCodes = ['USD', 'EUR', 'PLN', 'UAH']

const mocks = vi.hoisted(() => ({
  listTenants: vi.fn(),
  listTenantMembers: vi.fn(),
  listTenantInvites: vi.fn(),
  createTenant: vi.fn(),
  updateTenant: vi.fn(),
  createTenantInvite: vi.fn(),
  acceptTenantInvite: vi.fn(),
}))

vi.mock('../lib/finance/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/finance/api')>()
  return {
    ...actual,
    createSignalFinanceApiForAuth: vi.fn(() => ({ ...mocks })),
  }
})

vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

describe('Finance tenants page', () => {
  beforeEach(() => {
    window.localStorage.clear()
    const now = new Date('2026-06-20T12:00:00Z')
    Object.values(mocks).forEach((mock) => mock.mockReset())
    mocks.listTenants.mockResolvedValue([{ id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now }])
    mocks.listTenantMembers.mockResolvedValue([{ tenantId: 'tenant-1', userId: 'user-1', joinedAt: now }])
    mocks.listTenantInvites.mockResolvedValue([{ id: 'invite-1', tenantId: 'tenant-1', code: 'code-1', recipient: 'friend@example.com', createdByUserId: 'user-1', acceptedByUserId: null, createdAt: now, acceptedAt: null }])
    mocks.createTenant.mockResolvedValue({})
    mocks.createTenantInvite.mockResolvedValue({})
    mocks.acceptTenantInvite.mockResolvedValue({})
  })

  it('creates tenants with a supported currency selector and submits the selected code', async () => {
    const user = userEvent.setup()
    render(FinanceTenants)

    expect(await screen.findByText('user-1')).toBeInTheDocument()
    const createForm = screen.getByRole('button', { name: 'Create tenant' }).closest('form')
    expect(createForm).not.toBeNull()
    const createFormQueries = within(createForm as HTMLFormElement)

    expect(createFormQueries.queryByRole('textbox', { name: 'Display currency' })).not.toBeInTheDocument()
    expect(createFormQueries.getAllByRole('option').map((option) => option.textContent?.trim())).toEqual(supportedCurrencyCodes)

    await user.type(createFormQueries.getByLabelText('Tenant name'), 'Travel')
    await user.selectOptions(createFormQueries.getByRole('combobox', { name: 'Display currency' }), 'PLN')
    await user.click(createFormQueries.getByRole('button', { name: 'Create tenant' }))
    await waitFor(() => expect(mocks.createTenant).toHaveBeenCalledWith({ name: 'Travel', displayCurrency: 'PLN' }))
  })

  it('prefills the selected tenant edit form and reuses the supported currency selector', async () => {
    render(FinanceTenants)

    const editForm = (await screen.findByRole('heading', { name: 'Edit selected tenant' })).closest('form')
    expect(editForm).not.toBeNull()
    const editFormQueries = within(editForm as HTMLFormElement)

    expect(editFormQueries.getByLabelText('Tenant name')).toHaveValue('Household')
    expect(editFormQueries.getByRole('combobox', { name: 'Display currency' })).toHaveValue('USD')
    expect(editFormQueries.getAllByRole('option').map((option) => option.textContent?.trim())).toEqual(supportedCurrencyCodes)
  })

  it('updates the selected tenant and refreshes the visible shell tenant state without response data', async () => {
    const user = userEvent.setup()
    const now = new Date('2026-06-20T12:00:00Z')
    const updatedAt = new Date('2026-06-21T12:00:00Z')
    mocks.listTenants
      .mockResolvedValueOnce([{ id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now }])
      .mockResolvedValueOnce([{ id: 'tenant-1', name: 'Household Updated', displayCurrency: 'PLN', joinedAt: now, createdAt: now, updatedAt: updatedAt }])
    mocks.updateTenant.mockResolvedValue(undefined)

    render(FinanceTenants)

    const editForm = (await screen.findByRole('heading', { name: 'Edit selected tenant' })).closest('form')
    expect(editForm).not.toBeNull()
    const editFormQueries = within(editForm as HTMLFormElement)

    await user.clear(editFormQueries.getByLabelText('Tenant name'))
    await user.type(editFormQueries.getByLabelText('Tenant name'), 'Household Updated')
    await user.selectOptions(editFormQueries.getByRole('combobox', { name: 'Display currency' }), 'PLN')
    await user.click(editFormQueries.getByRole('button', { name: 'Save tenant changes' }))

    await waitFor(() => expect(mocks.updateTenant).toHaveBeenCalledWith({ tenantId: 'tenant-1', name: 'Household Updated', displayCurrency: 'PLN' }))
    await waitFor(() => expect(mocks.listTenants).toHaveBeenCalledTimes(2))
    expect(await screen.findByText('Active tenant · Household Updated')).toBeInTheDocument()
  })

  it('keeps the selected tenant context and shows a recoverable error when update fails', async () => {
    const user = userEvent.setup()
    mocks.updateTenant.mockRejectedValueOnce(new Error('tenant update exploded'))

    render(FinanceTenants)

    const editForm = (await screen.findByRole('heading', { name: 'Edit selected tenant' })).closest('form')
    expect(editForm).not.toBeNull()
    const editFormQueries = within(editForm as HTMLFormElement)

    await user.clear(editFormQueries.getByLabelText('Tenant name'))
    await user.type(editFormQueries.getByLabelText('Tenant name'), 'Household Edited')
    await user.click(editFormQueries.getByRole('button', { name: 'Save tenant changes' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('tenant update exploded')
    expect(editFormQueries.getByLabelText('Tenant name')).toHaveValue('Household Edited')
    expect(screen.getByText('Active tenant · Household')).toBeInTheDocument()
  })

  it('supports tenant invite create and invite accept flows', async () => {
    const user = userEvent.setup()
    render(FinanceTenants)

    expect(await screen.findByText('user-1')).toBeInTheDocument()

    await user.type(screen.getByLabelText('Invite recipient'), 'team@example.com')
    await user.click(screen.getByRole('button', { name: 'Create invite' }))
    await waitFor(() => expect(mocks.createTenantInvite).toHaveBeenCalledWith({ tenantId: 'tenant-1', recipient: 'team@example.com' }))

    await user.type(screen.getByLabelText('Invite code'), 'join-code')
    await user.click(screen.getByRole('button', { name: 'Accept invite' }))
    await waitFor(() => expect(mocks.acceptTenantInvite).toHaveBeenCalledWith({ code: 'join-code' }))
  })

  it('renders empty member and invite states for a selected tenant', async () => {
    mocks.listTenantMembers.mockResolvedValueOnce([])
    mocks.listTenantInvites.mockResolvedValueOnce([])

    render(FinanceTenants)

    expect(await screen.findByText('No members found.')).toBeInTheDocument()
    expect(screen.getByText('No invites yet.')).toBeInTheDocument()
  })

  it('renders a selector-only view when no tenants are joined yet', async () => {
    mocks.listTenants.mockResolvedValueOnce([])

    render(FinanceTenants)

    expect(await screen.findByRole('combobox', { name: 'Selected tenant' })).toBeInTheDocument()
    expect(screen.queryByText('Members')).not.toBeInTheDocument()
  })

  it('renders an error state when tenant loading fails', async () => {
    mocks.listTenants.mockRejectedValueOnce(new Error('tenants exploded'))

    render(FinanceTenants)

    expect(await screen.findByRole('alert')).toHaveTextContent('tenants exploded')
  })

  it('hides member detail when the tenant selection is cleared', async () => {
    const user = userEvent.setup()
    render(FinanceTenants)

    await user.selectOptions(await screen.findByRole('combobox', { name: 'Selected tenant' }), '')
    expect(screen.queryByText('Members')).not.toBeInTheDocument()
  })
})
