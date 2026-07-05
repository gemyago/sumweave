import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import AdminJobs from './AdminJobs.svelte'
import AdminJobDetail from './AdminJobDetail.svelte'
import FinanceJobDetail from './FinanceJobDetail.svelte'

const mocks = vi.hoisted(() => ({
  listJobs: vi.fn(),
  getJob: vi.fn(),
  listTenants: vi.fn(),
}))

vi.mock('../lib/jobs/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/jobs/api')>()
  return {
    ...actual,
    createSignalJobsApiForAuth: vi.fn(() => ({
      listJobs: mocks.listJobs,
      getJob: mocks.getJob,
    })),
  }
})

vi.mock('../lib/finance/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/finance/api')>()
  return {
    ...actual,
    createSignalFinanceApiForAuth: vi.fn(() => ({
      listTenants: mocks.listTenants,
    })),
  }
})

vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

describe('finance/admin wrapper pages', () => {
  beforeEach(() => {
    window.localStorage.clear()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listJobs.mockResolvedValue({ items: [], nextCursor: '' })
    mocks.getJob.mockResolvedValue({ id: 'job-1', jobType: 'finance.csv_import', status: 'queued', requester: { userId: 'user-1', source: 'operator', agentSessionId: '', agentRunId: '' }, input: { ingestionRunId: '', venue: '', symbol: '', assetClass: '', timeframe: '', start: now, end: now, pageSize: 0 }, createdAt: now, updatedAt: now, startedAt: null, completedAt: null, attemptCount: 1, workerId: '', lastAttemptAt: null })
    mocks.listTenants.mockResolvedValue([{ id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now }])
  })

  it('renders the admin jobs wrapper heading', async () => {
    render(AdminJobs)
    expect(await screen.findByRole('heading', { name: 'Admin jobs' })).toBeInTheDocument()
  })

  it('renders contextual job detail headings', async () => {
    render(AdminJobDetail, { params: { jobId: 'job-1' } })
    expect(await screen.findByRole('heading', { name: 'Admin job detail' })).toBeInTheDocument()

    render(FinanceJobDetail, { params: { jobId: 'job-1' } })
    expect(await screen.findByRole('heading', { name: 'Finance job detail' })).toBeInTheDocument()
  })

  it('keeps finance job deep links pending until an active tenant is chosen', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    const user = userEvent.setup()
    mocks.listTenants.mockResolvedValueOnce([
      { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now },
      { id: 'tenant-2', name: 'Travel', displayCurrency: 'EUR', joinedAt: now, createdAt: now, updatedAt: now },
    ])

    render(FinanceJobDetail, { params: { jobId: 'job-1' } })

    expect(await screen.findByText('Select an active tenant to continue on this finance route.')).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Summary' })).not.toBeInTheDocument()

    await user.selectOptions(screen.getByRole('combobox', { name: 'Tenant' }), 'tenant-2')

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Summary' })).toBeInTheDocument())
  })

  it('shows the join-tenant route hint when no finance tenants exist yet', async () => {
    mocks.listTenants.mockResolvedValueOnce([])

    render(FinanceJobDetail, { params: { jobId: 'job-1' } })

    expect(await screen.findByRole('link', { name: 'Finance tenants' })).toHaveAttribute(
      'href',
      '#/finance/tenants',
    )
    expect(
      screen.getByText(/before opening this finance job detail route\./),
    ).toBeInTheDocument()
  })

  it('surfaces finance workspace loading failures for finance job deep links', async () => {
    mocks.listTenants.mockRejectedValueOnce('boom')

    render(FinanceJobDetail, { params: { jobId: 'job-1' } })

    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to load finance workspace')
  })
})
