import type { AuthStore } from '../auth/auth-store.svelte'
import { createAuthFetch } from '../auth/auth-fetch'

export interface StrategyDefinitionInstrument {
  venue: string
  symbol: string
  assetClass: string
  active: boolean
}

export interface StrategyParameterSummary {
  fastWindow: number
  slowWindow: number
}

export interface StrategyDefinition {
  kind: string
  instrument: StrategyDefinitionInstrument
  timeframe: string
  parameters: StrategyParameterSummary
}

export interface StrategyFieldError {
  path: string
  message: string
}

export interface StrategyValidationPreview {
  schemaVersion: string
  kind: string
  instrument: StrategyDefinitionInstrument
  timeframe: string
  parameterSummary: StrategyParameterSummary
  canonicalJson: string
  artifactHash: string
  existingArtifact: boolean
}

export interface StrategyValidationResponse {
  valid: boolean
  preview: StrategyValidationPreview | null
  errors: StrategyFieldError[]
}

export interface StrategyVersionRow {
  strategyId: string
  version: string
  displayName: string
  status: string
  sourceType: string
  sourceLabel: string
  artifactHash: string
  schemaVersion: string
  kind: string
  instrument: StrategyDefinitionInstrument
  timeframe: string
  parameterSummary: StrategyParameterSummary
  notes: string
  createdAt: Date
  updatedAt: Date
}

export interface StrategyVersionDetail extends StrategyVersionRow {
  definition: StrategyDefinition
  parentStrategyId: string
  parentVersion: string
}

export interface StrategyVersionCandidate {
  strategyId: string
  version: string
  displayName: string
  status: string
  sourceType: string
  sourceLabel: string
  notes: string
  parentStrategyId: string
  parentVersion: string
  definition: StrategyDefinition
}

export interface CreateStrategyVersionRequest {
  strategyId: string
  version: string
  displayName: string
  notes?: string
  parentStrategyId?: string
  parentVersion?: string
  definition: StrategyDefinition
}

export interface EvaluationMetricSummary {
  tradeCount?: number
  blockedGovernorDecisionCount?: number
  rejectedGovernorDecisionCount?: number
  maxDrawdown?: number
}

export interface EvaluationEvidenceCounts {
  traces: number
  orderIntents: number
  governorDecisions: number
  executionRecords: number
  positionSnapshots: number
  portfolioSnapshots: number
}

export interface EvaluationAiReadyMetadata {
  requestSourceType: string
  strategySourceType: string
  strategySourceLabel: string
  note: string
  evidenceCounts: EvaluationEvidenceCounts
}

export interface EvaluationDatasetReference {
  datasetId: string
  replayChecksum: string
  createdAt: Date
}

export interface EvaluationPolicyReference {
  policyId: string
  policyVersion: string
  policyHash: string
}

export interface EvaluationRow {
  runId: string
  strategyId: string
  strategyVersion: string
  strategyArtifactHash: string
  sourceType: string
  sourceLabel: string
  instrument: StrategyDefinitionInstrument
  timeframe: string
  testedRangeStart: Date
  testedRangeEnd: Date
  status: string
  decision?: string
  metrics?: EvaluationMetricSummary
  failureReason: string
  failureDetails: string
  createdAt: Date
  updatedAt: Date
  aiReadyMetadata: EvaluationAiReadyMetadata
}

export interface EvaluationTraceRow {
  traceId: string
  decisionTime: Date
  result: string
  reasonCodes: string[]
  dataQuality: string
  runReference: string
}

export interface EvaluationOrderIntentRow {
  intentId: string
  traceId: string
  status: string
  actionKind: string
  requestedQuantity: number
  requestedNotional: number
  createdTime: Date
}

export interface EvaluationGovernorDecisionRow {
  decisionId: string
  intentId: string
  status: string
  reason: string
  reference: string
}

export interface EvaluationExecutionRow {
  commandId: string
  orderId: string
  fillId: string
  status: string
  eventTime?: Date
}

export interface EvaluationPositionSnapshotRow {
  snapshotId: string
  fillId: string
  quantity: number
  realizedPnl: number
  eventTime: Date
}

export interface EvaluationPortfolioSnapshotRow {
  snapshotId: string
  fillId: string
  grossExposure: number
  netExposure: number
  realizedPnl: number
  eventTime: Date
}

