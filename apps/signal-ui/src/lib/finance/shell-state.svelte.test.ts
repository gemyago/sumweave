import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'

const financeApiMocks = vi.hoisted(() => ({
  listTenants: vi.fn(),
}))

vi.mock('./api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('./api')>()
  return {
    ...actual,
    createSignalFinanceApiForAuth: vi.fn(() => financeApiMocks),
  }
})

vi.mock('../auth/auth-store.svelte', () => ({
  authStore: {},
}))

import {
  createFinanceShellState,
  FinanceShellState,
  isFinanceTenantScopedRoute,
} from './shell-state.svelte'
import ShellStateContextHarness from './shell-state.context-harness.svelte'

function createTenant(id: string, name = id) {
  const now = new Date('2026-07-04T12:00:00Z')
  return {
    id,
    name,
    displayCurrency: 'USD',
    joinedAt: now,
    createdAt: now,
    updatedAt: now,
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((innerResolve, innerReject) => {
    resolve = innerResolve
    reject = innerReject
  })
  return { promise, resolve, reject }
}

describe('FinanceShellState', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.localStorage.clear()
  })

  it('recognizes tenant-scoped finance routes only', () => {
    expect(isFinanceTenantScopedRoute('/finance')).toBe(true)
    expect(isFinanceTenantScopedRoute('/finance/accounts')).toBe(true)
    expect(isFinanceTenantScopedRoute('/finance/tenants')).toBe(false)
    expect(isFinanceTenantScopedRoute('/chat')).toBe(false)
  })

  it('reuses the initialized tenant list until forced to refresh', async () => {
    financeApiMocks.listTenants.mockResolvedValue([createTenant('tenant-1')])

    const state = new FinanceShellState()

    await state.initialize()
    await state.initialize()
    await state.refreshTenants()

    expect(financeApiMocks.listTenants).toHaveBeenCalledTimes(2)
    expect(state.loading).toBe(false)
    expect(state.error).toBeNull()
    expect(state.selectedTenantId).toBe('tenant-1')
    expect(state.selectedTenant?.id).toBe('tenant-1')
  })

  it('shares the in-flight tenant load across concurrent initialize calls', async () => {
    const pending = deferred<ReturnType<typeof createTenant>[]>()
    financeApiMocks.listTenants.mockReturnValue(pending.promise)

    const state = new FinanceShellState()
    const first = state.initialize()
    const second = state.initialize()

    expect(financeApiMocks.listTenants).toHaveBeenCalledTimes(1)
    expect(state.loading).toBe(true)

    pending.resolve([createTenant('tenant-1')])
    await Promise.all([first, second])

    expect(state.loading).toBe(false)
    expect(state.selectedTenantId).toBe('tenant-1')
  })

  it('stores a fallback shell error when tenant loading rejects without an Error', async () => {
    financeApiMocks.listTenants.mockRejectedValue('boom')

    const state = new FinanceShellState()

    await expect(state.initialize()).rejects.toBe('boom')
    expect(state.error).toBe('Failed to load finance workspace')
    expect(state.loading).toBe(false)
    expect(state.hasTenants).toBe(false)
  })

  it('creates a detached shell state by default', () => {
    const state = createFinanceShellState()

    expect(state).toBeInstanceOf(FinanceShellState)
    expect(state.embedded).toBe(false)
  })

  it('reports when tenant selection is still required for a multi-tenant workspace', () => {
    const state = new FinanceShellState()
    state.tenants = [createTenant('tenant-1'), createTenant('tenant-2')]

    expect(state.needsTenantSelection).toBe(true)
    expect(state.selectedTenant).toBeNull()
  })

  it('returns the selected tenant summary when the active tenant is known', () => {
    const state = new FinanceShellState()
    state.tenants = [createTenant('tenant-1'), createTenant('tenant-2')]
    state.selectedTenantId = 'tenant-2'

    expect(state.needsTenantSelection).toBe(false)
    expect(state.selectedTenant?.id).toBe('tenant-2')
  })

  it('reuses the provided shell state from context helpers', () => {
    const state = new FinanceShellState()
    state.selectedTenantId = 'tenant-2'

    render(ShellStateContextHarness, {
      providedState: state,
    })

    expect(screen.getByTestId('embedded')).toHaveTextContent('true')
    expect(screen.getByTestId('selected-tenant')).toHaveTextContent('tenant-2')
    expect(state.embedded).toBe(true)
  })

  it('falls back to a detached shell state when no context is provided', () => {
    render(ShellStateContextHarness)

    expect(screen.getByTestId('embedded')).toHaveTextContent('false')
    expect(screen.getByTestId('selected-tenant').textContent).toBe('')
  })
})
