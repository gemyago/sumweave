import { beforeEach, describe, expect, it, vi } from 'vitest'
import { faker } from '@faker-js/faker'

vi.mock('../auth/auth-fetch', () => ({
  createAuthFetch: vi.fn(),
}))

import { createAuthFetch } from '../auth/auth-fetch'
import {
  DataApiError,
  createSignalDataApi,
  createSignalDataApiForAuth,
  type CandleAvailabilityItem,
  type DataCandle,
  type RawPayloadMetadata,
} from './data-api'

const mockCreateAuthFetch = vi.mocked(createAuthFetch)

function makeCandleJson() {
  return {
    identity: faker.number.int({ min: 1, max: 9999 }),
    venue: 'hyperliquid-perps',
    symbol: `${faker.finance.currencyCode()}USD`,
    assetClass: 'crypto',
    timeframe: '1m',
    start: faker.date.recent().toISOString(),
    end: faker.date.recent().toISOString(),
    open: faker.number.float({ min: 1, max: 1000 }),
    high: faker.number.float({ min: 1, max: 1000 }),
    low: faker.number.float({ min: 1, max: 1000 }),
    close: faker.number.float({ min: 1, max: 1000 }),
    volume: faker.number.float({ min: 1, max: 1000 }),
    quality: 'validated',
    provenanceSource: faker.word.noun(),
    provenanceIdentity: faker.string.uuid(),
  }
}

function makeRawPayloadJson() {
  return {
    id: faker.string.uuid(),
    ingestionRunId: faker.string.uuid(),
    source: faker.word.noun(),
    venue: 'hyperliquid-perps',
    endpoint: `/${faker.word.noun()}`,
    requestType: faker.word.verb(),
    requestPayloadHash: faker.string.hexadecimal({ length: 16 }),
    requestAt: faker.date.recent().toISOString(),
    responseAt: faker.date.recent().toISOString(),
    httpStatus: faker.number.int({ min: 200, max: 299 }),
    responseBodyHash: faker.string.hexadecimal({ length: 16 }),
    payloadBodyRef: faker.system.filePath(),
    entityHint: faker.word.words(2),
    symbol: `${faker.finance.currencyCode()}USD`,
    assetClass: 'crypto',
    timeframe: '1m',
    start: faker.date.recent().toISOString(),
    end: faker.date.recent().toISOString(),
    receivedAt: faker.date.recent().toISOString(),
  }
}

function makeAvailabilityJson() {
  return {
    venue: 'hyperliquid-perps',
    symbol: `${faker.finance.currencyCode()}USD`,
    assetClass: 'crypto',
    timeframes: [
      {
        timeframe: '1m',
        start: faker.date.recent().toISOString(),
        end: faker.date.recent().toISOString(),
        count: faker.number.int({ min: 1, max: 500 }),
      },
    ],
    defaultSlice: {
      timeframe: '1m',
      start: faker.date.recent().toISOString(),
      end: faker.date.recent().toISOString(),
    },
  }
}