export interface EvaluationDetail extends EvaluationRow {
  strategySourceType: string
  strategySourceLabel: string
  datasetReference?: EvaluationDatasetReference
  policyReference: EvaluationPolicyReference
  traces: EvaluationTraceRow[]
  orderIntents: EvaluationOrderIntentRow[]
  governorDecisions: EvaluationGovernorDecisionRow[]
  executionRecords: EvaluationExecutionRow[]
  positionSnapshots: EvaluationPositionSnapshotRow[]
  portfolioSnapshots: EvaluationPortfolioSnapshotRow[]
}

export interface EvaluationReport {
  runId: string
  status: string
  decision?: string
  failureReason: string
  failureDetails: string
  metrics?: EvaluationMetricSummary
  datasetReference?: EvaluationDatasetReference
  policyReference: EvaluationPolicyReference
  aiReadyMetadata: EvaluationAiReadyMetadata
}

export interface EvaluationEvidence {
  runId: string
  status: string
  aiReadyMetadata: EvaluationAiReadyMetadata
  traces: EvaluationTraceRow[]
  orderIntents: EvaluationOrderIntentRow[]
  governorDecisions: EvaluationGovernorDecisionRow[]
  executionRecords: EvaluationExecutionRow[]
  positionSnapshots: EvaluationPositionSnapshotRow[]
  portfolioSnapshots: EvaluationPortfolioSnapshotRow[]
}

export interface CreateEvaluationBacktestRequest {
  strategyId: string
  strategyVersion: string
  start: Date
  end: Date
  quantity: number
  governorPolicyHash?: string
  note?: string
}

export interface ListEvaluationBacktestsParams {
  strategyId?: string
  status?: string
}

export interface SignalStrategyWorkspaceApi {
  listStrategies(): Promise<StrategyVersionRow[]>
  getStrategyVersion(params: { strategyId: string; version: string }): Promise<StrategyVersionDetail>
  validateStrategy(params: { definition: StrategyDefinition }): Promise<StrategyValidationResponse>
  createStrategyVersion(params: { body: CreateStrategyVersionRequest }): Promise<StrategyVersionDetail>
  duplicateStrategyVersion(params: {
    strategyId: string
    version: string
  }): Promise<StrategyVersionCandidate>
  createEvaluationBacktest(params: { body: CreateEvaluationBacktestRequest }): Promise<EvaluationDetail>
  listEvaluationBacktests(params: ListEvaluationBacktestsParams): Promise<EvaluationRow[]>
  getEvaluationBacktest(params: { runId: string }): Promise<EvaluationDetail>
  getEvaluationBacktestReport(params: { runId: string }): Promise<EvaluationReport>
  getEvaluationBacktestEvidence(params: { runId: string }): Promise<EvaluationEvidence>
}

type FetchLike = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>

export class StrategyWorkspaceApiError extends Error {
  readonly status: number
  readonly method: string
  readonly path: string

  constructor(params: { status: number; method: string; path: string; message: string }) {
    super(`Strategy workspace API ${params.method} ${params.path} failed: ${params.message}`)
    this.name = 'StrategyWorkspaceApiError'
    this.status = params.status
    this.method = params.method
    this.path = params.path
  }
}

export function createSignalStrategyWorkspaceApi(params: {
  baseUrl: string
  fetch: FetchLike
}): SignalStrategyWorkspaceApi {
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
      throw new StrategyWorkspaceApiError({
        status: response.status,
        method: requestParams.method,
        path: requestParams.path,
        message: extractErrorMessage(response, json),
      })
    }

    return json as T
  }

  return {
    async listStrategies() {
      const json = await request<{ items: RawStrategyVersionRow[] }>({
        method: 'GET',
        path: '/strategies',
      })
      return json.items.map(mapStrategyVersionRow)
    },

    async getStrategyVersion({ strategyId, version }) {
      const json = await request<RawStrategyVersionDetail>({
        method: 'GET',
        path: `/strategies/${encodeURIComponent(strategyId)}/versions/${encodeURIComponent(version)}`,
      })
      return mapStrategyVersionDetail(json)
    },

    async validateStrategy({ definition }) {
      const json = await request<RawStrategyValidationResponse>({
        method: 'POST',
        path: '/strategies/validate',
        body: { definition },
      })
      return {
        valid: json.valid,
        preview: json.preview ? mapStrategyValidationPreview(json.preview) : null,
        errors: (json.errors ?? []).map(mapStrategyFieldError),
      }
    },

    async createStrategyVersion({ body }) {
      const json = await request<RawStrategyVersionDetail>({
        method: 'POST',
        path: '/strategies/versions',
        body,
      })
      return mapStrategyVersionDetail(json)
    },

    async duplicateStrategyVersion({ strategyId, version }) {
      const json = await request<RawStrategyVersionCandidate>({
        method: 'POST',
        path: `/strategies/${encodeURIComponent(strategyId)}/versions/${encodeURIComponent(version)}/duplicate`,
      })
      return mapStrategyVersionCandidate(json)
    },

    async createEvaluationBacktest({ body }) {
      const json = await request<RawEvaluationDetail>({
        method: 'POST',
        path: '/evaluations/backtests',
        body,
      })
      return mapEvaluationDetail(json)
    },

    async listEvaluationBacktests(queryParams) {
      const json = await request<{ items: RawEvaluationRow[] }>({
        method: 'GET',
        path: '/evaluations/backtests',
        query: buildSearchParams(queryParams),
      })
      return json.items.map(mapEvaluationRow)
    },

    async getEvaluationBacktest({ runId }) {
      const json = await request<RawEvaluationDetail>({
        method: 'GET',
        path: `/evaluations/backtests/${encodeURIComponent(runId)}`,
      })
      return mapEvaluationDetail(json)
    },

    async getEvaluationBacktestReport({ runId }) {
      const json = await request<RawEvaluationReport>({
        method: 'GET',
        path: `/evaluations/backtests/${encodeURIComponent(runId)}/report`,
      })
      return mapEvaluationReport(json)
    },

    async getEvaluationBacktestEvidence({ runId }) {
      const json = await request<RawEvaluationEvidence>({
        method: 'GET',
        path: `/evaluations/backtests/${encodeURIComponent(runId)}/evidence`,
      })
      return mapEvaluationEvidence(json)
    },
  }
}

