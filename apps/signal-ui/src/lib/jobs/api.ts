import type { AuthStore } from '../auth/auth-store.svelte'
import { createAuthFetch } from '../auth/auth-fetch'

export interface JobRequester {
  userId: string
  source: string
  agentSessionId: string
  agentRunId: string
}

export interface HistoricalDataBackfillJobInput {
  ingestionRunId: string
  venue: string
  symbol: string
  assetClass: string
  timeframe: string
  start: Date
  end: Date
  pageSize: number
}

export interface JobTimeRange {
  start: Date
  end: Date
}

export interface HistoricalDataBackfillJobResult {
  ingestionRunId: string
  persistedCount: number
  expectedCount: number
  missingIntervalCount: number
  duplicateNaturalKeyCount: number
  firstPersistedStart: Date | null
  lastPersistedEnd: Date | null
  rawPayloadCount: number | null
  missingIntervalPreview: JobTimeRange[]
  missingIntervalPreviewCap: number
}

export interface JobExecutionError {
  code: string
  summary: string
  details: string
}

export interface JobSummary {
  id: string
  jobType: string
  status: string
  requester: JobRequester
  input: HistoricalDataBackfillJobInput
  result?: HistoricalDataBackfillJobResult
  error?: JobExecutionError
  createdAt: Date
  updatedAt: Date
  startedAt: Date | null
  completedAt: Date | null
  attemptCount: number
}

export interface JobDetail extends JobSummary {
  workerId: string
  lastAttemptAt: Date | null
}

export interface ListJobsParams {
  status?: string[]
  jobType?: string[]
  source?: string[]
  limit?: number
  cursor?: string
}

export interface ListJobsResponse {
  items: JobSummary[]
  nextCursor: string
}

export interface CreateHistoricalDataBackfillJobRequest {
  idempotencyKey?: string
  correlationId?: string
  venue: string
  symbol: string
  assetClass: string
  timeframe: string
  start: Date
  end: Date
  pageSize?: number
}

export interface SignalJobsApi {
  listJobs(params: ListJobsParams): Promise<ListJobsResponse>
  getJob(params: { jobId: string }): Promise<JobDetail>
  createHistoricalDataBackfillJob(params: {
    body: CreateHistoricalDataBackfillJobRequest
  }): Promise<JobDetail>
}

type FetchLike = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>

export class JobsApiError extends Error {
  readonly status: number
  readonly method: string
  readonly path: string

  constructor(params: { status: number; method: string; path: string; message: string }) {
    super(`Jobs API ${params.method} ${params.path} failed: ${params.message}`)
    this.name = 'JobsApiError'
    this.status = params.status
    this.method = params.method
    this.path = params.path
  }
}

export function createSignalJobsApi(params: {
  baseUrl: string
  fetch: FetchLike
}): SignalJobsApi {
  const request = async <T>(requestParams: {
    method: 'GET' | 'POST'
    path: string
    query?: URLSearchParams
    body?: unknown
  }): Promise<T> => {
    const url = new URL(`${params.baseUrl}${requestParams.path}`, window.location.origin)
    if (requestParams.query) {
      url.search = requestParams.query.toString()
    }

    const response = await params.fetch(url.toString(), {
      method: requestParams.method,
      headers: {
        Accept: 'application/json',
        ...(requestParams.body ? { 'Content-Type': 'application/json' } : {}),
      },
      ...(requestParams.body ? { body: JSON.stringify(serializeJson(requestParams.body)) } : {}),
    })

    const json = await response.json().catch(() => undefined)
    if (!response.ok) {
      throw new JobsApiError({
        status: response.status,
        method: requestParams.method,
        path: requestParams.path,
        message: extractErrorMessage(response, json),
      })
    }

    return json as T
  }

  return {
    async listJobs(queryParams) {
      const json = await request<RawJobListResponse>({
        method: 'GET',
        path: '/jobs',
        query: buildSearchParams(queryParams),
      })
      return {
        items: (json.items ?? []).map(mapJobSummary),
        nextCursor: json.nextCursor ?? '',
      }
    },

    async getJob({ jobId }) {
      const json = await request<RawJobDetail>({
        method: 'GET',
        path: `/jobs/${encodeURIComponent(jobId)}`,
      })
      return mapJobDetail(json)
    },

    async createHistoricalDataBackfillJob({ body }) {
      const json = await request<RawJobDetail>({
        method: 'POST',
        path: '/jobs/historical-data-backfills',
        body,
      })
      return mapJobDetail(json)
    },
  }
}

export function createSignalJobsApiForAuth(params: {
  baseUrl: string
  authStore: AuthStore
}): SignalJobsApi {
  return createSignalJobsApi({
    baseUrl: params.baseUrl,
    fetch: createAuthFetch(params.authStore),
  })
}

