import { beforeEach, describe, expect, it, vi } from 'vitest'
import { faker } from '@faker-js/faker'
import { render, screen, waitFor, within } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import Data from './Data.svelte'
import { DataApiError } from '../lib/data/data-api'

const chartSetData = vi.fn()

vi.mock('lightweight-charts', () => ({
  CandlestickSeries: Symbol('CandlestickSeries'),
  createChart: vi.fn(() => ({
    addSeries: () => ({ setData: chartSetData }),
    remove: vi.fn(),
  })),
}))

const mocks = vi.hoisted(() => ({
  listCandleAvailability: vi.fn(),
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
      listCandleAvailability: mocks.listCandleAvailability,
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

function makeAvailabilityItem(overrides: Partial<ReturnType<typeof baseAvailabilityItem>> = {}) {
  return { ...baseAvailabilityItem(), ...overrides }
}

function baseAvailabilityItem() {
  const latestStart = faker.date.recent()
  const latestEnd = faker.date.soon({ refDate: latestStart })
  const earlierStart = faker.date.past({ years: 1, refDate: latestStart })
  const earlierEnd = faker.date.soon({ refDate: earlierStart })

  return {
    venue: 'hyperliquid-perps',
    symbol: `${faker.finance.currencyCode()}USD`,
    assetClass: 'crypto',
    timeframes: [
      {
        timeframe: '1m',
        start: earlierStart,
        end: earlierEnd,
        count: faker.number.int({ min: 1, max: 500 }),
      },
      {
        timeframe: '5m',
        start: latestStart,
        end: latestEnd,
        count: faker.number.int({ min: 1, max: 500 }),
      },
    ],
    defaultSlice: {
      timeframe: '5m',
      start: latestStart,
      end: latestEnd,
    },
  }
}

function makeCandle(overrides: Partial<ReturnType<typeof baseCandle>> = {}) {
  return { ...baseCandle(), ...overrides }
}

function baseCandle() {
  const start = faker.date.recent()
  const end = faker.date.soon({ refDate: start })
  return {
    identity: faker.number.int({ min: 1, max: 9999 }),
    venue: 'hyperliquid-perps',
    symbol: `${faker.finance.currencyCode()}USD`,
    assetClass: 'crypto',
    timeframe: '1m',
    start,
    end,
    open: faker.number.float({ min: 1, max: 1000 }),
    high: faker.number.float({ min: 1, max: 1000 }),
    low: faker.number.float({ min: 1, max: 1000 }),
    close: faker.number.float({ min: 1, max: 1000 }),
    volume: faker.number.float({ min: 1, max: 1000 }),
    quality: 'validated',
    provenanceSource: faker.word.noun(),
    provenanceIdentity: faker.string.uuid(),
  }
}

function makeRawPayload(overrides: Partial<ReturnType<typeof baseRawPayload>> = {}) {
  return { ...baseRawPayload(), ...overrides }
}

function baseRawPayload() {
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
    symbol: `${faker.finance.currencyCode()}USD`,
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

function mockAvailabilityResponse(items: ReturnType<typeof makeAvailabilityItem>[]) {
  const first = items[0]
  mocks.listCandleAvailability.mockResolvedValue({
    items,
    ...(first
      ? {
          defaultSelection: {
            venue: first.venue,
            symbol: first.symbol,
            assetClass: first.assetClass,
            timeframe: first.defaultSlice.timeframe,
            start: first.defaultSlice.start,
            end: first.defaultSlice.end,
          },
        }
      : {}),
  })
}

describe('Data page', () => {
  beforeEach(() => {
    chartSetData.mockReset()
    mocks.listCandleAvailability.mockReset()
    mocks.listCandles.mockReset()
    mocks.listRawPayloads.mockReset()
    mocks.getRawPayloadDetail.mockReset()
    mocks.listCandleRawPayloads.mockReset()
    mocks.listCandleAvailability.mockResolvedValue({ items: [] })
    mocks.listCandles.mockResolvedValue({ items: [] })
    mocks.listRawPayloads.mockResolvedValue({ items: [] })
    mocks.listCandleRawPayloads.mockResolvedValue({ items: [] })
  })

  it('loads the first availability page, auto-loads the default slice, default-selects a candle, and skips broad raw browsing', async () => {
    const availability = makeAvailabilityItem()
    const candle = makeCandle({
      venue: availability.venue,
      symbol: availability.symbol,
      assetClass: availability.assetClass,
      timeframe: availability.defaultSlice.timeframe,
      start: availability.defaultSlice.start,
      end: availability.defaultSlice.end,
    })
    const linkedPayload = makeRawPayload()
    mockAvailabilityResponse([availability])
    mocks.listCandles.mockResolvedValue({ items: [candle] })
    mocks.listCandleRawPayloads.mockResolvedValue({ items: [linkedPayload] })

    render(Data)

    expect(await screen.findByText(availability.symbol)).toBeInTheDocument()
    await waitFor(() => {
      expect(mocks.listCandleAvailability).toHaveBeenCalledWith({})
      expect(mocks.listCandles).toHaveBeenCalledWith({
        venue: availability.venue,
        symbol: availability.symbol,
        assetClass: availability.assetClass,
        timeframe: availability.defaultSlice.timeframe,
        start: availability.defaultSlice.start,
        end: availability.defaultSlice.end,
      })
      expect(mocks.listCandleRawPayloads).toHaveBeenCalledWith(
        expect.objectContaining({
          provenanceSource: candle.provenanceSource,
          provenanceIdentity: candle.provenanceIdentity,
        }),
      )
    })
    expect(screen.getByLabelText('Venue')).toHaveValue(availability.venue)
    expect(await screen.findByText(linkedPayload.id)).toBeInTheDocument()
    expect(screen.getByText('1 normalized candles')).toBeInTheDocument()
    expect(screen.getByText('Raw payload metadata not loaded yet')).toBeInTheDocument()
    expect(screen.getByText(`${availability.timeframes[1].count} candles`)).toBeInTheDocument()
    expect(mocks.listRawPayloads).not.toHaveBeenCalled()
    expect(chartSetData).toHaveBeenCalled()
  })

  it('renders availability immediately while the default candle slice is still loading', async () => {
    const availability = makeAvailabilityItem()
    const deferredCandles = createDeferred<{ items: ReturnType<typeof makeCandle>[] }>()
    mockAvailabilityResponse([availability])
    mocks.listCandles.mockReturnValue(deferredCandles.promise)

    render(Data)

    expect(await screen.findByText(availability.symbol)).toBeInTheDocument()
    expect(screen.getByText(`${availability.timeframes[0].count} candles`)).toBeInTheDocument()
    expect(screen.getByText('Loading normalized candles…')).toBeInTheDocument()
    expect(screen.getByLabelText('Venue')).toHaveValue(availability.venue)
  })

  it('selecting a different availability entry uses that entry default slice', async () => {
    const firstAvailability = makeAvailabilityItem()
    const secondAvailability = makeAvailabilityItem({ symbol: `${faker.finance.currencyCode()}USD` })
    mockAvailabilityResponse([firstAvailability, secondAvailability])
    mocks.listCandles.mockResolvedValue({ items: [] })

    const user = userEvent.setup()
    render(Data)

    const secondCard = await screen.findByRole('button', {
      name: new RegExp(`${secondAvailability.venue}.*${secondAvailability.symbol}.*${secondAvailability.assetClass}`),
    })
    await user.click(secondCard)

    await waitFor(() => {
      expect(mocks.listCandles).toHaveBeenLastCalledWith({
        venue: secondAvailability.venue,
        symbol: secondAvailability.symbol,
        assetClass: secondAvailability.assetClass,
        timeframe: secondAvailability.defaultSlice.timeframe,
        start: secondAvailability.defaultSlice.start,
        end: secondAvailability.defaultSlice.end,
      })
    })
    expect(screen.getByLabelText('Symbol')).toHaveValue(secondAvailability.symbol)
  })

  it('ignores stale candle responses when a newer availability selection wins', async () => {
    const firstAvailability = makeAvailabilityItem()
    const secondAvailability = makeAvailabilityItem({ symbol: `${faker.finance.currencyCode()}USD` })
    const firstCandles = createDeferred<{ items: ReturnType<typeof makeCandle>[] }>()
    const firstCandle = makeCandle({
      symbol: firstAvailability.symbol,
      start: faker.date.past(),
      end: faker.date.recent(),
    })
    const secondCandle = makeCandle({
      symbol: secondAvailability.symbol,
      start: faker.date.recent(),
      end: faker.date.soon(),
      provenanceIdentity: faker.string.uuid(),
    })
    mockAvailabilityResponse([firstAvailability, secondAvailability])
    mocks.listCandles
      .mockReturnValueOnce(firstCandles.promise)
      .mockResolvedValueOnce({ items: [secondCandle] })
    mocks.listCandleRawPayloads.mockResolvedValue({ items: [] })

    const user = userEvent.setup()
    render(Data)

    const secondCard = await screen.findByRole('button', {
      name: new RegExp(`${secondAvailability.venue}.*${secondAvailability.symbol}.*${secondAvailability.assetClass}`),
    })
    await user.click(secondCard)

    expect(await screen.findByText(secondCandle.start.toISOString())).toBeInTheDocument()
    expect(screen.getByLabelText('Symbol')).toHaveValue(secondAvailability.symbol)

    firstCandles.resolve({ items: [firstCandle] })

    await waitFor(() => {
      expect(screen.getByLabelText('Symbol')).toHaveValue(secondAvailability.symbol)
      expect(screen.queryByText(firstCandle.start.toISOString())).not.toBeInTheDocument()
      expect(mocks.listCandleRawPayloads).toHaveBeenCalledTimes(1)
      expect(mocks.listCandleRawPayloads).toHaveBeenCalledWith(
        expect.objectContaining({
          provenanceSource: secondCandle.provenanceSource,
          provenanceIdentity: secondCandle.provenanceIdentity,
        }),
      )
    })
  })

  it('shows required-field validation and blocks manual candle loads', async () => {
    const user = userEvent.setup()
    render(Data)
    await screen.findByText('No normalized candle availability was found yet.')

    await user.click(screen.getByRole('button', { name: 'Load candles' }))

    expect(screen.getByRole('alert')).toHaveTextContent('Venue is required.')
    expect(mocks.listCandles).not.toHaveBeenCalled()
  })

  it('manual filter editing still uses the explicit exact candle read path', async () => {
    const user = userEvent.setup()
    render(Data)
    await screen.findByText('No normalized candle availability was found yet.')

    await fillRequiredFilters(user)
    await user.click(screen.getByRole('button', { name: 'Load candles' }))

    await waitFor(() => {
      expect(mocks.listCandles).toHaveBeenCalledWith(
        expect.objectContaining({
          venue: 'hyperliquid-perps',
          symbol: 'BTCUSD',
          assetClass: 'crypto',
          timeframe: '1m',
        }),
      )
    })
  })

  it('manual exact reads keep the matching availability entry selected when one exists', async () => {
    const availability = makeAvailabilityItem({ symbol: 'BTCUSD', defaultSlice: {
      timeframe: '5m',
      start: faker.date.recent(),
      end: faker.date.soon(),
    } })
    mocks.listCandleAvailability.mockResolvedValue({ items: [availability] })
    mocks.listCandles.mockResolvedValue({ items: [] })

    const user = userEvent.setup()
    render(Data)

    await screen.findByText(availability.symbol)
    await user.selectOptions(screen.getByLabelText('Venue'), 'hyperliquid-perps')
    await user.clear(screen.getByLabelText('Symbol'))
    await user.type(screen.getByLabelText('Symbol'), availability.symbol)
    await user.selectOptions(screen.getByLabelText('Asset class'), 'crypto')
    await user.selectOptions(screen.getByLabelText('Timeframe'), '1m')
    await user.clear(screen.getByLabelText('UTC start'))
    await user.type(screen.getByLabelText('UTC start'), '2026-06-15T12:00:00Z')
    await user.clear(screen.getByLabelText('UTC end'))
    await user.type(screen.getByLabelText('UTC end'), '2026-06-15T13:00:00Z')
    await user.click(screen.getByRole('button', { name: 'Load candles' }))

    expect(await screen.findByText('Selected availability entry')).toBeInTheDocument()
    expect(screen.getByText(`${availability.venue} / ${availability.symbol} / ${availability.assetClass}`)).toBeInTheDocument()
  })

  it('does not guess candle filters when availability is empty', async () => {
    render(Data)

    expect(await screen.findByText('No normalized candle availability was found yet.')).toBeInTheDocument()
    expect(mocks.listCandles).not.toHaveBeenCalled()
  })

  it('falls back gracefully when availability returns 404 from a mismatched backend', async () => {
    mocks.listCandleAvailability.mockRejectedValue(
      new DataApiError({ path: '/candle-availability', status: 404, message: '404 Not Found' }),
    )

    const user = userEvent.setup()

    render(Data)

    expect(
      await screen.findAllByText(
        'Browse-first availability returned 404. This usually means the UI is pointed at an older or stale backend process. You can still use the manual exact candle form below.',
      ),
    ).toHaveLength(1)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()

    await fillRequiredFilters(user)
    await user.click(screen.getByRole('button', { name: 'Load candles' }))

    await waitFor(() => {
      expect(mocks.listCandles).toHaveBeenCalledWith(
        expect.objectContaining({
          venue: 'hyperliquid-perps',
          symbol: 'BTCUSD',
          assetClass: 'crypto',
          timeframe: '1m',
        }),
      )
    })
  })

  it('shows non-404 availability API failures with alert semantics', async () => {
    mocks.listCandleAvailability.mockRejectedValue(new Error('Availability failed'))

    render(Data)

    expect(await screen.findByRole('alert')).toHaveTextContent('Availability failed')
  })

  it('shows candle API failures with alert semantics after default selection', async () => {
    const availability = makeAvailabilityItem()
    mockAvailabilityResponse([availability])
    mocks.listCandles.mockRejectedValue(new Error('Candle read failed'))

    render(Data)

    expect(await screen.findByRole('alert')).toHaveTextContent('Candle read failed')
  })

  it('shows the 10,000-interval cap validation on manual exact reads', async () => {
    const user = userEvent.setup()
    render(Data)
    await screen.findByText('No normalized candle availability was found yet.')

    await user.selectOptions(screen.getByLabelText('Venue'), 'hyperliquid-perps')
    await user.type(screen.getByLabelText('Symbol'), 'BTCUSD')
    await user.selectOptions(screen.getByLabelText('Asset class'), 'crypto')
    await user.selectOptions(screen.getByLabelText('Timeframe'), '1m')
    await user.type(screen.getByLabelText('UTC start'), '2026-06-15T12:00:00Z')
    await user.type(screen.getByLabelText('UTC end'), '2026-06-23T00:00:01Z')
    await user.click(screen.getByRole('button', { name: 'Load candles' }))

    expect(screen.getByRole('alert')).toHaveTextContent(
      'Selected range exceeds the server limit of 10,000 1m intervals.',
    )
    expect(mocks.listCandles).not.toHaveBeenCalled()
  })

  it('shows invalid UTC validation on manual exact reads', async () => {
    const user = userEvent.setup()
    render(Data)
    await screen.findByText('No normalized candle availability was found yet.')

    await user.selectOptions(screen.getByLabelText('Venue'), 'hyperliquid-perps')
    await user.type(screen.getByLabelText('Symbol'), 'BTCUSD')
    await user.selectOptions(screen.getByLabelText('Asset class'), 'crypto')
    await user.selectOptions(screen.getByLabelText('Timeframe'), '1m')
    await user.type(screen.getByLabelText('UTC start'), faker.date.recent().toISOString().replace('Z', ''))
    await user.type(screen.getByLabelText('UTC end'), '2026-06-15T13:00:00Z')
    await user.click(screen.getByRole('button', { name: 'Load candles' }))

    expect(screen.getByRole('alert')).toHaveTextContent(
      'UTC start must be a valid ISO-8601 timestamp.',
    )
  })

  it('shows start-before-end validation on manual exact reads', async () => {
    const user = userEvent.setup()
    render(Data)
    await screen.findByText('No normalized candle availability was found yet.')

    await user.selectOptions(screen.getByLabelText('Venue'), 'hyperliquid-perps')
    await user.type(screen.getByLabelText('Symbol'), 'BTCUSD')
    await user.selectOptions(screen.getByLabelText('Asset class'), 'crypto')
    await user.selectOptions(screen.getByLabelText('Timeframe'), '1m')
    await user.type(screen.getByLabelText('UTC start'), '2026-06-15T13:00:00Z')
    await user.type(screen.getByLabelText('UTC end'), '2026-06-15T13:00:00Z')
    await user.click(screen.getByRole('button', { name: 'Load candles' }))

    expect(screen.getByRole('alert')).toHaveTextContent(
      'UTC start must be earlier than UTC end.',
    )
    expect(mocks.listCandles).not.toHaveBeenCalled()
  })

  it('manually selecting a different candle reloads linked evidence by provenance', async () => {
    const availability = makeAvailabilityItem()
    const firstCandle = makeCandle({ symbol: availability.symbol, provenanceIdentity: faker.string.uuid() })
    const secondCandle = makeCandle({ symbol: availability.symbol, provenanceIdentity: faker.string.uuid() })
    mockAvailabilityResponse([availability])
    mocks.listCandles.mockResolvedValue({ items: [firstCandle, secondCandle] })
    mocks.listCandleRawPayloads.mockResolvedValue({ items: [] })

    const user = userEvent.setup()
    render(Data)

    await screen.findByText(firstCandle.start.toISOString())
    expect(screen.getByRole('button', { name: 'Selected' })).toBeDisabled()
    await user.click(screen.getByRole('button', { name: 'Select' }))

    await waitFor(() => {
      expect(mocks.listCandleRawPayloads).toHaveBeenLastCalledWith(
        expect.objectContaining({
          provenanceSource: secondCandle.provenanceSource,
          provenanceIdentity: secondCandle.provenanceIdentity,
        }),
      )
    })
    expect(screen.getByRole('button', { name: 'Selected' })).toBeDisabled()
  })

  it('shows empty linked evidence when a selected candle has no payload links', async () => {
    const availability = makeAvailabilityItem()
    const candle = makeCandle({ symbol: availability.symbol })
    mockAvailabilityResponse([availability])
    mocks.listCandles.mockResolvedValue({ items: [candle] })
    mocks.listCandleRawPayloads.mockResolvedValue({ items: [] })

    render(Data)

    expect(
      await screen.findByText('No linked raw evidence was found for this candle.'),
    ).toBeInTheDocument()
  })

  it('shows linked evidence failures with alert semantics', async () => {
    const availability = makeAvailabilityItem()
    const candle = makeCandle({ symbol: availability.symbol })
    mockAvailabilityResponse([availability])
    mocks.listCandles.mockResolvedValue({ items: [candle] })
    mocks.listCandleRawPayloads.mockRejectedValue(new Error('Linked evidence failed'))

    render(Data)

    expect(await screen.findByRole('alert')).toHaveTextContent('Linked evidence failed')
  })

  it('keeps broad raw payload browsing explicit for the current candle scope', async () => {
    const availability = makeAvailabilityItem()
    const candle = makeCandle({
      venue: availability.venue,
      symbol: availability.symbol,
      assetClass: availability.assetClass,
      timeframe: availability.defaultSlice.timeframe,
      start: availability.defaultSlice.start,
      end: availability.defaultSlice.end,
    })
    const rawPayload = makeRawPayload({
      venue: candle.venue,
      symbol: candle.symbol,
      assetClass: candle.assetClass,
      timeframe: candle.timeframe,
      start: candle.start,
      end: candle.end,
    })
    mockAvailabilityResponse([availability])
    mocks.listCandles.mockResolvedValue({ items: [candle] })
    mocks.listRawPayloads.mockResolvedValue({ items: [rawPayload] })

    const user = userEvent.setup()
    render(Data)

    await screen.findByText('Raw payload metadata not loaded yet')
    await user.click(screen.getByRole('button', { name: 'Load raw payload metadata' }))

    await waitFor(() => {
      expect(mocks.listRawPayloads).toHaveBeenCalledWith(
        expect.objectContaining({
          venue: candle.venue,
          symbol: candle.symbol,
          assetClass: candle.assetClass,
          timeframe: candle.timeframe,
          start: candle.start,
          end: candle.end,
        }),
      )
    })
    expect(await screen.findByText(rawPayload.id)).toBeInTheDocument()
  })

  it('shows explicit raw payload loading and empty states', async () => {
    const availability = makeAvailabilityItem()
    const candle = makeCandle({ symbol: availability.symbol })
    const deferredRaw = createDeferred<{ items: ReturnType<typeof makeRawPayload>[] }>()
    mockAvailabilityResponse([availability])
    mocks.listCandles.mockResolvedValue({ items: [candle] })
    mocks.listRawPayloads.mockReturnValue(deferredRaw.promise)

    const user = userEvent.setup()
    render(Data)

    await screen.findByRole('button', { name: 'Load raw payload metadata' })
    await user.click(screen.getByRole('button', { name: 'Load raw payload metadata' }))
    expect(screen.getByText('Loading raw payload metadata…')).toBeInTheDocument()

    deferredRaw.resolve({ items: [] })

    expect(await screen.findByText('No raw payload metadata matched these filters.')).toBeInTheDocument()
  })

  it('shows explicit raw payload failures with alert semantics', async () => {
    const availability = makeAvailabilityItem()
    const candle = makeCandle({ symbol: availability.symbol })
    mockAvailabilityResponse([availability])
    mocks.listCandles.mockResolvedValue({ items: [candle] })
    mocks.listRawPayloads.mockRejectedValue(new Error('Raw payload read failed'))

    const user = userEvent.setup()
    render(Data)

    await screen.findByRole('button', { name: 'Load raw payload metadata' })
    await user.click(screen.getByRole('button', { name: 'Load raw payload metadata' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Raw payload read failed')
  })

  it('opens the raw payload detail drawer after explicit raw metadata browsing', async () => {
    const availability = makeAvailabilityItem()
    const candle = makeCandle({ symbol: availability.symbol })
    const rawPayload = makeRawPayload({ symbol: candle.symbol })
    mockAvailabilityResponse([availability])
    mocks.listCandles.mockResolvedValue({ items: [candle] })
    mocks.listRawPayloads.mockResolvedValue({ items: [rawPayload] })
    mocks.getRawPayloadDetail.mockResolvedValue({
      metadata: rawPayload,
      responseBodySizeBytes: faker.number.int({ min: 100, max: 1000 }),
      responseBodyPreview: faker.lorem.paragraph(),
      responseBodyPreviewTruncated: true,
    })

    const user = userEvent.setup()
    render(Data)

    await screen.findByRole('button', { name: 'Load raw payload metadata' })
    await user.click(screen.getByRole('button', { name: 'Load raw payload metadata' }))
    await screen.findByText(rawPayload.id)
    await user.click(screen.getByRole('button', { name: 'View detail' }))

    const dialog = await screen.findByRole('dialog', { name: 'Raw payload detail' })
    await waitFor(() => {
      expect(mocks.getRawPayloadDetail).toHaveBeenCalledWith(rawPayload.id)
    })
    expect(within(dialog).getByText(rawPayload.payloadBodyRef)).toBeInTheDocument()
    expect(
      within(dialog).getByText(
        'This API exposes only a truncated preview. Use the body ref below to inspect the full payload in storage.',
      ),
    ).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Copy preview' })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Copy body ref' })).toBeInTheDocument()
  })

  it('closes the raw payload detail drawer when dismissed', async () => {
    const availability = makeAvailabilityItem()
    const candle = makeCandle({ symbol: availability.symbol })
    const rawPayload = makeRawPayload({ symbol: candle.symbol })
    mockAvailabilityResponse([availability])
    mocks.listCandles.mockResolvedValue({ items: [candle] })
    mocks.listRawPayloads.mockResolvedValue({ items: [rawPayload] })
    mocks.getRawPayloadDetail.mockResolvedValue({
      metadata: rawPayload,
      responseBodySizeBytes: faker.number.int({ min: 100, max: 1000 }),
      responseBodyPreview: faker.lorem.paragraph(),
      responseBodyPreviewTruncated: false,
    })

    const user = userEvent.setup()
    render(Data)

    await screen.findByRole('button', { name: 'Load raw payload metadata' })
    await user.click(screen.getByRole('button', { name: 'Load raw payload metadata' }))
    await screen.findByText(rawPayload.id)
    await user.click(screen.getByRole('button', { name: 'View detail' }))
    const dialog = await screen.findByRole('dialog', { name: 'Raw payload detail' })
    expect(dialog).toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: 'Close' }))

    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: 'Raw payload detail' })).not.toBeInTheDocument()
    })
  })

  it('shows raw payload detail failures with alert semantics', async () => {
    const availability = makeAvailabilityItem()
    const candle = makeCandle({ symbol: availability.symbol })
    const rawPayload = makeRawPayload({ symbol: candle.symbol })
    mockAvailabilityResponse([availability])
    mocks.listCandles.mockResolvedValue({ items: [candle] })
    mocks.listRawPayloads.mockResolvedValue({ items: [rawPayload] })
    mocks.getRawPayloadDetail.mockRejectedValue(new Error('Detail failed'))

    const user = userEvent.setup()
    render(Data)

    await screen.findByRole('button', { name: 'Load raw payload metadata' })
    await user.click(screen.getByRole('button', { name: 'Load raw payload metadata' }))
    await screen.findByText(rawPayload.id)
    await user.click(screen.getByRole('button', { name: 'View detail' }))

    const dialog = await screen.findByRole('dialog', { name: 'Raw payload detail' })
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('Detail failed')
  })
})
