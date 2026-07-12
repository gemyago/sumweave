import type { AuthStore } from '../auth/auth-store.svelte'
import { createAuthFetch } from '../auth/auth-fetch'
import { ResponseTimestampError, parseRequiredResponseTimestamp } from '../timestamp'

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

export const historicalDataBackfillJobType = 'data.historical_raw_candle_backfill'

interface JobMetadata {
  id: string
  jobType: string
  status: string
  requester: JobRequester
  error?: JobExecutionError
  createdAt: Date
  updatedAt: Date
  startedAt?: Date | null
  completedAt?: Date | null
  attemptCount: number
}

export interface HistoricalDataBackfillJob extends JobMetadata {
  jobType: typeof historicalDataBackfillJobType
  input: HistoricalDataBackfillJobInput
  result?: HistoricalDataBackfillJobResult
}

export interface OtherJob extends JobMetadata {
  input?: undefined
  result?: undefined
}

export type JobSummary = HistoricalDataBackfillJob | OtherJob

interface JobDetailMetadata {
  workerId: string
  lastAttemptAt?: Date | null
}

export type JobDetail = JobSummary & JobDetailMetadata

export function isHistoricalDataBackfillJob(
  job: JobSummary | JobDetail,
): job is HistoricalDataBackfillJob & JobDetailMetadata {
  return job.jobType === historicalDataBackfillJobType
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

export class JobsResponseError extends ResponseTimestampError {
  constructor(params: { field: string; issue: string }) {
    super({ api: 'Jobs', ...params })
    this.name = 'JobsResponseError'
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
  input?: unknown
  result?: unknown
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
  const summary: JobMetadata = {
    id: raw.id,
    jobType: raw.jobType,
    status: raw.status,
    requester: raw.requester,
    ...(raw.error ? { error: raw.error } : {}),
    createdAt: parseJobsRequiredTimestamp(raw.createdAt, 'jobs.job.createdAt'),
    updatedAt: parseJobsRequiredTimestamp(raw.updatedAt, 'jobs.job.updatedAt'),
    startedAt: parseJobsOptionalTimestamp(raw.startedAt, 'jobs.job.startedAt'),
    completedAt: parseJobsOptionalTimestamp(raw.completedAt, 'jobs.job.completedAt'),
    attemptCount: raw.attemptCount,
  }

  if (raw.jobType !== historicalDataBackfillJobType) {
    return summary
  }

  return {
    ...summary,
    jobType: historicalDataBackfillJobType,
    input: mapJobInput(raw.input),
    ...(raw.result === undefined ? {} : { result: mapJobResult(raw.result) }),
  }
}

function mapJobDetail(raw: RawJobDetail): JobDetail {
  return {
    ...mapJobSummary(raw),
    workerId: raw.workerId ?? '',
    lastAttemptAt: parseJobsOptionalTimestamp(raw.lastAttemptAt, 'jobs.job.lastAttemptAt'),
  }
}

function mapJobInput(raw: unknown): HistoricalDataBackfillJobInput {
  const input = requireRecord(raw, 'jobs.job.input')
  return {
    ingestionRunId: requireString(input.ingestionRunId, 'jobs.job.input.ingestionRunId'),
    venue: requireString(input.venue, 'jobs.job.input.venue'),
    symbol: requireString(input.symbol, 'jobs.job.input.symbol'),
    assetClass: requireString(input.assetClass, 'jobs.job.input.assetClass'),
    timeframe: requireString(input.timeframe, 'jobs.job.input.timeframe'),
    start: parseJobsRequiredTimestamp(input.start, 'jobs.job.input.start'),
    end: parseJobsRequiredTimestamp(input.end, 'jobs.job.input.end'),
    pageSize: requireInteger(input.pageSize, 'jobs.job.input.pageSize'),
  }
}

function mapJobResult(raw: unknown): HistoricalDataBackfillJobResult {
  const result = requireRecord(raw, 'jobs.job.result')
  const missingIntervalPreview = result.missingIntervalPreview === undefined
    ? []
    : requireArray(result.missingIntervalPreview, 'jobs.job.result.missingIntervalPreview').map((item) => {
      const interval = requireRecord(item, 'jobs.job.result.missingIntervalPreview')
      return {
        start: parseJobsRequiredTimestamp(interval.start, 'jobs.job.result.missingIntervalPreview.start'),
        end: parseJobsRequiredTimestamp(interval.end, 'jobs.job.result.missingIntervalPreview.end'),
      }
    })

  return {
    ingestionRunId: requireString(result.ingestionRunId, 'jobs.job.result.ingestionRunId'),
    persistedCount: requireInteger(result.persistedCount, 'jobs.job.result.persistedCount'),
    expectedCount: requireInteger(result.expectedCount, 'jobs.job.result.expectedCount'),
    missingIntervalCount: requireInteger(result.missingIntervalCount, 'jobs.job.result.missingIntervalCount'),
    duplicateNaturalKeyCount: requireInteger(result.duplicateNaturalKeyCount, 'jobs.job.result.duplicateNaturalKeyCount'),
    firstPersistedStart: parseJobsOptionalTimestamp(result.firstPersistedStart, 'jobs.job.result.firstPersistedStart') ?? null,
    lastPersistedEnd: parseJobsOptionalTimestamp(result.lastPersistedEnd, 'jobs.job.result.lastPersistedEnd') ?? null,
    rawPayloadCount: result.rawPayloadCount === undefined || result.rawPayloadCount === null
      ? null
      : requireInteger(result.rawPayloadCount, 'jobs.job.result.rawPayloadCount'),
    missingIntervalPreview,
    missingIntervalPreviewCap: requireInteger(result.missingIntervalPreviewCap, 'jobs.job.result.missingIntervalPreviewCap'),
  }
}

function requireRecord(value: unknown, field: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new JobsResponseError({ field, issue: 'is required' })
  }
  return value as Record<string, unknown>
}

function requireArray(value: unknown, field: string): unknown[] {
  if (!Array.isArray(value)) {
    throw new JobsResponseError({ field, issue: 'must be an array' })
  }
  return value
}

function requireString(value: unknown, field: string): string {
  if (typeof value !== 'string') {
    throw new JobsResponseError({ field, issue: 'is required' })
  }
  return value
}

function requireInteger(value: unknown, field: string): number {
  if (typeof value !== 'number' || !Number.isInteger(value)) {
    throw new JobsResponseError({ field, issue: 'is required' })
  }
  return value
}

function parseJobsRequiredTimestamp(value: unknown, field: string): Date {
  try {
    return parseRequiredResponseTimestamp(value, { api: 'Jobs', field })
  } catch (error) {
    if (error instanceof ResponseTimestampError) throw new JobsResponseError({ field, issue: error.message.split(`${field} `)[1] ?? 'is invalid' })
    throw error
  }
}

function parseJobsOptionalTimestamp(value: unknown, field: string): Date | null | undefined {
  if (value === undefined || value === null) return value
  return parseJobsRequiredTimestamp(value, field)
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
