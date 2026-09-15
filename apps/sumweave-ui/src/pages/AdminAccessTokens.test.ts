import { beforeEach, describe, expect, it, vi } from 'vitest'
import { faker } from '@faker-js/faker'
import { render, screen } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import AdminAccessTokens from './AdminAccessTokens.svelte'

const mocks = vi.hoisted(() => ({ list: vi.fn(), create: vi.fn(), rotate: vi.fn(), revoke: vi.fn(), copy: vi.fn() }))

vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'session-token', clearAuth: vi.fn(), setAuth: vi.fn() } }))
vi.mock('../lib/auth/auth-fetch', () => ({ createAuthFetch: vi.fn(() => fetch) }))
vi.mock('../lib/auth/access-tokens-api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/auth/access-tokens-api')>()),
  createAccessTokensApi: vi.fn(() => mocks),
}))

function tokenFixture(status: 'active' | 'expired' | 'revoked' = 'active') {
  return {
    id: faker.string.uuid(), name: faker.word.noun(), hint: `swat_${faker.string.alphanumeric(8)}...`, permission: 'read-only' as const, status,
    expiresAt: null, revokedAt: status === 'revoked' ? faker.date.recent().toISOString() : null,
    createdAt: faker.date.recent().toISOString(), updatedAt: faker.date.recent().toISOString(),
  }
}

describe('Admin access tokens page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.list.mockResolvedValue([])
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: mocks.copy } })
  })

  it('shows bounded loading and empty states', async () => {
    render(AdminAccessTokens)
    expect(screen.getByRole('status')).toHaveTextContent('Loading access tokens…')
    expect(await screen.findByText('You have not created an access token yet.')).toBeInTheDocument()
  })

  it('renders active, expired, and revoked metadata with active actions only', async () => {
    const active = tokenFixture()
    mocks.list.mockResolvedValue([active, tokenFixture('expired'), tokenFixture('revoked')])
    render(AdminAccessTokens)
    expect(await screen.findByText(active.name)).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Rotate' })).toHaveLength(1)
    expect(screen.getAllByRole('button', { name: 'Revoke' })).toHaveLength(1)
    expect(screen.getByText(/expired/)).toBeInTheDocument()
    expect(screen.getByText(/revoked/)).toBeInTheDocument()
  })

  it('validates, creates, copies, and dismisses a one-time secret', async () => {
    const user = userEvent.setup()
    const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue()
    const issued = { token: tokenFixture(), apiToken: `swat_${faker.string.alphanumeric(48)}` }
    mocks.create.mockResolvedValue(issued)
    render(AdminAccessTokens)
    await user.click(screen.getByRole('button', { name: 'Create token' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Enter a token name.')
    await user.type(screen.getByLabelText('Name'), issued.token.name)
    await user.click(screen.getByRole('button', { name: 'Create token' }))
    expect(await screen.findByLabelText('Issued access token')).toHaveTextContent(issued.apiToken)
    await user.click(screen.getByRole('button', { name: 'Copy token' }))
    expect(writeText).toHaveBeenCalledWith(issued.apiToken)
    await user.click(screen.getByRole('button', { name: 'Dismiss' }))
    expect(screen.queryByLabelText('Issued access token')).not.toBeInTheDocument()
  })

  it('confirms rotate and refreshes revoked metadata', async () => {
    const user = userEvent.setup()
    const token = tokenFixture()
    mocks.list.mockResolvedValueOnce([token]).mockResolvedValueOnce([{ ...token, status: 'revoked' }])
    const replacement = tokenFixture()
    mocks.rotate.mockResolvedValue({ token: replacement, apiToken: faker.string.alphanumeric(48) })
    mocks.revoke.mockResolvedValue(undefined)
    render(AdminAccessTokens)
    await user.click(await screen.findByRole('button', { name: 'Rotate' }))
    expect(screen.getByRole('heading', { name: 'Confirm rotate' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Confirm rotate' }))
    expect(mocks.rotate).toHaveBeenCalledWith(token.id, { expiresAt: null })
    await user.click(screen.getByRole('button', { name: 'Dismiss' }))
    await user.click(screen.getByRole('button', { name: 'Revoke' }))
    await user.click(screen.getByRole('button', { name: 'Confirm revoke' }))
    expect(mocks.revoke).toHaveBeenCalledWith(replacement.id)
    expect(mocks.list).toHaveBeenCalledTimes(2)
  })

  it('sends an explicit null expiry when rotating metadata that omits it', async () => {
    const user = userEvent.setup()
    const { expiresAt, ...token } = tokenFixture()
    void expiresAt
    mocks.list.mockResolvedValue([token])
    mocks.rotate.mockResolvedValue({ token: tokenFixture(), apiToken: faker.string.alphanumeric(48) })
    render(AdminAccessTokens)

    await user.click(await screen.findByRole('button', { name: 'Rotate' }))
    await user.click(screen.getByRole('button', { name: 'Confirm rotate' }))

    expect(mocks.rotate).toHaveBeenCalledWith(token.id, { expiresAt: null })
  })
})