interface RawJobRequester {
  userId: string
  source: string
  agentSessionId: string
  agentRunId: string
}

interface RawHistoricalDataBackfillJobInput {
  ingestionRunId: string
  venue: string
  symbol: string
  assetClass: string
  timeframe: string
  start: string
  end: string
  pageSize: number
}

interface RawJobTimeRange {
  start: string
  end: string
}

interface RawHistoricalDataBackfillJobResult {
  ingestionRunId: string
  persistedCount: number
  expectedCount: number
  missingIntervalCount: number
  duplicateNaturalKeyCount: number
  firstPersistedStart?: string | null
  lastPersistedEnd?: string | null
  rawPayloadCount?: number | null
  missingIntervalPreview?: RawJobTimeRange[]
  missingIntervalPreviewCap: number
}

interface RawJobExecutionError {
  code: string
  summary: string
  details: string
}

interface RawJobSummary {
  id: string
  jobType: string
  status: string
  requester: RawJobRequester
  input: RawHistoricalDataBackfillJobInput
  result?: RawHistoricalDataBackfillJobResult
  error?: RawJobExecutionError
  createdAt: string
  updatedAt: string
  startedAt?: string | null
  completedAt?: string | null
  attemptCount: number
}

interface RawJobDetail extends RawJobSummary {
  workerId?: string
  lastAttemptAt?: string | null
}

interface RawJobListResponse {
  items: RawJobSummary[]
  nextCursor?: string
}

function mapJobSummary(raw: RawJobSummary): JobSummary {
  return {
    id: raw.id,
    jobType: raw.jobType,
    status: raw.status,
    requester: raw.requester,
    input: mapJobInput(raw.input),
    ...(raw.result ? { result: mapJobResult(raw.result) } : {}),
    ...(raw.error ? { error: raw.error } : {}),
    createdAt: new Date(raw.createdAt),
    updatedAt: new Date(raw.updatedAt),
    startedAt: raw.startedAt ? new Date(raw.startedAt) : null,
    completedAt: raw.completedAt ? new Date(raw.completedAt) : null,
    attemptCount: raw.attemptCount,
  }
}

function mapJobDetail(raw: RawJobDetail): JobDetail {
  return {
    ...mapJobSummary(raw),
    workerId: raw.workerId ?? '',
    lastAttemptAt: raw.lastAttemptAt ? new Date(raw.lastAttemptAt) : null,
  }
}

function mapJobInput(raw: RawHistoricalDataBackfillJobInput): HistoricalDataBackfillJobInput {
  return {
    ...raw,
    start: new Date(raw.start),
    end: new Date(raw.end),
  }
}

function mapJobResult(raw: RawHistoricalDataBackfillJobResult): HistoricalDataBackfillJobResult {
  return {
    ...raw,
    firstPersistedStart: raw.firstPersistedStart ? new Date(raw.firstPersistedStart) : null,
    lastPersistedEnd: raw.lastPersistedEnd ? new Date(raw.lastPersistedEnd) : null,
    rawPayloadCount: raw.rawPayloadCount ?? null,
    missingIntervalPreview: (raw.missingIntervalPreview ?? []).map((item) => ({
      start: new Date(item.start),
      end: new Date(item.end),
    })),
  }
}

function buildSearchParams(params: ListJobsParams): URLSearchParams {
  const searchParams = new URLSearchParams()
  for (const status of params.status ?? []) {
    if (status.trim()) {
      searchParams.append('status', status.trim())
    }
  }
  for (const jobType of params.jobType ?? []) {
    if (jobType.trim()) {
      searchParams.append('jobType', jobType.trim())
    }
  }
  for (const source of params.source ?? []) {
    if (source.trim()) {
      searchParams.append('source', source.trim())
    }
  }
  if (typeof params.limit === 'number') {
    searchParams.set('limit', String(params.limit))
  }
  if (params.cursor?.trim()) {
    searchParams.set('cursor', params.cursor.trim())
  }
  return searchParams
}

function serializeJson(value: unknown): unknown {
  if (value instanceof Date) {
    return value.toISOString()
  }
  if (Array.isArray(value)) {
    return value.map(serializeJson)
  }
  if (value && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value).flatMap(([key, nestedValue]) => {
        if (nestedValue === undefined) {
          return []
        }
        return [[key, serializeJson(nestedValue)]]
      }),
    )
  }
  return value
}

function extractErrorMessage(response: Response, json: unknown): string {
  if (typeof json === 'object' && json !== null && 'message' in json && typeof json.message === 'string') {
    return json.message
  }
  return `${response.status} ${response.statusText}`.trim()
}
