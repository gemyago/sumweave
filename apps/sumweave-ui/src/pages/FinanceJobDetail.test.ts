import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceJobDetail from './FinanceJobDetail.svelte'
import { rememberObservedDispatch } from '../lib/jobs/observed-dispatch'

const mocks = vi.hoisted(() => ({ getJob: vi.fn(), replace: vi.fn() }))

vi.mock('../lib/jobs/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/jobs/api')>()),
  createSignalJobsApiForAuth: vi.fn(() => ({ getJob: mocks.getJob })),
}))
vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))
vi.mock('svelte-spa-router', async (importOriginal) => ({ ...(await importOriginal<typeof import('svelte-spa-router')>()), replace: mocks.replace }))

function detailFixture() {
  return { id: 'job-1', jobType: 'finance.bank_connection_sync', status: 'failed', requester: { userId: 'user-1', source: 'operator' }, createdAt: new Date(), updatedAt: new Date(), attemptCount: 3, workerId: '', error: { code: 'sync_failed', summary: 'Bank declined', details: 'safe detail' } }
}

describe('Finance job detail page', () => {
  beforeEach(() => vi.clearAllMocks())
  afterEach(() => vi.useRealTimers())

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

  it('polls queued and running jobs until a terminal status', async () => {
    vi.useFakeTimers()
    mocks.getJob
      .mockResolvedValueOnce({ ...detailFixture(), status: 'queued', error: undefined })
      .mockResolvedValueOnce({ ...detailFixture(), status: 'running', error: undefined })
      .mockResolvedValueOnce(detailFixture())

    render(FinanceJobDetail, { jobId: 'job-1' })

    await vi.advanceTimersByTimeAsync(0)
    expect(screen.getByText('queued')).toBeInTheDocument()
    await vi.advanceTimersByTimeAsync(2_000)
    expect(screen.getByText('running')).toBeInTheDocument()
    await vi.advanceTimersByTimeAsync(2_000)
    expect(screen.getByText('failed')).toBeInTheDocument()
    await vi.advanceTimersByTimeAsync(10_000)
    expect(mocks.getJob).toHaveBeenCalledTimes(3)
  })

  it('cleans up polling when the detail page is unmounted', async () => {
    vi.useFakeTimers()
    mocks.getJob.mockResolvedValue({ ...detailFixture(), status: 'running', error: undefined })
    const page = render(FinanceJobDetail, { jobId: 'job-1' })

    await vi.advanceTimersByTimeAsync(0)
    page.unmount()
    await vi.advanceTimersByTimeAsync(2_000)

    expect(mocks.getJob).toHaveBeenCalledTimes(1)
  })

  it('cancels the previous route timer when navigating to another job', async () => {
    vi.useFakeTimers()
    mocks.getJob
      .mockResolvedValueOnce({ ...detailFixture(), status: 'running', error: undefined })
      .mockResolvedValueOnce(detailFixture())
    const page = render(FinanceJobDetail, { jobId: 'job-1' })

    await vi.advanceTimersByTimeAsync(0)
    await page.rerender({ jobId: 'job-2' })
    await vi.advanceTimersByTimeAsync(0)
    await vi.advanceTimersByTimeAsync(2_000)

    expect(mocks.getJob).toHaveBeenCalledTimes(2)
    expect(mocks.getJob).toHaveBeenLastCalledWith({ jobId: 'job-2' })
  })

  it('loads the job ID supplied by the finance route params', async () => {
    mocks.getJob.mockResolvedValue(detailFixture())
    render(FinanceJobDetail, { params: { jobId: 'routed-job-1' } })

    await screen.findByText('finance.bank_connection_sync')
    expect(mocks.getJob).toHaveBeenCalledWith({ jobId: 'routed-job-1' })
  })

  it('polls a pre-materialization 404 for an observed dispatch from this SPA flow', async () => {
    vi.useFakeTimers()
    const { JobsApiError } = await import('../lib/jobs/api')
    rememberObservedDispatch('pending-job-1')
    mocks.getJob
      .mockRejectedValueOnce(new JobsApiError({ status: 404, method: 'GET', path: '/jobs/pending-job-1', message: 'Not Found' }))
      .mockResolvedValueOnce(detailFixture())

    render(FinanceJobDetail, { jobId: 'pending-job-1' })

    await vi.advanceTimersByTimeAsync(0)
    expect(screen.getByRole('status')).toHaveTextContent('Waiting for a worker to receive this job…')
    await vi.advanceTimersByTimeAsync(2_000)
    expect(screen.getByText('finance.bank_connection_sync')).toBeInTheDocument()
  })

  it('shows an arbitrary deep-link 404 as an error', async () => {
    vi.useFakeTimers()
    const { JobsApiError } = await import('../lib/jobs/api')
    mocks.getJob.mockRejectedValue(new JobsApiError({ status: 404, method: 'GET', path: '/jobs/missing-job-1', message: 'Not Found' }))

    render(FinanceJobDetail, { jobId: 'missing-job-1' })

    await vi.advanceTimersByTimeAsync(0)
    expect(screen.getByRole('alert')).toHaveTextContent('Jobs API GET /jobs/missing-job-1 failed: Not Found')
  })
})
