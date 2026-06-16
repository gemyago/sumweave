import { beforeEach, describe, expect, it, vi } from 'vitest'
import { faker } from '@faker-js/faker'

vi.mock('../auth/auth-fetch', () => ({
  createAuthFetch: vi.fn(),
}))

import { createAuthFetch } from '../auth/auth-fetch'
import {
  StrategyWorkspaceApiError,
  createSignalStrategyWorkspaceApi,
  createSignalStrategyWorkspaceApiForAuth,
} from './api'

const mockCreateAuthFetch = vi.mocked(createAuthFetch)

function makeStrategyRowJson() {
  return {
    strategyId: faker.word.noun(),
    version: `v${faker.number.int({ min: 1, max: 9 })}`,
    displayName: faker.commerce.productName(),
    status: 'ready',
    sourceType: 'demo',
    sourceLabel: 'Demo example',
    artifactHash: faker.string.hexadecimal({ length: 16 }),
    schemaVersion: 'strategy-artifact.v0',
    kind: 'moving-average-crossover',
    instrument: {
      venue: 'binance',
      symbol: 'BTCUSDT',
      assetClass: 'crypto',
      active: true,
    },
    timeframe: '1h',
    parameterSummary: { fastWindow: 9, slowWindow: 21 },
    notes: faker.lorem.sentence(),
    createdAt: faker.date.recent().toISOString(),
    updatedAt: faker.date.recent().toISOString(),
  }
}

function makeEvaluationDetailJson() {
  const testedRangeStart = faker.date.recent()
  const testedRangeEnd = faker.date.soon({ refDate: testedRangeStart })
  const createdAt = faker.date.recent()
  const updatedAt = faker.date.soon({ refDate: createdAt })
  const evidenceTime = faker.date.recent()
  return {
    runId: faker.string.uuid(),
    strategyId: faker.word.noun(),
    strategyVersion: `v${faker.number.int({ min: 1, max: 9 })}`,
    strategyArtifactHash: faker.string.hexadecimal({ length: 16 }),
    sourceType: 'human',
    sourceLabel: 'Human',
    strategySourceType: 'demo',
    strategySourceLabel: 'Demo example',
    instrument: {
      venue: 'binance',
      symbol: 'BTCUSDT',
      assetClass: 'crypto',
      active: true,
    },
    timeframe: '1h',
    testedRangeStart: testedRangeStart.toISOString(),
    testedRangeEnd: testedRangeEnd.toISOString(),
    status: 'completed',
    decision: 'needs_review',
    failureReason: '',
    failureDetails: '',
    metrics: { tradeCount: 2, blockedGovernorDecisionCount: 1 },
    datasetReference: {
      datasetId: faker.string.uuid(),
      replayChecksum: faker.string.hexadecimal({ length: 16 }),
      createdAt: createdAt.toISOString(),
    },
    policyReference: {
      policyId: 'default-paper-governor-policy',
      policyVersion: 'v0',
      policyHash: faker.string.hexadecimal({ length: 16 }),
    },
    createdAt: createdAt.toISOString(),
    updatedAt: updatedAt.toISOString(),
    aiReadyMetadata: {
      requestSourceType: 'human',
      strategySourceType: 'demo',
      strategySourceLabel: 'Demo example',
      note: faker.lorem.sentence(),
      evidenceCounts: {
        traces: 1,
        orderIntents: 1,
        governorDecisions: 1,
        executionRecords: 1,
        positionSnapshots: 1,
        portfolioSnapshots: 1,
      },
    },
    traces: [
      {
        traceId: faker.string.uuid(),
        decisionTime: evidenceTime.toISOString(),
        result: 'intent_created',
        reasonCodes: ['CROSSOVER'],
        dataQuality: 'raw',
        runReference: faker.string.uuid(),
      },
    ],
    orderIntents: [
      {
        intentId: faker.string.uuid(),
        traceId: faker.string.uuid(),
        status: 'approved',
        actionKind: 'long',
        requestedQuantity: 1,
        requestedNotional: 100,
        createdTime: evidenceTime.toISOString(),
      },
    ],
    governorDecisions: [
      {
        decisionId: faker.string.uuid(),
        intentId: faker.string.uuid(),
        status: 'approved',
        reason: 'ok',
        reference: faker.string.uuid(),
      },
    ],
    executionRecords: [
      {
        commandId: faker.string.uuid(),
        orderId: faker.string.uuid(),
        fillId: faker.string.uuid(),
        status: 'filled',
        eventTime: evidenceTime.toISOString(),
      },
    ],
    positionSnapshots: [
      {
        snapshotId: faker.string.uuid(),
        fillId: faker.string.uuid(),
        quantity: 1,
        realizedPnl: 0,
        eventTime: evidenceTime.toISOString(),
      },
    ],
    portfolioSnapshots: [
      {
        snapshotId: faker.string.uuid(),
        fillId: faker.string.uuid(),
        grossExposure: 100,
        netExposure: 100,
        realizedPnl: 0,
        eventTime: evidenceTime.toISOString(),
      },
    ],
  }
}

