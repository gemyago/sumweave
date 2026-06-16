import { beforeEach, describe, expect, it, vi } from 'vitest'
import { faker } from '@faker-js/faker'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import Strategies from './Strategies.svelte'
import { formatCompactIdentifier } from '../lib/compact-identifier'
import type {
  StrategyValidationResponse,
  StrategyVersionCandidate,
  StrategyVersionDetail,
  StrategyVersionRow,
} from '../lib/strategy-workspace/api'

const mocks = vi.hoisted(() => ({
  listStrategies: vi.fn(),
  getStrategyVersion: vi.fn(),
  validateStrategy: vi.fn(),
  createStrategyVersion: vi.fn(),
  duplicateStrategyVersion: vi.fn(),
  push: vi.fn(),
}))

vi.mock('../lib/strategy-workspace/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/strategy-workspace/api')>()
  return {
    ...actual,
    createSignalStrategyWorkspaceApiForAuth: vi.fn(() => ({
      listStrategies: mocks.listStrategies,
      getStrategyVersion: mocks.getStrategyVersion,
      validateStrategy: mocks.validateStrategy,
      createStrategyVersion: mocks.createStrategyVersion,
      duplicateStrategyVersion: mocks.duplicateStrategyVersion,
      listEvaluationBacktests: vi.fn(),
      createEvaluationBacktest: vi.fn(),
      getEvaluationBacktest: vi.fn(),
      getEvaluationBacktestReport: vi.fn(),
      getEvaluationBacktestEvidence: vi.fn(),
    })),
  }
})

vi.mock('../lib/auth/auth-store.svelte', () => ({
  authStore: { accessToken: faker.string.alphanumeric(32) },
}))

vi.mock('svelte-spa-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('svelte-spa-router')>()
  return { ...actual, push: mocks.push }
})

function makeStrategyRow(overrides?: Partial<StrategyVersionRow>): StrategyVersionRow {
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
    instrument: { venue: 'binance', symbol: 'BTCUSDT', assetClass: 'crypto', active: true },
    timeframe: '1h',
    parameterSummary: { fastWindow: 9, slowWindow: 21 },
    notes: faker.lorem.sentence(),
    createdAt: faker.date.recent(),
    updatedAt: faker.date.recent(),
    ...overrides,
  }
}

function makeStrategyDetail(overrides?: Partial<StrategyVersionDetail>): StrategyVersionDetail {
  const row = makeStrategyRow()
  return {
    ...row,
    definition: {
      kind: row.kind,
      instrument: row.instrument,
      timeframe: row.timeframe,
      parameters: row.parameterSummary,
    },
    parentStrategyId: '',
    parentVersion: '',
    ...overrides,
  }
}

function makeCandidate(overrides?: Partial<StrategyVersionCandidate>): StrategyVersionCandidate {
  return {
    strategyId: faker.word.noun(),
    version: '',
    displayName: faker.commerce.productName(),
    status: 'draft',
    sourceType: 'human',
    sourceLabel: 'Human',
    notes: faker.lorem.sentence(),
    parentStrategyId: faker.word.noun(),
    parentVersion: `v${faker.number.int({ min: 1, max: 9 })}`,
    definition: {
      kind: 'moving-average-crossover',
      instrument: { venue: 'binance', symbol: 'BTCUSDT', assetClass: 'crypto', active: true },
      timeframe: '1h',
      parameters: { fastWindow: 9, slowWindow: 21 },
    },
    ...overrides,
  }
}