export function createSignalStrategyWorkspaceApiForAuth(params: {
  baseUrl: string
  authStore: AuthStore
}): SignalStrategyWorkspaceApi {
  return createSignalStrategyWorkspaceApi({
    baseUrl: params.baseUrl,
    fetch: createAuthFetch(params.authStore),
  })
}

interface RawStrategyValidationResponse {
  valid: boolean
  preview?: RawStrategyValidationPreview | null
  errors?: RawStrategyFieldError[]
}

interface RawStrategyFieldError {
  path: string
  message: string
}

interface RawStrategyValidationPreview {
  schemaVersion: string
  kind: string
  instrument: StrategyDefinitionInstrument
  timeframe: string
  parameterSummary: StrategyParameterSummary
  canonicalJson: string
  artifactHash: string
  existingArtifact: boolean
}

interface RawStrategyVersionRow {
  strategyId: string
  version: string
  displayName: string
  status: string
  sourceType: string
  sourceLabel: string
  artifactHash: string
  schemaVersion: string
  kind: string
  instrument: StrategyDefinitionInstrument
  timeframe: string
  parameterSummary: StrategyParameterSummary
  notes: string
  createdAt: string
  updatedAt: string
}

interface RawStrategyVersionDetail extends RawStrategyVersionRow {
  definition: StrategyDefinition
  parentStrategyId: string
  parentVersion: string
}

interface RawStrategyVersionCandidate {
  strategyId: string
  version: string
  displayName: string
  status: string
  sourceType: string
  sourceLabel: string
  notes: string
  parentStrategyId: string
  parentVersion: string
  definition: StrategyDefinition
}

interface RawEvaluationMetricSummary {
  tradeCount?: number
  blockedGovernorDecisionCount?: number
  rejectedGovernorDecisionCount?: number
  maxDrawdown?: number
}

interface RawEvaluationEvidenceCounts {
  traces: number
  orderIntents: number
  governorDecisions: number
  executionRecords: number
  positionSnapshots: number
  portfolioSnapshots: number
}

interface RawEvaluationAiReadyMetadata {
  requestSourceType: string
  strategySourceType: string
  strategySourceLabel: string
  note: string
  evidenceCounts: RawEvaluationEvidenceCounts
}

interface RawEvaluationDatasetReference {
  datasetId: string
  replayChecksum: string
  createdAt: string
}

interface RawEvaluationPolicyReference {
  policyId: string
  policyVersion: string
  policyHash: string
}

interface RawEvaluationRow {
  runId: string
  strategyId: string
  strategyVersion: string
  strategyArtifactHash: string
  sourceType: string
  sourceLabel: string
  instrument: StrategyDefinitionInstrument
  timeframe: string
  testedRangeStart: string
  testedRangeEnd: string
  status: string
  decision?: string
  metrics?: RawEvaluationMetricSummary
  failureReason: string
  failureDetails: string
  createdAt: string
  updatedAt: string
  aiReadyMetadata: RawEvaluationAiReadyMetadata
}

