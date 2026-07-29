import { getContext, setContext } from 'svelte'
import { authStore } from '../auth/auth-store.svelte'
import {
  createSignalFinanceApiForAuth,
  type FinanceTenantSummary,
} from './api'
import {
  chooseFinanceTenantId,
  setPreferredFinanceTenantId,
} from './tenant-selection'

const FINANCE_SHELL_CONTEXT = Symbol('finance-shell-state')

export function isFinanceTenantScopedRoute(path: string): boolean {
  return path !== '/finance/tenants' && path.startsWith('/finance')
}

export class FinanceShellState {
  embedded: boolean
  loading = $state(true)
  error = $state<string | null>(null)
  tenants = $state<FinanceTenantSummary[]>([])
  selectedTenantId = $state('')

  #initializePromise: Promise<void> | null = null
  #initialized = false
  #appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'

  constructor(embedded = false) {
    this.embedded = embedded
  }

  get selectedTenant(): FinanceTenantSummary | null {
    return this.tenants.find((item) => item.id === this.selectedTenantId) ?? null
  }

  get needsTenantSelection(): boolean {
    return this.tenants.length > 1 && this.selectedTenantId === ''
  }

  get hasTenants(): boolean {
    return this.tenants.length > 0
  }

  get hasMultipleTenants(): boolean {
    return this.tenants.length > 1
  }

  async initialize(force = false): Promise<void> {
    if (this.#initialized && !force) {
      this.loading = false
      return
    }
    if (this.#initializePromise) {
      return this.#initializePromise
    }

    this.#initializePromise = this.#loadTenants()
    try {
      await this.#initializePromise
    } finally {
      this.#initializePromise = null
    }
  }

  async refreshTenants(): Promise<void> {
    await this.initialize(true)
  }

  selectTenant(tenantId: string): void {
    this.selectedTenantId = tenantId
    setPreferredFinanceTenantId(tenantId)
  }

  async #loadTenants(): Promise<void> {
    this.loading = true
    this.error = null

    try {
      const financeApi = createSignalFinanceApiForAuth({
        baseUrl: this.#appBaseUrl,
        authStore,
      })
      this.tenants = await financeApi.listTenants()
      this.selectTenant(chooseFinanceTenantId(this.tenants))
      this.#initialized = true
    } catch (loadError) {
      this.error =
        loadError instanceof Error
          ? loadError.message
          : 'Failed to load finance workspace'
      throw loadError
    } finally {
      this.loading = false
    }
  }
}

export function createFinanceShellState(): FinanceShellState {
  return new FinanceShellState(false)
}

export function provideFinanceShellState(
  state: FinanceShellState,
): FinanceShellState {
  state.embedded = true
  setContext(FINANCE_SHELL_CONTEXT, state)
  return state
}

export function useFinanceShellState(): FinanceShellState {
  return getContext<FinanceShellState | undefined>(FINANCE_SHELL_CONTEXT) ??
    createFinanceShellState()
}
