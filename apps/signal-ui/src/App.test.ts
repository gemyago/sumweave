import { describe, it, expect, beforeEach, vi } from 'vitest'
import { faker } from '@faker-js/faker'
import { render, screen } from '@testing-library/svelte'
import { waitFor } from '@testing-library/dom'
import userEvent from '@testing-library/user-event'
import App from './App.svelte'
import { POST_LOGIN_DESTINATION_KEY } from './lib/routing/post-login-destination'

vi.mock('./lib/strategy-workspace/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('./lib/strategy-workspace/api')>()
  return {
    ...actual,
    createSignalStrategyWorkspaceApiForAuth: vi.fn(() => ({
      listStrategies: vi.fn().mockResolvedValue([]),
      getStrategyVersion: vi.fn(),
      validateStrategy: vi.fn(),
      createStrategyVersion: vi.fn(),
      duplicateStrategyVersion: vi.fn(),
      createEvaluationBacktest: vi.fn(),
      listEvaluationBacktests: vi.fn().mockResolvedValue([]),
      getEvaluationBacktest: vi.fn(),
      getEvaluationBacktestReport: vi.fn(),
      getEvaluationBacktestEvidence: vi.fn(),
    })),
  }
})

vi.mock('./lib/jobs/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('./lib/jobs/api')>()
  return {
    ...actual,
    createSignalJobsApiForAuth: vi.fn(() => ({
      listJobs: vi.fn().mockResolvedValue({ items: [], nextCursor: '' }),
      getJob: vi.fn(),
      createHistoricalDataBackfillJob: vi.fn(),
    })),
  }
})

const financeApiMocks = vi.hoisted(() => ({
  listTenants: vi.fn(),
  getDashboard: vi.fn(),
  getFXDiagnostics: vi.fn(),
  listTenantMembers: vi.fn(),
  listTenantInvites: vi.fn(),
  listAccounts: vi.fn(),
  listCategories: vi.fn(),
  listTags: vi.fn(),
  listTransactions: vi.fn(),
  listConnections: vi.fn(),
  getTransaction: vi.fn(),
  updateTransaction: vi.fn(),
  createTenant: vi.fn(),
  createTenantInvite: vi.fn(),
  acceptTenantInvite: vi.fn(),
  createAccount: vi.fn(),
  createCategory: vi.fn(),
  createTag: vi.fn(),
  createTransaction: vi.fn(),
  linkTokenConnection: vi.fn(),
  startRedirectConnection: vi.fn(),
  finishRedirectConnection: vi.fn(),
  getSyntheticLinkState: vi.fn(),
  saveSyntheticLinkState: vi.fn(),
  deleteConnection: vi.fn(),
  triggerConnectionSync: vi.fn(),
  triggerFXSync: vi.fn(),
  previewCSVImport: vi.fn(),
  confirmCSVImport: vi.fn(),
  getCSVImportAudit: vi.fn(),
  getTenant: vi.fn(),
}))

vi.mock('./lib/finance/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('./lib/finance/api')>()
  return {
    ...actual,
    createSignalFinanceApiForAuth: vi.fn(() => financeApiMocks),
  }
})

// Mock the auth store so tests can control authentication state
const mocks = vi.hoisted(() => ({
  isAuthenticated: true,
  restoring: false,
  tryRestoreSession: vi.fn().mockResolvedValue(undefined),
  clearAuth: vi.fn(),
  accessToken: null as string | null,
  user: null as { id: string; username: string } | null,
}))

vi.mock('./lib/auth/auth-store.svelte', () => ({
  authStore: mocks,
}))

function navigateHash(hash: string) {
  window.location.hash = hash
  window.dispatchEvent(new HashChangeEvent('hashchange'))
}

function createFinanceConnectionFixture(overrides: Record<string, unknown> = {}) {
  const now = new Date('2026-06-20T12:00:00Z')
  return {
    id: 'connection-1',
    tenantId: 'tenant-1',
    provider: 'synthetic',
    displayName: 'Synthetic household',
    providerReference: 'state-1',
    externalId: '',
    state: 'active',
    lastSyncJobId: '',
    lastSyncStartedAt: null,
    lastSuccessfulSyncAt: null,
    lastSyncError: '',
    createdAt: now,
    updatedAt: now,
    schedule: null,
    ...overrides,
  }
}

