import { beforeEach, describe, expect, it, vi } from 'vitest'
import { faker } from '@faker-js/faker'

vi.mock('../auth/auth-fetch', () => ({
  createAuthFetch: vi.fn(),
}))

import { createAuthFetch } from '../auth/auth-fetch'
import { JobsApiError, createSignalJobsApi, createSignalJobsApiForAuth } from './api'

const mockCreateAuthFetch = vi.mocked(createAuthFetch)

function makeJobJson(overrides: Partial<ReturnType<typeof baseJobJson>> = {}) {
  return { ...baseJobJson(), ...overrides }
}

function baseJobJson() {
  const createdAt = faker.date.recent()
  const updatedAt = faker.date.soon({ refDate: createdAt })
  const startedAt = faker.date.soon({ refDate: updatedAt })
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
      start: faker.date.past().toISOString(),
      end: faker.date.recent().toISOString(),
      pageSize: 0,
    },
    result: {
      ingestionRunId: faker.string.uuid(),
      persistedCount: 42,
      expectedCount: 50,
      missingIntervalCount: 1,
      duplicateNaturalKeyCount: 0,
      firstPersistedStart: faker.date.past().toISOString(),
      lastPersistedEnd: faker.date.recent().toISOString(),
      rawPayloadCount: 7,
      missingIntervalPreview: [
        {
          start: faker.date.past().toISOString(),
          end: faker.date.recent().toISOString(),
        },
      ],
      missingIntervalPreviewCap: 10,
    },
    error: {
      code: 'stale_running_requeued',
      summary: faker.lorem.sentence(),
      details: faker.lorem.sentence(),
    },
    createdAt: createdAt.toISOString(),
    updatedAt: updatedAt.toISOString(),
    startedAt: startedAt.toISOString(),
    completedAt: completedAt.toISOString(),
    attemptCount: 2,
    workerId: 'worker-a',
    lastAttemptAt: completedAt.toISOString(),
  }
}