describe('createSignalStrategyWorkspaceApi', () => {
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

    const api = createSignalStrategyWorkspaceApi({ baseUrl: '/api/v1', fetch: authFetch })
    await api.listStrategies()

    expect(authFetch).toHaveBeenCalledOnce()
    expect(globalFetch).not.toHaveBeenCalled()
  })

  it('serializes evaluation query params and strategy create bodies', async () => {
    const strategyRow = makeStrategyRowJson()
    const authFetch = vi
      .fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ items: [] }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify(strategyRow), { status: 200 }))
    const api = createSignalStrategyWorkspaceApi({ baseUrl: '/api/v1', fetch: authFetch })
    const start = faker.date.recent()
    const end = faker.date.soon({ refDate: start })

    await api.listEvaluationBacktests({ strategyId: strategyRow.strategyId, status: 'completed' })
    await api.createStrategyVersion({
      body: {
        strategyId: strategyRow.strategyId,
        version: strategyRow.version,
        displayName: strategyRow.displayName,
        notes: strategyRow.notes,
        definition: {
          kind: 'moving-average-crossover',
          instrument: strategyRow.instrument,
          timeframe: strategyRow.timeframe,
          parameters: strategyRow.parameterSummary,
        },
        parentStrategyId: strategyRow.strategyId,
        parentVersion: 'v0',
      },
    })

    const listUrl = new URL(authFetch.mock.calls[0][0] as string)
    expect(listUrl.pathname).toBe('/api/v1/evaluations/backtests')
    expect(listUrl.searchParams.get('strategyId')).toBe(strategyRow.strategyId)
    expect(listUrl.searchParams.get('status')).toBe('completed')

    const createRequestBody = JSON.parse(String(authFetch.mock.calls[1][1]?.body))
    expect(createRequestBody).toEqual(
      expect.objectContaining({
        strategyId: strategyRow.strategyId,
        version: strategyRow.version,
        parentStrategyId: strategyRow.strategyId,
        parentVersion: 'v0',
      }),
    )

    const runFetch = vi.fn().mockResolvedValue(new Response(JSON.stringify(makeEvaluationDetailJson()), { status: 200 }))
    const runApi = createSignalStrategyWorkspaceApi({ baseUrl: '/api/v1', fetch: runFetch })
    await runApi.createEvaluationBacktest({
      body: {
        strategyId: strategyRow.strategyId,
        strategyVersion: strategyRow.version,
        start,
        end,
        quantity: 1,
      },
    })

    expect(JSON.parse(String(runFetch.mock.calls[0][1]?.body))).toEqual(
      expect.objectContaining({
        start: start.toISOString(),
        end: end.toISOString(),
        quantity: 1,
      }),
    )
  })

  it('maps validation payloads and typed date fields', async () => {
    const strategyRow = makeStrategyRowJson()
    const evaluationDetail = makeEvaluationDetailJson()
    const authFetch = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            valid: false,
            errors: [
              {
                path: 'definition.parameters.fastWindow',
                message: 'must be less than slowWindow',
              },
            ],
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ items: [strategyRow] }), { status: 200 }),
      )
      .mockResolvedValueOnce(new Response(JSON.stringify(evaluationDetail), { status: 200 }))
    const api = createSignalStrategyWorkspaceApi({ baseUrl: '/api/v1', fetch: authFetch })

    const validation = await api.validateStrategy({
      definition: {
        kind: 'moving-average-crossover',
        instrument: strategyRow.instrument,
        timeframe: strategyRow.timeframe,
        parameters: strategyRow.parameterSummary,
      },
    })
    const strategies = await api.listStrategies()
    const detail = await api.getEvaluationBacktest({ runId: evaluationDetail.runId })

    expect(validation.valid).toBe(false)
    expect(validation.errors[0]?.path).toBe('definition.parameters.fastWindow')
    expect(strategies[0]?.createdAt).toBeInstanceOf(Date)
    expect(detail.testedRangeStart).toBeInstanceOf(Date)
    expect(detail.datasetReference?.createdAt).toBeInstanceOf(Date)
    expect(detail.traces[0]?.decisionTime).toBeInstanceOf(Date)
  })

  it('maps report and evidence responses with omitted optional fields', async () => {
    const runId = faker.string.uuid()
    const authFetch = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            runId,
            status: 'failed',
            failureReason: faker.word.words(2),
            failureDetails: faker.lorem.sentence(),
            policyReference: {
              policyId: 'default-paper-governor-policy',
              policyVersion: 'v0',
              policyHash: faker.string.hexadecimal({ length: 16 }),
            },
            aiReadyMetadata: {
              requestSourceType: 'human',
              strategySourceType: 'demo',
              strategySourceLabel: 'Demo example',
              note: '',
              evidenceCounts: {
                traces: 0,
                orderIntents: 0,
                governorDecisions: 0,
                executionRecords: 0,
                positionSnapshots: 0,
                portfolioSnapshots: 0,
              },
            },
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            runId,
            status: 'failed',
            aiReadyMetadata: {
              requestSourceType: 'human',
              strategySourceType: 'demo',
              strategySourceLabel: 'Demo example',
              note: '',
              evidenceCounts: {
                traces: 0,
                orderIntents: 0,
                governorDecisions: 0,
                executionRecords: 0,
                positionSnapshots: 0,
                portfolioSnapshots: 0,
              },
            },
            traces: [],
            orderIntents: [],
            governorDecisions: [],
            executionRecords: [
              {
                commandId: faker.string.uuid(),
                orderId: faker.string.uuid(),
                fillId: faker.string.uuid(),
                status: 'rejected',
              },
            ],
            positionSnapshots: [],
            portfolioSnapshots: [],
          }),
          { status: 200 },
        ),
      )
    const api = createSignalStrategyWorkspaceApi({ baseUrl: '/api/v1', fetch: authFetch })

    const report = await api.getEvaluationBacktestReport({ runId })
    const evidence = await api.getEvaluationBacktestEvidence({ runId })

    expect(report.metrics).toBeUndefined()
    expect(report.datasetReference).toBeUndefined()
    expect(evidence.executionRecords[0]?.eventTime).toBeUndefined()
  })

  it('throws a typed API error with parsed backend message', async () => {
    const authFetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ detail: 'strategy version not found' }), {
        status: 404,
        statusText: 'Not Found',
      }),
    )
    const api = createSignalStrategyWorkspaceApi({ baseUrl: '/api/v1', fetch: authFetch })

    await expect(
      api.getStrategyVersion({
        strategyId: faker.word.noun(),
        version: `v${faker.number.int({ min: 1, max: 9 })}`,
      }),
    ).rejects.toEqual(
      expect.objectContaining<Partial<StrategyWorkspaceApiError>>({
        name: 'StrategyWorkspaceApiError',
        status: 404,
        path: expect.stringMatching(/^\/strategies\//),
      }),
    )
  })

  it('falls back to title or status text for backend errors', async () => {
    const titleFetch = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ title: 'request rejected' }), {
        status: 409,
        statusText: 'Conflict',
      }),
    )
    const fallbackFetch = vi.fn().mockResolvedValue(
      new Response(null, { status: 502, statusText: 'Bad Gateway' }),
    )
    const titleApi = createSignalStrategyWorkspaceApi({ baseUrl: '/api/v1', fetch: titleFetch })
    const fallbackApi = createSignalStrategyWorkspaceApi({ baseUrl: '/api/v1', fetch: fallbackFetch })

    await expect(titleApi.listStrategies()).rejects.toThrow('request rejected')
    await expect(fallbackApi.listStrategies()).rejects.toThrow('502 Bad Gateway')
  })

  it('wires the auth-store helper through createAuthFetch', () => {
    const authFetch = vi.fn()
    mockCreateAuthFetch.mockReturnValue(authFetch)
    const authStore = { accessToken: faker.string.alphanumeric(32) } as never

    const api = createSignalStrategyWorkspaceApiForAuth({
      baseUrl: '/api/v1',
      authStore,
    })

    expect(mockCreateAuthFetch).toHaveBeenCalledWith(authStore)
    expect(api).toBeDefined()
  })
})