describe('App shell', () => {
  beforeEach(() => {
    const now = new Date('2026-06-20T12:00:00Z')
    vi.clearAllMocks()
    window.localStorage.clear()
    window.sessionStorage.clear()
    window.location.hash = ''
    mocks.isAuthenticated = true
    mocks.restoring = false
    mocks.tryRestoreSession.mockResolvedValue(undefined)
    mocks.accessToken = faker.string.alphanumeric(32)
    financeApiMocks.listTenants.mockResolvedValue([
      {
        id: 'tenant-1',
        name: 'Household',
        displayCurrency: 'USD',
        joinedAt: now,
        createdAt: now,
        updatedAt: now,
      },
    ])
    financeApiMocks.getDashboard.mockResolvedValue({
      period: {
        preset: 'current_month',
        startDate: now,
        endDate: now,
        previous: { startDate: now, endDate: now },
        next: { startDate: now, endDate: now },
      },
      settled: { displayCurrency: 'USD', incomeMinor: 10000, expenseMinor: 5000, netMinor: 5000, transactionCount: 2, complete: true },
      pending: { displayCurrency: 'USD', incomeMinor: 0, expenseMinor: 1000, netMinor: -1000, transactionCount: 1, complete: true },
      categoryBreakdowns: [],
      accountBalances: [],
      alerts: [],
      missingFx: [],
      nativeSettledTotals: [],
    })
    financeApiMocks.getFXDiagnostics.mockResolvedValue({
      defaultProvider: 'frankfurter',
      storedRatesCount: 12,
      providers: [{ name: 'frankfurter', default: true, ready: true }],
    })
    financeApiMocks.listTenantMembers.mockResolvedValue([])
    financeApiMocks.listTenantInvites.mockResolvedValue([])
    financeApiMocks.listAccounts.mockResolvedValue([])
    financeApiMocks.listCategories.mockResolvedValue([])
    financeApiMocks.listTags.mockResolvedValue([])
    financeApiMocks.listTransactions.mockResolvedValue([])
    financeApiMocks.listConnections.mockResolvedValue([])
    financeApiMocks.getTransaction.mockResolvedValue({
      id: 'tx-1',
      tenantId: 'tenant-1',
      accountId: 'account-1',
      source: 'provider',
      status: 'pending',
      kind: 'expense',
      amountMinor: 1200,
      currency: 'USD',
      description: 'Coffee',
      effectiveAt: now,
      categoryId: null,
      transferGroupId: null,
      transferMatchedAt: null,
      hiddenAt: null,
      providerOriginal: null,
      createdAt: now,
      updatedAt: now,
    })
    financeApiMocks.updateTransaction.mockResolvedValue(undefined)
    financeApiMocks.createTenant.mockResolvedValue(undefined)
    financeApiMocks.createTenantInvite.mockResolvedValue(undefined)
    financeApiMocks.acceptTenantInvite.mockResolvedValue(undefined)
    financeApiMocks.createAccount.mockResolvedValue(undefined)
    financeApiMocks.createCategory.mockResolvedValue(undefined)
    financeApiMocks.createTag.mockResolvedValue(undefined)
    financeApiMocks.createTransaction.mockResolvedValue(undefined)
    financeApiMocks.linkTokenConnection.mockResolvedValue(undefined)
    financeApiMocks.startRedirectConnection.mockResolvedValue({ provider: 'pko', authorizationUrl: 'https://bank.example.test/authorize', state: 'state-1' })
    financeApiMocks.finishRedirectConnection.mockResolvedValue(createFinanceConnectionFixture())
    financeApiMocks.getSyntheticLinkState.mockResolvedValue({ provider: 'synthetic', state: 'state-1', configuredAccounts: [], canFinish: false })
    financeApiMocks.saveSyntheticLinkState.mockResolvedValue({ provider: 'synthetic', state: 'state-1', configuredAccounts: [], canFinish: false })
    financeApiMocks.deleteConnection.mockResolvedValue(undefined)
    financeApiMocks.triggerConnectionSync.mockResolvedValue({ jobId: 'job-1', jobType: 'finance.bank_connection_sync' })
    financeApiMocks.triggerFXSync.mockResolvedValue({ jobId: 'job-2', jobType: 'finance.fx_rates_sync', provider: 'frankfurter' })
    financeApiMocks.previewCSVImport.mockResolvedValue(undefined)
    financeApiMocks.confirmCSVImport.mockResolvedValue(undefined)
    financeApiMocks.getCSVImportAudit.mockResolvedValue(undefined)
    financeApiMocks.getTenant.mockResolvedValue(undefined)
  })

  it('shows Chat heading on initial load when authenticated', async () => {
    render(App)
    navigateHash('#/chat')
    expect(
      await screen.findByRole('heading', { name: 'Chat' }),
    ).toBeInTheDocument()
  })

  it('redirects the authenticated root route to data', async () => {
    window.location.hash = ''

    render(App)

    expect(
      await screen.findByRole('heading', { name: 'Historical data' }),
    ).toBeInTheDocument()
  })

  it('navigates to Providers when Providers link is clicked', async () => {
    const user = userEvent.setup()
    render(App)
    await user.click(screen.getByRole('link', { name: 'Providers' }))
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Providers' })).toBeInTheDocument()
    })
  })

  it('navigates to Strategies when Strategies link is clicked', async () => {
    const user = userEvent.setup()
    render(App)
    await user.click(screen.getByRole('link', { name: 'Strategies' }))
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Strategies' })).toBeInTheDocument()
    })
  })

  it('navigates to Evaluations when Evaluations link is clicked', async () => {
    const user = userEvent.setup()
    render(App)
    await user.click(screen.getByRole('link', { name: 'Evaluations' }))
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Evaluations' })).toBeInTheDocument()
    })
  })

  it('navigates to Jobs when Jobs link is clicked', async () => {
    const user = userEvent.setup()
    render(App)
    await user.click(screen.getByRole('link', { name: 'Jobs' }))
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Jobs' })).toBeInTheDocument()
    })
  })

  it('shows protected navigation links when authenticated', async () => {
    render(App)

    expect(screen.getByRole('link', { name: 'Chat' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Data' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Providers' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Strategies' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Evaluations' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Jobs' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Finance' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Admin' })).toBeInTheDocument()
  })

  it('renders finance and admin routes when authenticated', async () => {
    render(App)
    navigateHash('#/finance')
    expect(await screen.findByRole('heading', { name: 'Finance' })).toBeInTheDocument()

    navigateHash('#/admin')
    expect(await screen.findByRole('heading', { name: 'Admin' })).toBeInTheDocument()
  })

  it('renders all supported finance routes inside the dedicated finance shell', async () => {
    render(App)

    const cases = [
      ['#/finance', 'Finance'],
      ['#/finance/tenants', 'Finance tenants'],
      ['#/finance/accounts', 'Finance accounts'],
      ['#/finance/accounts/account-1', 'Finance account detail'],
      ['#/finance/connections', 'Finance connections'],
      ['#/finance/connections/synthetic?state=state-1', 'Synthetic setup'],
      ['#/finance/transactions', 'Finance transactions'],
      ['#/finance/transactions/new', 'Record transaction'],
      ['#/finance/transactions/tx-1', 'Edit transaction'],
      ['#/finance/categories', 'Finance categories and tags'],
      ['#/finance/imports', 'Finance imports'],
      ['#/finance/jobs/job-1', 'Finance job route'],
    ] as const

    for (const [hash, heading] of cases) {
      navigateHash(hash)
      expect(await screen.findByRole('heading', { name: heading })).toBeInTheDocument()
      expect(screen.getByLabelText('Finance navigation')).toBeInTheDocument()
      expect(screen.getByLabelText('Finance utilities')).toBeInTheDocument()
      expect(screen.queryByRole('combobox', { name: 'Active tenant' })).not.toBeInTheDocument()
      expect(screen.queryByLabelText('Main')).not.toBeInTheDocument()
      expect(screen.queryByLabelText('Finance sections')).not.toBeInTheDocument()
      expect(screen.queryByRole('link', { name: 'Rules' })).not.toBeInTheDocument()
      expect(screen.queryByRole('link', { name: 'Settings' })).not.toBeInTheDocument()
    }
  })

  it('reuses one compact shell-level tenant control across tenant-aware finance routes', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    financeApiMocks.listTenants.mockResolvedValueOnce([
      {
        id: 'tenant-1',
        name: 'Household',
        displayCurrency: 'USD',
        joinedAt: now,
        createdAt: now,
        updatedAt: now,
      },
      {
        id: 'tenant-2',
        name: 'Travel',
        displayCurrency: 'EUR',
        joinedAt: now,
        createdAt: now,
        updatedAt: now,
      },
    ])
    const user = userEvent.setup()
    render(App)
    navigateHash('#/finance')

    expect(await screen.findByRole('heading', { name: 'Finance' })).toBeInTheDocument()
    expect(screen.getAllByRole('combobox', { name: 'Active tenant' })).toHaveLength(1)
    expect(screen.queryByRole('combobox', { name: 'Tenant' })).not.toBeInTheDocument()
    expect(screen.queryByText('Tenant workspace')).not.toBeInTheDocument()

    await user.selectOptions(screen.getByRole('combobox', { name: 'Active tenant' }), 'tenant-2')

    const cases = [
      ['#/finance', 'Finance'],
      ['#/finance/accounts', 'Finance accounts'],
      ['#/finance/accounts/account-1', 'Finance account detail'],
      ['#/finance/transactions', 'Finance transactions'],
      ['#/finance/transactions/new', 'Record transaction'],
      ['#/finance/transactions/tx-1', 'Edit transaction'],
      ['#/finance/categories', 'Finance categories and tags'],
      ['#/finance/connections', 'Finance connections'],
      ['#/finance/imports', 'Finance imports'],
      ['#/finance/jobs/job-1', 'Finance job route'],
    ] as const

    for (const [hash, heading] of cases) {
      navigateHash(hash)
      expect(await screen.findByRole('heading', { name: heading })).toBeInTheDocument()
      expect(screen.getAllByRole('combobox', { name: 'Active tenant' })).toHaveLength(1)
      expect(screen.getByRole('combobox', { name: 'Active tenant' })).toHaveValue('tenant-2')
      expect(screen.queryByRole('combobox', { name: 'Tenant' })).not.toBeInTheDocument()
      expect(screen.queryByText('Tenant workspace')).not.toBeInTheDocument()
    }
  })

  it('preserves tenant-scoped finance deep links while waiting for explicit tenant selection', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    const cases = [
      ['#/finance', 'Finance'],
      ['#/finance/accounts', 'Finance accounts'],
      ['#/finance/accounts/account-1', 'Finance account detail'],
      ['#/finance/transactions', 'Finance transactions'],
      ['#/finance/transactions/new', 'Record transaction'],
      ['#/finance/transactions/tx-1', 'Edit transaction'],
      ['#/finance/categories', 'Finance categories and tags'],
      ['#/finance/connections', 'Finance connections'],
      ['#/finance/connections/synthetic?state=state-1', 'Synthetic setup'],
      ['#/finance/imports', 'Finance imports'],
      ['#/finance/jobs/job-1', 'Finance job route'],
    ] as const

    for (const [hash, heading] of cases) {
      financeApiMocks.listTenants.mockResolvedValueOnce([
        {
          id: 'tenant-1',
          name: 'Household',
          displayCurrency: 'USD',
          joinedAt: now,
          createdAt: now,
          updatedAt: now,
        },
        {
          id: 'tenant-2',
          name: 'Travel',
          displayCurrency: 'EUR',
          joinedAt: now,
          createdAt: now,
          updatedAt: now,
        },
      ])
      const user = userEvent.setup()
      const app = render(App)

      navigateHash(hash)

      expect(await screen.findByText('Select an active tenant to continue on this finance route.')).toBeInTheDocument()
      expect(window.location.hash).toBe(hash)

      await user.selectOptions(screen.getByRole('combobox', { name: 'Active tenant' }), 'tenant-2')

      await waitFor(() => {
        expect(screen.getByRole('heading', { name: heading })).toBeInTheDocument()
      })
      expect(window.location.hash).toBe(hash)

      app.unmount()
      window.localStorage.clear()
      window.location.hash = ''
    }
  })

  it('reuses a previously selected tenant across direct finance deep links', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    window.localStorage.setItem('signal-ui-finance-tenant-id', 'tenant-2')
    financeApiMocks.listTenants.mockResolvedValueOnce([
      {
        id: 'tenant-1',
        name: 'Household',
        displayCurrency: 'USD',
        joinedAt: now,
        createdAt: now,
        updatedAt: now,
      },
      {
        id: 'tenant-2',
        name: 'Travel',
        displayCurrency: 'EUR',
        joinedAt: now,
        createdAt: now,
        updatedAt: now,
      },
    ])

    render(App)
    navigateHash('#/finance/transactions/tx-1')

    expect(await screen.findByRole('heading', { name: 'Edit transaction' })).toBeInTheDocument()
    expect(screen.queryByText('Select an active tenant to continue on this finance route.')).not.toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Active tenant' })).toHaveValue('tenant-2')
  })

  it('renders the shared finance transaction editor routes when authenticated', async () => {
    render(App)

    navigateHash('#/finance/transactions/new')
    expect(
      await screen.findByRole('heading', { name: 'Record transaction' }),
    ).toBeInTheDocument()

    navigateHash('#/finance/transactions/tx-1')
    expect(
      await screen.findByRole('heading', { name: 'Edit transaction' }),
    ).toBeInTheDocument()
  })

  it('renders the synthetic finance setup route when authenticated', async () => {
    render(App)

    navigateHash('#/finance/connections/synthetic?state=state-1')

    expect(await screen.findByRole('heading', { name: 'Synthetic setup' })).toBeInTheDocument()
  })

  it('completes synthetic setup and returns to connections with the new link visible', async () => {
    const user = userEvent.setup()
    financeApiMocks.listConnections
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([createFinanceConnectionFixture()])
    financeApiMocks.startRedirectConnection.mockResolvedValueOnce({
      provider: 'synthetic',
      authorizationUrl: '#/finance/connections/synthetic?state=state-1',
      state: 'state-1',
    })
    financeApiMocks.saveSyntheticLinkState.mockResolvedValueOnce({
      provider: 'synthetic',
      state: 'state-1',
      configuredAccounts: [{ key: 'account-1', name: 'Cash', currency: 'USD' }],
      canFinish: true,
    })
    financeApiMocks.finishRedirectConnection.mockResolvedValueOnce(createFinanceConnectionFixture())

    render(App)
    navigateHash('#/finance/connections')

    await user.click(await screen.findByRole('button', { name: 'Start synthetic setup' }))
    await user.type(await screen.findByLabelText('Account name 1'), 'Cash')
    await user.type(screen.getByLabelText('Account currency 1'), 'USD')
    await user.click(screen.getByRole('button', { name: 'Finish link' }))

    expect(await screen.findByRole('heading', { name: 'Finance connections' })).toBeInTheDocument()
    expect(await screen.findByText('Synthetic household')).toBeInTheDocument()
  })

  it('renders the data browser route shell when authenticated', async () => {
    render(App)
    navigateHash('#/data')

    expect(
      await screen.findByRole('heading', { name: 'Historical data' }),
    ).toBeInTheDocument()
  })

  it('shows the message composer when the hash includes a session id', async () => {
    const sessionSlug = faker.string.uuid()
    render(App)
    navigateHash(`#/chat/${sessionSlug}`)
    expect(
      await screen.findByRole('textbox', { name: 'Message' }),
    ).toBeInTheDocument()
  })

  it('renders the jobs detail route shell when authenticated', async () => {
    render(App)
    navigateHash('#/jobs/job-123')

    expect(
      await screen.findByRole('heading', { name: 'Job detail' }),
    ).toBeInTheDocument()
  })

  it('redirects to /login when navigating to protected route unauthenticated', async () => {
    mocks.isAuthenticated = false
    render(App)
    navigateHash('#/data')
    await waitFor(() => {
      expect(window.location.hash).toBe('#/login')
    })
    expect(window.sessionStorage.getItem(POST_LOGIN_DESTINATION_KEY)).toBe('/data')
  })

  it('preserves explicit protected deep links before redirecting to login', async () => {
    const sessionSlug = faker.string.uuid()
    mocks.isAuthenticated = false

    render(App)
    navigateHash(`#/chat/${sessionSlug}`)

    await waitFor(() => {
      expect(window.location.hash).toBe('#/login')
    })
    expect(window.sessionStorage.getItem(POST_LOGIN_DESTINATION_KEY)).toBe(
      `/chat/${sessionSlug}`,
    )
  })

  it('preserves finance transaction deep links before redirecting to login', async () => {
    mocks.isAuthenticated = false

    render(App)
    navigateHash('#/finance/transactions/new')

    await waitFor(() => {
      expect(window.location.hash).toBe('#/login')
    })
    expect(window.sessionStorage.getItem(POST_LOGIN_DESTINATION_KEY)).toBe(
      '/finance/transactions/new',
    )
  })

  it('shows loading indicator while session is restoring', async () => {
    mocks.restoring = true
    render(App)
    expect(screen.getByLabelText('Loading')).toBeInTheDocument()
  })
})
