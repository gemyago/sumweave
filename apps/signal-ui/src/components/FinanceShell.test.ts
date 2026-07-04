import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceShell from './FinanceShell.svelte'

let compactViewport = false

const mocks = vi.hoisted(() => {
  const shellState = {
    embedded: false,
    loading: false,
    error: null,
    tenants: [
      {
        id: 'tenant-1',
        name: 'Household',
        displayCurrency: 'USD',
      },
    ],
    selectedTenantId: 'tenant-1',
    hasTenants: true,
    get hasMultipleTenants() {
      return this.tenants.length > 1
    },
    initialize: vi.fn().mockResolvedValue(undefined),
    selectTenant: vi.fn(),
  }

  return {
    shellState,
    clearAuth: vi.fn(),
    replace: vi.fn(),
  }
})

vi.mock('../lib/auth/auth-store.svelte', () => ({
  authStore: { clearAuth: mocks.clearAuth },
}))

vi.mock('svelte-spa-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('svelte-spa-router')>()
  return {
    ...actual,
    replace: mocks.replace,
  }
})

vi.mock('../lib/finance/shell-state.svelte', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/finance/shell-state.svelte')>()
  return {
    ...actual,
    createFinanceShellState: vi.fn(() => mocks.shellState),
    provideFinanceShellState: vi.fn((state) => state),
  }
})

describe('FinanceShell', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    compactViewport = false
    vi.stubGlobal(
      'matchMedia',
      vi.fn(() => ({
        matches: compactViewport,
        media: '(max-width: 960px)',
        onchange: null,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    )
    mocks.shellState.loading = false
    mocks.shellState.selectedTenantId = 'tenant-1'
    mocks.shellState.tenants = [
      {
        id: 'tenant-1',
        name: 'Household',
        displayCurrency: 'USD',
      },
    ]
    mocks.shellState.hasTenants = true
  })

  it('hides the shared tenant selector on the tenants route', () => {
    render(FinanceShell, {
      currentPath: '/finance/tenants',
    })

    expect(mocks.shellState.initialize).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('combobox', { name: 'Active tenant' })).not.toBeInTheDocument()
  })

  it('keeps unsupported paths out of the rail active state', () => {
    render(FinanceShell, {
      currentPath: '/outside-finance',
    })

    expect(screen.getByLabelText('Finance navigation')).toBeInTheDocument()
    expect(screen.queryByRole('link', { current: 'page' })).not.toBeInTheDocument()
  })

  it('keeps parent destinations active for nested finance detail routes', () => {
    render(FinanceShell, {
      currentPath: '/finance/accounts/account-1',
    })

    expect(screen.getByRole('link', { name: 'Accounts', current: 'page' })).toBeInTheDocument()
  })

  it('keeps connections active for the nested synthetic setup route', () => {
    render(FinanceShell, {
      currentPath: '/finance/connections/synthetic',
    })

    expect(
      screen.getByRole('link', { name: 'Connections & sync', current: 'page' }),
    ).toBeInTheDocument()
  })

  it('keeps connections active for the nested synthetic setup route with query state', () => {
    render(FinanceShell, {
      currentPath: '/finance/connections/synthetic?state=state-1',
    })

    expect(screen.getByRole('link', { name: 'Connections & sync', current: 'page' })).toBeInTheDocument()
  })

  it('keeps transactions active for nested transaction mutation routes', () => {
    render(FinanceShell, {
      currentPath: '/finance/transactions/new',
    })

    expect(screen.getByRole('link', { name: 'Transactions', current: 'page' })).toBeInTheDocument()
  })

  it('shows the loading disabled selector and empty-state tenant copy when no tenants are available', () => {
    mocks.shellState.loading = true
    mocks.shellState.selectedTenantId = ''
    mocks.shellState.tenants = []
    mocks.shellState.hasTenants = false

    render(FinanceShell, {
      currentPath: '/finance',
    })

    expect(screen.queryByRole('combobox', { name: 'Active tenant' })).not.toBeInTheDocument()
  })

  it('hides the shared selector when the workspace resolves to a single tenant', () => {
    mocks.shellState.selectedTenantId = ''
    mocks.shellState.tenants = [
      {
        id: 'tenant-1',
        name: 'Household',
        displayCurrency: 'USD',
      },
    ]

    render(FinanceShell, {
      currentPath: '/finance/accounts',
    })

    expect(screen.queryByRole('combobox', { name: 'Active tenant' })).not.toBeInTheDocument()
  })

  it('shows the select-tenant placeholder when multiple tenant options exist but none is active yet', () => {
    mocks.shellState.selectedTenantId = ''
    mocks.shellState.tenants = [
      {
        id: 'tenant-1',
        name: 'Household',
        displayCurrency: 'USD',
      },
      {
        id: 'tenant-2',
        name: 'Operations',
        displayCurrency: 'EUR',
      },
    ]

    render(FinanceShell, {
      currentPath: '/finance/accounts',
    })

    expect(screen.getByRole('option', { name: 'Select tenant' })).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Active tenant' })).toBeEnabled()
  })

  it('lets the user change tenants and sign out from the shell utility row', async () => {
    const user = userEvent.setup()
    mocks.shellState.tenants = [
      {
        id: 'tenant-1',
        name: 'Household',
        displayCurrency: 'USD',
      },
      {
        id: 'tenant-2',
        name: 'Operations',
        displayCurrency: 'EUR',
      },
    ]

    render(FinanceShell, {
      currentPath: '/finance/accounts',
    })

    expect(screen.getByRole('option', { name: 'Operations · EUR' })).toBeInTheDocument()
    await user.selectOptions(screen.getByRole('combobox', { name: 'Active tenant' }), 'tenant-2')
    await user.click(screen.getByRole('button', { name: 'Sign out' }))

    expect(mocks.shellState.selectTenant).toHaveBeenCalledWith('tenant-2')
    expect(mocks.clearAuth).toHaveBeenCalledTimes(1)
    expect(mocks.replace).toHaveBeenCalledWith('/login')
  })

  it('collapses finance navigation behind a compact route menu on narrow viewports', async () => {
    const user = userEvent.setup()
    compactViewport = true
    mocks.shellState.tenants = [
      {
        id: 'tenant-1',
        name: 'Household',
        displayCurrency: 'USD',
      },
      {
        id: 'tenant-2',
        name: 'Operations',
        displayCurrency: 'EUR',
      },
    ]

    render(FinanceShell, {
      currentPath: '/finance/transactions',
    })

    expect(screen.getAllByText('Finance / Transactions').length).toBeGreaterThan(0)
    expect(screen.getByRole('button', { name: 'Open menu' })).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByRole('link', { name: 'Connections & sync' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Open menu' }))

    expect(screen.getByRole('button', { name: 'Close menu' })).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByRole('link', { name: 'Connections & sync' })).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Active tenant' })).toBeInTheDocument()
  })
})
