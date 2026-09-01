import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/svelte'
import JobStatus from './JobStatus.svelte'

const mocks = vi.hoisted(() => ({ getJob: vi.fn() }))

vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))
vi.mock('../lib/jobs/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/jobs/api')>()),
  createSignalJobsApiForAuth: vi.fn(() => ({ getJob: mocks.getJob })),
}))

function job(status: string, error?: { summary: string }) {
  return { id: 'job-1', status, jobType: 'finance.bank_connection_sync', ...(error ? { error } : {}) }
}

describe('JobStatus', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mocks.getJob.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('polls queued and running jobs until they succeed, retaining an open link', async () => {
    mocks.getJob
      .mockResolvedValueOnce(job('queued'))
      .mockResolvedValueOnce(job('running'))
      .mockResolvedValueOnce(job('succeeded'))

    render(JobStatus, { jobId: 'job-1', openHref: '/finance/jobs/job-1', label: 'Sync' })

    await vi.advanceTimersByTimeAsync(0)
    expect(screen.getByText('Queued — waiting for a worker.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open job' })).toHaveAttribute('href', '#/finance/jobs/job-1')

    await vi.advanceTimersByTimeAsync(2_000)
    expect(screen.getByText('Running now.')).toBeInTheDocument()
    await vi.advanceTimersByTimeAsync(2_000)
    expect(screen.getByText('Completed.')).toBeInTheDocument()

    await vi.advanceTimersByTimeAsync(10_000)
    expect(mocks.getJob).toHaveBeenCalledTimes(3)
  })

  it('stops polling when unmounted', async () => {
    mocks.getJob.mockResolvedValue(job('queued'))
    const page = render(JobStatus, { jobId: 'job-1', openHref: '/finance/jobs/job-1' })

    await vi.advanceTimersByTimeAsync(0)
    page.unmount()
    await vi.advanceTimersByTimeAsync(2_000)

    expect(mocks.getJob).toHaveBeenCalledTimes(1)
  })

  it('keeps a recoverable fetch error visible and retries status loading', async () => {
    mocks.getJob.mockRejectedValueOnce(new Error('Status unavailable')).mockResolvedValueOnce(job('failed', { summary: 'Provider rejected sync.' }))
    render(JobStatus, { jobId: 'job-1', openHref: '/finance/jobs/job-1' })

    await vi.advanceTimersByTimeAsync(0)
    expect(screen.getByText('Status unavailable')).toBeInTheDocument()

    await fireEvent.click(screen.getByRole('button', { name: 'Retry status' }))
    await vi.advanceTimersByTimeAsync(0)
    expect(screen.getByText('Provider rejected sync.')).toBeInTheDocument()
  })

  it('keeps polling a pre-materialization 404 for its observed dispatch', async () => {
    const { JobsApiError } = await import('../lib/jobs/api')
    mocks.getJob
      .mockRejectedValueOnce(new JobsApiError({ status: 404, method: 'GET', path: '/jobs/job-404', message: 'Not Found' }))
      .mockResolvedValueOnce(job('succeeded'))

    render(JobStatus, { jobId: 'job-404', openHref: '/finance/jobs/job-404', observedDispatch: true })

    await vi.advanceTimersByTimeAsync(0)
    expect(screen.getByText('Waiting for a worker to receive this job…')).toBeInTheDocument()
    await vi.advanceTimersByTimeAsync(2_000)
    expect(screen.getByText('Completed.')).toBeInTheDocument()
  })

  it('shows an arbitrary 404 as a recoverable error', async () => {
    const { JobsApiError } = await import('../lib/jobs/api')
    mocks.getJob.mockRejectedValue(new JobsApiError({ status: 404, method: 'GET', path: '/jobs/job-404', message: 'Not Found' }))

    render(JobStatus, { jobId: 'job-404', openHref: '/finance/jobs/job-404' })

    await vi.advanceTimersByTimeAsync(0)
    expect(screen.getByText(/Jobs API GET \/jobs\/job-404 failed: Not Found/)).toBeInTheDocument()
    expect(mocks.getJob).toHaveBeenCalledTimes(1)
  })
})
