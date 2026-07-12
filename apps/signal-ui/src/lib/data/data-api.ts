import type { AuthStore } from '../auth/auth-store.svelte'
import { createAuthFetch } from '../auth/auth-fetch'
import { ResponseTimestampError, parseRequiredResponseTimestamp, serializeRequestTimestamp } from '../timestamp'

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
  start?: Date | null
  end?: Date | null
  receivedAt: Date
}

export interface DataCandleListResponse {
  items: DataCandle[]
}

export interface CandleAvailabilityTimeframeSummary {
  timeframe: DataTimeframe | string
  start: Date
  end: Date
  count: number
}

export interface CandleAvailabilityDefaultSlice {
  timeframe: DataTimeframe | string
  start: Date
  end: Date
}

export interface CandleAvailabilityItem {
  venue: string
  symbol: string
  assetClass: string
  timeframes: CandleAvailabilityTimeframeSummary[]
  defaultSlice: CandleAvailabilityDefaultSlice
}

export interface CandleAvailabilityDefaultSelection {
  venue: string
  symbol: string
  assetClass: string
  timeframe: DataTimeframe | string
  start: Date
  end: Date
}

export interface CandleAvailabilityListResponse {
  items: CandleAvailabilityItem[]
  nextCursor?: string
  defaultSelection?: CandleAvailabilityDefaultSelection
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

export interface ListCandleAvailabilityParams {
  venue?: string
  symbol?: string
  assetClass?: string
  limit?: number
  cursor?: string
}

export interface ListCandleRawPayloadsParams extends ListDataCandlesParams {
  provenanceSource: string
  provenanceIdentity: string
}

export interface SignalDataApi {
  listCandleAvailability(
    params: ListCandleAvailabilityParams,
  ): Promise<CandleAvailabilityListResponse>
  listCandles(params: ListDataCandlesParams): Promise<DataCandleListResponse>
  listRawPayloads(params: ListRawPayloadsParams): Promise<RawPayloadMetadataListResponse>
  getRawPayloadDetail(id: string): Promise<RawPayloadDetailResponse>
  listCandleRawPayloads(
    params: ListCandleRawPayloadsParams,
  ): Promise<CandleRawPayloadMetadataListResponse>
}

type FetchLike = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>

export class DataApiError extends Error {
  readonly status: number
  readonly path: string

  constructor(params: { path: string; status: number; message: string }) {
    super(`Data API GET ${params.path} failed: ${params.message}`)
    this.name = 'DataApiError'
    this.status = params.status
    this.path = params.path
  }
}

export class DataResponseError extends ResponseTimestampError {
  constructor(params: { field: string; issue: string }) {
    super({ api: 'Data', ...params })
    this.name = 'DataResponseError'
  }
}

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
      throw new DataApiError({ path, status: response.status, message })
    }

    return json as T
  }

  return {
    async listCandleAvailability(paramsIn) {
      const json = await request<{
        items: RawCandleAvailabilityItem[]
        nextCursor?: string
        defaultSelection?: RawCandleAvailabilityDefaultSelection
      }>('/candle-availability', buildSearchParams(paramsIn))

      return {
        items: json.items.map(mapCandleAvailabilityItem),
        ...(json.nextCursor ? { nextCursor: json.nextCursor } : {}),
        ...(json.defaultSelection
          ? { defaultSelection: mapCandleAvailabilityDefaultSelection(json.defaultSelection) }
          : {}),
      }
    },

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

interface RawCandleAvailabilityTimeframeSummary {
  timeframe: string
  start: string
  end: string
  count: number
}

interface RawCandleAvailabilityDefaultSelection {
  venue: string
  symbol: string
  assetClass: string
  timeframe: string
  start: string
  end: string
}

interface RawCandleAvailabilityItem {
  venue: string
  symbol: string
  assetClass: string
  timeframes: RawCandleAvailabilityTimeframeSummary[]
  defaultSlice: {
    timeframe: string
    start: string
    end: string
  }
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
    string | number | Date | null | undefined
  ]>) {
    if (value === undefined) {
      continue
    }
    if (value === null) {
      throw new TypeError(`Cannot serialize null query parameter: ${key}`)
    }
    if (typeof value === 'string' && value.trim() === '') {
      continue
    }
    searchParams.set(key, value instanceof Date ? serializeRequestTimestamp(value) : String(value))
  }

  return searchParams
}

function mapDataCandle(item: RawDataCandle): DataCandle {
  return {
    ...item,
    start: parseDataRequiredTimestamp(item.start, 'data.candle.start'),
    end: parseDataRequiredTimestamp(item.end, 'data.candle.end'),
  }
}

function mapCandleAvailabilityItem(item: RawCandleAvailabilityItem): CandleAvailabilityItem {
  return {
    ...item,
    timeframes: item.timeframes.map(mapCandleAvailabilityTimeframeSummary),
    defaultSlice: mapCandleAvailabilityDefaultSlice(item.defaultSlice),
  }
}

function mapCandleAvailabilityTimeframeSummary(
  item: RawCandleAvailabilityTimeframeSummary,
): CandleAvailabilityTimeframeSummary {
  return {
    ...item,
    start: parseDataRequiredTimestamp(item.start, 'data.candleAvailability.timeframe.start'),
    end: parseDataRequiredTimestamp(item.end, 'data.candleAvailability.timeframe.end'),
  }
}

function mapCandleAvailabilityDefaultSlice(
  item: RawCandleAvailabilityItem['defaultSlice'],
): CandleAvailabilityDefaultSlice {
  return {
    ...item,
    start: parseDataRequiredTimestamp(item.start, 'data.candleAvailability.defaultSlice.start'),
    end: parseDataRequiredTimestamp(item.end, 'data.candleAvailability.defaultSlice.end'),
  }
}

function mapCandleAvailabilityDefaultSelection(
  item: RawCandleAvailabilityDefaultSelection,
): CandleAvailabilityDefaultSelection {
  return {
    ...item,
    start: parseDataRequiredTimestamp(item.start, 'data.candleAvailability.defaultSelection.start'),
    end: parseDataRequiredTimestamp(item.end, 'data.candleAvailability.defaultSelection.end'),
  }
}

function mapRawPayloadMetadata(item: RawRawPayloadMetadata): RawPayloadMetadata {
  return {
    ...item,
    symbol: item.symbol ?? null,
    assetClass: item.assetClass ?? null,
    timeframe: item.timeframe ?? null,
    requestAt: parseDataRequiredTimestamp(item.requestAt, 'data.rawPayload.requestAt'),
    responseAt: parseDataRequiredTimestamp(item.responseAt, 'data.rawPayload.responseAt'),
    start: parseDataOptionalTimestamp(item.start, 'data.rawPayload.start'),
    end: parseDataOptionalTimestamp(item.end, 'data.rawPayload.end'),
    receivedAt: parseDataRequiredTimestamp(item.receivedAt, 'data.rawPayload.receivedAt'),
  }
}

function parseDataRequiredTimestamp(value: unknown, field: string): Date {
  try {
    return parseRequiredResponseTimestamp(value, { api: 'Data', field })
  } catch (error) {
    if (error instanceof ResponseTimestampError) throw new DataResponseError({ field, issue: error.message.split(`${field} `)[1] ?? 'is invalid' })
    throw error
  }
}

function parseDataOptionalTimestamp(value: unknown, field: string): Date | null | undefined {
  if (value === undefined || value === null) return value
  return parseDataRequiredTimestamp(value, field)
}
