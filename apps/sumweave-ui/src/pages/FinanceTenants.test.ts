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
  archiveTenant: vi.fn(),
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
    mocks.listTenantMembers.mockResolvedValue([
      { tenantId: 'tenant-1', userId: 'user-1', username: 'casey', joinedAt: now },
      { tenantId: 'tenant-1', userId: 'user-2', joinedAt: now },
    ])
    mocks.listTenantInvites.mockResolvedValue([
      { id: 'invite-pending', tenantId: 'tenant-1', code: 'pending-code', recipient: 'friend@example.com', createdByUserId: 'user-1', acceptedByUserId: null, createdAt: now, acceptedAt: null },
      { id: 'invite-accepted', tenantId: 'tenant-1', code: 'accepted-code', recipient: 'teammate@example.com', createdByUserId: 'user-1', acceptedByUserId: 'user-2', createdAt: now, acceptedAt: now },
    ])
    mocks.createTenant.mockResolvedValue({})
    mocks.createTenantInvite.mockResolvedValue({})
    mocks.acceptTenantInvite.mockResolvedValue({})
  })

  it('keeps tenant forms closed until their action is selected and opens one panel at a time', async () => {
    const user = userEvent.setup()
    render(FinanceTenants)

    await screen.findByText('casey')
    expect(screen.queryByLabelText('Tenant name')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Invite code')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Create tenant' }))
    expect(screen.getByRole('heading', { name: 'Create tenant' })).toBeInTheDocument()
    expect(screen.getByLabelText('Add starter categories and tags')).toBeChecked()

    await user.click(screen.getByRole('button', { name: 'Join by code' }))
    expect(screen.getByRole('heading', { name: 'Join by invite code' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Create tenant' })).not.toBeInTheDocument()
  })

  it('creates tenants with a supported currency selector and explicit starter-category toggle', async () => {
    const user = userEvent.setup()
    render(FinanceTenants)

    await user.click(await screen.findByRole('button', { name: 'Create tenant' }))
    const createForm = screen.getByRole('heading', { name: 'Create tenant' }).closest('form')
    expect(createForm).not.toBeNull()
    const form = within(createForm as HTMLFormElement)
    expect(form.getAllByRole('option').map((option) => option.textContent?.trim())).toEqual(supportedCurrencyCodes)

    await user.type(form.getByLabelText('Tenant name'), 'Bare workspace')
    await user.click(form.getByLabelText('Add starter categories and tags'))
    await user.click(form.getByRole('button', { name: 'Create tenant' }))

    await waitFor(() => expect(mocks.createTenant).toHaveBeenCalledWith({ name: 'Bare workspace', displayCurrency: 'USD', seedDefaults: false }))
  })

  it('preserves create-panel values after a recoverable error', async () => {
    const user = userEvent.setup()
    mocks.createTenant.mockRejectedValueOnce(new Error('tenant creation exploded'))
    render(FinanceTenants)

    await user.click(await screen.findByRole('button', { name: 'Create tenant' }))
    await user.type(screen.getByLabelText('Tenant name'), 'Travel')
    const form = screen.getByRole('heading', { name: 'Create tenant' }).closest('form')
    expect(form).not.toBeNull()
    await user.click(within(form as HTMLFormElement).getByRole('button', { name: 'Create tenant' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('tenant creation exploded')
    expect(screen.getByLabelText('Tenant name')).toHaveValue('Travel')
    expect(screen.getByRole('heading', { name: 'Create tenant' })).toBeInTheDocument()
  })

  it('prefills and saves the selected tenant from the edit panel', async () => {
    const user = userEvent.setup()
    const now = new Date('2026-06-20T12:00:00Z')
    const updatedAt = new Date('2026-06-21T12:00:00Z')
    mocks.listTenants
      .mockResolvedValueOnce([{ id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now }])
      .mockResolvedValueOnce([{ id: 'tenant-1', name: 'Household Updated', displayCurrency: 'PLN', joinedAt: now, createdAt: now, updatedAt }])
    mocks.updateTenant.mockResolvedValue(undefined)
    render(FinanceTenants)

    await user.click(await screen.findByRole('button', { name: 'Edit selected' }))
    const editForm = screen.getByRole('heading', { name: 'Edit selected tenant' }).closest('form')
    expect(editForm).not.toBeNull()
    const form = within(editForm as HTMLFormElement)
    expect(form.getByLabelText('Tenant name')).toHaveValue('Household')
    await user.clear(form.getByLabelText('Tenant name'))
    await user.type(form.getByLabelText('Tenant name'), 'Household Updated')
    await user.selectOptions(form.getByRole('combobox', { name: 'Display currency' }), 'PLN')
    await user.click(form.getByRole('button', { name: 'Save tenant changes' }))

    await waitFor(() => expect(mocks.updateTenant).toHaveBeenCalledWith({ tenantId: 'tenant-1', name: 'Household Updated', displayCurrency: 'PLN' }))
    expect(screen.queryByRole('heading', { name: 'Edit selected tenant' })).not.toBeInTheDocument()
  })

  it('preserves selected context and entered edit values after an update error', async () => {
    const user = userEvent.setup()
    mocks.updateTenant.mockRejectedValueOnce(new Error('tenant update exploded'))
    render(FinanceTenants)

    await user.click(await screen.findByRole('button', { name: 'Edit selected' }))
    await user.clear(screen.getByLabelText('Tenant name'))
    await user.type(screen.getByLabelText('Tenant name'), 'Household Edited')
    await user.click(screen.getByRole('button', { name: 'Save tenant changes' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('tenant update exploded')
    expect(screen.getByLabelText('Tenant name')).toHaveValue('Household Edited')
    expect(screen.getByText('Active tenant · Household')).toBeInTheDocument()
  })

  it('uses focused invite and join panels for their respective mutations', async () => {
    const user = userEvent.setup()
    render(FinanceTenants)

    await user.click((await screen.findAllByRole('button', { name: 'Invite member' }))[0])
    await user.type(screen.getByLabelText('Invite recipient'), 'team@example.com')
    await user.click(screen.getByRole('button', { name: 'Create invite' }))
    await waitFor(() => expect(mocks.createTenantInvite).toHaveBeenCalledWith({ tenantId: 'tenant-1', recipient: 'team@example.com' }))

    await user.click(screen.getByRole('button', { name: 'Join by code' }))
    await user.type(screen.getByLabelText('Invite code'), 'join-code')
    await user.click(screen.getByRole('button', { name: 'Accept invite' }))
    await waitFor(() => expect(mocks.acceptTenantInvite).toHaveBeenCalledWith({ code: 'join-code' }))
  })

  it('shows username first with UUID as technical identity and falls back when missing', async () => {
    render(FinanceTenants)

    expect(await screen.findByText('casey')).toBeInTheDocument()
    expect(screen.getByText('casey').parentElement).toHaveTextContent('user-1')
    const unavailable = screen.getByText('Username unavailable').parentElement
    expect(unavailable).toHaveTextContent('user-2')
  })

  it('groups pending and accepted invites and only reveals or copies pending codes', async () => {
    const user = userEvent.setup()
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(window.navigator, 'clipboard', { configurable: true, value: { writeText } })
    render(FinanceTenants)

    const pending = await screen.findByRole('heading', { name: 'Pending' })
    expect(pending).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Accepted' })).toBeInTheDocument()
    expect(screen.getByText('friend@example.com')).toBeInTheDocument()
    expect(screen.getByText('teammate@example.com')).toBeInTheDocument()
    expect(screen.queryByText('pending-code')).not.toBeInTheDocument()
    expect(screen.queryByText('accepted-code')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Copy code' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Reveal code' }))
    expect(screen.getByText('pending-code')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Copy code' }))
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('pending-code'))
  })

  it('renders empty tenant, member, and invite states without persistent forms', async () => {
    mocks.listTenants.mockResolvedValueOnce([])
    render(FinanceTenants)

    expect(await screen.findByText('No joined active tenants. Use Create tenant or Join by code to get started.')).toBeInTheDocument()
    expect(screen.queryByText('Members')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Tenant name')).not.toBeInTheDocument()
  })

  it('archives a tenant from its compact list row and refreshes the active list', async () => {
    const user = userEvent.setup()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants
      .mockResolvedValueOnce([
        { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now },
        { id: 'tenant-2', name: 'Travel', displayCurrency: 'EUR', joinedAt: now, createdAt: now, updatedAt: now },
      ])
      .mockResolvedValueOnce([{ id: 'tenant-2', name: 'Travel', displayCurrency: 'EUR', joinedAt: now, createdAt: now, updatedAt: now }])
    render(FinanceTenants)

    const tenantRow = await screen.findByRole('article', { name: 'Tenant Household' })
    await user.click(within(tenantRow).getByRole('button', { name: 'Archive' }))
    await user.click(within(tenantRow).getByRole('button', { name: 'Confirm archive' }))

    await waitFor(() => expect(mocks.archiveTenant).toHaveBeenCalledWith({ tenantId: 'tenant-1' }))
    expect(screen.queryByRole('article', { name: 'Tenant Household' })).not.toBeInTheDocument()
  })

  it('renders a loading error', async () => {
    mocks.listTenants.mockRejectedValueOnce(new Error('tenants exploded'))
    render(FinanceTenants)

    expect(await screen.findByRole('alert')).toHaveTextContent('tenants exploded')
  })
})