interface RawEvaluationTraceRow {
  traceId: string
  decisionTime: string
  result: string
  reasonCodes: string[]
  dataQuality: string
  runReference: string
}

interface RawEvaluationOrderIntentRow {
  intentId: string
  traceId: string
  status: string
  actionKind: string
  requestedQuantity: number
  requestedNotional: number
  createdTime: string
}

interface RawEvaluationGovernorDecisionRow {
  decisionId: string
  intentId: string
  status: string
  reason: string
  reference: string
}

interface RawEvaluationExecutionRow {
  commandId: string
  orderId: string
  fillId: string
  status: string
  eventTime?: string
}

interface RawEvaluationPositionSnapshotRow {
  snapshotId: string
  fillId: string
  quantity: number
  realizedPnl: number
  eventTime: string
}

interface RawEvaluationPortfolioSnapshotRow {
  snapshotId: string
  fillId: string
  grossExposure: number
  netExposure: number
  realizedPnl: number
  eventTime: string
}

interface RawEvaluationDetail extends RawEvaluationRow {
  strategySourceType: string
  strategySourceLabel: string
  datasetReference?: RawEvaluationDatasetReference
  policyReference: RawEvaluationPolicyReference
  traces: RawEvaluationTraceRow[]
  orderIntents: RawEvaluationOrderIntentRow[]
  governorDecisions: RawEvaluationGovernorDecisionRow[]
  executionRecords: RawEvaluationExecutionRow[]
  positionSnapshots: RawEvaluationPositionSnapshotRow[]
  portfolioSnapshots: RawEvaluationPortfolioSnapshotRow[]
}

interface RawEvaluationReport {
  runId: string
  status: string
  decision?: string
  failureReason: string
  failureDetails: string
  metrics?: RawEvaluationMetricSummary
  datasetReference?: RawEvaluationDatasetReference
  policyReference: RawEvaluationPolicyReference
  aiReadyMetadata: RawEvaluationAiReadyMetadata
}

interface RawEvaluationEvidence {
  runId: string
  status: string
  aiReadyMetadata: RawEvaluationAiReadyMetadata
  traces: RawEvaluationTraceRow[]
  orderIntents: RawEvaluationOrderIntentRow[]
  governorDecisions: RawEvaluationGovernorDecisionRow[]
  executionRecords: RawEvaluationExecutionRow[]
  positionSnapshots: RawEvaluationPositionSnapshotRow[]
  portfolioSnapshots: RawEvaluationPortfolioSnapshotRow[]
}

