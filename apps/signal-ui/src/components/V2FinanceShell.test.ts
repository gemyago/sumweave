import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import V2FinanceShell from './V2FinanceShell.svelte'

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
    setPreference: vi.fn(),
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

describe('V2FinanceShell', () => {
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
    mocks.themeStore.preference = 'auto'
    mocks.themeStore.effective = 'dark'
  })

  it('renders the dedicated finance shell without canonical finance chrome', () => {
    render(V2FinanceShell, {
      currentPath: '/v2/finance',
    })

    expect(mocks.shellState.initialize).toHaveBeenCalledTimes(1)
    expect(screen.getByLabelText('Finance navigation')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Overview', current: 'page' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Accounts' })).toHaveAttribute('href', '#/finance/accounts')
    expect(screen.getByRole('radiogroup', { name: 'Theme' })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: 'Auto' })).toBeChecked()
    expect(screen.queryByRole('combobox', { name: 'Active tenant' })).not.toBeInTheDocument()
  })

  it('keeps shell-level tenant, theme, and sign-out controls in the finance shell', async () => {
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
        name: 'Travel',
        displayCurrency: 'EUR',
      },
    ]

    const { container } = render(V2FinanceShell, {
      currentPath: '/v2/finance',
    })

    await user.selectOptions(screen.getByRole('combobox', { name: 'Active tenant' }), 'tenant-2')
    await user.click(screen.getByRole('radio', { name: 'Light' }))
    await user.click(screen.getByRole('button', { name: 'Sign out' }))

    expect(mocks.shellState.selectTenant).toHaveBeenCalledWith('tenant-2')
    expect(mocks.themeStore.setPreference).toHaveBeenCalledWith('light')
    expect(mocks.clearAuth).toHaveBeenCalledTimes(1)
    expect(mocks.replace).toHaveBeenCalledWith('/v2/login')
    expect(container.firstElementChild).toHaveAttribute('data-bs-theme', 'dark')
  })
})
