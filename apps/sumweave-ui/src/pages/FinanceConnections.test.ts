import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceConnections from './FinanceConnections.svelte'
import { formatFinanceDateTime } from '../lib/finance/format'

const mocks = vi.hoisted(() => ({
  listTenants: vi.fn(),
  listConnections: vi.fn(),
  listConnectionSyncedAccounts: vi.fn(),
  linkTokenConnection: vi.fn(),
  startRedirectConnection: vi.fn(),
  finishRedirectConnection: vi.fn(),
  getSyntheticLinkState: vi.fn(),
  saveSyntheticLinkState: vi.fn(),
  deleteConnection: vi.fn(),
  renameConnection: vi.fn(),
  triggerConnectionSync: vi.fn(),
  getJob: vi.fn(),
}))
vi.mock('../lib/finance/api', async (importOriginal) => ({ ...(await importOriginal<typeof import('../lib/finance/api')>()), createSignalFinanceApiForAuth: vi.fn(() => ({ ...mocks })) }))
vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))
vi.mock('../lib/jobs/api', () => ({
  createSignalJobsApiForAuth: vi.fn(() => ({ getJob: mocks.getJob })),
}))

function createTenantFixture() {
  const now = new Date('2026-06-20T12:00:00Z')
  return { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now }
}

function createConnectionFixture(overrides: Partial<ReturnType<typeof createConnectionFixtureBase>> = {}) {
  return { ...createConnectionFixtureBase(), ...overrides }
}

function createConnectionFixtureBase() {
  const now = new Date('2026-06-20T12:00:00Z')
  return {
    id: 'connection-1',
    tenantId: 'tenant-1',
    provider: 'monobank',
    displayName: 'Mono',
    providerReference: 'ref',
    state: 'active',
    lastSyncJobId: 'job-1',
    lastSyncStartedAt: now,
    lastSuccessfulSyncAt: now,
    lastSyncError: '',
    createdAt: now,
    updatedAt: now,
    schedule: {
      connectionId: 'connection-1',
      intervalSeconds: 900,
      nextRunAt: now,
      lastScheduledAt: now,
      lastStartedAt: now,
      lastCompletedAt: now,
      lastJobId: 'job-1',
      enabled: true,
      createdAt: now,
      updatedAt: now,
    },
  }
}

function renderPage() {
  return render(FinanceConnections)
}

