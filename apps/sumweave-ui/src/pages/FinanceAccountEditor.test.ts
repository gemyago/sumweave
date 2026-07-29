import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceAccountEditor from './FinanceAccountEditor.svelte'

const mocks = vi.hoisted(() => ({ listTenants: vi.fn(), createAccount: vi.fn() }))
vi.mock('../lib/finance/api', async (importOriginal) => ({ ...(await importOriginal<typeof import('../lib/finance/api')>()), createSignalFinanceApiForAuth: vi.fn(() => ({ ...mocks })) }))
vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

describe('Finance account editor page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.localStorage.clear()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants.mockResolvedValue([{ id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now }])
    mocks.createAccount.mockResolvedValue({})
  })

  it('creates an account from the focused route', async () => {
    const user = userEvent.setup()
    render(FinanceAccountEditor)

    await user.type(await screen.findByLabelText('Account name'), 'Savings')
    await user.selectOptions(screen.getByLabelText('Account currency'), 'EUR')
    await user.click(screen.getByRole('button', { name: 'Create account' }))

    await waitFor(() => expect(mocks.createAccount).toHaveBeenCalledWith({ tenantId: 'tenant-1', name: 'Savings', currency: 'EUR', kind: 'manual' }))
    expect(screen.getByRole('status')).toHaveTextContent('Account created.')
  })

  it('keeps a recoverable create error on the editor', async () => {
    const user = userEvent.setup()
    mocks.createAccount.mockRejectedValueOnce(new Error('Name unavailable'))
    render(FinanceAccountEditor)

    await user.type(await screen.findByLabelText('Account name'), 'Savings')
    await user.click(screen.getByRole('button', { name: 'Create account' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Name unavailable')
    expect(screen.getByLabelText('Account name')).toHaveValue('Savings')
  })
})
