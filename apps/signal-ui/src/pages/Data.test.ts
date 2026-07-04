import { beforeEach, describe, expect, it, vi } from 'vitest'
import { faker } from '@faker-js/faker'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
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
  createHistoricalDataBackfillJob: vi.fn(),
}))

let availabilityCounter = 0

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

vi.mock('../lib/jobs/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/jobs/api')>()
  return {
    ...actual,
    createSignalJobsApiForAuth: vi.fn(() => ({
      createHistoricalDataBackfillJob: mocks.createHistoricalDataBackfillJob,
    })),
  }
})

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
  const symbolId = availabilityCounter++
  const latestStart = faker.date.recent()
  const latestEnd = faker.date.soon({ refDate: latestStart })
  const earlierStart = faker.date.past({ years: 1, refDate: latestStart })
  const earlierEnd = faker.date.soon({ refDate: earlierStart })
  const earlierCount = symbolId * 10 + 100
  const latestCount = earlierCount + 5

  return {
    venue: 'hyperliquid-perps',
    symbol: `COIN${symbolId}USD`,
    assetClass: 'crypto',
    timeframes: [
      {
        timeframe: '1m',
        start: earlierStart,
        end: earlierEnd,
        count: earlierCount,
      },
      {
        timeframe: '5m',
        start: latestStart,
        end: latestEnd,
        count: latestCount,
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
  const symbolInput = screen.getByLabelText('Symbol') as HTMLInputElement
  await fireEvent.input(symbolInput, { target: { value: 'BTCUSD' } })
  await fireEvent.change(symbolInput, { target: { value: 'BTCUSD' } })
  await user.selectOptions(screen.getByLabelText('Asset class'), 'crypto')
  await user.selectOptions(screen.getByLabelText('Timeframe'), '1m')
  await fillUtcRange(user, {
    startDate: '2026-06-15',
    startTime: '12:00:00',
    endDate: '2026-06-15',
    endTime: '13:00:00',
  })
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

function availabilityCardNameMatcher(
  item: Pick<ReturnType<typeof makeAvailabilityItem>, 'venue' | 'symbol' | 'assetClass'>,
) {
  return (_accessibleName: string, element: Element) => {
    const text = element.textContent ?? ''
    return text.includes(item.venue) && text.includes(item.symbol) && text.includes(item.assetClass)
  }
}

async function findAvailabilityCard(item: ReturnType<typeof makeAvailabilityItem>) {
  const list = await screen.findByLabelText('Candle availability entries')
  return within(list).findByRole('button', { name: availabilityCardNameMatcher(item) })
}

async function waitForDefaultAvailabilityLoad() {
  await waitFor(() => {
    expect(mocks.listCandles).toHaveBeenCalledTimes(1)
  })
}

describe('Data page', () => {
  beforeEach(() => {
    availabilityCounter = 0
    window.location.hash = '#/data'
    chartSetData.mockReset()
    mocks.listCandleAvailability.mockReset()
    mocks.listCandles.mockReset()
    mocks.listRawPayloads.mockReset()
    mocks.getRawPayloadDetail.mockReset()
    mocks.listCandleRawPayloads.mockReset()
    mocks.createHistoricalDataBackfillJob.mockReset()
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
    const firstAvailability = makeAvailabilityItem({ symbol: 'BTCUSD' })
    const secondAvailability = makeAvailabilityItem({ symbol: 'ETHUSD' })
    mockAvailabilityResponse([firstAvailability, secondAvailability])
    mocks.listCandles.mockResolvedValue({ items: [] })

    const user = userEvent.setup()
    render(Data)

    await waitForDefaultAvailabilityLoad()
    const secondCard = await findAvailabilityCard(secondAvailability)
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
    const firstAvailability = makeAvailabilityItem({ symbol: 'BTCUSD' })
    const secondAvailability = makeAvailabilityItem({ symbol: 'ETHUSD' })
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

    await waitForDefaultAvailabilityLoad()
    const secondCard = await findAvailabilityCard(secondAvailability)
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

    expect(screen.getByText('Venue is required.')).toBeInTheDocument()
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

  it('loads the exact data scope encoded in the route query instead of the availability default slice', async () => {
    const availability = makeAvailabilityItem({
      symbol: 'BTCUSD',
      defaultSlice: {
        timeframe: '5m',
        start: new Date('2026-06-10T00:00:00.000Z'),
        end: new Date('2026-06-10T01:00:00.000Z'),
      },
    })
    const routeScope = {
      venue: 'hyperliquid-perps',
      symbol: 'BTC',
      assetClass: 'future',
      timeframe: '1h',
      start: new Date('2026-06-15T12:00:00.000Z'),
      end: new Date('2026-06-15T13:00:00.000Z'),
    }
    const routeCandle = makeCandle(routeScope)
    mockAvailabilityResponse([availability])
    mocks.listCandles.mockResolvedValue({ items: [routeCandle] })
    mocks.listCandleRawPayloads.mockResolvedValue({ items: [] })
    window.location.hash =
      '#/data?venue=hyperliquid-perps&symbol=BTC&assetClass=future&timeframe=1h&start=2026-06-15T12%3A00%3A00.000Z&end=2026-06-15T13%3A00%3A00.000Z'

    render(Data)

    await waitFor(() => {
      expect(mocks.listCandles).toHaveBeenCalledTimes(1)
      expect(mocks.listCandles).toHaveBeenCalledWith(routeScope)
    })
    expect(screen.getByLabelText('Venue')).toHaveValue(routeScope.venue)
    expect(screen.getByLabelText('Symbol')).toHaveValue(routeScope.symbol)
    expect(screen.getByLabelText('Asset class')).toHaveValue(routeScope.assetClass)
    expect(screen.getByLabelText('Timeframe')).toHaveValue(routeScope.timeframe)
  })

  it('still loads the routed data scope when availability falls back with a 404 compatibility note', async () => {
    const routeScope = {
      venue: 'hyperliquid-perps',
      symbol: 'BTC',
      assetClass: 'future',
      timeframe: '1h',
      start: new Date('2026-06-15T12:00:00.000Z'),
      end: new Date('2026-06-15T13:00:00.000Z'),
    }
    mocks.listCandleAvailability.mockRejectedValue(
      new DataApiError({ path: '/candle-availability', status: 404, message: '404 Not Found' }),
    )
    mocks.listCandles.mockResolvedValue({ items: [makeCandle(routeScope)] })
    mocks.listCandleRawPayloads.mockResolvedValue({ items: [] })
    window.location.hash =
      '#/data?venue=hyperliquid-perps&symbol=BTC&assetClass=future&timeframe=1h&start=2026-06-15T12%3A00%3A00.000Z&end=2026-06-15T13%3A00%3A00.000Z'

    render(Data)

    expect(
      await screen.findByText(
        'Browse-first availability returned 404. This usually means the UI is pointed at an older or stale backend process. You can still use the manual exact candle form below.',
      ),
    ).toBeInTheDocument()
    await waitFor(() => {
      expect(mocks.listCandles).toHaveBeenCalledWith(routeScope)
    })
  })

  it('keeps browse-first and explicit candle loads read-only until start historical backfill is used', async () => {
    const availability = makeAvailabilityItem()
    mockAvailabilityResponse([availability])
    mocks.listCandles.mockResolvedValue({ items: [] })

    const user = userEvent.setup()
    render(Data)

    await screen.findByText(availability.symbol)
    expect(mocks.createHistoricalDataBackfillJob).not.toHaveBeenCalled()

    await user.click(screen.getByRole('button', { name: 'Load candles' }))

    expect(mocks.createHistoricalDataBackfillJob).not.toHaveBeenCalled()
  })

  it('validates explicit historical backfill action and does not submit when required fields are missing', async () => {
    const user = userEvent.setup()
    render(Data)
    await screen.findByText('No normalized candle availability was found yet.')

    await user.click(screen.getByRole('button', { name: 'Start historical backfill' }))

    expect(screen.getByText('Venue is required.')).toBeInTheDocument()
    expect(mocks.createHistoricalDataBackfillJob).not.toHaveBeenCalled()
  })

  it(
    'starts an explicit historical backfill job from the current data scope and shows the created job link with reload action',
    async () => {
    const user = userEvent.setup()
    mocks.createHistoricalDataBackfillJob.mockResolvedValue({
      id: 'job-123',
      jobType: 'historical_raw_candle_backfill',
      status: 'queued',
      requester: { userId: 'user-1', source: 'operator', agentSessionId: '', agentRunId: '' },
      input: {
        ingestionRunId: 'ingest-1',
        venue: 'hyperliquid-perps',
        symbol: 'BTCUSD',
        assetClass: 'future',
        timeframe: '1m',
        start: new Date('2026-06-15T12:00:00.000Z'),
        end: new Date('2026-06-15T13:00:00.000Z'),
        pageSize: 0,
      },
      createdAt: new Date('2026-06-15T11:59:00.000Z'),
      updatedAt: new Date('2026-06-15T11:59:00.000Z'),
      startedAt: null,
      completedAt: null,
      attemptCount: 0,
      workerId: '',
      lastAttemptAt: null,
    })

    render(Data)
    await screen.findByText('No normalized candle availability was found yet.')

    await fillRequiredFilters(user)
    await user.click(screen.getByRole('button', { name: 'Start historical backfill' }))

    await waitFor(() => {
      expect(mocks.createHistoricalDataBackfillJob).toHaveBeenCalledWith({
        body: {
          venue: 'hyperliquid-perps',
          symbol: 'BTCUSD',
          assetClass: 'future',
          timeframe: '1m',
          start: new Date('2026-06-15T12:00:00.000Z'),
          end: new Date('2026-06-15T13:00:00.000Z'),
          pageSize: 500,
        },
      })
    })

    expect(await screen.findByText('Created job job-123 with status queued.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open created job' })).toHaveAttribute('href', '#/jobs/job-123')
    await user.click(screen.getByRole('button', { name: 'Reload availability' }))
    await waitFor(() => {
      expect(mocks.listCandleAvailability).toHaveBeenCalledTimes(2)
    })
    },
    10_000,
  )

  it('allows zero backfill page size to use the backend default path', async () => {
    const user = userEvent.setup()
    mocks.createHistoricalDataBackfillJob.mockResolvedValue({
      id: 'job-zero',
      jobType: 'historical_raw_candle_backfill',
      status: 'queued',
      requester: { userId: 'user-1', source: 'operator', agentSessionId: '', agentRunId: '' },
      input: {
        ingestionRunId: 'ingest-zero',
        venue: 'hyperliquid-perps',
        symbol: 'BTCUSD',
        assetClass: 'future',
        timeframe: '1m',
        start: new Date('2026-06-15T12:00:00.000Z'),
        end: new Date('2026-06-15T13:00:00.000Z'),
        pageSize: 0,
      },
      createdAt: new Date('2026-06-15T11:59:00.000Z'),
      updatedAt: new Date('2026-06-15T11:59:00.000Z'),
      startedAt: null,
      completedAt: null,
      attemptCount: 0,
      workerId: '',
      lastAttemptAt: null,
    })

    render(Data)
    await screen.findByText('No normalized candle availability was found yet.')

    await fillRequiredFilters(user)
    await user.clear(screen.getByLabelText('Backfill page size'))
    await user.type(screen.getByLabelText('Backfill page size'), '0')
    await user.click(screen.getByRole('button', { name: 'Start historical backfill' }))

    await waitFor(() => {
      expect(mocks.createHistoricalDataBackfillJob).toHaveBeenCalledWith({
        body: {
          venue: 'hyperliquid-perps',
          symbol: 'BTCUSD',
          assetClass: 'future',
          timeframe: '1m',
          start: new Date('2026-06-15T12:00:00.000Z'),
          end: new Date('2026-06-15T13:00:00.000Z'),
          pageSize: 0,
        },
      })
    })

    expect(screen.queryByText('Backfill page size must be zero or a positive integer.')).not.toBeInTheDocument()
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
    await fillUtcRange(user, {
      startDate: '2026-06-15',
      startTime: '12:00:00',
      endDate: '2026-06-15',
      endTime: '13:00:00',
    })
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

    expect(await screen.findByText('Candle read failed')).toBeInTheDocument()
  })

  it('shows the 10,000-interval cap validation on manual exact reads', async () => {
    const user = userEvent.setup()
    render(Data)
    await screen.findByText('No normalized candle availability was found yet.')

    await user.selectOptions(screen.getByLabelText('Venue'), 'hyperliquid-perps')
    await user.type(screen.getByLabelText('Symbol'), 'BTCUSD')
    await user.selectOptions(screen.getByLabelText('Asset class'), 'crypto')
    await user.selectOptions(screen.getByLabelText('Timeframe'), '1m')
    await fillUtcRange(user, {
      startDate: '2026-06-15',
      startTime: '12:00:00',
      endDate: '2026-06-23',
      endTime: '00:00:01',
    })
    await user.click(screen.getByRole('button', { name: 'Load candles' }))

    expect(await screen.findByText(
      'Selected range exceeds the server limit of 10,000 1m intervals.',
    )).toBeInTheDocument()
    expect(mocks.listCandles).not.toHaveBeenCalled()
  })

  it('resolves data presets against the selected availability timeframe without auto-loading candles', async () => {
    const availability = makeAvailabilityItem({
      defaultSlice: {
        timeframe: '1m',
        start: new Date('2026-06-15T00:00:00.000Z'),
        end: new Date('2026-06-15T12:00:00.000Z'),
      },
      timeframes: [
        {
          timeframe: '1m',
          start: new Date('2026-06-10T00:00:00.000Z'),
          end: new Date('2026-06-15T12:00:00.000Z'),
          count: 100,
        },
      ],
    })
    mockAvailabilityResponse([availability])
    mocks.listCandles.mockResolvedValue({ items: [] })

    const user = userEvent.setup()
    render(Data)

    await screen.findByText(availability.symbol)
    mocks.listCandles.mockClear()

    await user.click(screen.getByRole('button', { name: 'Last 24h' }))

    expect(screen.getByText('2026-06-14T12:00:00.000Z')).toBeInTheDocument()
    expect(screen.getAllByText('2026-06-15T12:00:00.000Z').length).toBeGreaterThan(0)
    expect(mocks.listCandles).not.toHaveBeenCalled()
  })

  it('shows selected availability bound validation on manual exact reads', async () => {
    const availability = makeAvailabilityItem({
      defaultSlice: {
        timeframe: '1m',
        start: new Date('2026-06-15T00:00:00.000Z'),
        end: new Date('2026-06-15T12:00:00.000Z'),
      },
      timeframes: [
        {
          timeframe: '1m',
          start: new Date('2026-06-15T00:00:00.000Z'),
          end: new Date('2026-06-15T12:00:00.000Z'),
          count: 100,
        },
      ],
    })
    mockAvailabilityResponse([availability])
    mocks.listCandles.mockResolvedValue({ items: [] })

    const user = userEvent.setup()
    render(Data)

    await screen.findByText(availability.symbol)
    mocks.listCandles.mockClear()

    await fillUtcRange(user, {
      startDate: '2026-06-14',
      startTime: '23:00:00',
      endDate: '2026-06-15',
      endTime: '12:00:00',
    })
    await user.click(screen.getByRole('button', { name: 'Load candles' }))

    expect(screen.getByText(
      'UTC range must stay within the selected availability window.',
    )).toBeInTheDocument()
    expect(mocks.listCandles).not.toHaveBeenCalled()
  })

  it('shows start-before-end validation on manual exact reads', async () => {
    const user = userEvent.setup()
    render(Data)
    await screen.findByText('No normalized candle availability was found yet.')

    await user.selectOptions(screen.getByLabelText('Venue'), 'hyperliquid-perps')
    await user.type(screen.getByLabelText('Symbol'), 'BTCUSD')
    await user.selectOptions(screen.getByLabelText('Asset class'), 'crypto')
    await user.selectOptions(screen.getByLabelText('Timeframe'), '1m')
    await fillUtcRange(user, {
      startDate: '2026-06-15',
      startTime: '13:00:00',
      endDate: '2026-06-15',
      endTime: '13:00:00',
    })
    await user.click(screen.getByRole('button', { name: 'Load candles' }))

    expect(await screen.findByText(
      'UTC start must be earlier than UTC end.',
    )).toBeInTheDocument()
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

    expect(await screen.findByText('Linked evidence failed')).toBeInTheDocument()
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

    expect(await screen.findByText('Raw payload read failed')).toBeInTheDocument()
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
