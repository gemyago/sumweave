import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import FinanceJobDetail from './FinanceJobDetail.svelte'

const mocks = vi.hoisted(() => ({
  getJob: vi.fn(),
  shellState: {
    embedded: false,
    loading: false,
    error: null,
    tenants: [{ id: 'tenant-1', name: 'Household', displayCurrency: 'USD' }],
    selectedTenantId: 'tenant-1',
    selectedTenant: { id: 'tenant-1', name: 'Household', displayCurrency: 'USD' },
    needsTenantSelection: false,
    hasTenants: true,
    initialize: vi.fn().mockResolvedValue(undefined),
    selectTenant: vi.fn(),
  },
}))

vi.mock('../lib/jobs/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/jobs/api')>()),
  createSignalJobsApiForAuth: vi.fn(() => ({
    getJob: mocks.getJob,
  })),
}))

vi.mock('../lib/finance/shell-state.svelte', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/finance/shell-state.svelte')>()),
  useFinanceShellState: vi.fn(() => mocks.shellState),
}))

vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

describe('Finance job detail page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.shellState.selectedTenantId = 'tenant-1'
    mocks.shellState.selectedTenant = {
      id: 'tenant-1',
      name: 'Household',
      displayCurrency: 'USD',
    }
    mocks.shellState.needsTenantSelection = false
    mocks.getJob.mockResolvedValue({
      id: 'job-1',
      jobType: 'data.historical_raw_candle_backfill',
      status: 'failed',
      requester: { userId: 'user-1', source: 'operator', agentSessionId: '', agentRunId: '' },
      input: {
        ingestionRunId: 'run-1',
        venue: 'demo',
        symbol: 'BTCUSD',
        assetClass: 'crypto',
        timeframe: '1h',
        start: now,
        end: now,
        pageSize: 50,
      },
      result: {
        persistedCount: 10,
        expectedCount: 12,
        missingIntervalCount: 2,
        duplicateNaturalKeyCount: 0,
        rawPayloadCount: 12,
        missingIntervalPreviewCap: 10,
        missingIntervalPreview: [{ start: now, end: new Date('2026-06-20T13:00:00Z') }],
      },
      error: { summary: 'Sync failed', code: 'job.failed', details: 'Upstream provider timed out.' },
      createdAt: now,
      updatedAt: now,
      startedAt: now,
      completedAt: now,
      attemptCount: 2,
      workerId: 'worker-1',
      lastAttemptAt: now,
    })
  })

  it('renders bootstrap summary, input, timeline, result, and error cards for historical jobs', async () => {
    render(FinanceJobDetail, { params: { jobId: 'job-1' } })

    expect(await screen.findByRole('heading', { name: 'Summary' })).toBeInTheDocument()
    expect(screen.getByText('data.historical_raw_candle_backfill')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open data scope' })).toHaveAttribute('href', expect.stringContaining('#/data?'))
    expect(screen.getByRole('heading', { name: 'Missing interval preview' })).toBeInTheDocument()
    expect(screen.getByText('Sync failed')).toBeInTheDocument()
    expect(screen.getByText('Upstream provider timed out.')).toBeInTheDocument()
  })

  it('shows an inline error when the job id is missing', async () => {
    render(FinanceJobDetail, { params: {} })

    expect(await screen.findByRole('alert')).toHaveTextContent('Job id is required.')
  })

  it('renders the successful scheduled Finance sync response without historical sections', async () => {
    const startedAt = new Date('2026-07-10T22:49:21.992094+02:00')
    const completedAt = new Date('2026-07-10T22:49:22.004058+02:00')
    mocks.getJob.mockReset()
    mocks.getJob.mockResolvedValue({
      id: '019f4dca-c2ad-729d-a5ce-2f9bfa61703a',
      jobType: 'finance.bank_connection_sync',
      status: 'succeeded',
      requester: { userId: '', source: 'system', agentSessionId: '', agentRunId: '' },
      createdAt: new Date('2026-07-10T22:48:10.777316+02:00'),
      updatedAt: completedAt,
      startedAt,
      completedAt,
      attemptCount: 1,
      workerId: '',
      lastAttemptAt: completedAt,
    })

    render(FinanceJobDetail, { params: { jobId: '019f4dca-c2ad-729d-a5ce-2f9bfa61703a' } })

    expect(await screen.findByText('Input details are not available for this job type in the current API surface.')).toBeInTheDocument()
    expect(screen.getByText('Result details are not yet specialized for this job type.')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Open data scope' })).not.toBeInTheDocument()
    expect(screen.getByText('finance.bank_connection_sync')).toBeInTheDocument()
    expect(screen.getByText('system')).toBeInTheDocument()
    expect(screen.getByText('1')).toBeInTheDocument()
    expect(screen.getByText('succeeded')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Timeline and worker' })).toBeInTheDocument()
    expect(screen.getByText('Created').parentElement).toHaveClass('col-12', 'col-md-6')
  })

  it('surfaces finance job detail load failures', async () => {
    mocks.getJob.mockRejectedValueOnce('boom')

    render(FinanceJobDetail, { params: { jobId: 'job-1' } })

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Failed to load finance job detail'))
  })
})
