import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'

type MockTenantSummary = {
  id: string
  name: string
  displayCurrency: string
}

const mocks = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listCategories: vi.fn(),
  listTags: vi.fn(),
  listTransactions: vi.fn(),
  shellState: {
    embedded: true,
    loading: false,
    error: null,
    tenants: [] as MockTenantSummary[],
    selectedTenantId: 'tenant-1',
    selectedTenant: {
      id: 'tenant-1',
      name: 'Household',
      displayCurrency: 'USD',
    } as MockTenantSummary | null,
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
    listCategories: mocks.listCategories,
    listTags: mocks.listTags,
    listTransactions: mocks.listTransactions,
  })),
}))

vi.mock('../lib/finance/shell-state.svelte', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/finance/shell-state.svelte')>()),
  useFinanceShellState: vi.fn(() => mocks.shellState),
}))

vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

import FinanceTransactions from './FinanceTransactions.svelte'

describe('Finance transactions page inside the embedded shell', () => {
  beforeEach(() => {
    mocks.listAccounts.mockReset()
    mocks.listCategories.mockReset()
    mocks.listTags.mockReset()
    mocks.listTransactions.mockReset()
    mocks.listCategories.mockResolvedValue([])
    mocks.listTags.mockResolvedValue([])
    mocks.shellState.selectedTenantId = 'tenant-1'
    mocks.shellState.selectedTenant = {
      id: 'tenant-1',
      name: 'Household',
      displayCurrency: 'USD',
    }
    mocks.shellState.needsTenantSelection = false
  })

  it('shows transaction content without repeating tenant chrome from the shell', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listAccounts.mockResolvedValueOnce([
      { id: 'account-1', tenantId: 'tenant-1', name: 'Checking', currency: 'USD', kind: 'manual', provider: '', providerAccountId: '', hiddenAt: null, createdAt: now, updatedAt: now },
    ])
    mocks.listTransactions.mockResolvedValueOnce([
      {
        id: 'tx-1',
        tenantId: 'tenant-1',
        accountId: 'account-1',
        source: 'manual',
        status: 'booked',
        kind: 'expense',
        amountMinor: 1200,
        currency: 'USD',
        description: 'Groceries',
        effectiveAt: now,
        categoryId: null,
        tagIds: [],
        transferGroupId: null,
        transferMatchedAt: null,
        hiddenAt: null,
        providerOriginal: null,
        createdAt: now,
        updatedAt: now,
      },
    ])

    render(FinanceTransactions)

    expect(await screen.findByText('Groceries')).toBeInTheDocument()
    expect(screen.queryByText('Tenant workspace')).not.toBeInTheDocument()
    expect(screen.queryByRole('combobox', { name: 'Tenant' })).not.toBeInTheDocument()
  })

  it('stays free of duplicate tenant chrome even if the shell has not hydrated the tenant summary yet', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.shellState.selectedTenant = null
    mocks.listAccounts.mockResolvedValueOnce([])
    mocks.listTransactions.mockResolvedValueOnce([
      {
        id: 'tx-1',
        tenantId: 'tenant-1',
        accountId: 'account-1',
        source: 'manual',
        status: 'booked',
        kind: 'expense',
        amountMinor: 1200,
        currency: 'USD',
        description: 'Groceries',
        effectiveAt: now,
        categoryId: null,
        tagIds: [],
        transferGroupId: null,
        transferMatchedAt: null,
        hiddenAt: null,
        providerOriginal: null,
        createdAt: now,
        updatedAt: now,
      },
    ])

    render(FinanceTransactions)

    await screen.findByText('Groceries')
    expect(screen.queryByText('Tenant workspace')).not.toBeInTheDocument()
    expect(screen.queryByRole('combobox', { name: 'Tenant' })).not.toBeInTheDocument()
  })

  it('waits for the shared shell tenant choice without rendering a local tenant selector', async () => {
    mocks.shellState.selectedTenantId = ''
    mocks.shellState.selectedTenant = null
    mocks.shellState.needsTenantSelection = true

    render(FinanceTransactions)

    expect(
      await screen.findByText('Select an active tenant to continue on this finance route.'),
    ).toBeInTheDocument()
    expect(screen.queryByRole('combobox', { name: 'Tenant' })).not.toBeInTheDocument()
  })
})
