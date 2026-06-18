import { beforeEach, describe, expect, it, vi } from 'vitest'
import { faker } from '@faker-js/faker'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import Jobs from './Jobs.svelte'

const mocks = vi.hoisted(() => ({
  listJobs: vi.fn(),
}))

vi.mock('../lib/jobs/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/jobs/api')>()
  return {
    ...actual,
    createSignalJobsApiForAuth: vi.fn(() => ({
      listJobs: mocks.listJobs,
    })),
  }
})

vi.mock('../lib/auth/auth-store.svelte', () => ({
  authStore: { accessToken: faker.string.alphanumeric(32) },
}))

function makeJobSummary(overrides: Record<string, unknown> = {}) {
  const createdAt = faker.date.recent()
  const updatedAt = faker.date.soon({ refDate: createdAt })
  return {
    id: faker.string.uuid(),
    jobType: 'historical_raw_candle_backfill',
    status: 'queued',
    requester: { userId: faker.string.uuid(), source: 'operator', agentSessionId: '', agentRunId: '' },
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
    createdAt,
    updatedAt,
    attemptCount: 1,
    ...overrides,
  }
}

describe('Jobs page', () => {
  beforeEach(() => {
    mocks.listJobs.mockReset()
    mocks.listJobs.mockResolvedValue({ items: [], nextCursor: '' })
  })

  it('renders loading then empty state', async () => {
    render(Jobs)

    expect(screen.getByText('Loading jobs…')).toBeInTheDocument()
    expect(await screen.findByText('No durable jobs matched the current filters.')).toBeInTheDocument()
  })

  it('renders an API error state', async () => {
    mocks.listJobs.mockRejectedValue(new Error('jobs exploded'))

    render(Jobs)

    expect(await screen.findByRole('alert')).toHaveTextContent('jobs exploded')
  })

  it('applies filters, refreshes, and renders stacked job summaries with open actions', async () => {
    const user = userEvent.setup()
    const first = makeJobSummary({
      result: {
        ingestionRunId: faker.string.uuid(),
        persistedCount: 9,
        expectedCount: 10,
        missingIntervalCount: 1,
        duplicateNaturalKeyCount: 0,
        firstPersistedStart: faker.date.past(),
        lastPersistedEnd: faker.date.recent(),
        rawPayloadCount: 4,
        missingIntervalPreview: [],
        missingIntervalPreviewCap: 10,
      },
    })
    const second = makeJobSummary({
      id: faker.string.uuid(),
      status: 'running',
      requester: { userId: faker.string.uuid(), source: 'agent', agentSessionId: 'session-1', agentRunId: 'run-1' },
      error: { code: 'job_failed', summary: 'safe error', details: 'detail' },
    })
    const third = makeJobSummary({ id: faker.string.uuid(), status: 'succeeded' })
    mocks.listJobs
      .mockResolvedValueOnce({ items: [first, second], nextCursor: 'cursor-2' })
      .mockResolvedValueOnce({ items: [first, second], nextCursor: 'cursor-2' })
      .mockResolvedValueOnce({ items: [third], nextCursor: '' })
      .mockResolvedValue({ items: [first, second], nextCursor: 'cursor-2' })

    render(Jobs)

    expect(await screen.findByText(first.id)).toBeInTheDocument()
    expect(screen.getByText(second.id)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Load more' })).toBeInTheDocument()
    expect(screen.getByText('Result: 9 persisted / 10 expected · 1 missing intervals')).toBeInTheDocument()
    expect(screen.getByText('safe error')).toBeInTheDocument()
    expect(screen.getAllByRole('link', { name: 'Open job detail' })).toHaveLength(2)

    await user.selectOptions(screen.getByLabelText('Status'), 'running')
    await user.selectOptions(screen.getByLabelText('Job type'), 'historical_raw_candle_backfill')
    await user.selectOptions(screen.getByLabelText('Source'), 'agent')
    await user.click(screen.getByRole('button', { name: 'Apply filters' }))

    await waitFor(() => {
      expect(mocks.listJobs).toHaveBeenLastCalledWith({
        status: ['running'],
        jobType: ['historical_raw_candle_backfill'],
        source: ['agent'],
        limit: 25,
        cursor: '',
      })
    })

    await user.click(screen.getByRole('button', { name: 'Load more' }))

    await waitFor(() => {
      expect(mocks.listJobs).toHaveBeenNthCalledWith(3, {
        status: ['running'],
        jobType: ['historical_raw_candle_backfill'],
        source: ['agent'],
        limit: 25,
        cursor: 'cursor-2',
      })
    })
    expect(await screen.findByText(third.id)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Refresh jobs' }))

    expect(mocks.listJobs).toHaveBeenCalledTimes(4)
  })
})
