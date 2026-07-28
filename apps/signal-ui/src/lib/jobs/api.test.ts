import { describe, expect, it, vi } from 'vitest'
import {
  createSignalJobsApi,
  createSignalJobsApiForAuth,
  JobsApiError,
  JobsResponseError,
} from './api'

vi.mock('../auth/auth-fetch', () => ({
  createAuthFetch: vi.fn(() => vi.fn()),
}))

function jobFixture() {
  return {
    id: 'job-1',
    jobType: 'finance.csv_import',
    status: 'completed',
    requester: { userId: 'user-1', source: 'operator', agentSessionId: '', agentRunId: '' },
    createdAt: '2026-07-27T12:00:00Z',
    updatedAt: '2026-07-27T12:01:00Z',
    startedAt: null,
    completedAt: '2026-07-27T12:02:00Z',
    attemptCount: 2,
  }
}

describe('jobs api', () => {
  it('lists finance jobs with repeated filters and maps metadata timestamps', async () => {
    let requestedURL = ''
    const fetch = vi.fn(async (input: RequestInfo | URL) => {
      requestedURL = String(input)
      return { ok: true, status: 200, statusText: 'OK', json: async () => ({ items: [jobFixture()], nextCursor: 'cursor-2' }) } as Response
    })

    const result = await createSignalJobsApi({ baseUrl: '/api/v1', fetch }).listJobs({
      status: ['queued', ' '],
      jobType: ['finance.csv_import'],
      source: ['operator'],
      limit: 20,
      cursor: ' cursor-1 ',
    })

    const url = new URL(requestedURL)
    expect(url.pathname).toBe('/api/v1/jobs')
    expect([...url.searchParams.entries()]).toEqual([
      ['status', 'queued'], ['jobType', 'finance.csv_import'], ['source', 'operator'], ['limit', '20'], ['cursor', 'cursor-1'],
    ])
    expect(result.items[0]).toMatchObject({ id: 'job-1', attemptCount: 2, startedAt: null })
    expect(result.items[0].completedAt).toEqual(new Date('2026-07-27T12:02:00Z'))
    expect(result.nextCursor).toBe('cursor-2')
  })

  it('gets a job with optional worker and attempt metadata omitted', async () => {
    let requestedURL = ''
    const fetch = vi.fn(async (input: RequestInfo | URL) => {
      requestedURL = String(input)
      return { ok: true, status: 200, statusText: 'OK', json: async () => jobFixture() } as Response
    })

    const job = await createSignalJobsApi({ baseUrl: '/api/v1', fetch }).getJob({ jobId: 'job / 1' })

    expect(requestedURL).toContain('/jobs/job%20%2F%201')
    expect(job).toMatchObject({ workerId: '', lastAttemptAt: undefined })
  })

  it('accepts an empty list response without a next cursor', async () => {
    const fetch = vi.fn(async () => ({ ok: true, status: 200, statusText: 'OK', json: async () => ({}) }) as Response)
    await expect(createSignalJobsApi({ baseUrl: '/api/v1', fetch }).listJobs({})).resolves.toEqual({ items: [], nextCursor: '' })
  })

  it('maps a safe job error and optional timestamps when present', async () => {
    const fetch = vi.fn(async () => ({
      ok: true, status: 200, statusText: 'OK',
      json: async () => ({ ...jobFixture(), startedAt: '2026-07-27T12:00:30Z', error: { code: 'failed', summary: 'Import failed', details: 'safe details' }, workerId: 'worker-1', lastAttemptAt: '2026-07-27T12:03:00Z' }),
    }) as Response)

    const job = await createSignalJobsApi({ baseUrl: '/api/v1', fetch }).getJob({ jobId: 'job-1' })

    expect(job.error?.summary).toBe('Import failed')
    expect(job.workerId).toBe('worker-1')
    expect(job.lastAttemptAt).toEqual(new Date('2026-07-27T12:03:00Z'))
  })

  it('uses the API error message or status metadata for failed requests', async () => {
    const bodyMessage = vi.fn(async () => ({ message: 'Jobs are unavailable' }))
    const bodyError = createSignalJobsApi({ baseUrl: '/api/v1', fetch: vi.fn(async () => ({ ok: false, status: 503, statusText: 'Unavailable', json: bodyMessage }) as unknown as Response) }).listJobs({})
    await expect(bodyError).rejects.toMatchObject({ status: 503, path: '/jobs' } satisfies Partial<JobsApiError>)
    await expect(bodyError).rejects.toThrow('Jobs are unavailable')

    const statusError = createSignalJobsApi({ baseUrl: '/api/v1', fetch: vi.fn(async () => ({ ok: false, status: 500, statusText: 'Broken', json: async () => ({}) }) as Response) }).getJob({ jobId: 'job-1' })
    await expect(statusError).rejects.toThrow('500 Broken')

    const invalidBodyError = createSignalJobsApi({ baseUrl: '/api/v1', fetch: vi.fn(async () => ({ ok: false, status: 502, statusText: 'Gateway', json: async () => { throw new Error('invalid JSON') } }) as unknown as Response) }).getJob({ jobId: 'job-1' })
    await expect(invalidBodyError).rejects.toThrow('502 Gateway')
  })

  it('rejects malformed required job timestamps', async () => {
    const fetch = vi.fn(async () => ({ ok: true, status: 200, statusText: 'OK', json: async () => ({ ...jobFixture(), createdAt: '' }) }) as Response)

    await expect(createSignalJobsApi({ baseUrl: '/api/v1', fetch }).getJob({ jobId: 'job-1' })).rejects.toBeInstanceOf(JobsResponseError)
  })

  it('builds an authenticated jobs API', async () => {
    const api = createSignalJobsApiForAuth({ baseUrl: '/api/v1', authStore: {} as never })
    expect(api).toHaveProperty('listJobs')
    expect(api).toHaveProperty('getJob')
  })
})
