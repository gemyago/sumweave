import type { AuthStore } from '../auth/auth-store.svelte'
import { createAuthFetch } from '../auth/auth-fetch'

export type DataTimeframe = '1m' | '5m' | '15m' | '1h' | '4h' | '1d'

export interface DataCandle {
  identity: number
  venue: string
  symbol: string
  assetClass: string
  timeframe: DataTimeframe | string
  start: Date
  end: Date
  open: number
  high: number
  low: number
  close: number
  volume: number
  quality: string
  provenanceSource: string
  provenanceIdentity: string
}

export interface RawPayloadMetadata {
  id: string
  ingestionRunId: string
  source: string
  venue: string
  endpoint: string
  requestType: string
  requestPayloadHash: string
  requestAt: Date
  responseAt: Date
  httpStatus: number
  responseBodyHash: string
  payloadBodyRef: string
  entityHint: string
  symbol: string | null
  assetClass: string | null
  timeframe: string | null
  start: Date | null
  end: Date | null
  receivedAt: Date
}

export interface DataCandleListResponse {
  items: DataCandle[]
}

export interface RawPayloadMetadataListResponse {
  items: RawPayloadMetadata[]
  nextCursor?: string
}

export interface CandleRawPayloadMetadataListResponse {
  items: RawPayloadMetadata[]
}

export interface RawPayloadDetailResponse {
  metadata: RawPayloadMetadata
  responseBodySizeBytes: number
  responseBodyPreview: string
  responseBodyPreviewTruncated: boolean
}

export interface ListDataCandlesParams {
  venue: string
  symbol: string
  assetClass: string
  timeframe: string
  start: Date
  end: Date
}

export interface ListRawPayloadsParams {
  venue: string
  symbol?: string
  assetClass?: string
  timeframe?: string
  start?: Date
  end?: Date
  ingestionRunId?: string
  entityHint?: string
  endpoint?: string
  requestType?: string
  limit?: number
  cursor?: string
}

export interface ListCandleRawPayloadsParams extends ListDataCandlesParams {
  provenanceSource: string
  provenanceIdentity: string
}

export interface SignalDataApi {
  listCandles(params: ListDataCandlesParams): Promise<DataCandleListResponse>
  listRawPayloads(params: ListRawPayloadsParams): Promise<RawPayloadMetadataListResponse>
  getRawPayloadDetail(id: string): Promise<RawPayloadDetailResponse>
  listCandleRawPayloads(
    params: ListCandleRawPayloadsParams,
  ): Promise<CandleRawPayloadMetadataListResponse>
}

type FetchLike = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>

export function createSignalDataApi(params: {
  baseUrl: string
  fetch: FetchLike
}): SignalDataApi {
  const request = async <T>(path: string, query?: URLSearchParams): Promise<T> => {
    const url = new URL(`${params.baseUrl}${path}`, window.location.origin)
    if (query) {
      url.search = query.toString()
    }

    const response = await params.fetch(url.toString(), {
      method: 'GET',
      headers: { Accept: 'application/json' },
    })

    const json = await response.json().catch(() => undefined)

    if (!response.ok) {
      const message =
        typeof json === 'object' && json !== null && 'message' in json && typeof json.message === 'string'
          ? json.message
          : `${response.status} ${response.statusText}`.trim()
      throw new Error(`Data API GET ${path} failed: ${message}`)
    }

    return json as T
  }

  return {
    async listCandles(paramsIn) {
      const json = await request<{ items: RawDataCandle[] }>('/candles', buildSearchParams(paramsIn))
      return { items: json.items.map(mapDataCandle) }
    },

    async listRawPayloads(paramsIn) {
      const json = await request<{
        items: RawRawPayloadMetadata[]
        nextCursor?: string
      }>('/raw-payloads', buildSearchParams(paramsIn))
      return {
        items: json.items.map(mapRawPayloadMetadata),
        ...(json.nextCursor ? { nextCursor: json.nextCursor } : {}),
      }
    },

    async getRawPayloadDetail(id) {
      const json = await request<{
        metadata: RawRawPayloadMetadata
        responseBodySizeBytes: number
        responseBodyPreview: string
        responseBodyPreviewTruncated: boolean
      }>(`/raw-payloads/${encodeURIComponent(id)}`)
      return {
        metadata: mapRawPayloadMetadata(json.metadata),
        responseBodySizeBytes: json.responseBodySizeBytes,
        responseBodyPreview: json.responseBodyPreview,
        responseBodyPreviewTruncated: json.responseBodyPreviewTruncated,
      }
    },

    async listCandleRawPayloads(paramsIn) {
      const json = await request<{ items: RawRawPayloadMetadata[] }>(
        '/candle-raw-payloads',
        buildSearchParams(paramsIn),
      )
      return { items: json.items.map(mapRawPayloadMetadata) }
    },
  }
}

export function createSignalDataApiForAuth(params: {
  baseUrl: string
  authStore: AuthStore
}): SignalDataApi {
  return createSignalDataApi({
    baseUrl: params.baseUrl,
    fetch: createAuthFetch(params.authStore),
  })
}

interface RawDataCandle {
  identity: number
  venue: string
  symbol: string
  assetClass: string
  timeframe: string
  start: string
  end: string
  open: number
  high: number
  low: number
  close: number
  volume: number
  quality: string
  provenanceSource: string
  provenanceIdentity: string
}

interface RawRawPayloadMetadata {
  id: string
  ingestionRunId: string
  source: string
  venue: string
  endpoint: string
  requestType: string
  requestPayloadHash: string
  requestAt: string
  responseAt: string
  httpStatus: number
  responseBodyHash: string
  payloadBodyRef: string
  entityHint: string
  symbol?: string | null
  assetClass?: string | null
  timeframe?: string | null
  start?: string | null
  end?: string | null
  receivedAt: string
}

function buildSearchParams<T extends object>(params: T): URLSearchParams {
  const searchParams = new URLSearchParams()

  for (const [key, value] of Object.entries(params) as Array<[
    string,
    string | number | Date | undefined
  ]>) {
    if (value === undefined) {
      continue
    }
    if (typeof value === 'string' && value.trim() === '') {
      continue
    }
    searchParams.set(key, value instanceof Date ? value.toISOString() : String(value))
  }

  return searchParams
}

function mapDataCandle(item: RawDataCandle): DataCandle {
  return {
    ...item,
    start: new Date(item.start),
    end: new Date(item.end),
  }
}

function mapRawPayloadMetadata(item: RawRawPayloadMetadata): RawPayloadMetadata {
  return {
    ...item,
    symbol: item.symbol ?? null,
    assetClass: item.assetClass ?? null,
    timeframe: item.timeframe ?? null,
    requestAt: new Date(item.requestAt),
    responseAt: new Date(item.responseAt),
    start: item.start ? new Date(item.start) : null,
    end: item.end ? new Date(item.end) : null,
    receivedAt: new Date(item.receivedAt),
  }
}
