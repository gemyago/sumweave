import { beforeEach, describe, expect, it, vi } from 'vitest'
import { faker } from '@faker-js/faker'
import { render, screen, waitFor, within } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import Data from './Data.svelte'

const chartSetData = vi.fn()

vi.mock('lightweight-charts', () => ({
  CandlestickSeries: Symbol('CandlestickSeries'),
  createChart: vi.fn(() => ({
    addSeries: () => ({ setData: chartSetData }),
    remove: vi.fn(),
  })),
}))

const mocks = vi.hoisted(() => ({
  listCandles: vi.fn(),
  listRawPayloads: vi.fn(),
  getRawPayloadDetail: vi.fn(),
  listCandleRawPayloads: vi.fn(),
}))

vi.mock('../lib/data/data-api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/data/data-api')>()
  return {
    ...actual,
    createSignalDataApiForAuth: vi.fn(() => ({
      listCandles: mocks.listCandles,
      listRawPayloads: mocks.listRawPayloads,
      getRawPayloadDetail: mocks.getRawPayloadDetail,
      listCandleRawPayloads: mocks.listCandleRawPayloads,
    })),
  }
})

vi.mock('../lib/auth/auth-store.svelte', () => ({
  authStore: { accessToken: faker.string.alphanumeric(32) },
}))

function createDeferred<T>() {
  let resolve: (value: T) => void = () => undefined
  let reject: (reason?: unknown) => void = () => undefined
  const promise = new Promise<T>((promiseResolve, promiseReject) => {
    resolve = promiseResolve
    reject = promiseReject
  })
  return { promise, resolve, reject }
}

function makeCandle() {
  const start = faker.date.recent()
  const end = faker.date.soon({ refDate: start })
  return {
    identity: faker.number.int({ min: 1, max: 9999 }),
    venue: 'hyperliquid-perps',
    symbol: 'BTCUSD',
    assetClass: 'crypto',
    timeframe: '1m',
    start,
    end,
    open: 100,
    high: 110,
    low: 95,
    close: 108,
    volume: 7,
    quality: 'validated',
    provenanceSource: faker.word.noun(),
    provenanceIdentity: faker.string.uuid(),
  }
}

function makeRawPayload() {
  const start = faker.date.recent()
  const end = faker.date.soon({ refDate: start })
  return {
    id: faker.string.uuid(),
    ingestionRunId: faker.string.uuid(),
    source: faker.word.noun(),
    venue: 'hyperliquid-perps',
    endpoint: `/${faker.word.noun()}`,
    requestType: faker.word.verb(),
    requestPayloadHash: faker.string.hexadecimal({ length: 16 }),
    requestAt: faker.date.recent(),
    responseAt: faker.date.recent(),
    httpStatus: faker.number.int({ min: 200, max: 299 }),
    responseBodyHash: faker.string.hexadecimal({ length: 16 }),
    payloadBodyRef: faker.system.filePath(),
    entityHint: faker.word.words(2),
    symbol: 'BTCUSD',
    assetClass: 'crypto',
    timeframe: '1m',
    start,
    end,
    receivedAt: faker.date.recent(),
  }
}

async function fillRequiredFilters(user: ReturnType<typeof userEvent.setup>) {
  await user.selectOptions(screen.getByLabelText('Venue'), 'hyperliquid-perps')
  await user.type(screen.getByLabelText('Symbol'), 'BTCUSD')
  await user.selectOptions(screen.getByLabelText('Asset class'), 'crypto')
  await user.selectOptions(screen.getByLabelText('Timeframe'), '1m')
  await user.type(screen.getByLabelText('UTC start'), '2026-06-15T12:00:00Z')
  await user.type(screen.getByLabelText('UTC end'), '2026-06-15T13:00:00Z')
}

function formatDateTime(value: Date): string {
  return value.toISOString()
}

