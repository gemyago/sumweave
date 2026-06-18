import { beforeEach, describe, expect, it, vi } from 'vitest'
import { faker } from '@faker-js/faker'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import Evaluations from './Evaluations.svelte'
import EvaluationDetail from './EvaluationDetail.svelte'
import { formatCompactIdentifier } from '../lib/compact-identifier'
import type { EvaluationDetail as EvaluationDetailModel, EvaluationEvidence, EvaluationReport, EvaluationRow, StrategyVersionRow } from '../lib/strategy-workspace/api'

const mocks = vi.hoisted(() => ({
  listStrategies: vi.fn(),
  listEvaluationBacktests: vi.fn(),
  createEvaluationBacktest: vi.fn(),
  getEvaluationBacktest: vi.fn(),
  getEvaluationBacktestReport: vi.fn(),
  getEvaluationBacktestEvidence: vi.fn(),
}))

vi.mock('../lib/strategy-workspace/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/strategy-workspace/api')>()
  return {
    ...actual,
    createSignalStrategyWorkspaceApiForAuth: vi.fn(() => ({
      listStrategies: mocks.listStrategies,
      listEvaluationBacktests: mocks.listEvaluationBacktests,
      createEvaluationBacktest: mocks.createEvaluationBacktest,
      getEvaluationBacktest: mocks.getEvaluationBacktest,
      getEvaluationBacktestReport: mocks.getEvaluationBacktestReport,
      getEvaluationBacktestEvidence: mocks.getEvaluationBacktestEvidence,
      getStrategyVersion: vi.fn(),
      validateStrategy: vi.fn(),
      createStrategyVersion: vi.fn(),
      duplicateStrategyVersion: vi.fn(),
    })),
  }
})

vi.mock('../lib/auth/auth-store.svelte', () => ({
  authStore: { accessToken: faker.string.alphanumeric(32) },
}))

function makeStrategy(overrides?: Partial<StrategyVersionRow>): StrategyVersionRow {
  return {
    strategyId: faker.word.noun(),
    version: `v${faker.number.int({ min: 1, max: 9 })}`,
    displayName: faker.commerce.productName(),
    status: 'ready',
    sourceType: 'human',
    sourceLabel: 'Human',
    artifactHash: faker.string.hexadecimal({ length: 16 }),
    schemaVersion: 'strategy-artifact.v0',
    kind: 'moving-average-crossover',
    instrument: { venue: 'binance', symbol: 'BTCUSDT', assetClass: 'crypto', active: true },
    timeframe: '1h',
    parameterSummary: { fastWindow: 9, slowWindow: 21 },
    notes: faker.lorem.sentence(),
    createdAt: faker.date.recent(),
    updatedAt: faker.date.recent(),
    ...overrides,
  }
}

function makeEvaluationRow(overrides?: Partial<EvaluationRow>): EvaluationRow {
  return {
    runId: faker.string.uuid(),
    strategyId: faker.word.noun(),
    strategyVersion: `v${faker.number.int({ min: 1, max: 9 })}`,
    strategyArtifactHash: faker.string.hexadecimal({ length: 16 }),
    sourceType: 'human',
    sourceLabel: 'Human',
    instrument: { venue: 'binance', symbol: 'BTCUSDT', assetClass: 'crypto', active: true },
    timeframe: '1h',
    testedRangeStart: faker.date.recent(),
    testedRangeEnd: faker.date.soon(),
    status: 'completed',
    decision: 'needs_review',
    metrics: { tradeCount: 2, blockedGovernorDecisionCount: 1, rejectedGovernorDecisionCount: 0 },
    failureReason: '',
    failureDetails: '',
    createdAt: faker.date.recent(),
    updatedAt: faker.date.recent(),
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
    ...overrides,
  }
}