describe('createSignalJobsApi', () => {
  beforeEach(() => {
    vi.unstubAllGlobals()
    mockCreateAuthFetch.mockReset()
  })

  it('uses the provided auth-aware fetch wrapper', async () => {
    const authFetch = vi
      .fn()
      .mockResolvedValue(new Response(JSON.stringify({ items: [], nextCursor: '' }), { status: 200 }))
    const globalFetch = vi.fn()
    vi.stubGlobal('fetch', globalFetch)
    const api = createSignalJobsApi({ baseUrl: '/api/v1', fetch: authFetch })

    await api.listJobs({})

    expect(authFetch).toHaveBeenCalledOnce()
    expect(globalFetch).not.toHaveBeenCalled()
  })

  it('serializes list filters as repeated query parameters', async () => {
    const authFetch = vi
      .fn()
      .mockResolvedValue(new Response(JSON.stringify({ items: [], nextCursor: 'cursor-2' }), { status: 200 }))
    const api = createSignalJobsApi({ baseUrl: '/api/v1', fetch: authFetch })

    const response = await api.listJobs({
      status: ['queued', 'running'],
      jobType: ['historical_raw_candle_backfill'],
      source: ['operator', 'agent'],
      limit: 25,
      cursor: 'cursor-1',
    })

    const requestUrl = new URL(authFetch.mock.calls[0][0] as string)
    expect(requestUrl.pathname).toBe('/api/v1/jobs')
    expect(requestUrl.searchParams.getAll('status')).toEqual(['queued', 'running'])
    expect(requestUrl.searchParams.getAll('jobType')).toEqual(['historical_raw_candle_backfill'])
    expect(requestUrl.searchParams.getAll('source')).toEqual(['operator', 'agent'])
    expect(requestUrl.searchParams.get('limit')).toBe('25')
    expect(requestUrl.searchParams.get('cursor')).toBe('cursor-1')
    expect(response.nextCursor).toBe('cursor-2')
  })

  it('omits blank list filters and cursor values', async () => {
    const authFetch = vi
      .fn()
      .mockResolvedValue(new Response(JSON.stringify({ items: [], nextCursor: '' }), { status: 200 }))
    const api = createSignalJobsApi({ baseUrl: '/api/v1', fetch: authFetch })

    await api.listJobs({
      status: ['   '],
      jobType: [''],
      source: ['  '],
      cursor: '   ',
    })

    const requestUrl = new URL(authFetch.mock.calls[0][0] as string)
    expect(requestUrl.searchParams.get('status')).toBeNull()
    expect(requestUrl.searchParams.get('jobType')).toBeNull()
    expect(requestUrl.searchParams.get('source')).toBeNull()
    expect(requestUrl.searchParams.get('cursor')).toBeNull()
  })

  it('maps list and detail responses into typed job models', async () => {
    const job = makeJobJson()
    const authFetch = vi
      .fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ items: [job], nextCursor: '' }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify(job), { status: 200 }))
    const api = createSignalJobsApi({ baseUrl: '/api/v1', fetch: authFetch })

    const list = await api.listJobs({})
    const detail = await api.getJob({ jobId: job.id })

    expect(list.items[0].createdAt).toBeInstanceOf(Date)
    expect(list.items[0].input.start).toBeInstanceOf(Date)
    expect(list.items[0].result?.missingIntervalPreview[0].start).toBeInstanceOf(Date)
    expect(detail.completedAt).toBeInstanceOf(Date)
    expect(detail.lastAttemptAt).toBeInstanceOf(Date)
  })

  it('serializes create requests and maps the response', async () => {
    const job = makeJobJson({ status: 'queued', result: undefined, error: undefined, completedAt: undefined })
    const authFetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(job), { status: 200 }))
    const api = createSignalJobsApi({ baseUrl: '/api/v1', fetch: authFetch })
    const start = faker.date.past()
    const end = faker.date.recent()

    const created = await api.createHistoricalDataBackfillJob({
      body: {
        idempotencyKey: 'idem-1',
        venue: 'hyperliquid-perps',
        symbol: 'BTC',
        assetClass: 'future',
        timeframe: '1h',
        start,
        end,
        pageSize: 0,
      },
    })

    expect(JSON.parse(String(authFetch.mock.calls[0][1]?.body))).toEqual(
      expect.objectContaining({
        idempotencyKey: 'idem-1',
        start: start.toISOString(),
        end: end.toISOString(),
        pageSize: 0,
      }),
    )
    expect(created.status).toBe('queued')
  })

  it('serializes nested arrays and omits undefined body properties', async () => {
    const job = makeJobJson({ status: 'queued', result: undefined, error: undefined, completedAt: undefined })
    const authFetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(job), { status: 200 }))
    const api = createSignalJobsApi({ baseUrl: '/api/v1', fetch: authFetch })
    const start = faker.date.past()
    const end = faker.date.recent()

    await api.createHistoricalDataBackfillJob({
      body: {
        venue: 'hyperliquid-perps',
        symbol: 'BTC',
        assetClass: 'future',
        timeframe: '1h',
        start,
        end,
        pageSize: undefined,
        extraDates: [start, end],
        optionalValue: undefined,
      } as never,
    })

    expect(JSON.parse(String(authFetch.mock.calls[0][1]?.body))).toEqual(
      expect.objectContaining({
        extraDates: [start.toISOString(), end.toISOString()],
      }),
    )
    expect(JSON.parse(String(authFetch.mock.calls[0][1]?.body))).not.toHaveProperty('pageSize')
    expect(JSON.parse(String(authFetch.mock.calls[0][1]?.body))).not.toHaveProperty('optionalValue')
  })

  it('creates an auth-backed jobs API client with createAuthFetch', async () => {
    const authFetch = vi
      .fn()
      .mockResolvedValue(new Response(JSON.stringify({ items: [], nextCursor: '' }), { status: 200 }))
    mockCreateAuthFetch.mockReturnValue(authFetch)
    const authStore = { accessToken: faker.string.alphanumeric(32) } as never
    const api = createSignalJobsApiForAuth({ baseUrl: '/api/v1', authStore })

    await api.listJobs({})

    expect(mockCreateAuthFetch).toHaveBeenCalledWith(authStore)
    expect(authFetch).toHaveBeenCalledOnce()
  })

  it('throws API errors with backend messages when requests fail', async () => {
    const authFetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ message: 'bad filter' }), {
        status: 400,
        statusText: 'Bad Request',
      }),
    )
    const api = createSignalJobsApi({ baseUrl: '/api/v1', fetch: authFetch })

    await expect(api.listJobs({ status: ['queued'] })).rejects.toThrow(/Jobs API GET \/jobs failed:/)
  })

  it('includes structured metadata on API errors', async () => {
    const authFetch = vi.fn().mockResolvedValue(
      new Response('', {
        status: 404,
        statusText: 'Not Found',
      }),
    )
    const api = createSignalJobsApi({ baseUrl: '/api/v1', fetch: authFetch })

    const request = api.getJob({ jobId: faker.string.uuid() })

    await expect(request).rejects.toBeInstanceOf(JobsApiError)
    await expect(request).rejects.toMatchObject({
      status: 404,
      method: 'GET',
      path: expect.stringMatching(/^\/jobs\//),
    })
  })

  it('falls back to HTTP status text when an error body is not JSON', async () => {
    const authFetch = vi.fn().mockResolvedValue(
      new Response('server exploded', {
        status: 500,
        statusText: 'Internal Server Error',
      }),
    )
    const api = createSignalJobsApi({ baseUrl: '/api/v1', fetch: authFetch })

    await expect(api.createHistoricalDataBackfillJob({
      body: {
        venue: 'hyperliquid-perps',
        symbol: 'BTC',
        assetClass: 'future',
        timeframe: '1h',
        start: new Date(faker.date.past()),
        end: new Date(faker.date.recent()),
        pageSize: 0,
      },
    })).rejects.toThrow(/500 Internal Server Error/)
  })
})
