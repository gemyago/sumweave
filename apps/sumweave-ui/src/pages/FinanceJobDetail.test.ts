import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceJobDetail from './FinanceJobDetail.svelte'

const mocks = vi.hoisted(() => ({ getJob: vi.fn(), replace: vi.fn() }))

vi.mock('../lib/jobs/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/jobs/api')>()),
  createSignalJobsApiForAuth: vi.fn(() => ({ getJob: mocks.getJob })),
}))
vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))
vi.mock('svelte-spa-router', async (importOriginal) => ({ ...(await importOriginal<typeof import('svelte-spa-router')>()), replace: mocks.replace }))

function detailFixture() {
  return { id: 'job-1', jobType: 'finance.bank_connection_sync', status: 'failed', requester: { userId: 'user-1', source: 'operator', agentSessionId: '', agentRunId: '' }, createdAt: new Date(), updatedAt: new Date(), attemptCount: 3, workerId: '', error: { code: 'sync_failed', summary: 'Bank declined', details: 'safe detail' } }
}

describe('Finance job detail page', () => {
  beforeEach(() => vi.clearAllMocks())

  it('renders finance-safe metadata, error summary, and navigation', async () => {
    mocks.getJob.mockResolvedValue(detailFixture())
    const user = userEvent.setup()
    render(FinanceJobDetail, { jobId: 'job-1' })

    expect(await screen.findByText('finance.bank_connection_sync')).toBeInTheDocument()
    expect(screen.getByText('Bank declined')).toBeInTheDocument()
    expect(screen.getByText('—')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Finance' })).toHaveAttribute('href', '#/finance')
    await user.click(screen.getByRole('button', { name: 'Back to jobs' }))
    expect(mocks.replace).toHaveBeenCalledWith('/admin/jobs')
  })

  it('shows the API error instead of a detail payload', async () => {
    mocks.getJob.mockRejectedValue(new Error('Job unavailable'))
    render(FinanceJobDetail, { jobId: 'job-1' })
    expect(await screen.findByRole('alert')).toHaveTextContent('Job unavailable')
  })

  it('uses bounded copy for a non-Error detail failure', async () => {
    mocks.getJob.mockRejectedValue('unavailable')
    render(FinanceJobDetail, { jobId: 'job-1' })
    expect(await screen.findByRole('alert')).toHaveTextContent('Unable to load job.')
  })

  it('renders a running job without an error or worker fallback', async () => {
    mocks.getJob.mockResolvedValue({ ...detailFixture(), status: 'running', workerId: 'worker-1', error: undefined })
    render(FinanceJobDetail, { jobId: 'job-1' })
    expect(await screen.findByText('running')).toBeInTheDocument()
    expect(screen.getByText('worker-1')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('loads the job ID supplied by the finance route params', async () => {
    mocks.getJob.mockResolvedValue(detailFixture())
    render(FinanceJobDetail, { params: { jobId: 'routed-job-1' } })

    await screen.findByText('finance.bank_connection_sync')
    expect(mocks.getJob).toHaveBeenCalledWith({ jobId: 'routed-job-1' })
  })
})