function makeEvaluationDetail(overrides?: Partial<EvaluationDetailModel>): EvaluationDetailModel {
  const row = makeEvaluationRow()
  return {
    ...row,
    strategySourceType: 'demo',
    strategySourceLabel: 'Demo example',
    datasetReference: {
      datasetId: faker.string.uuid(),
      replayChecksum: faker.string.hexadecimal({ length: 16 }),
      createdAt: faker.date.recent(),
    },
    policyReference: {
      policyId: 'default-paper-governor-policy',
      policyVersion: 'v0',
      policyHash: faker.string.hexadecimal({ length: 16 }),
    },
    traces: [
      {
        traceId: faker.string.uuid(),
        decisionTime: faker.date.recent(),
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
        createdTime: faker.date.recent(),
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
        eventTime: faker.date.recent(),
      },
    ],
    positionSnapshots: [],
    portfolioSnapshots: [],
    ...overrides,
  }
}

function makeEvaluationReport(overrides?: Partial<EvaluationReport>): EvaluationReport {
  const detail = makeEvaluationDetail()
  return {
    runId: detail.runId,
    status: detail.status,
    decision: detail.decision,
    failureReason: detail.failureReason,
    failureDetails: detail.failureDetails,
    metrics: detail.metrics,
    datasetReference: detail.datasetReference,
    policyReference: detail.policyReference,
    aiReadyMetadata: detail.aiReadyMetadata,
    ...overrides,
  }
}