describe('Data page', () => {
  beforeEach(() => {
    chartSetData.mockReset()
    mocks.listCandles.mockReset()
    mocks.listRawPayloads.mockReset()
    mocks.getRawPayloadDetail.mockReset()
    mocks.listCandleRawPayloads.mockReset()
  })

  it('renders the filter shell and does not auto-query on first render', () => {
    render(Data)

    expect(screen.getByRole('heading', { name: 'Historical data' })).toBeInTheDocument()
    expect(screen.getByRole('form', { name: 'Historical data filters' })).toBeInTheDocument()
    expect(mocks.listCandles).not.toHaveBeenCalled()
    expect(mocks.listRawPayloads).not.toHaveBeenCalled()
  })

  it('shows required-field validation and blocks API calls', async () => {
    const user = userEvent.setup()
    render(Data)

    await user.click(screen.getByRole('button', { name: 'Load' }))

    expect(screen.getByRole('alert')).toHaveTextContent('Venue is required.')
    expect(mocks.listCandles).not.toHaveBeenCalled()
    expect(mocks.listRawPayloads).not.toHaveBeenCalled()
  })

  it('shows the client-side 10,000-interval cap message before loading', async () => {
    const user = userEvent.setup()
    render(Data)

    await user.selectOptions(screen.getByLabelText('Venue'), 'hyperliquid-perps')
    await user.type(screen.getByLabelText('Symbol'), 'BTCUSD')
    await user.selectOptions(screen.getByLabelText('Asset class'), 'crypto')
    await user.selectOptions(screen.getByLabelText('Timeframe'), '1m')
    await user.type(screen.getByLabelText('UTC start'), '2026-06-15T12:00:00Z')
    await user.type(screen.getByLabelText('UTC end'), '2026-06-23T00:00:01Z')

    await user.click(screen.getByRole('button', { name: 'Load' }))

    expect(screen.getByRole('alert')).toHaveTextContent(
      'Selected range exceeds the server limit of 10,000 1m intervals.',
    )
    expect(mocks.listCandles).not.toHaveBeenCalled()
  })

  it('shows invalid UTC timestamp validation before loading', async () => {
    const user = userEvent.setup()
    render(Data)

    await user.selectOptions(screen.getByLabelText('Venue'), 'hyperliquid-perps')
    await user.type(screen.getByLabelText('Symbol'), 'BTCUSD')
    await user.selectOptions(screen.getByLabelText('Asset class'), 'crypto')
    await user.selectOptions(screen.getByLabelText('Timeframe'), '1m')
    await user.type(screen.getByLabelText('UTC start'), faker.date.recent().toISOString().replace('Z', ''))
    await user.type(screen.getByLabelText('UTC end'), '2026-06-15T13:00:00Z')

    await user.click(screen.getByRole('button', { name: 'Load' }))

    expect(screen.getByRole('alert')).toHaveTextContent(
      'UTC start must be a valid ISO-8601 timestamp.',
    )
    expect(mocks.listCandles).not.toHaveBeenCalled()
  })

  it('loads candles and raw payload metadata with expected params and renders tables', async () => {
    const user = userEvent.setup()
    const candle = makeCandle()
    const rawPayload = makeRawPayload()
    mocks.listCandles.mockResolvedValue({ items: [candle] })
    mocks.listRawPayloads.mockResolvedValue({ items: [rawPayload] })
    render(Data)

    await fillRequiredFilters(user)
    await user.type(screen.getByLabelText('Ingestion run ID'), rawPayload.ingestionRunId)
    await user.click(screen.getByRole('button', { name: 'Load' }))

    await waitFor(() => {
      expect(mocks.listCandles).toHaveBeenCalledWith(
        expect.objectContaining({
          venue: 'hyperliquid-perps',
          symbol: 'BTCUSD',
          assetClass: 'crypto',
          timeframe: '1m',
        }),
      )
      expect(mocks.listRawPayloads).toHaveBeenCalledWith(
        expect.objectContaining({ ingestionRunId: rawPayload.ingestionRunId }),
      )
    })
    expect(await screen.findByText('1 normalized candles')).toBeInTheDocument()
    expect(screen.getByText(rawPayload.id)).toBeInTheDocument()
    expect(chartSetData).toHaveBeenCalledWith(
      expect.arrayContaining([
        expect.objectContaining({ open: candle.open, high: candle.high, low: candle.low, close: candle.close }),
      ]),
    )
  })

  it('shows loading and empty states', async () => {
    const user = userEvent.setup()
    let resolveCandles: ((value: { items: ReturnType<typeof makeCandle>[] }) => void) | undefined
    let resolveRaw: ((value: { items: ReturnType<typeof makeRawPayload>[] }) => void) | undefined
    mocks.listCandles.mockReturnValue(new Promise((resolve) => { resolveCandles = resolve }))
    mocks.listRawPayloads.mockReturnValue(new Promise((resolve) => { resolveRaw = resolve }))
    render(Data)

    await fillRequiredFilters(user)
    await user.click(screen.getByRole('button', { name: 'Load' }))

    expect(screen.getByText('Loading normalized candles and raw payload metadata…')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Load' })).toBeDisabled()

    resolveCandles?.({ items: [] })
    resolveRaw?.({ items: [] })

    await waitFor(() => {
      expect(screen.getByText('No normalized candles matched these filters.')).toBeInTheDocument()
      expect(screen.getByText('No raw payload metadata matched these filters.')).toBeInTheDocument()
    })
  })

  it('shows alert semantics when loading fails', async () => {
    const user = userEvent.setup()
    mocks.listCandles.mockRejectedValue(new Error('Candle read failed'))
    mocks.listRawPayloads.mockResolvedValue({ items: [] })
    render(Data)

    await fillRequiredFilters(user)
    await user.click(screen.getByRole('button', { name: 'Load' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Candle read failed')
  })

  it('clears stale top-level results when a later load partially fails', async () => {
    const user = userEvent.setup()
    const firstCandle = makeCandle()
    const firstRawPayload = makeRawPayload()
    const secondRawPayload = makeRawPayload()

    mocks.listCandles
      .mockResolvedValueOnce({ items: [firstCandle] })
      .mockRejectedValueOnce(new Error('Candle read failed'))
    mocks.listRawPayloads
      .mockResolvedValueOnce({ items: [firstRawPayload] })
      .mockResolvedValueOnce({ items: [secondRawPayload] })
    render(Data)

    await fillRequiredFilters(user)
    await user.click(screen.getByRole('button', { name: 'Load' }))

    expect(await screen.findByText('1 normalized candles')).toBeInTheDocument()
    expect(screen.getByText(firstRawPayload.id)).toBeInTheDocument()

    await user.clear(screen.getByLabelText('UTC end'))
    await user.type(screen.getByLabelText('UTC end'), '2026-06-15T14:00:00Z')
    await user.click(screen.getByRole('button', { name: 'Load' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Candle read failed')
    expect(screen.getByText('0 normalized candles')).toBeInTheDocument()
    expect(screen.getByText('1 raw payload rows')).toBeInTheDocument()
    expect(screen.queryByText(formatDateTime(firstCandle.start))).not.toBeInTheDocument()
    expect(screen.queryByText(firstRawPayload.id)).not.toBeInTheDocument()
    expect(screen.getByText(secondRawPayload.id)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Select' })).not.toBeInTheDocument()
  })

  it('opens the raw payload detail drawer after fetching detail', async () => {
    const user = userEvent.setup()
    const candle = makeCandle()
    const rawPayload = makeRawPayload()
    mocks.listCandles.mockResolvedValue({ items: [candle] })
    mocks.listRawPayloads.mockResolvedValue({ items: [rawPayload] })
    mocks.getRawPayloadDetail.mockResolvedValue({
      metadata: rawPayload,
      responseBodySizeBytes: faker.number.int({ min: 100, max: 1000 }),
      responseBodyPreview: faker.lorem.paragraph(),
      responseBodyPreviewTruncated: true,
    })
    render(Data)

    await fillRequiredFilters(user)
    await user.click(screen.getByRole('button', { name: 'Load' }))
    await screen.findByText(rawPayload.id)

    await user.click(screen.getByRole('button', { name: 'View detail' }))

    await waitFor(() => {
      expect(mocks.getRawPayloadDetail).toHaveBeenCalledWith(rawPayload.id)
    })
    const dialog = await screen.findByRole('dialog', { name: 'Raw payload detail' })
    expect(within(dialog).getByText(rawPayload.payloadBodyRef)).toBeInTheDocument()
    expect(within(dialog).getByText('Yes')).toBeInTheDocument()
  })

  it('loads linked evidence with selected candle provenance fields', async () => {
    const user = userEvent.setup()
    const candle = makeCandle()
    const rawPayload = makeRawPayload()
    const linkedPayload = makeRawPayload()
    mocks.listCandles.mockResolvedValue({ items: [candle] })
    mocks.listRawPayloads.mockResolvedValue({ items: [rawPayload] })
    mocks.listCandleRawPayloads.mockResolvedValue({ items: [linkedPayload] })
    render(Data)

    await fillRequiredFilters(user)
    await user.click(screen.getByRole('button', { name: 'Load' }))
    await screen.findByText(rawPayload.id)

    await user.click(screen.getAllByRole('button', { name: 'Select' })[0])

    await waitFor(() => {
      expect(mocks.listCandleRawPayloads).toHaveBeenCalledWith(
        expect.objectContaining({
          provenanceSource: candle.provenanceSource,
          provenanceIdentity: candle.provenanceIdentity,
        }),
      )
    })
    expect(await screen.findByText(linkedPayload.id)).toBeInTheDocument()
  })

  it('shows empty linked evidence state for candles without payload links', async () => {
    const user = userEvent.setup()
    const candle = makeCandle()
    const rawPayload = makeRawPayload()
    mocks.listCandles.mockResolvedValue({ items: [candle] })
    mocks.listRawPayloads.mockResolvedValue({ items: [rawPayload] })
    mocks.listCandleRawPayloads.mockResolvedValue({ items: [] })
    render(Data)

    await fillRequiredFilters(user)
    await user.click(screen.getByRole('button', { name: 'Load' }))
    await screen.findByText(rawPayload.id)

    await user.click(screen.getAllByRole('button', { name: 'Select' })[0])

    expect(
      await screen.findByText('No linked raw evidence was found for this candle.'),
    ).toBeInTheDocument()
  })

  it('shows alert semantics when linked evidence loading fails', async () => {
    const user = userEvent.setup()
    const candle = makeCandle()
    const rawPayload = makeRawPayload()
    mocks.listCandles.mockResolvedValue({ items: [candle] })
    mocks.listRawPayloads.mockResolvedValue({ items: [rawPayload] })
    mocks.listCandleRawPayloads.mockRejectedValue(new Error('Linked evidence failed'))
    render(Data)

    await fillRequiredFilters(user)
    await user.click(screen.getByRole('button', { name: 'Load' }))
    await screen.findByText(rawPayload.id)

    await user.click(screen.getAllByRole('button', { name: 'Select' })[0])

    expect(await screen.findByRole('alert')).toHaveTextContent('Linked evidence failed')
  })

  it('ignores stale linked evidence responses when a newer candle selection wins', async () => {
    const user = userEvent.setup()
    const firstCandle = makeCandle()
    const secondCandle = makeCandle()
    const rawPayload = makeRawPayload()
    const firstLinkedPayload = makeRawPayload()
    const secondLinkedPayload = makeRawPayload()
    const firstLinkedEvidence = createDeferred<{ items: ReturnType<typeof makeRawPayload>[] }>()
    const secondLinkedEvidence = createDeferred<{ items: ReturnType<typeof makeRawPayload>[] }>()

    mocks.listCandles.mockResolvedValue({ items: [firstCandle, secondCandle] })
    mocks.listRawPayloads.mockResolvedValue({ items: [rawPayload] })
    mocks.listCandleRawPayloads
      .mockReturnValueOnce(firstLinkedEvidence.promise)
      .mockReturnValueOnce(secondLinkedEvidence.promise)
    render(Data)

    await fillRequiredFilters(user)
    await user.click(screen.getByRole('button', { name: 'Load' }))
    await screen.findByText(rawPayload.id)

    const selectButtons = screen.getAllByRole('button', { name: 'Select' })
    await user.click(selectButtons[0])
    await user.click(selectButtons[1])

    secondLinkedEvidence.resolve({ items: [secondLinkedPayload] })

    await waitFor(() => {
      expect(screen.getByText(secondLinkedPayload.id)).toBeInTheDocument()
    })

    firstLinkedEvidence.resolve({ items: [firstLinkedPayload] })

    await waitFor(() => {
      expect(screen.queryByText(firstLinkedPayload.id)).not.toBeInTheDocument()
      expect(screen.getByText(secondLinkedPayload.id)).toBeInTheDocument()
    })
  })

  it('shows detail drawer errors with alert semantics', async () => {
    const user = userEvent.setup()
    const candle = makeCandle()
    const rawPayload = makeRawPayload()
    mocks.listCandles.mockResolvedValue({ items: [candle] })
    mocks.listRawPayloads.mockResolvedValue({ items: [rawPayload] })
    mocks.getRawPayloadDetail.mockRejectedValue(new Error('Detail failed'))
    render(Data)

    await fillRequiredFilters(user)
    await user.click(screen.getByRole('button', { name: 'Load' }))
    await screen.findByText(rawPayload.id)

    await user.click(screen.getByRole('button', { name: 'View detail' }))

    const dialog = await screen.findByRole('dialog', { name: 'Raw payload detail' })
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('Detail failed')
  })

  it('ignores stale raw payload detail responses when a newer row selection wins', async () => {
    const user = userEvent.setup()
    const candle = makeCandle()
    const firstRawPayload = makeRawPayload()
    const secondRawPayload = makeRawPayload()
    const firstDetail = createDeferred<{
      metadata: ReturnType<typeof makeRawPayload>
      responseBodySizeBytes: number
      responseBodyPreview: string
      responseBodyPreviewTruncated: boolean
    }>()
    const secondDetail = createDeferred<{
      metadata: ReturnType<typeof makeRawPayload>
      responseBodySizeBytes: number
      responseBodyPreview: string
      responseBodyPreviewTruncated: boolean
    }>()
    const secondPreview = faker.lorem.paragraph()

    mocks.listCandles.mockResolvedValue({ items: [candle] })
    mocks.listRawPayloads.mockResolvedValue({ items: [firstRawPayload, secondRawPayload] })
    mocks.getRawPayloadDetail.mockReturnValueOnce(firstDetail.promise).mockReturnValueOnce(secondDetail.promise)
    render(Data)

    await fillRequiredFilters(user)
    await user.click(screen.getByRole('button', { name: 'Load' }))
    await screen.findByText(firstRawPayload.id)

    const detailButtons = screen.getAllByRole('button', { name: 'View detail' })
    await user.click(detailButtons[0])
    await user.click(detailButtons[1])

    secondDetail.resolve({
      metadata: secondRawPayload,
      responseBodySizeBytes: faker.number.int({ min: 100, max: 1000 }),
      responseBodyPreview: secondPreview,
      responseBodyPreviewTruncated: true,
    })

    const dialog = await screen.findByRole('dialog', { name: 'Raw payload detail' })
    await waitFor(() => {
      expect(within(dialog).getByText(secondRawPayload.payloadBodyRef)).toBeInTheDocument()
      expect(within(dialog).getByText(secondPreview)).toBeInTheDocument()
    })

    firstDetail.resolve({
      metadata: firstRawPayload,
      responseBodySizeBytes: faker.number.int({ min: 100, max: 1000 }),
      responseBodyPreview: faker.lorem.paragraph(),
      responseBodyPreviewTruncated: false,
    })

    await waitFor(() => {
      expect(within(dialog).queryByText(firstRawPayload.payloadBodyRef)).not.toBeInTheDocument()
      expect(within(dialog).getByText(secondRawPayload.payloadBodyRef)).toBeInTheDocument()
    })
  })
})
