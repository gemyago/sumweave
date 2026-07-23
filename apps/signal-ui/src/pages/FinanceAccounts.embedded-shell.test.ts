import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'

type MockTenantSummary = {
  id: string
  name: string
  displayCurrency: string
}

const mocks = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  shellState: {
    embedded: true,
    loading: false,
    error: null,
    tenants: [] as MockTenantSummary[],
    selectedTenantId: '',
    selectedTenant: null as MockTenantSummary | null,
    needsTenantSelection: false,
    hasTenants: true,
    initialize: vi.fn().mockResolvedValue(undefined),
    selectTenant: vi.fn(),
  },
}))

vi.mock('../lib/finance/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/finance/api')>()),
  createSignalFinanceApiForAuth: vi.fn(() => ({
    listAccounts: mocks.listAccounts,
  })),
}))

vi.mock('../lib/finance/shell-state.svelte', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/finance/shell-state.svelte')>()),
  useFinanceShellState: vi.fn(() => mocks.shellState),
}))

vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

import FinanceAccounts from './FinanceAccounts.svelte'

describe('Finance accounts page inside the embedded shell', () => {
  beforeEach(() => {
    mocks.listAccounts.mockReset()
    mocks.shellState.selectedTenantId = ''
    mocks.shellState.selectedTenant = null
  })

  it('keeps the embedded page free of duplicate tenant chrome while the shell resolves selection', async () => {
    mocks.listAccounts.mockResolvedValueOnce([])

    render(FinanceAccounts)

    await screen.findByRole('link', { name: 'Create account' })
    expect(screen.queryByText('Tenant workspace')).not.toBeInTheDocument()
    expect(screen.queryByRole('combobox', { name: 'Tenant' })).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Create account' })).toHaveAttribute('href', '#/finance/accounts/new')
  })

  it('shows account content without repeating tenant chrome once the shell has resolved a tenant', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.shellState.selectedTenantId = 'tenant-1'
    mocks.shellState.selectedTenant = {
      id: 'tenant-1',
      name: 'Household',
      displayCurrency: 'USD',
    }
    mocks.listAccounts.mockResolvedValueOnce([
      {
        id: 'account-1',
        tenantId: 'tenant-1',
        name: 'Checking',
        currency: 'USD',
        kind: 'manual',
        provider: '',
        providerAccountId: '',
        hiddenAt: null,
        createdAt: now,
        updatedAt: now,
      },
    ])

    render(FinanceAccounts)

    expect(await screen.findByText('Checking')).toBeInTheDocument()
    expect(screen.queryByText('Tenant workspace')).not.toBeInTheDocument()
    expect(screen.getByText('Checking')).toBeInTheDocument()
    expect(screen.queryByRole('combobox', { name: 'Tenant' })).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Create account' })).toHaveAttribute('href', '#/finance/accounts/new')
  })
})