describe('Finance connections page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.history.replaceState({}, '', '/#/finance/connections')
    mocks.listTenants.mockResolvedValue([createTenantFixture()])
    mocks.listConnections.mockResolvedValue([createConnectionFixture()])
    mocks.listConnectionSyncedAccounts.mockResolvedValue([])
    mocks.linkTokenConnection.mockResolvedValue(createConnectionFixture())
    mocks.startRedirectConnection.mockResolvedValue({ provider: 'pko', authorizationUrl: 'https://bank.example/authorize', state: 'state-1' })
    mocks.finishRedirectConnection.mockResolvedValue(createConnectionFixture())
    mocks.getSyntheticLinkState.mockResolvedValue({ provider: 'synthetic', state: 'state-1', configuredAccounts: [], canFinish: false })
    mocks.saveSyntheticLinkState.mockResolvedValue({ provider: 'synthetic', state: 'state-1', configuredAccounts: [], canFinish: false })
    mocks.deleteConnection.mockResolvedValue(undefined)
    mocks.renameConnection.mockResolvedValue(undefined)
    mocks.triggerConnectionSync.mockResolvedValue({ jobId: 'job-2', jobType: 'finance.bank_connection_sync' })
    mocks.getJob.mockResolvedValue({ id: 'job-2', status: 'queued', jobType: 'finance.bank_connection_sync' })
  })

  it('renders connection cards with schedule and job links', async () => {
    renderPage()
    expect(await screen.findByText('Mono')).toBeInTheDocument()
    expect(screen.getByText('Provider ref: ref')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open last sync job' })).toHaveAttribute('href', '#/finance/jobs/job-1')
  })

  it('lazily loads safe synced-account fields once, retries failures, and invalidates after sync', async () => {
    const user = userEvent.setup()
    mocks.listConnectionSyncedAccounts
      .mockRejectedValueOnce(new Error('accounts unavailable'))
      .mockResolvedValue([{ financeAccountId: 'account-1', name: 'Checking', currency: 'USD', lastSuccessfulSyncAt: new Date('2026-06-20T12:00:00Z') }])

    renderPage()
    await user.click(await screen.findByText('Synced accounts'))
    expect(await screen.findByRole('alert')).toHaveTextContent('accounts unavailable')
    await user.click(screen.getByRole('button', { name: 'Retry synced accounts' }))
    expect(await screen.findByRole('link', { name: /Checking · USD/ })).toHaveAttribute('href', '#/finance/accounts/account-1')
    expect(mocks.listConnectionSyncedAccounts).toHaveBeenCalledTimes(2)
    await user.click(screen.getByText('Synced accounts'))
    await user.click(screen.getByText('Synced accounts'))
    expect(mocks.listConnectionSyncedAccounts).toHaveBeenCalledTimes(2)
    await user.click(screen.getByRole('button', { name: 'Sync now' }))
    await waitFor(() => expect(mocks.triggerConnectionSync).toHaveBeenCalled())
    await waitFor(() => expect(mocks.listConnectionSyncedAccounts).toHaveBeenCalledTimes(3))
  })

  it('uses created time as an accessible fallback when provider reference is empty', async () => {
    const connection = createConnectionFixture({ providerReference: '' })
    mocks.listConnections.mockResolvedValueOnce([connection])

    renderPage()

    expect(await screen.findByText(`Created: ${formatFinanceDateTime(connection.createdAt)}`)).toBeInTheDocument()
    expect(screen.getByRole('button', {
      name: `Rename connection Mono (Created: ${formatFinanceDateTime(connection.createdAt)})`,
    })).toBeInTheDocument()
  })

  it('submits the monobank token form without a free-text provider field', async () => {
    const user = userEvent.setup()
    renderPage()

    expect(screen.queryByLabelText('Provider')).not.toBeInTheDocument()
    await user.type(await screen.findByLabelText('Monobank token'), 'token-1')
    await user.click(screen.getByRole('button', { name: 'Link monobank' }))

    await waitFor(() => expect(mocks.linkTokenConnection).toHaveBeenCalledWith({ tenantId: 'tenant-1', provider: 'monobank', token: 'token-1' }))
  })

  it('surfaces monobank token submit failures in the page error state', async () => {
    const user = userEvent.setup()
    mocks.linkTokenConnection.mockRejectedValueOnce(new Error('Monobank token failed'))

    renderPage()

    await user.type(await screen.findByLabelText('Monobank token'), 'token-1')
    await user.click(screen.getByRole('button', { name: 'Link monobank' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Monobank token failed')
  })

  it('uses the fallback monobank error message for non-Error submit failures', async () => {
    const user = userEvent.setup()
    mocks.linkTokenConnection.mockRejectedValueOnce('boom')

    renderPage()

    await user.type(await screen.findByLabelText('Monobank token'), 'token-1')
    await user.click(screen.getByRole('button', { name: 'Link monobank' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to link monobank connection')
  })

  it('shows the triggered job status on its connection card', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Sync now' }))
    await waitFor(() => expect(mocks.triggerConnectionSync).toHaveBeenCalled())
    expect(await screen.findByRole('link', { name: 'Open job' })).toHaveAttribute('href', '#/finance/jobs/job-2')
    expect(screen.getByText('Queued — waiting for a worker.')).toBeInTheDocument()
  })

  it('deletes a linked connection after confirmation', async () => {
    const user = userEvent.setup()

    mocks.listConnections.mockResolvedValueOnce([
      createConnectionFixture({ id: 'connection-1', providerReference: 'pko-ref-1', displayName: 'PKO linked' }),
      createConnectionFixture({ id: 'connection-2', providerReference: 'pko-ref-2', displayName: 'PKO linked', schedule: { ...createConnectionFixture().schedule!, connectionId: 'connection-2' } }),
    ])

    renderPage()

    await user.click((await screen.findAllByRole('button', { name: 'Delete link' }))[1])
    expect(screen.getByText('Delete PKO linked (Provider ref: pko-ref-2)? This removes only the local link metadata and schedule. Imported ledger history stays.')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Confirm delete' }))

    await waitFor(() =>
      expect(mocks.deleteConnection).toHaveBeenCalledWith({ tenantId: 'tenant-1', connectionId: 'connection-2' }),
    )
    await waitFor(() => expect(screen.getAllByText('PKO linked')).toHaveLength(1))
  })

  it('renames a connection inline and updates its card without reloading the list', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Rename connection Mono (Provider ref: ref)' }))
    const name = screen.getByLabelText('Connection name')
    await user.clear(name)
    await user.type(name, 'Joint checking')
    await user.click(screen.getByRole('button', { name: 'Save connection name' }))

    await waitFor(() => expect(mocks.renameConnection).toHaveBeenCalledWith({
      tenantId: 'tenant-1',
      connectionId: 'connection-1',
      name: 'Joint checking',
    }))
    expect(screen.getByText('Joint checking')).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('Renamed connection to Joint checking.')
    expect(screen.getByRole('button', { name: 'Rename connection Joint checking (Provider ref: ref)' })).toHaveFocus()
    expect(mocks.listConnections).toHaveBeenCalledTimes(1)
  })

  it('focuses the connection name input when rename opens and restores Rename focus after cancel', async () => {
    const user = userEvent.setup()
    renderPage()

    const rename = await screen.findByRole('button', { name: 'Rename connection Mono (Provider ref: ref)' })
    await user.click(rename)
    expect(screen.getByLabelText('Connection name')).toHaveFocus()

    await user.click(screen.getByRole('button', { name: 'Cancel connection name edit' }))
    expect(screen.getByRole('button', { name: 'Rename connection Mono (Provider ref: ref)' })).toHaveFocus()
  })

  it('keeps the rename form and draft with a card-local input-associated error when saving fails', async () => {
    const user = userEvent.setup()
    mocks.renameConnection.mockRejectedValueOnce(new Error('Rename failed'))
    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Rename connection Mono (Provider ref: ref)' }))
    const name = screen.getByLabelText('Connection name')
    await user.clear(name)
    await user.type(name, 'Joint checking')
    await user.click(screen.getByRole('button', { name: 'Save connection name' }))

    const renameError = await screen.findByRole('alert')
    const nameInput = screen.getByLabelText('Connection name')
    expect(renameError).toHaveTextContent('Rename failed')
    expect(nameInput).toHaveValue('Joint checking')
    expect(nameInput).toHaveAttribute('aria-describedby', 'finance-connection-rename-error-connection-1')
    expect(nameInput).toHaveFocus()
  })

  it('gives duplicate connection names distinct Rename button names using their visible secondary identifiers', async () => {
    mocks.listConnections.mockResolvedValueOnce([
      createConnectionFixture({ id: 'connection-1', displayName: 'PKO linked', providerReference: 'pko-ref-1' }),
      createConnectionFixture({ id: 'connection-2', displayName: 'PKO linked', providerReference: 'pko-ref-2', schedule: { ...createConnectionFixture().schedule!, connectionId: 'connection-2' } }),
    ])
    renderPage()

    expect(await screen.findByRole('button', { name: 'Rename connection PKO linked (Provider ref: pko-ref-1)' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Rename connection PKO linked (Provider ref: pko-ref-2)' })).toBeInTheDocument()
  })

  it('keeps the connection when in-place delete confirmation is canceled', async () => {
    const user = userEvent.setup()

    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Delete link' }))
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(mocks.deleteConnection).not.toHaveBeenCalled()
    expect(screen.getByText('Mono')).toBeInTheDocument()
  })

  it('surfaces delete failures in the page error state', async () => {
    const user = userEvent.setup()
    mocks.deleteConnection.mockRejectedValueOnce(new Error('Delete failed'))

    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Delete link' }))
    await user.click(screen.getByRole('button', { name: 'Confirm delete' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Delete failed')
    expect(screen.getByText('Mono')).toBeInTheDocument()
  })

  it('starts the PKO redirect flow with the fixed callback url', async () => {
    const user = userEvent.setup()

    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Connect PKO with bank login' }))

    await waitFor(() => expect(mocks.startRedirectConnection).toHaveBeenCalledWith({
      tenantId: 'tenant-1',
      provider: 'pko',
      callbackUrl: `${window.location.origin}/#/finance/connections`,
    }))
  })

  it('surfaces PKO redirect start failures in the page error state', async () => {
    const user = userEvent.setup()
    mocks.startRedirectConnection.mockRejectedValueOnce(new Error('PKO start failed'))

    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Connect PKO with bank login' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('PKO start failed')
  })

  it('starts synthetic setup and navigates to the fixed synthetic route returned by state', async () => {
    const user = userEvent.setup()
    mocks.startRedirectConnection.mockResolvedValueOnce({
      provider: 'synthetic',
      authorizationUrl: '#/finance/connections/synthetic?state=state-1',
      state: 'state-1',
    })

    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Start synthetic setup' }))

    await waitFor(() =>
      expect(mocks.startRedirectConnection).toHaveBeenCalledWith({
        tenantId: 'tenant-1',
        provider: 'synthetic',
        callbackUrl: `${window.location.origin}/#/finance/connections`,
      }),
    )
    expect(window.location.hash).toBe('#/finance/connections/synthetic?state=state-1')
  })

  it('surfaces synthetic start failures in the page error state', async () => {
    const user = userEvent.setup()
    mocks.startRedirectConnection.mockRejectedValueOnce(new Error('Synthetic start failed'))

    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Start synthetic setup' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Synthetic start failed')
  })

  it('renders failed connection state badges', async () => {
    mocks.listConnections.mockResolvedValueOnce([
      createConnectionFixture({ state: 'failed', providerReference: '' }),
    ])

    renderPage()

    const failedBadge = await screen.findByText('failed')
    expect(failedBadge).toHaveClass('text-bg-danger')
  })

  it('renders fallback connection state badges for non-active states', async () => {
    mocks.listConnections.mockResolvedValueOnce([
      createConnectionFixture({ state: 'paused', providerReference: '' }),
    ])

    renderPage()

    const pausedBadge = await screen.findByText('paused')
    expect(pausedBadge).toHaveClass('text-bg-secondary')
  })

  it('surfaces sync trigger failures with the fallback error message', async () => {
    const user = userEvent.setup()
    mocks.triggerConnectionSync.mockRejectedValueOnce('boom')

    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Sync now' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to trigger sync')
    expect(screen.queryByRole('link', { name: 'Open latest finance job' })).not.toBeInTheDocument()
  })

  it('uses the fallback PKO error message for non-Error start failures', async () => {
    const user = userEvent.setup()
    mocks.startRedirectConnection.mockRejectedValueOnce('boom')

    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Connect PKO with bank login' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to start PKO connection')
  })

  it('finishes the PKO redirect flow once and clears the consumed query string', async () => {
    window.history.replaceState({}, '', '/?code=code-1&state=state-1#/finance/connections')

    renderPage()

    await waitFor(() => expect(mocks.finishRedirectConnection).toHaveBeenCalledWith({
      tenantId: 'tenant-1',
      provider: 'pko',
      code: 'code-1',
      state: 'state-1',
    }))
    await waitFor(() => expect(mocks.finishRedirectConnection).toHaveBeenCalledTimes(1))
    expect(window.location.search).toBe('')
    expect(window.location.hash).toBe('#/finance/connections')
  })

  it('keeps PKO return params after a transient finish failure so the operator can retry on reopen', async () => {
    window.history.replaceState({}, '', '/?code=code-1&state=state-1#/finance/connections')
    mocks.finishRedirectConnection
      .mockRejectedValueOnce(new Error('PKO finish failed'))
      .mockResolvedValueOnce(createConnectionFixture())

    const firstPage = renderPage()

    expect(await screen.findByRole('alert')).toHaveTextContent('PKO finish failed')
    expect(await screen.findByRole('button', { name: 'Connect PKO with bank login' })).toBeEnabled()
    expect(window.location.search).toBe('?code=code-1&state=state-1')
    expect(window.location.hash).toBe('#/finance/connections')
    expect(mocks.finishRedirectConnection).toHaveBeenCalledTimes(1)

    firstPage.unmount()
    renderPage()

    await waitFor(() => expect(mocks.finishRedirectConnection).toHaveBeenCalledTimes(2))
    expect(window.location.search).toBe('')
    expect(window.location.hash).toBe('#/finance/connections')
  })

  it('renders the empty state when no connections exist', async () => {
    mocks.listConnections.mockResolvedValueOnce([])
    renderPage()
    expect(await screen.findByText('No connections yet.')).toBeInTheDocument()
  })
})
