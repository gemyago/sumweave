import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceShell from './FinanceShell.svelte'
import FinanceShellSource from './FinanceShell.svelte?raw'
import BootstrapFinanceDashboardSource from '../pages/BootstrapFinanceDashboard.svelte?raw'

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
    themeStore: {
      preference: 'auto',
      effective: 'dark',
      setPreference: vi.fn(),
    },
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

vi.mock('../lib/theme/theme-store.svelte', () => ({
  themeStore: mocks.themeStore,
}))

describe('FinanceShell', () => {
  beforeEach(() => {
    vi.clearAllMocks()
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
    mocks.themeStore.preference = 'auto'
    mocks.themeStore.effective = 'dark'
  })

  it('renders the shared bootstrap finance shell without the legacy finance subnav', () => {
    const { container } = render(FinanceShell, {
      currentPath: '/finance',
    })

    expect(mocks.shellState.initialize).toHaveBeenCalledTimes(1)
    expect(container.firstElementChild).toHaveAttribute('data-bootstrap-finance-shell', 'true')
    expect(container.firstElementChild).toHaveAttribute('data-bs-theme', 'dark')
    expect(screen.getByRole('link', { name: 'Signal Foundry' })).toHaveAttribute('href', '#/finance')
    expect(screen.getByLabelText('Finance navigation')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Dashboard', current: 'page' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Transactions' })).toHaveAttribute('href', '#/finance/transactions')
    expect(screen.getByRole('link', { name: 'Accounts' })).toHaveAttribute('href', '#/finance/accounts')
    expect(screen.getByRole('link', { name: 'Categories' })).toHaveAttribute('href', '#/finance/categories')
    expect(screen.getByRole('link', { name: 'Connections & sync' })).toHaveAttribute('href', '#/finance/connections')
    expect(screen.getByRole('link', { name: 'Imports' })).toHaveAttribute('href', '#/finance/imports')
    expect(screen.getByRole('link', { name: 'Tenants' })).toHaveAttribute('href', '#/finance/tenants')
    expect(screen.getByLabelText('Finance utilities')).toBeInTheDocument()
    expect(screen.getByRole('radiogroup', { name: 'Theme' })).toBeInTheDocument()
    expect(screen.queryByLabelText('Finance sections')).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Overview' })).not.toBeInTheDocument()
    expect(screen.queryByText('Bootstrap pilot')).not.toBeInTheDocument()
  })

  it('uses compact, wrapping Bootstrap shell chrome on narrow screens while keeping every destination visible', () => {
    render(FinanceShell, {
      currentPath: '/finance',
    })

    expect(screen.getByLabelText('Finance navigation')).toHaveClass('flex-row', 'flex-lg-column', 'gap-2')
    for (const label of [
      'Dashboard',
      'Transactions',
      'Accounts',
      'Categories',
      'Connections & sync',
      'Imports',
      'Tenants',
    ]) {
      expect(screen.getByRole('link', { name: label })).toHaveClass(
        'flex-grow-1',
        'flex-lg-grow-0',
        'px-2',
        'px-lg-3',
        'py-2',
        'text-nowrap',
      )
    }
    expect(screen.getByText('Finance / Dashboard').parentElement).toHaveClass('d-none', 'd-sm-block')
  })

  it('hides the shared tenant selector on the tenants route', () => {
    render(FinanceShell, {
      currentPath: '/finance/tenants',
    })

    expect(screen.queryByRole('combobox', { name: 'Active tenant' })).not.toBeInTheDocument()
  })

  it('keeps unsupported paths out of the nav active state', () => {
    render(FinanceShell, {
      currentPath: '/outside-finance',
    })

    expect(screen.queryByRole('link', { current: 'page' })).not.toBeInTheDocument()
    expect(screen.getByText('Finance / Workspace')).toBeInTheDocument()
  })

  it('keeps parent destinations active for nested finance detail and synthetic routes', () => {
    const { rerender } = render(FinanceShell, {
      currentPath: '/finance/accounts/account-1',
    })

    expect(screen.getByRole('link', { name: 'Accounts', current: 'page' })).toBeInTheDocument()

    rerender({ currentPath: '/finance/connections/synthetic?state=state-1' })
    expect(
      screen.getByRole('link', { name: 'Connections & sync', current: 'page' }),
    ).toBeInTheDocument()

    rerender({ currentPath: '/finance/transactions/new' })
    expect(screen.getByRole('link', { name: 'Transactions', current: 'page' })).toBeInTheDocument()
  })

  it('shows the shell-level tenant chooser only for multi-tenant tenant-scoped routes', () => {
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

    expect(screen.getByRole('combobox', { name: 'Active tenant' })).toBeEnabled()
    expect(screen.getByRole('option', { name: 'Select tenant' })).toBeInTheDocument()
    expect(screen.queryByRole('combobox', { name: 'Tenant' })).not.toBeInTheDocument()
  })

  it('lets the user change tenants, switch theme, and sign out from the bootstrap shell', async () => {
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

    await user.selectOptions(screen.getByRole('combobox', { name: 'Active tenant' }), 'tenant-2')
    await user.click(screen.getByRole('radio', { name: 'Light' }))
    await user.click(screen.getByRole('button', { name: 'Sign out' }))

    expect(mocks.shellState.selectTenant).toHaveBeenCalledWith('tenant-2')
    expect(mocks.themeStore.setPreference).toHaveBeenCalledWith('light')
    expect(mocks.clearAuth).toHaveBeenCalledTimes(1)
    expect(mocks.replace).toHaveBeenCalledWith('/login')
  })

  it('does not define route-local styles or style attributes', () => {
    expect(FinanceShellSource).not.toMatch(/<style[\s>]/)
    expect(FinanceShellSource).not.toMatch(/\sstyle=/)
    expect(BootstrapFinanceDashboardSource).toContain('class="card-body p-3 p-xl-5"')
    expect(BootstrapFinanceDashboardSource).toContain('class="d-none d-sm-block text-body-secondary mb-0"')
    expect(BootstrapFinanceDashboardSource).toContain('class="d-none d-sm-block text-uppercase text-body-secondary fw-semibold small mb-2"')
    expect(BootstrapFinanceDashboardSource).toContain('<span class="d-sm-none">Transactions</span>')
    expect(BootstrapFinanceDashboardSource).toContain('class="my-3 my-xl-4"')
  })
})