function makeEvaluationEvidence(overrides?: Partial<EvaluationEvidence>): EvaluationEvidence {
  const detail = makeEvaluationDetail()
  return {
    runId: detail.runId,
    status: detail.status,
    aiReadyMetadata: detail.aiReadyMetadata,
    traces: detail.traces,
    orderIntents: detail.orderIntents,
    governorDecisions: detail.governorDecisions,
    executionRecords: detail.executionRecords,
    positionSnapshots: detail.positionSnapshots,
    portfolioSnapshots: detail.portfolioSnapshots,
    ...overrides,
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((innerResolve) => {
    resolve = innerResolve
  })

  return { promise, resolve }
}

function formatDate(value: Date): string {
  return value.toISOString().replace('T', ' ').slice(0, 16) + 'Z'
}

function hasExactTextContent(expected: string) {
  return (_content: string, element: Element | null) => element?.textContent === expected
}

async function fillUtcRange(
  _user: ReturnType<typeof userEvent.setup>,
  range: { startDate: string; startTime: string; endDate: string; endTime: string },
) {
  const startDate = screen.getByLabelText('UTC start date') as HTMLInputElement
  const startTime = screen.getByLabelText('UTC start time') as HTMLInputElement
  const endDate = screen.getByLabelText('UTC end date') as HTMLInputElement
  const endTime = screen.getByLabelText('UTC end time') as HTMLInputElement

  await fireEvent.input(startDate, { target: { value: range.startDate } })
  await fireEvent.change(startDate, { target: { value: range.startDate } })
  await fireEvent.blur(startDate)
  await fireEvent.input(startTime, { target: { value: range.startTime } })
  await fireEvent.change(startTime, { target: { value: range.startTime } })
  await fireEvent.input(endDate, { target: { value: range.endDate } })
  await fireEvent.change(endDate, { target: { value: range.endDate } })
  await fireEvent.blur(endDate)
  await fireEvent.input(endTime, { target: { value: range.endTime } })
  await fireEvent.change(endTime, { target: { value: range.endTime } })
}

describe('Evaluations pages', () => {
  beforeEach(() => {
    mocks.listStrategies.mockReset()
    mocks.listEvaluationBacktests.mockReset()
    mocks.createEvaluationBacktest.mockReset()
    mocks.getEvaluationBacktest.mockReset()
    mocks.getEvaluationBacktestReport.mockReset()
    mocks.getEvaluationBacktestEvidence.mockReset()
    mocks.listStrategies.mockResolvedValue([])
    mocks.listEvaluationBacktests.mockResolvedValue([])
  })

  it('loads ready strategy versions and applies history filters', async () => {
    const strategy = makeStrategy()
    mocks.listStrategies.mockResolvedValue([strategy])
    mocks.listEvaluationBacktests
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([makeEvaluationRow({ strategyId: strategy.strategyId, status: 'completed' })])

    render(Evaluations)
    const user = userEvent.setup()

    expect(await screen.findByRole('option', { name: new RegExp(strategy.displayName) })).toBeInTheDocument()

    await user.type(screen.getByLabelText('Strategy id'), strategy.strategyId)
    await user.selectOptions(screen.getByLabelText('Status'), 'completed')
    await user.click(screen.getByRole('button', { name: 'Apply filters' }))

    await waitFor(() => {
      expect(mocks.listEvaluationBacktests).toHaveBeenLastCalledWith({
        strategyId: strategy.strategyId,
        status: 'completed',
      })
    })
  })

  it('renders required evaluation history metadata fields from the api response', async () => {
    const createdAt = faker.date.recent()
    const updatedAt = faker.date.soon({ refDate: createdAt })
    const testedRangeStart = faker.date.recent({ refDate: createdAt })
    const testedRangeEnd = faker.date.soon({ refDate: updatedAt })
    const row = makeEvaluationRow({
      strategyArtifactHash: faker.string.hexadecimal({ length: 16 }),
      instrument: { venue: 'kraken', symbol: 'ETHUSD', assetClass: 'crypto', active: true },
      timeframe: '4h',
      testedRangeStart,
      testedRangeEnd,
      createdAt,
      updatedAt,
    })
    mocks.listEvaluationBacktests.mockResolvedValue([row])

    render(Evaluations)

    expect(await screen.findByText(formatCompactIdentifier(row.strategyArtifactHash))).toBeInTheDocument()
    expect(screen.getByText('kraken/ETHUSD/crypto')).toBeInTheDocument()
    expect(screen.getByText('4h')).toBeInTheDocument()
    expect(screen.getByText(`${formatDate(testedRangeStart)} → ${formatDate(testedRangeEnd)}`)).toBeInTheDocument()
    expect(screen.getByText(hasExactTextContent(`Created: ${formatDate(createdAt)}`))).toBeInTheDocument()
    expect(screen.getByText(hasExactTextContent(`Updated: ${formatDate(updatedAt)}`))).toBeInTheDocument()
  })

  it('validates strategy selection, utc range, and quantity before creating a run', async () => {
    render(Evaluations)
    const user = userEvent.setup()

    await screen.findByText(/No evaluation runs matched the current filters/)

    await fillUtcRange(user, {
      startDate: '2026-06-15',
      startTime: '11:00:00',
      endDate: '2026-06-15',
      endTime: '12:00:00',
    })
    await user.clear(screen.getByLabelText('Quantity'))
    await user.type(screen.getByLabelText('Quantity'), '1')
    await user.click(screen.getByRole('button', { name: 'Start evaluation' }))

    expect(await screen.findByText(/Select a ready strategy version/)).toBeInTheDocument()
    expect(mocks.createEvaluationBacktest).not.toHaveBeenCalled()
  })

  it('resolves evaluation presets once into explicit utc values before submission', async () => {
    const strategy = makeStrategy()
    mocks.listStrategies.mockResolvedValue([strategy])
    mocks.createEvaluationBacktest.mockResolvedValue(makeEvaluationDetail())

    render(Evaluations, { props: { params: { strategyId: strategy.strategyId, version: strategy.version } } })
    const user = userEvent.setup()

    await screen.findByRole('option', { name: new RegExp(strategy.displayName) })
    await user.click(screen.getByRole('button', { name: 'Last 24h' }))
    await user.click(screen.getByRole('button', { name: 'Start evaluation' }))

    await waitFor(() => {
      expect(mocks.createEvaluationBacktest).toHaveBeenCalledWith(
        expect.objectContaining({
          body: expect.objectContaining({
            start: expect.any(Date),
            end: expect.any(Date),
          }),
        }),
      )
    })

    const payload = mocks.createEvaluationBacktest.mock.calls[0]?.[0]
    expect(payload?.body.end.toISOString()).toMatch(/Z$/)
    expect(payload?.body.end.getTime() - payload?.body.start.getTime()).toBe(24 * 60 * 60 * 1000)
  })

  it('rejects evaluation runs when utc start is not earlier than utc end', async () => {
    const strategy = makeStrategy()
    mocks.listStrategies.mockResolvedValue([strategy])

    render(Evaluations, { props: { params: { strategyId: strategy.strategyId, version: strategy.version } } })
    const user = userEvent.setup()

    await screen.findByRole('option', { name: new RegExp(strategy.displayName) })
    await fillUtcRange(user, {
      startDate: '2026-06-15',
      startTime: '12:00:00',
      endDate: '2026-06-15',
      endTime: '12:00:00',
    })
    await user.click(screen.getByRole('button', { name: 'Start evaluation' }))

    expect((await screen.findAllByText('UTC start must be earlier than UTC end.')).length).toBeGreaterThan(0)
    expect(mocks.createEvaluationBacktest).not.toHaveBeenCalled()
  })

  it('shows synchronous run status after creating a completed or failed evaluation', async () => {
    const strategy = makeStrategy()
    const createdRun = makeEvaluationDetail({
      strategyId: strategy.strategyId,
      strategyVersion: strategy.version,
      status: 'failed',
      failureReason: 'replay-data-unavailable',
    })
    mocks.listStrategies.mockResolvedValue([strategy])
    mocks.createEvaluationBacktest.mockResolvedValue(createdRun)
    mocks.listEvaluationBacktests.mockResolvedValue([makeEvaluationRow({ runId: createdRun.runId, status: createdRun.status })])

    render(Evaluations, { props: { params: { strategyId: strategy.strategyId, version: strategy.version } } })
    const user = userEvent.setup()

    await screen.findByRole('option', { name: new RegExp(strategy.displayName) })
    await fillUtcRange(user, {
      startDate: '2026-06-15',
      startTime: '11:00:00',
      endDate: '2026-06-15',
      endTime: '12:00:00',
    })
    await user.click(screen.getByRole('button', { name: 'Start evaluation' }))

    await waitFor(() => {
      expect(screen.getAllByText(formatCompactIdentifier(createdRun.runId)).length).toBeGreaterThan(0)
      expect(screen.getByRole('status')).toHaveTextContent('status failed')
      expect(screen.getByRole('status')).toHaveTextContent('start a bounded historical backfill')
      expect(screen.getByRole('link', { name: 'Historical data' })).toHaveAttribute('href', '#/data')
      for (const link of screen.getAllByRole('link', { name: 'Open evaluation detail' })) {
        expect(link).toHaveAttribute('href', `#/evaluations/${encodeURIComponent(createdRun.runId)}`)
      }
    })
  })

  it('updates run preselection when the route strategy params change', async () => {
    const firstStrategy = makeStrategy({ strategyId: faker.word.noun(), version: 'v1', displayName: faker.commerce.productName() })
    const secondStrategy = makeStrategy({ strategyId: faker.word.noun(), version: 'v2', displayName: faker.commerce.productName() })
    mocks.listStrategies.mockResolvedValue([firstStrategy, secondStrategy])

    const view = render(Evaluations, {
      props: { params: { strategyId: firstStrategy.strategyId, version: firstStrategy.version } },
    })

    const select = await screen.findByLabelText('Strategy version')
    expect(select).toHaveValue(JSON.stringify([firstStrategy.strategyId, firstStrategy.version]))

    await view.rerender({ params: { strategyId: secondStrategy.strategyId, version: secondStrategy.version } })

    await waitFor(() => {
      expect(screen.getByLabelText('Strategy version')).toHaveValue(
        JSON.stringify([secondStrategy.strategyId, secondStrategy.version]),
      )
    })
  })

  it('submits the full strategy id and version when they contain slashes', async () => {
    const strategy = makeStrategy({
      strategyId: `${faker.word.noun()}/${faker.word.noun()}`,
      version: `v${faker.number.int({ min: 1, max: 9 })}/${faker.word.noun()}`,
    })
    mocks.listStrategies.mockResolvedValue([strategy])
    mocks.createEvaluationBacktest.mockResolvedValue(
      makeEvaluationDetail({ strategyId: strategy.strategyId, strategyVersion: strategy.version }),
    )

    render(Evaluations, { props: { params: { strategyId: strategy.strategyId, version: strategy.version } } })
    const user = userEvent.setup()

    await screen.findByRole('option', { name: new RegExp(strategy.displayName) })
    await fillUtcRange(user, {
      startDate: '2026-06-15',
      startTime: '11:00:00',
      endDate: '2026-06-15',
      endTime: '12:00:00',
    })
    await user.click(screen.getByRole('button', { name: 'Start evaluation' }))

    await waitFor(() => {
      expect(mocks.createEvaluationBacktest).toHaveBeenCalledWith(
        expect.objectContaining({
          body: expect.objectContaining({
            strategyId: strategy.strategyId,
            strategyVersion: strategy.version,
          }),
        }),
      )
    })
  })

  it('renders strategy, run, and history api errors', async () => {
    const strategy = makeStrategy()
    mocks.listStrategies.mockRejectedValue(new Error('strategies failed'))
    mocks.listEvaluationBacktests.mockRejectedValue(new Error('history failed'))
    mocks.createEvaluationBacktest.mockRejectedValue(new Error('run failed'))

    render(Evaluations)
    expect(await screen.findByText('strategies failed')).toBeInTheDocument()
    expect(screen.getByText('history failed')).toBeInTheDocument()

    cleanup()

    mocks.listStrategies.mockResolvedValueOnce([strategy])
    mocks.listEvaluationBacktests.mockResolvedValueOnce([])

    render(Evaluations, { props: { params: { strategyId: strategy.strategyId, version: strategy.version } } })
    const user = userEvent.setup()
    await screen.findByRole('option', { name: new RegExp(strategy.displayName) })
    await fillUtcRange(user, {
      startDate: '2026-06-15',
      startTime: '11:00:00',
      endDate: '2026-06-15',
      endTime: '12:00:00',
    })
    await user.click(screen.getByRole('button', { name: 'Start evaluation' }))

    expect(await screen.findByText('run failed')).toBeInTheDocument()
  })

  it('renders evaluation detail summary and evidence tables', async () => {
    const detail = makeEvaluationDetail()
    const report = makeEvaluationReport({ runId: detail.runId, policyReference: detail.policyReference })
    const evidence = makeEvaluationEvidence({ runId: detail.runId })
    mocks.getEvaluationBacktest.mockResolvedValue(detail)
    mocks.getEvaluationBacktestReport.mockResolvedValue(report)
    mocks.getEvaluationBacktestEvidence.mockResolvedValue(evidence)

    render(EvaluationDetail, { props: { params: { runId: detail.runId } } })

    expect(await screen.findByText(formatCompactIdentifier(detail.strategyArtifactHash))).toBeInTheDocument()
    expect(screen.getByText(formatCompactIdentifier(report.policyReference.policyHash))).toBeInTheDocument()
    expect(screen.getByText(detail.traces[0]!.result)).toBeInTheDocument()
    expect(screen.getByText(detail.orderIntents[0]!.actionKind)).toBeInTheDocument()
    expect(screen.getByText(detail.governorDecisions[0]!.reason)).toBeInTheDocument()
    expect(screen.getByText(detail.executionRecords[0]!.status)).toBeInTheDocument()
  })

  it('shows empty evidence state and api errors in evaluation detail', async () => {
    const detail = makeEvaluationDetail({
      traces: [],
      orderIntents: [],
      governorDecisions: [],
      executionRecords: [],
      failureReason: 'replay-data-unavailable',
      failureDetails: faker.lorem.sentence(),
    })
    const report = makeEvaluationReport({ runId: detail.runId, failureReason: 'replay-data-unavailable', failureDetails: faker.lorem.sentence() })
    const evidence = makeEvaluationEvidence({ runId: detail.runId, traces: [], orderIntents: [], governorDecisions: [], executionRecords: [] })
    mocks.getEvaluationBacktest.mockResolvedValue(detail)
    mocks.getEvaluationBacktestReport.mockResolvedValue(report)
    mocks.getEvaluationBacktestEvidence.mockResolvedValue(evidence)

    render(EvaluationDetail, { props: { params: { runId: detail.runId } } })

    expect(await screen.findByText(/No traces were persisted/)).toBeInTheDocument()
    expect(screen.getByText(/No order intents were persisted/)).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent('replay-data-unavailable')

    mocks.getEvaluationBacktest.mockRejectedValueOnce(new Error('boom'))
    mocks.getEvaluationBacktestReport.mockRejectedValueOnce(new Error('boom'))
    mocks.getEvaluationBacktestEvidence.mockRejectedValueOnce(new Error('boom'))

    render(EvaluationDetail, { props: { params: { runId: faker.string.uuid() } } })
    expect(await screen.findAllByRole('alert')).not.toHaveLength(0)
  })

  it('requires a run id before loading evaluation detail', async () => {
    render(EvaluationDetail)
    expect(await screen.findByRole('alert')).toHaveTextContent('Evaluation run id is required.')
  })

  it('reloads evaluation detail when the route run id changes', async () => {
    const firstDetail = makeEvaluationDetail()
    const secondDetail = makeEvaluationDetail()
    const firstReport = makeEvaluationReport({ runId: firstDetail.runId, policyReference: firstDetail.policyReference })
    const secondReport = makeEvaluationReport({ runId: secondDetail.runId, policyReference: secondDetail.policyReference })
    const firstEvidence = makeEvaluationEvidence({ runId: firstDetail.runId })
    const secondEvidence = makeEvaluationEvidence({ runId: secondDetail.runId })

    mocks.getEvaluationBacktest.mockResolvedValueOnce(firstDetail).mockResolvedValueOnce(secondDetail)
    mocks.getEvaluationBacktestReport.mockResolvedValueOnce(firstReport).mockResolvedValueOnce(secondReport)
    mocks.getEvaluationBacktestEvidence.mockResolvedValueOnce(firstEvidence).mockResolvedValueOnce(secondEvidence)

    const view = render(EvaluationDetail, { props: { params: { runId: firstDetail.runId } } })

    expect(await screen.findByText(formatCompactIdentifier(firstDetail.strategyArtifactHash))).toBeInTheDocument()

    await view.rerender({ params: { runId: secondDetail.runId } })

    await waitFor(() => {
      expect(screen.getByText(formatCompactIdentifier(secondDetail.strategyArtifactHash))).toBeInTheDocument()
    })

    expect(mocks.getEvaluationBacktest).toHaveBeenNthCalledWith(1, { runId: firstDetail.runId })
    expect(mocks.getEvaluationBacktest).toHaveBeenNthCalledWith(2, { runId: secondDetail.runId })
  })

  it('ignores stale evaluation detail responses after navigating to a newer run', async () => {
    const firstDetail = makeEvaluationDetail()
    const secondDetail = makeEvaluationDetail()
    const firstReport = makeEvaluationReport({ runId: firstDetail.runId, policyReference: firstDetail.policyReference })
    const secondReport = makeEvaluationReport({ runId: secondDetail.runId, policyReference: secondDetail.policyReference })
    const firstEvidence = makeEvaluationEvidence({ runId: firstDetail.runId })
    const secondEvidence = makeEvaluationEvidence({ runId: secondDetail.runId })

    const firstDetailRequest = deferred<EvaluationDetailModel>()
    const firstReportRequest = deferred<EvaluationReport>()
    const firstEvidenceRequest = deferred<EvaluationEvidence>()

    mocks.getEvaluationBacktest.mockImplementationOnce(() => firstDetailRequest.promise).mockResolvedValueOnce(secondDetail)
    mocks.getEvaluationBacktestReport.mockImplementationOnce(() => firstReportRequest.promise).mockResolvedValueOnce(secondReport)
    mocks.getEvaluationBacktestEvidence.mockImplementationOnce(() => firstEvidenceRequest.promise).mockResolvedValueOnce(secondEvidence)

    const view = render(EvaluationDetail, { props: { params: { runId: firstDetail.runId } } })
    await view.rerender({ params: { runId: secondDetail.runId } })

    expect(await screen.findByText(formatCompactIdentifier(secondDetail.strategyArtifactHash))).toBeInTheDocument()

    firstDetailRequest.resolve(firstDetail)
    firstReportRequest.resolve(firstReport)
    firstEvidenceRequest.resolve(firstEvidence)

    await waitFor(() => {
      expect(screen.queryByText(formatCompactIdentifier(firstDetail.strategyArtifactHash))).not.toBeInTheDocument()
    })
  })
})
