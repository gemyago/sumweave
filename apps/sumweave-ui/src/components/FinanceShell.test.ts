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

  it('renders primary navbar links and keeps tenant identity first in the user menu', async () => {
    const user = userEvent.setup()
    const { container } = render(FinanceShell, {
      currentPath: '/finance',
    })

    expect(mocks.shellState.initialize).toHaveBeenCalledTimes(1)
    expect(container.firstElementChild).toHaveAttribute('data-bootstrap-finance-shell', 'true')
    expect(container.firstElementChild).toHaveAttribute('data-bs-theme', 'dark')
    expect(screen.getByRole('link', { name: 'Sumweave' })).toHaveAttribute('href', '#/finance')
    expect(screen.getByLabelText('Finance navigation')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Dashboard', current: 'page' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Transactions' })).toHaveAttribute('href', '#/finance/transactions')
    expect(screen.getByRole('link', { name: 'Accounts' })).toHaveAttribute('href', '#/finance/accounts')
    expect(screen.queryByRole('link', { name: 'Categories' })).not.toBeInTheDocument()
    expect(container.querySelector('aside')).toBeNull()
    expect(screen.getByLabelText('Finance navigation').querySelectorAll('a')).toHaveLength(3)
    expect(container.querySelector('header')).toHaveClass('navbar', 'navbar-expand-sm')
    expect(screen.queryByText('Household')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'User menu' })).not.toHaveTextContent('Household')
    await user.click(screen.getByRole('button', { name: 'User menu' }))
    expect(container.querySelector('#finance-user-menu')?.firstElementChild).toHaveTextContent('Active tenant Household')
    expect(screen.getByRole('link', { name: 'Categories' })).toHaveAttribute('href', '#/finance/categories')
    expect(screen.getByRole('link', { name: 'Rules' })).toHaveAttribute('href', '#/finance/rules')
    expect(screen.getByRole('link', { name: 'Connections & sync' })).toHaveAttribute('href', '#/finance/connections')
    expect(screen.getByRole('link', { name: 'Imports' })).toHaveAttribute('href', '#/finance/imports')
    expect(screen.getByRole('link', { name: 'Tenants' })).toHaveAttribute('href', '#/finance/tenants')
    expect(screen.getByLabelText('Finance utilities')).toBeInTheDocument()
    expect(screen.getByRole('radiogroup', { name: 'Theme' })).toBeInTheDocument()
    expect(screen.queryByLabelText('Finance sections')).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Overview' })).not.toBeInTheDocument()
    expect(screen.queryByText('Bootstrap pilot')).not.toBeInTheDocument()
  })

  it('uses Bootstrap responsive navbar collapse and hides redundant mobile section breadcrumbs', () => {
    render(FinanceShell, {
      currentPath: '/finance',
    })

    expect(screen.getByLabelText('Finance navigation')).toHaveClass('collapse', 'navbar-collapse', 'order-3', 'order-sm-0')
    for (const label of [
      'Dashboard',
      'Transactions',
      'Accounts',
    ]) {
      expect(screen.getByRole('link', { name: label })).toHaveClass(
        'px-2',
        'py-2',
        'text-nowrap',
      )
    }
    const breadcrumb = screen.getByRole('navigation', { name: 'Breadcrumb' })
    expect(breadcrumb).toHaveClass('d-none', 'd-sm-block')
    expect(screen.getByRole('link', { name: 'Finance' })).toHaveAttribute('href', '#/finance')
    expect(breadcrumb.querySelector('[aria-current="page"]')).toHaveTextContent('Dashboard')
    const breadcrumbItems = breadcrumb.querySelectorAll('li')
    expect(breadcrumbItems).toHaveLength(2)
  })

  it('marks the phone Finance navigation targets for shared 44px sizing', () => {
    render(FinanceShell, { currentPath: '/finance' })

    expect(screen.getByRole('button', { name: 'Toggle Finance navigation' })).toHaveClass('finance-primary-nav-target')
    for (const label of ['Dashboard', 'Transactions', 'Accounts']) {
      expect(screen.getByRole('link', { name: label })).toHaveClass('finance-primary-nav-target')
    }
  })

  it('toggles phone navigation and closes it on Escape or primary navigation', async () => {
    const user = userEvent.setup()
    render(FinanceShell, { currentPath: '/finance' })
    const trigger = screen.getByRole('button', { name: 'Toggle Finance navigation' })
    await user.click(trigger)
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByLabelText('Finance navigation')).toHaveClass('show')
    await user.keyboard('{Escape}')
    expect(trigger).toHaveFocus()
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    await user.click(trigger)
    await user.click(screen.getByRole('link', { name: 'Transactions' }))
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
  })

  it('focuses the top tenant selector before secondary destinations', async () => {
    const user = userEvent.setup()
    mocks.shellState.tenants.push({ id: 'tenant-2', name: 'Operations', displayCurrency: 'EUR' })
    const { container } = render(FinanceShell, { currentPath: '/finance' })
    await user.click(screen.getByRole('button', { name: 'User menu' }))
    expect(screen.getByRole('combobox', { name: 'Active tenant' })).toHaveFocus()
    expect(container.querySelector('#finance-user-menu')?.firstElementChild?.querySelector('select')).toBe(screen.getByRole('combobox', { name: 'Active tenant' }))
  })

  it('adds breadcrumb a11y hooks for shared-style targeting', () => {
    const { container } = render(FinanceShell, {
      currentPath: '/finance',
    })

    expect(FinanceShellSource).toContain('finance-shell-breadcrumb-nav')
    expect(FinanceShellSource).toContain('finance-shell-breadcrumb')

    const breadcrumbNav = container.querySelector('nav.finance-shell-breadcrumb-nav')
    const breadcrumb = container.querySelector('ol.finance-shell-breadcrumb')
    expect(breadcrumbNav).toBeTruthy()
    expect(breadcrumb).toBeTruthy()
    expect(breadcrumbNav).toHaveClass('d-none', 'd-sm-block')
    expect(breadcrumb).toHaveClass('breadcrumb', 'mb-0')
  })

  it('hides the shared tenant selector on the tenants route even with multiple tenants', async () => {
    const user = userEvent.setup()
    mocks.shellState.tenants.push({ id: 'tenant-2', name: 'Operations', displayCurrency: 'EUR' })
    render(FinanceShell, {
      currentPath: '/finance/tenants',
    })

    await user.click(screen.getByRole('button', { name: 'User menu' }))
    expect(screen.queryByRole('combobox', { name: 'Active tenant' })).not.toBeInTheDocument()
  })

  it('keeps unsupported paths out of the nav active state', () => {
    render(FinanceShell, {
      currentPath: '/outside-finance',
    })

    expect(screen.queryByRole('link', { current: 'page' })).not.toBeInTheDocument()
    expect(screen.getByText('Workspace')).toHaveAttribute('aria-current', 'page')
  })

  it('keeps parent destinations active for nested finance detail and synthetic routes', async () => {
    const user = userEvent.setup()
    const { rerender } = render(FinanceShell, {
      currentPath: '/finance/accounts/account-1',
    })

    expect(screen.getByRole('link', { name: 'Accounts', current: 'page' })).toBeInTheDocument()

    rerender({ currentPath: '/finance/connections/synthetic?state=state-1' })
    await user.click(screen.getByRole('button', { name: 'User menu' }))
    expect(
      screen.getByRole('link', { name: 'Connections & sync', current: 'page' }),
    ).toBeInTheDocument()

    rerender({ currentPath: '/finance/transactions/new' })
    expect(screen.getByRole('link', { name: 'Transactions', current: 'page' })).toBeInTheDocument()
  })

  it('uses linked section parents and non-link current crumbs for detail routes', () => {
    const { rerender } = render(FinanceShell, {
      currentPath: '/finance/accounts/account-1',
    })

    let breadcrumb = screen.getByRole('navigation', { name: 'Breadcrumb' })
    expect(breadcrumb.querySelectorAll('.breadcrumb-item')).toHaveLength(3)
    expect(screen.getAllByRole('link', { name: 'Accounts' }).at(-1)).toHaveAttribute('href', '#/finance/accounts')
    expect(breadcrumb.querySelector('[aria-current="page"]')).toHaveTextContent('Account detail')
    expect(breadcrumb.querySelector('[aria-current="page"]')?.querySelector('a')).toBeNull()

    rerender({ currentPath: '/finance/transactions/tx-1' })
    breadcrumb = screen.getByRole('navigation', { name: 'Breadcrumb' })
    expect(screen.getAllByRole('link', { name: 'Transactions' }).at(-1)).toHaveAttribute('href', '#/finance/transactions')
    expect(breadcrumb.querySelector('[aria-current="page"]')).toHaveTextContent('Transaction')
    expect(breadcrumb.querySelector('[aria-current="page"]')?.querySelector('a')).toBeNull()
  })

  it('names the synthetic detail breadcrumb without repeating its section parent', () => {
    render(FinanceShell, { currentPath: '/finance/connections/synthetic?state=state-1' })
    const breadcrumb = screen.getByRole('navigation', { name: 'Breadcrumb' })
    expect(breadcrumb.querySelectorAll('.breadcrumb-item')).toHaveLength(3)
    expect(screen.getByRole('link', { name: 'Connections & sync' })).toHaveAttribute('href', '#/finance/connections')
    expect(breadcrumb.querySelector('[aria-current="page"]')).toHaveTextContent('Synthetic setup')
    expect(breadcrumb.querySelector('[aria-current="page"]')?.querySelector('a')).toBeNull()
  })

  it('uses theme-aware text for user-menu group headings', async () => {
    const user = userEvent.setup()
    render(FinanceShell, { currentPath: '/finance' })
    await user.click(screen.getByRole('button', { name: 'User menu' }))
    for (const name of ['Finance setup', 'Workspace', 'Preferences']) {
      expect(screen.getByRole('heading', { name })).toHaveClass('text-body-secondary')
    }
  })

  it('shows the shell-level tenant chooser only for multi-tenant tenant-scoped routes', async () => {
    const user = userEvent.setup()
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

    await user.click(screen.getByRole('button', { name: 'User menu' }))
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

    await user.click(screen.getByRole('button', { name: 'User menu' }))
    await user.selectOptions(screen.getByRole('combobox', { name: 'Active tenant' }), 'tenant-2')
    expect(mocks.replace).not.toHaveBeenCalled()
    expect(screen.getByRole('link', { name: 'Accounts', current: 'page' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'User menu' })).toHaveAttribute('aria-expanded', 'true')
    await user.click(screen.getByRole('radio', { name: 'Light' }))
    await user.click(screen.getByRole('button', { name: 'Sign out' }))

    expect(mocks.shellState.selectTenant).toHaveBeenCalledWith('tenant-2')
    expect(mocks.themeStore.setPreference).toHaveBeenCalledWith('light')
    expect(mocks.clearAuth).toHaveBeenCalledTimes(1)
    expect(mocks.replace).toHaveBeenCalledWith('/login')
  })

  it('opens by keyboard and returns focus to the trigger on Escape', async () => {
    const user = userEvent.setup()
    render(FinanceShell, { currentPath: '/finance' })
    const trigger = screen.getByRole('button', { name: 'User menu' })
    trigger.focus()
    await user.keyboard('{Enter}')
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByRole('link', { name: 'Categories' })).toHaveFocus()
    await user.tab()
    expect(screen.getByRole('link', { name: 'Rules' })).toHaveFocus()
    await user.keyboard('{Escape}')
    expect(trigger).toHaveFocus()
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByRole('link', { name: 'Rules' })).not.toBeInTheDocument()
  })

  it('dismisses on outside click', async () => {
    const user = userEvent.setup()
    render(FinanceShell, { currentPath: '/finance' })
    await user.click(screen.getByRole('button', { name: 'User menu' }))
    await user.click(screen.getByLabelText('Finance navigation'))
    expect(screen.getByRole('button', { name: 'User menu' })).toHaveAttribute('aria-expanded', 'false')
  })

  it('closes after secondary navigation even when the shell route prop has not changed yet', async () => {
    const user = userEvent.setup()
    render(FinanceShell, { currentPath: '/finance' })
    await user.click(screen.getByRole('button', { name: 'User menu' }))
    await user.click(screen.getByRole('link', { name: 'Imports' }))
    expect(screen.getByRole('button', { name: 'User menu' })).toHaveAttribute('aria-expanded', 'false')
    expect(screen.getByRole('button', { name: 'User menu' })).toHaveFocus()
  })

  it('closes after an external route change', async () => {
    const user = userEvent.setup()
    const { rerender } = render(FinanceShell, { currentPath: '/finance' })
    await user.click(screen.getByRole('button', { name: 'User menu' }))
    await rerender({ currentPath: '/finance/transactions/new' })
    expect(screen.getByRole('button', { name: 'User menu' })).toHaveAttribute('aria-expanded', 'false')
    expect(screen.getByRole('navigation', { name: 'Breadcrumb' })).not.toHaveClass('d-none')
  })

  it('closes when keyboard focus leaves the dropdown', async () => {
    const user = userEvent.setup()
    render(FinanceShell, { currentPath: '/finance' })
    await user.click(screen.getByRole('button', { name: 'User menu' }))
    screen.getByRole('button', { name: 'Sign out' }).focus()
    await user.tab()
    expect(screen.getByRole('button', { name: 'User menu' })).toHaveAttribute('aria-expanded', 'false')
    expect(screen.getByRole('link', { name: 'Finance' })).toHaveFocus()
  })

  it('does not define route-local styles or style attributes', () => {
    expect(FinanceShellSource).not.toMatch(/<style[\s>]/)
    expect(FinanceShellSource).not.toMatch(/\sstyle=/)
    expect(BootstrapFinanceDashboardSource).toContain('class="card-body p-2 p-sm-3"')
    expect(BootstrapFinanceDashboardSource).toContain('class="d-none d-sm-block text-body-secondary mb-0"')
    expect(BootstrapFinanceDashboardSource).not.toContain('Finance overview')
    expect(BootstrapFinanceDashboardSource).not.toContain('Open accounts')
    expect(BootstrapFinanceDashboardSource).toContain('class="d-none d-sm-block my-3 my-xl-4"')
    expect(BootstrapFinanceDashboardSource).toContain('Open User menu and choose Active tenant to load this dashboard.')
    expect(BootstrapFinanceDashboardSource).not.toContain('header tenant selector')
  })
})
