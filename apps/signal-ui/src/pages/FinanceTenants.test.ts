import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceTenants from './FinanceTenants.svelte'

const mocks = vi.hoisted(() => ({
  listTenants: vi.fn(),
  listTenantMembers: vi.fn(),
  listTenantInvites: vi.fn(),
  createTenant: vi.fn(),
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
    const now = new Date('2026-06-20T12:00:00Z')
    Object.values(mocks).forEach((mock) => mock.mockReset())
    mocks.listTenants.mockResolvedValue([{ id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now }])
    mocks.listTenantMembers.mockResolvedValue([{ tenantId: 'tenant-1', userId: 'user-1', joinedAt: now }])
    mocks.listTenantInvites.mockResolvedValue([{ id: 'invite-1', tenantId: 'tenant-1', code: 'code-1', recipient: 'friend@example.com', createdByUserId: 'user-1', acceptedByUserId: null, createdAt: now, acceptedAt: null }])
    mocks.createTenant.mockResolvedValue({})
    mocks.createTenantInvite.mockResolvedValue({})
    mocks.acceptTenantInvite.mockResolvedValue({})
  })

  it('supports tenant create, invite create, and invite accept flows', async () => {
    const user = userEvent.setup()
    render(FinanceTenants)

    expect(await screen.findByText('user-1')).toBeInTheDocument()
    await user.type(screen.getByLabelText('Tenant name'), 'Travel')
    await user.click(screen.getByRole('button', { name: 'Create tenant' }))
    await waitFor(() => expect(mocks.createTenant).toHaveBeenCalled())

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