function buildSearchParams<T extends object>(params: T): URLSearchParams {
  const searchParams = new URLSearchParams()
  for (const [key, value] of Object.entries(params) as Array<[string, string | undefined]>) {
    if (value === undefined || value.trim() === '') {
      continue
    }
    searchParams.set(key, value)
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
  if (typeof value === 'object' && value !== null) {
    return Object.fromEntries(
      Object.entries(value).flatMap(([key, nested]) =>
        nested === undefined ? [] : [[key, serializeJson(nested)]],
      ),
    )
  }
  return value
}

function extractErrorMessage(response: Response, json: unknown): string {
  if (typeof json === 'object' && json !== null) {
    if ('message' in json && typeof json.message === 'string') {
      return json.message
    }
    if ('detail' in json && typeof json.detail === 'string') {
      return json.detail
    }
    if ('title' in json && typeof json.title === 'string') {
      return json.title
    }
  }
  return `${response.status} ${response.statusText}`.trim()
}

function mapStrategyFieldError(value: RawStrategyFieldError): StrategyFieldError {
  return { path: value.path, message: value.message }
}

function mapStrategyValidationPreview(value: RawStrategyValidationPreview): StrategyValidationPreview {
  return { ...value }
}

function mapStrategyVersionRow(value: RawStrategyVersionRow): StrategyVersionRow {
  return {
    ...value,
    createdAt: new Date(value.createdAt),
    updatedAt: new Date(value.updatedAt),
  }
}

function mapStrategyVersionDetail(value: RawStrategyVersionDetail): StrategyVersionDetail {
  return {
    ...mapStrategyVersionRow(value),
    definition: value.definition,
    parentStrategyId: value.parentStrategyId,
    parentVersion: value.parentVersion,
  }
}

function mapStrategyVersionCandidate(value: RawStrategyVersionCandidate): StrategyVersionCandidate {
  return { ...value }
}

function mapEvaluationMetricSummary(value?: RawEvaluationMetricSummary): EvaluationMetricSummary | undefined {
  return value ? { ...value } : undefined
}

function mapEvaluationAiReadyMetadata(value: RawEvaluationAiReadyMetadata): EvaluationAiReadyMetadata {
  return { ...value, evidenceCounts: { ...value.evidenceCounts } }
}

function mapEvaluationDatasetReference(
  value?: RawEvaluationDatasetReference,
): EvaluationDatasetReference | undefined {
  return value
    ? { ...value, createdAt: new Date(value.createdAt) }
    : undefined
}

function mapEvaluationPolicyReference(value: RawEvaluationPolicyReference): EvaluationPolicyReference {
  return { ...value }
}

function mapEvaluationRow(value: RawEvaluationRow): EvaluationRow {
  return {
    ...value,
    testedRangeStart: new Date(value.testedRangeStart),
    testedRangeEnd: new Date(value.testedRangeEnd),
    createdAt: new Date(value.createdAt),
    updatedAt: new Date(value.updatedAt),
    aiReadyMetadata: mapEvaluationAiReadyMetadata(value.aiReadyMetadata),
    metrics: mapEvaluationMetricSummary(value.metrics),
  }
}

function mapEvaluationTraceRow(value: RawEvaluationTraceRow): EvaluationTraceRow {
  return { ...value, decisionTime: new Date(value.decisionTime) }
}

function mapEvaluationOrderIntentRow(value: RawEvaluationOrderIntentRow): EvaluationOrderIntentRow {
  return { ...value, createdTime: new Date(value.createdTime) }
}

function mapEvaluationGovernorDecisionRow(
  value: RawEvaluationGovernorDecisionRow,
): EvaluationGovernorDecisionRow {
  return { ...value }
}

function mapEvaluationExecutionRow(value: RawEvaluationExecutionRow): EvaluationExecutionRow {
  return {
    commandId: value.commandId,
    orderId: value.orderId,
    fillId: value.fillId,
    status: value.status,
    ...(value.eventTime ? { eventTime: new Date(value.eventTime) } : {}),
  }
}

function mapEvaluationPositionSnapshotRow(
  value: RawEvaluationPositionSnapshotRow,
): EvaluationPositionSnapshotRow {
  return { ...value, eventTime: new Date(value.eventTime) }
}

function mapEvaluationPortfolioSnapshotRow(
  value: RawEvaluationPortfolioSnapshotRow,
): EvaluationPortfolioSnapshotRow {
  return { ...value, eventTime: new Date(value.eventTime) }
}

function mapEvaluationDetail(value: RawEvaluationDetail): EvaluationDetail {
  return {
    ...mapEvaluationRow(value),
    strategySourceType: value.strategySourceType,
    strategySourceLabel: value.strategySourceLabel,
    datasetReference: mapEvaluationDatasetReference(value.datasetReference),
    policyReference: mapEvaluationPolicyReference(value.policyReference),
    traces: value.traces.map(mapEvaluationTraceRow),
    orderIntents: value.orderIntents.map(mapEvaluationOrderIntentRow),
    governorDecisions: value.governorDecisions.map(mapEvaluationGovernorDecisionRow),
    executionRecords: value.executionRecords.map(mapEvaluationExecutionRow),
    positionSnapshots: value.positionSnapshots.map(mapEvaluationPositionSnapshotRow),
    portfolioSnapshots: value.portfolioSnapshots.map(mapEvaluationPortfolioSnapshotRow),
  }
}

function mapEvaluationReport(value: RawEvaluationReport): EvaluationReport {
  return {
    ...value,
    metrics: mapEvaluationMetricSummary(value.metrics),
    datasetReference: mapEvaluationDatasetReference(value.datasetReference),
    policyReference: mapEvaluationPolicyReference(value.policyReference),
    aiReadyMetadata: mapEvaluationAiReadyMetadata(value.aiReadyMetadata),
  }
}

function mapEvaluationEvidence(value: RawEvaluationEvidence): EvaluationEvidence {
  return {
    ...value,
    aiReadyMetadata: mapEvaluationAiReadyMetadata(value.aiReadyMetadata),
    traces: value.traces.map(mapEvaluationTraceRow),
    orderIntents: value.orderIntents.map(mapEvaluationOrderIntentRow),
    governorDecisions: value.governorDecisions.map(mapEvaluationGovernorDecisionRow),
    executionRecords: value.executionRecords.map(mapEvaluationExecutionRow),
    positionSnapshots: value.positionSnapshots.map(mapEvaluationPositionSnapshotRow),
    portfolioSnapshots: value.portfolioSnapshots.map(mapEvaluationPortfolioSnapshotRow),
  }
}
