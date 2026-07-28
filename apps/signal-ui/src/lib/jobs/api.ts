import type { AuthStore } from '../auth/auth-store.svelte'
import { createAuthFetch } from '../auth/auth-fetch'
import { ResponseTimestampError, parseRequiredResponseTimestamp } from '../timestamp'

export interface JobRequester { userId: string; source: string; agentSessionId: string; agentRunId: string }
export interface JobExecutionError { code: string; summary: string; details: string }
export interface JobSummary { id: string; jobType: string; status: string; requester: JobRequester; error?: JobExecutionError; createdAt: Date; updatedAt: Date; startedAt?: Date | null; completedAt?: Date | null; attemptCount: number }
export interface JobDetail extends JobSummary { workerId: string; lastAttemptAt?: Date | null }
export interface ListJobsParams { status?: string[]; jobType?: string[]; source?: string[]; limit?: number; cursor?: string }
export interface ListJobsResponse { items: JobSummary[]; nextCursor: string }
export interface SignalJobsApi { listJobs(params: ListJobsParams): Promise<ListJobsResponse>; getJob(params: { jobId: string }): Promise<JobDetail> }
type FetchLike = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>
export class JobsApiError extends Error { readonly status: number; readonly method: string; readonly path: string; constructor(params: { status: number; method: string; path: string; message: string }) { super(`Jobs API ${params.method} ${params.path} failed: ${params.message}`); this.name = 'JobsApiError'; this.status = params.status; this.method = params.method; this.path = params.path } }
export class JobsResponseError extends ResponseTimestampError { constructor(params: { field: string; issue: string }) { super({ api: 'Jobs', ...params }); this.name = 'JobsResponseError' } }
export function createSignalJobsApi(params: { baseUrl: string; fetch: FetchLike }): SignalJobsApi {
  const request = async <T>(path: string, query?: URLSearchParams): Promise<T> => { const url = new URL(`${params.baseUrl}${path}`, window.location.origin); if (query) url.search = query.toString(); const response = await params.fetch(url.toString(), { method: 'GET', headers: { Accept: 'application/json' } }); const json = await response.json().catch(() => undefined); if (!response.ok) throw new JobsApiError({ status: response.status, method: 'GET', path, message: extractErrorMessage(response, json) }); return json as T }
  return { async listJobs(queryParams) { const json = await request<RawJobListResponse>('/jobs', buildSearchParams(queryParams)); return { items: (json.items ?? []).map(mapJobSummary), nextCursor: json.nextCursor ?? '' } }, async getJob({ jobId }) { return mapJobDetail(await request<RawJobDetail>(`/jobs/${encodeURIComponent(jobId)}`)) } }
}
export function createSignalJobsApiForAuth(params: { baseUrl: string; authStore: AuthStore }): SignalJobsApi { return createSignalJobsApi({ baseUrl: params.baseUrl, fetch: createAuthFetch(params.authStore) }) }
interface RawJobRequester { userId: string; source: string; agentSessionId: string; agentRunId: string }
interface RawJobExecutionError { code: string; summary: string; details: string }
interface RawJobSummary { id: string; jobType: string; status: string; requester: RawJobRequester; error?: RawJobExecutionError; createdAt: string; updatedAt: string; startedAt?: string | null; completedAt?: string | null; attemptCount: number }
interface RawJobDetail extends RawJobSummary { workerId?: string; lastAttemptAt?: string | null }
interface RawJobListResponse { items: RawJobSummary[]; nextCursor?: string }
function mapJobSummary(raw: RawJobSummary): JobSummary { return { id: raw.id, jobType: raw.jobType, status: raw.status, requester: raw.requester, ...(raw.error ? { error: raw.error } : {}), createdAt: parseTimestamp(raw.createdAt, 'jobs.job.createdAt'), updatedAt: parseTimestamp(raw.updatedAt, 'jobs.job.updatedAt'), startedAt: parseOptionalTimestamp(raw.startedAt, 'jobs.job.startedAt'), completedAt: parseOptionalTimestamp(raw.completedAt, 'jobs.job.completedAt'), attemptCount: raw.attemptCount } }
function mapJobDetail(raw: RawJobDetail): JobDetail { return { ...mapJobSummary(raw), workerId: raw.workerId ?? '', lastAttemptAt: parseOptionalTimestamp(raw.lastAttemptAt, 'jobs.job.lastAttemptAt') } }
function parseTimestamp(value: unknown, field: string): Date { try { return parseRequiredResponseTimestamp(value, { api: 'Jobs', field }) } catch (error) { if (error instanceof ResponseTimestampError) throw new JobsResponseError({ field, issue: 'is invalid' }); throw error } }
function parseOptionalTimestamp(value: unknown, field: string): Date | null | undefined { if (value === undefined || value === null) return value; return parseTimestamp(value, field) }
function buildSearchParams(params: ListJobsParams): URLSearchParams { const query = new URLSearchParams(); for (const status of params.status ?? []) if (status.trim()) query.append('status', status.trim()); for (const jobType of params.jobType ?? []) if (jobType.trim()) query.append('jobType', jobType.trim()); for (const source of params.source ?? []) if (source.trim()) query.append('source', source.trim()); if (typeof params.limit === 'number') query.set('limit', String(params.limit)); if (params.cursor?.trim()) query.set('cursor', params.cursor.trim()); return query }
function extractErrorMessage(response: Response, json: unknown): string { if (typeof json === 'object' && json !== null && 'message' in json && typeof json.message === 'string') return json.message; return `${response.status} ${response.statusText}`.trim() }