describe('createSignalDataApi', () => {
  beforeEach(() => {
    vi.unstubAllGlobals()
    mockCreateAuthFetch.mockReset()
  })

  it('uses the provided auth-aware fetch wrapper', async () => {
    const authFetch = vi
      .fn()
      .mockResolvedValue(new Response(JSON.stringify({ items: [] }), { status: 200 }))
    const globalFetch = vi.fn()
    vi.stubGlobal('fetch', globalFetch)
    const api = createSignalDataApi({ baseUrl: '/api/v1/data', fetch: authFetch })

    await api.listCandles({
      venue: 'hyperliquid-perps',
      symbol: 'BTCUSD',
      assetClass: 'crypto',
      timeframe: '1m',
      start: new Date(faker.date.recent()),
      end: new Date(faker.date.recent()),
    })

    expect(authFetch).toHaveBeenCalledOnce()
    expect(globalFetch).not.toHaveBeenCalled()
  })

  it('serializes query parameters and omits blank optional values', async () => {
    const authFetch = vi
      .fn()
      .mockResolvedValue(new Response(JSON.stringify({ items: [] }), { status: 200 }))
    const api = createSignalDataApi({ baseUrl: '/api/v1/data', fetch: authFetch })
    const start = faker.date.recent()
    const end = faker.date.soon()

    await api.listRawPayloads({
      venue: 'hyperliquid-perps',
      symbol: 'BTCUSD',
      assetClass: 'crypto',
      timeframe: '1m',
      start,
      end,
      ingestionRunId: faker.string.uuid(),
      entityHint: '   ',
      endpoint: '',
      requestType: faker.word.verb(),
      limit: 50,
      cursor: faker.string.alphanumeric(12),
    })

    const requestUrl = new URL(authFetch.mock.calls[0][0] as string)
    expect(requestUrl.pathname).toBe('/api/v1/data/raw-payloads')
    expect(requestUrl.searchParams.get('venue')).toBe('hyperliquid-perps')
    expect(requestUrl.searchParams.get('symbol')).toBe('BTCUSD')
    expect(requestUrl.searchParams.get('assetClass')).toBe('crypto')
    expect(requestUrl.searchParams.get('timeframe')).toBe('1m')
    expect(requestUrl.searchParams.get('start')).toBe(start.toISOString())
    expect(requestUrl.searchParams.get('end')).toBe(end.toISOString())
    expect(requestUrl.searchParams.get('entityHint')).toBeNull()
    expect(requestUrl.searchParams.get('endpoint')).toBeNull()
  })

  it('omits undefined optional timestamp filters', async () => {
    const authFetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({ items: [] }), { status: 200 }))
    const api = createSignalDataApi({ baseUrl: '/api/v1/data', fetch: authFetch })

    await api.listRawPayloads({ venue: 'hyperliquid-perps' })
    expect(new URL(authFetch.mock.calls[0][0] as string).searchParams.has('start')).toBe(false)
    expect(authFetch).toHaveBeenCalledOnce()
  })

  it('serializes availability filters and maps nested availability response dates', async () => {
    const availability = makeAvailabilityJson()
    const nextCursor = faker.string.alphanumeric(10)
    const authFetch = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          items: [availability],
          nextCursor,
          defaultSelection: {
            venue: availability.venue,
            symbol: availability.symbol,
            assetClass: availability.assetClass,
            timeframe: availability.defaultSlice.timeframe,
            start: availability.defaultSlice.start,
            end: availability.defaultSlice.end,
          },
        }),
        { status: 200 },
      ),
    )
    const api = createSignalDataApi({ baseUrl: '/api/v1/data', fetch: authFetch })

    const response = await api.listCandleAvailability({
      venue: availability.venue,
      symbol: '   ',
      assetClass: availability.assetClass,
      limit: 50,
      cursor: nextCursor,
    })

    const requestUrl = new URL(authFetch.mock.calls[0][0] as string)
    expect(requestUrl.pathname).toBe('/api/v1/data/candle-availability')
    expect(requestUrl.searchParams.get('venue')).toBe(availability.venue)
    expect(requestUrl.searchParams.get('assetClass')).toBe(availability.assetClass)
    expect(requestUrl.searchParams.get('limit')).toBe('50')
    expect(requestUrl.searchParams.get('cursor')).toBe(nextCursor)
    expect(requestUrl.searchParams.get('symbol')).toBeNull()
    expect(response.items[0]).toEqual(
      expect.objectContaining<Partial<CandleAvailabilityItem>>({
        venue: availability.venue,
        symbol: availability.symbol,
        assetClass: availability.assetClass,
      }),
    )
    expect(response.items[0].timeframes[0].start).toBeInstanceOf(Date)
    expect(response.items[0].defaultSlice.end).toBeInstanceOf(Date)
    expect(response.defaultSelection?.start).toBeInstanceOf(Date)
    expect(response.nextCursor).toBe(nextCursor)
  })

  it('maps candle, raw payload, and detail responses into typed models', async () => {
    const candle = makeCandleJson()
    const rawPayload = makeRawPayloadJson()
    const authFetch = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ items: [candle] }), { status: 200 }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ items: [rawPayload], nextCursor: faker.string.alphanumeric(8) }), {
          status: 200,
        }),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            metadata: rawPayload,
            responseBodySizeBytes: faker.number.int({ min: 100, max: 2000 }),
            responseBodyPreview: faker.lorem.paragraph(),
            responseBodyPreviewTruncated: true,
          }),
          { status: 200 },
        ),
      )
    const api = createSignalDataApi({ baseUrl: '/api/v1/data', fetch: authFetch })

    const candles = await api.listCandles({
      venue: candle.venue,
      symbol: candle.symbol,
      assetClass: candle.assetClass,
      timeframe: candle.timeframe,
      start: new Date(candle.start),
      end: new Date(candle.end),
    })
    const rawPayloads = await api.listRawPayloads({ venue: rawPayload.venue })
    const detail = await api.getRawPayloadDetail(rawPayload.id)

    expect(candles.items[0]).toEqual(
      expect.objectContaining<Partial<DataCandle>>({
        identity: candle.identity,
        provenanceSource: candle.provenanceSource,
        provenanceIdentity: candle.provenanceIdentity,
      }),
    )
    expect(candles.items[0].start).toBeInstanceOf(Date)
    expect(rawPayloads.items[0]).toEqual(
      expect.objectContaining<Partial<RawPayloadMetadata>>({
        id: rawPayload.id,
        payloadBodyRef: rawPayload.payloadBodyRef,
      }),
    )
    expect(rawPayloads.items[0].receivedAt).toBeInstanceOf(Date)
    expect(detail.metadata.requestAt).toBeInstanceOf(Date)
    expect(detail.responseBodyPreviewTruncated).toBe(true)
  })

  it('maps candle-linked raw payload metadata responses', async () => {
    const rawPayload = makeRawPayloadJson()
    const authFetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ items: [{ ...rawPayload, symbol: null, start: null, end: null }] }), {
        status: 200,
      }),
    )
    const api = createSignalDataApi({ baseUrl: '/api/v1/data', fetch: authFetch })

    const response = await api.listCandleRawPayloads({
      venue: 'hyperliquid-perps',
      symbol: 'BTCUSD',
      assetClass: 'crypto',
      timeframe: '1m',
      start: new Date(faker.date.recent()),
      end: new Date(faker.date.soon()),
      provenanceSource: faker.word.noun(),
      provenanceIdentity: faker.string.uuid(),
    })

    expect(response.items[0].symbol).toBeNull()
    expect(response.items[0].start).toBeNull()
    expect(response.items[0].end).toBeNull()
  })

  it('preserves null and omitted raw-payload range timestamps but rejects empty or malformed response timestamps', async () => {
    const nullablePayload = { ...makeRawPayloadJson(), start: null, end: undefined }
    const malformedPayload = { ...makeRawPayloadJson(), requestAt: '', responseAt: 'not-a-timestamp' }
    const authFetch = vi
      .fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ items: [nullablePayload] }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ items: [malformedPayload] }), { status: 200 }))
    const api = createSignalDataApi({ baseUrl: '/api/v1/data', fetch: authFetch })

    const nullable = await api.listRawPayloads({ venue: nullablePayload.venue })

    expect(nullable.items[0].start).toBeNull()
    expect(nullable.items[0].end).toBeUndefined()
    await expect(api.listRawPayloads({ venue: malformedPayload.venue })).rejects.toMatchObject({
      name: 'DataResponseError',
      field: 'data.rawPayload.requestAt',
    })
  })

  it('creates an auth-backed data API client with createAuthFetch', async () => {
    const authFetch = vi
      .fn()
      .mockResolvedValue(new Response(JSON.stringify({ items: [] }), { status: 200 }))
    mockCreateAuthFetch.mockReturnValue(authFetch)
    const authStore = { accessToken: faker.string.alphanumeric(32) } as never
    const api = createSignalDataApiForAuth({ baseUrl: '/api/v1/data', authStore })

    await api.listRawPayloads({ venue: 'hyperliquid-perps' })

    expect(mockCreateAuthFetch).toHaveBeenCalledWith(authStore)
    expect(authFetch).toHaveBeenCalledOnce()
  })

  it('throws API errors with backend messages when requests fail', async () => {
    const authFetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ message: faker.lorem.sentence() }), {
        status: 400,
        statusText: 'Bad Request',
      }),
    )
    const api = createSignalDataApi({ baseUrl: '/api/v1/data', fetch: authFetch })

    await expect(
      api.listCandles({
        venue: 'hyperliquid-perps',
        symbol: 'BTCUSD',
        assetClass: 'crypto',
        timeframe: '1m',
        start: new Date(faker.date.recent()),
        end: new Date(faker.date.recent()),
      }),
    ).rejects.toThrow(/Data API GET \/candles failed:/)
  })

  it('includes structured status metadata on API errors', async () => {
    const authFetch = vi.fn().mockResolvedValue(
      new Response('', {
        status: 404,
        statusText: 'Not Found',
      }),
    )
    const api = createSignalDataApi({ baseUrl: '/api/v1/data', fetch: authFetch })

    const request = api.listCandleAvailability({})

    await expect(request).rejects.toBeInstanceOf(DataApiError)
    await expect(request).rejects.toMatchObject({
      status: 404,
      path: '/candle-availability',
      message: 'Data API GET /candle-availability failed: 404 Not Found',
    })
  })

  it('falls back to HTTP status text when an error body is not JSON', async () => {
    const authFetch = vi.fn().mockResolvedValue(
      new Response('server exploded', {
        status: 500,
        statusText: 'Internal Server Error',
      }),
    )
    const api = createSignalDataApi({ baseUrl: '/api/v1/data', fetch: authFetch })

    const request = api.getRawPayloadDetail(faker.string.uuid())

    await expect(request).rejects.toThrow('Data API GET /raw-payloads/')
    await expect(request).rejects.toThrow(/500 Internal Server Error/)
  })
})
