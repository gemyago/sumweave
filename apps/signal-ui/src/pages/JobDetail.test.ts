import { beforeEach, describe, expect, it, vi } from 'vitest'
import { faker } from '@faker-js/faker'
import { render, screen } from '@testing-library/svelte'
import JobDetail from './JobDetail.svelte'

const mocks = vi.hoisted(() => ({
  getJob: vi.fn(),
}))

vi.mock('../lib/jobs/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/jobs/api')>()
  return {
    ...actual,
    createSignalJobsApiForAuth: vi.fn(() => ({
      getJob: mocks.getJob,
    })),
  }
})

vi.mock('../lib/auth/auth-store.svelte', () => ({
  authStore: { accessToken: faker.string.alphanumeric(32) },
}))

function makeJobDetail(overrides: Record<string, unknown> = {}) {
  const startedAt = faker.date.recent()
  const completedAt = faker.date.soon({ refDate: startedAt })
  return {
    id: faker.string.uuid(),
    jobType: 'historical_raw_candle_backfill',
    status: 'succeeded',
    requester: {
      userId: faker.string.uuid(),
      source: 'operator',
      agentSessionId: '',
      agentRunId: '',
    },
    input: {
      ingestionRunId: faker.string.uuid(),
      venue: 'hyperliquid-perps',
      symbol: 'BTC',
      assetClass: 'future',
      timeframe: '1h',
      start: faker.date.past(),
      end: faker.date.recent(),
      pageSize: 0,
    },
    result: {
      ingestionRunId: faker.string.uuid(),
      persistedCount: 42,
      expectedCount: 50,
      missingIntervalCount: 1,
      duplicateNaturalKeyCount: 0,
      firstPersistedStart: faker.date.past(),
      lastPersistedEnd: faker.date.recent(),
      rawPayloadCount: 7,
      missingIntervalPreview: [{ start: faker.date.past(), end: faker.date.recent() }],
      missingIntervalPreviewCap: 10,
    },
    createdAt: faker.date.past(),
    updatedAt: faker.date.recent(),
    startedAt,
    completedAt,
    attemptCount: 2,
    workerId: 'worker-a',
    lastAttemptAt: completedAt,
    ...overrides,
  }
}

describe('Job detail page', () => {
  beforeEach(() => {
    mocks.getJob.mockReset()
  })

  it('renders succeeded job detail, missing interval preview, and navigation links', async () => {
    const job = makeJobDetail()
    mocks.getJob.mockResolvedValue(job)

    render(JobDetail, { params: { jobId: job.id } })

    expect(await screen.findByText(job.id)).toBeInTheDocument()
    expect(screen.getByText('persistedCount')).toBeInTheDocument()
    expect(screen.getByText(String(job.result.persistedCount))).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Back to jobs' })).toHaveAttribute('href', '#/jobs')
    expect(screen.getByRole('link', { name: 'Back to data' })).toHaveAttribute('href', '#/data')
  })

  it('renders queued/running and failed-specific fields', async () => {
    const job = makeJobDetail({
      status: 'failed',
      result: undefined,
      error: { code: 'job_failed', summary: 'Execution failed', details: 'safe detail' },
    })
    mocks.getJob.mockResolvedValue(job)

    render(JobDetail, { params: { jobId: job.id } })

    expect(await screen.findByText('Execution failed')).toBeInTheDocument()
    expect(screen.getByText('safe detail')).toBeInTheDocument()
  })

  it('renders validation and loading states', async () => {
    mocks.getJob.mockResolvedValue(makeJobDetail())

    const { rerender } = render(JobDetail, { params: {} })
    expect(screen.getByRole('alert')).toHaveTextContent('Job id is required.')

    rerender({ params: { jobId: 'job-123' } })
    expect(screen.getByText('Loading job detail…')).toBeInTheDocument()
  })

  it('renders a detail API error state', async () => {
    mocks.getJob.mockRejectedValue(new Error('detail blew up'))

    render(JobDetail, { params: { jobId: 'job-123' } })

    expect(await screen.findByRole('alert')).toHaveTextContent('detail blew up')
  })
})