describe('Strategies page', () => {
  beforeEach(() => {
    mocks.listStrategies.mockReset()
    mocks.getStrategyVersion.mockReset()
    mocks.validateStrategy.mockReset()
    mocks.createStrategyVersion.mockReset()
    mocks.duplicateStrategyVersion.mockReset()
    mocks.push.mockReset()
    mocks.listStrategies.mockResolvedValue([])
  })

  it('renders strategy list rows without any latest evaluation summary column', async () => {
    const ready = makeStrategyRow({ status: 'ready', sourceType: 'demo', sourceLabel: 'Demo example' })
    const archived = makeStrategyRow({ status: 'archived', sourceType: 'human', sourceLabel: 'Human' })
    mocks.listStrategies.mockResolvedValue([ready, archived])

    render(Strategies)

    expect(await screen.findByText(ready.displayName)).toBeInTheDocument()
    expect(screen.getByText('ready')).toBeInTheDocument()
    expect(screen.getByText('archived')).toBeInTheDocument()
    expect(screen.getByText(/Example only, not a recommendation/)).toBeInTheDocument()
    expect(screen.queryByText(/Latest evaluation/i)).not.toBeInTheDocument()
  })

  it('shows constrained moving-average fields and validation preview output', async () => {
    const preview: StrategyValidationResponse = {
      valid: true,
      errors: [],
      preview: {
        schemaVersion: 'strategy-artifact.v0',
        kind: 'moving-average-crossover',
        instrument: { venue: 'binance', symbol: 'BTCUSDT', assetClass: 'crypto', active: true },
        timeframe: '1h',
        parameterSummary: { fastWindow: 9, slowWindow: 21 },
        canonicalJson: '{"schemaVersion":"strategy-artifact.v0"}',
        artifactHash: faker.string.hexadecimal({ length: 16 }),
        existingArtifact: true,
      },
    }
    mocks.validateStrategy.mockResolvedValue(preview)

    render(Strategies)
    const user = userEvent.setup()

    expect(await screen.findByDisplayValue('moving-average-crossover')).toBeDisabled()
    expect(screen.getByLabelText('Fast window')).toHaveValue(9)
    expect(screen.getByLabelText('Slow window')).toHaveValue(21)

    await user.click(screen.getByRole('button', { name: 'Validate' }))

    await waitFor(() => {
      expect(screen.getByText(/Definition is valid/)).toBeInTheDocument()
      expect(screen.getByText(formatCompactIdentifier(preview.preview!.artifactHash))).toBeInTheDocument()
      expect(screen.getByText(preview.preview!.canonicalJson)).toBeInTheDocument()
    })
  })

  it('shows backend validation errors inline', async () => {
    mocks.validateStrategy.mockResolvedValue({
      valid: false,
      preview: null,
      errors: [{ path: 'definition.parameters.fastWindow', message: 'must be less than slowWindow' }],
    })

    render(Strategies)
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Validate' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('definition.parameters.fastWindow')
  })

  it('duplicates an immutable saved version into a local draft and saves a new ready version', async () => {
    const detail = makeStrategyDetail({ status: 'ready' })
    const candidate = makeCandidate({
      strategyId: detail.strategyId,
      parentStrategyId: detail.strategyId,
      parentVersion: detail.version,
    })
    const saved = makeStrategyDetail({
      strategyId: detail.strategyId,
      version: `v${faker.number.int({ min: 10, max: 99 })}`,
      parentStrategyId: detail.strategyId,
      parentVersion: detail.version,
    })
    mocks.listStrategies.mockResolvedValue([detail])
    mocks.getStrategyVersion.mockResolvedValue(detail)
    mocks.duplicateStrategyVersion.mockResolvedValue(candidate)
    mocks.createStrategyVersion.mockResolvedValue(saved)

    const view = render(Strategies, { props: { params: { strategyId: detail.strategyId, version: detail.version } } })
    const user = userEvent.setup()

    expect(await screen.findByRole('heading', { level: 1, name: detail.displayName })).toBeInTheDocument()
    expect(screen.getByText(/Saved versions are immutable/)).toBeInTheDocument()
    expect(screen.getAllByText(formatCompactIdentifier(detail.artifactHash)).length).toBeGreaterThan(0)
    expect(screen.getByRole('link', { name: 'Run evaluation' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Duplicate to draft' }))
    expect(mocks.push).toHaveBeenCalledWith('/strategies')

    await view.rerender({ params: {} })
    await user.type(screen.getByLabelText('Version'), `v${faker.number.int({ min: 100, max: 999 })}`)
    await user.click(screen.getByRole('button', { name: 'Save version' }))

    await waitFor(() => {
      expect(mocks.createStrategyVersion).toHaveBeenCalledWith(
        expect.objectContaining({
          body: expect.objectContaining({
            parentStrategyId: detail.strategyId,
            parentVersion: detail.version,
          }),
        }),
      )
      expect(mocks.push).toHaveBeenCalledWith(
        `/strategies/${encodeURIComponent(saved.strategyId)}/${encodeURIComponent(saved.version)}`,
      )
    })
  })

  it('hides the run evaluation action for archived versions', async () => {
    const detail = makeStrategyDetail({ status: 'archived' })
    mocks.listStrategies.mockResolvedValue([detail])
    mocks.getStrategyVersion.mockResolvedValue(detail)

    render(Strategies, { props: { params: { strategyId: detail.strategyId, version: detail.version } } })

    expect(await screen.findByRole('heading', { level: 1, name: detail.displayName })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Run evaluation' })).not.toBeInTheDocument()
  })

  it('shows list/detail/save errors and keeps the draft local', async () => {
    const listError = new Error('list failed')
    mocks.listStrategies.mockRejectedValueOnce(listError).mockResolvedValueOnce([])
    mocks.createStrategyVersion.mockRejectedValueOnce(new Error('save failed'))

    render(Strategies)
    const user = userEvent.setup()

    expect(await screen.findByText('list failed')).toBeInTheDocument()

    await user.type(screen.getByLabelText('Strategy ID'), faker.word.noun())
    await user.type(screen.getByLabelText('Version'), `v${faker.number.int({ min: 1, max: 9 })}`)
    await user.type(screen.getByLabelText('Display name'), faker.commerce.productName())
    await user.click(screen.getByRole('button', { name: 'Save version' }))

    expect(await screen.findByText('save failed')).toBeInTheDocument()
    expect(screen.getByText(/Draft fields stay local/)).toBeInTheDocument()
  })

  it('renders strategy detail load failures', async () => {
    const detail = makeStrategyDetail()
    mocks.listStrategies.mockResolvedValue([detail])
    mocks.getStrategyVersion.mockRejectedValue(new Error('detail failed'))

    render(Strategies, { props: { params: { strategyId: detail.strategyId, version: detail.version } } })

    expect(await screen.findByRole('alert')).toHaveTextContent('detail failed')
  })
})
