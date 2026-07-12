<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import DataCandlestickChart from '../components/DataCandlestickChart.svelte'
  import DateRangePicker from '../components/DateRangePicker.svelte'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { toChartCandleRows } from '../lib/data/charting'
  import {
    createSignalDataApiForAuth,
    type CandleAvailabilityDefaultSelection,
    type CandleAvailabilityItem,
    DataApiError,
    type DataCandle,
    type DataTimeframe,
    type ListDataCandlesParams,
    type RawPayloadDetailResponse,
    type RawPayloadMetadata,
  } from '../lib/data/data-api'
  import {
    createSignalJobsApiForAuth,
    type JobDetail,
  } from '../lib/jobs/api'
  import { parseTimestamp, validateRange } from '../lib/date-range'
  import { formatLocalDateTime } from '../lib/timestamp'

  const dataBaseUrl = import.meta.env.VITE_DATA_API_BASE_URL ?? '/api/v1/data'
  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'

  const dataApi = $derived.by(() =>
    createSignalDataApiForAuth({ baseUrl: dataBaseUrl, authStore }),
  )
  const jobsApi = $derived.by(() => createSignalJobsApiForAuth({ baseUrl: appBaseUrl, authStore }))

  const timeframeDurationsMs: Record<DataTimeframe, number> = {
    '1m': 60_000,
    '5m': 300_000,
    '15m': 900_000,
    '1h': 3_600_000,
    '4h': 14_400_000,
    '1d': 86_400_000,
  }

  const maxIntervals = 10_000

  let venue = $state('')
  let symbol = $state('')
  let assetClass = $state('')
  let timeframe = $state('')
  let utcStart = $state<Date | undefined>()
  let utcEnd = $state<Date | undefined>()
  let ingestionRunId = $state('')

  let validationErrors = $state<string[]>([])
  let showRangeValidation = $state(false)

  let availabilityItems = $state<CandleAvailabilityItem[]>([])
  let availabilityLoading = $state(true)
  let availabilityError = $state<string | null>(null)
  let availabilityCompatibilityNote = $state<string | null>(null)
  let selectedAvailabilityKey = $state<string | null>(null)
  let availabilityRequestToken = 0

  let candles = $state<DataCandle[]>([])
  let candlesLoading = $state(false)
  let candlesError = $state<string | null>(null)
  let candlesRequestToken = 0
  let currentScope = $state<ListDataCandlesParams | null>(null)

  let rawPayloads = $state<RawPayloadMetadata[]>([])
  let rawPayloadsLoading = $state(false)
  let rawPayloadsLoaded = $state(false)
  let rawPayloadsError = $state<string | null>(null)
  let rawPayloadsRequestToken = 0

  let selectedCandleIdentity = $state<number | null>(null)
  let linkedEvidence = $state<RawPayloadMetadata[]>([])
  let linkedEvidenceLoading = $state(false)
  let linkedEvidenceError = $state<string | null>(null)
  let linkedEvidenceRequestToken = 0

  let detailDrawerOpen = $state(false)
  let detailLoading = $state(false)
  let detailError = $state<string | null>(null)
  let detailFeedback = $state<string | null>(null)
  let selectedRawPayloadId = $state<string | null>(null)
  let rawPayloadDetail = $state<RawPayloadDetailResponse | null>(null)
  let detailRequestToken = 0

  let jobSubmitting = $state(false)
  let jobError = $state<string | null>(null)
  let createdJob = $state<JobDetail | null>(null)
  let backfillIdempotencyKey = $state('')
  let backfillPageSize = $state('500')

  const chartRows = $derived(toChartCandleRows(candles))
  const selectedCandle = $derived(
    selectedCandleIdentity === null
      ? null
      : candles.find((candle) => candle.identity === selectedCandleIdentity) ?? null,
  )
  const selectedAvailability = $derived(
    selectedAvailabilityKey === null
      ? null
      : availabilityItems.find((item) => availabilityKey(item) === selectedAvailabilityKey) ?? null,
  )
  const selectedAvailabilityTimeframe = $derived(
    selectedAvailability
      ? selectedAvailability.timeframes.find((item) => item.timeframe === timeframe) ?? null
      : null,
  )
  const selectedTimeframeDurationMs = $derived(
    timeframe in timeframeDurationsMs ? timeframeDurationsMs[timeframe as DataTimeframe] : null,
  )
  const selectedTimeframeMaxIntervalsMessage = $derived(
    selectedTimeframeDurationMs === null
      ? undefined
      : `Selected range exceeds the server limit of 10,000 ${timeframe} intervals.`,
  )

  onMount(() => {
    const initialRouteScope = readRouteScopeFromHash()
    if (initialRouteScope) {
      validationErrors = []
      showRangeValidation = false
      applyScopeToFilters(initialRouteScope)
    }
    void loadAvailability({ initialRouteScope })
  })

  async function loadAvailability(options: { initialRouteScope?: ListDataCandlesParams | null } = {}) {
    const requestToken = ++availabilityRequestToken
    const initialRouteScope = options.initialRouteScope ?? null
    let shouldLoadInitialRouteScope = initialRouteScope !== null
    availabilityLoading = true
    availabilityError = null
    availabilityCompatibilityNote = null

    try {
      const response = await dataApi.listCandleAvailability({})
      if (requestToken !== availabilityRequestToken) {
        return
      }

      availabilityItems = response.items

      if (initialRouteScope) {
        shouldLoadInitialRouteScope = false
        void loadCandlesForScope(
          initialRouteScope,
          findAvailabilityKeyForScope(initialRouteScope, response.items),
        )
        return
      }

      const shouldAutoLoadDefaultSelection =
        currentScope === null &&
        venue.trim() === '' &&
        symbol.trim() === '' &&
        assetClass.trim() === '' &&
        timeframe.trim() === '' &&
        utcStart === undefined &&
        utcEnd === undefined

      if (response.defaultSelection && shouldAutoLoadDefaultSelection) {
        const defaultKey = availabilityKeyFromSelection(response.defaultSelection)
        selectedAvailabilityKey = defaultKey
        void loadCandlesForScope(mapDefaultSelectionToScope(response.defaultSelection), defaultKey)
      }
    } catch (error) {
      if (requestToken !== availabilityRequestToken) {
        return
      }
      availabilityItems = []
      if (error instanceof DataApiError && error.status === 404) {
        availabilityCompatibilityNote =
          'Browse-first availability returned 404. This usually means the UI is pointed at an older or stale backend process. You can still use the manual exact candle form below.'
      } else {
        availabilityError =
          error instanceof Error ? error.message : 'Failed to load candle availability'
      }
    } finally {
      if (requestToken === availabilityRequestToken) {
        availabilityLoading = false
        if (shouldLoadInitialRouteScope && initialRouteScope) {
          void loadCandlesForScope(initialRouteScope, null)
        }
      }
    }
  }

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    showRangeValidation = true
    const parsed = validateAndBuildQuery()
    validationErrors = parsed.errors
    if (parsed.errors.length > 0 || parsed.rangeErrors.length > 0 || !parsed.start || !parsed.end) {
      return
    }

    const scope = {
      venue: venue.trim(),
      symbol: symbol.trim(),
      assetClass: assetClass.trim(),
      timeframe: timeframe.trim(),
      start: parsed.start,
      end: parsed.end,
    }

    await loadCandlesForScope(scope, findAvailabilityKeyForScope(scope))
  }

  async function handleAvailabilitySelection(item: CandleAvailabilityItem) {
    const scope = mapAvailabilityItemToScope(item)
    const key = availabilityKey(item)
    validationErrors = []
    showRangeValidation = false
    selectedAvailabilityKey = key
    await loadCandlesForScope(scope, key)
  }

  async function loadCandlesForScope(scope: ListDataCandlesParams, availabilityKeyValue: string | null) {
    const requestToken = ++candlesRequestToken
    candlesLoading = true
    candlesError = null
    currentScope = scope
    selectedAvailabilityKey = availabilityKeyValue
    applyScopeToFilters(scope)
    resetLinkedEvidenceState()
    resetRawPayloadScopeState()
    resetDetailState()
    candles = []

    try {
      const response = await dataApi.listCandles(scope)
      if (requestToken !== candlesRequestToken) {
        return
      }

      candles = response.items
      if (response.items.length > 0) {
        await selectCandle(response.items[0])
      }
    } catch (error) {
      if (requestToken !== candlesRequestToken) {
        return
      }
      candlesError = error instanceof Error ? error.message : 'Failed to load normalized candles'
      candles = []
    } finally {
      if (requestToken === candlesRequestToken) {
        candlesLoading = false
      }
    }
  }

  async function startHistoricalBackfill() {
    showRangeValidation = true
    jobError = null
    createdJob = null
    const parsed = validateAndBuildQuery()
    validationErrors = parsed.errors
    if (parsed.errors.length > 0 || parsed.rangeErrors.length > 0 || !parsed.start || !parsed.end) {
      return
    }

    const pageSizeNumber = Number(backfillPageSize.trim() || '0')
    if (!Number.isInteger(pageSizeNumber) || pageSizeNumber < 0) {
      jobError = 'Backfill page size must be zero or a positive integer.'
      return
    }

    jobSubmitting = true
    try {
      createdJob = await jobsApi.createHistoricalDataBackfillJob({
        body: {
          ...(backfillIdempotencyKey.trim() ? { idempotencyKey: backfillIdempotencyKey.trim() } : {}),
          venue: venue.trim(),
          symbol: symbol.trim(),
          assetClass: mapAssetClassForBackfill(assetClass.trim(), venue.trim()),
          timeframe: timeframe.trim(),
          start: parsed.start,
          end: parsed.end,
          pageSize: pageSizeNumber,
        },
      })
    } catch (error) {
      jobError = error instanceof Error ? error.message : 'Failed to create historical backfill job'
      createdJob = null
    } finally {
      jobSubmitting = false
    }
  }

  async function loadRawPayloadsForCurrentScope() {
    if (!currentScope) {
      return
    }

    const requestToken = ++rawPayloadsRequestToken
    rawPayloadsLoading = true
    rawPayloadsLoaded = true
    rawPayloadsError = null
    rawPayloads = []
    resetDetailState()

    try {
      const response = await dataApi.listRawPayloads({
        ...currentScope,
        ingestionRunId: ingestionRunId.trim(),
      })
      if (requestToken !== rawPayloadsRequestToken) {
        return
      }
      rawPayloads = response.items
    } catch (error) {
      if (requestToken !== rawPayloadsRequestToken) {
        return
      }
      rawPayloadsError =
        error instanceof Error ? error.message : 'Failed to load raw payload metadata'
      rawPayloads = []
    } finally {
      if (requestToken === rawPayloadsRequestToken) {
        rawPayloadsLoading = false
      }
    }
  }

  async function selectCandle(candle: DataCandle) {
    selectedCandleIdentity = candle.identity
    const requestToken = ++linkedEvidenceRequestToken
    linkedEvidenceLoading = true
    linkedEvidenceError = null
    linkedEvidence = []

    try {
      const response = await dataApi.listCandleRawPayloads({
        venue: candle.venue,
        symbol: candle.symbol,
        assetClass: candle.assetClass,
        timeframe: candle.timeframe,
        start: candle.start,
        end: candle.end,
        provenanceSource: candle.provenanceSource,
        provenanceIdentity: candle.provenanceIdentity,
      })
      if (requestToken !== linkedEvidenceRequestToken) {
        return
      }
      linkedEvidence = response.items
    } catch (error) {
      if (requestToken !== linkedEvidenceRequestToken) {
        return
      }
      linkedEvidenceError = error instanceof Error ? error.message : 'Failed to load linked evidence'
      linkedEvidence = []
    } finally {
      if (requestToken === linkedEvidenceRequestToken) {
        linkedEvidenceLoading = false
      }
    }
  }

  async function openRawPayloadDetail(id: string) {
    const requestToken = ++detailRequestToken
    detailDrawerOpen = true
    detailLoading = true
    detailError = null
    detailFeedback = null
    selectedRawPayloadId = id
    rawPayloadDetail = null

    try {
      const detail = await dataApi.getRawPayloadDetail(id)
      if (requestToken !== detailRequestToken) {
        return
      }
      rawPayloadDetail = detail
    } catch (error) {
      if (requestToken !== detailRequestToken) {
        return
      }
      detailError = error instanceof Error ? error.message : 'Failed to load raw payload detail'
    } finally {
      if (requestToken === detailRequestToken) {
        detailLoading = false
      }
    }
  }

  function closeRawPayloadDetail() {
    detailRequestToken += 1
    detailDrawerOpen = false
    detailLoading = false
    detailError = null
    detailFeedback = null
    selectedRawPayloadId = null
    rawPayloadDetail = null
  }

  function resetLinkedEvidenceState() {
    selectedCandleIdentity = null
    linkedEvidenceRequestToken += 1
    linkedEvidence = []
    linkedEvidenceLoading = false
    linkedEvidenceError = null
  }

  function resetRawPayloadScopeState() {
    rawPayloadsRequestToken += 1
    rawPayloads = []
    rawPayloadsLoading = false
    rawPayloadsLoaded = false
    rawPayloadsError = null
  }

  function resetDetailState() {
    detailRequestToken += 1
    detailDrawerOpen = false
    detailLoading = false
    detailError = null
    detailFeedback = null
    selectedRawPayloadId = null
    rawPayloadDetail = null
  }

  async function copyDetailValue(value: string, label: string) {
    if (!window.navigator.clipboard?.writeText) {
      detailFeedback = 'Clipboard copy is unavailable in this browser.'
      return
    }

    try {
      await window.navigator.clipboard.writeText(value)
      detailFeedback = `${label} copied.`
    } catch {
      detailFeedback = `Failed to copy ${label.toLowerCase()}.`
    }
  }

  function validateAndBuildQuery(): {
    errors: string[]
    rangeErrors: string[]
    start: Date | null
    end: Date | null
  } {
    const errors: string[] = []
    const trimmedVenue = venue.trim()
    const trimmedSymbol = symbol.trim()
    const trimmedAssetClass = assetClass.trim()
    const trimmedTimeframe = timeframe.trim()
    const start = utcStart ?? null
    const end = utcEnd ?? null

    if (!trimmedVenue) errors.push('Venue is required.')
    if (!trimmedSymbol) errors.push('Symbol is required.')
    if (!trimmedAssetClass) errors.push('Asset class is required.')
    if (!trimmedTimeframe) errors.push('Timeframe is required.')

    const timeframeDurationMs =
      trimmedTimeframe && trimmedTimeframe in timeframeDurationsMs
        ? timeframeDurationsMs[trimmedTimeframe as DataTimeframe]
        : null

    const rangeErrors = validateRange({
      start: utcStart,
      end: utcEnd,
       requiredStartMessage: 'Start is required.',
       requiredEndMessage: 'End is required.',
       invalidStartMessage: 'Start must be a valid timestamp.',
       invalidEndMessage: 'End must be a valid timestamp.',
       notEarlierMessage: 'Start must be earlier than end.',
      min: selectedAvailabilityTimeframe?.start,
      max: selectedAvailabilityTimeframe?.end,
      outOfBoundsMessage: selectedAvailabilityTimeframe
         ? 'Range must stay within the selected availability window.'
        : undefined,
      timeframeDurationMs,
      maxIntervals,
      maxIntervalsMessage:
        timeframeDurationMs === null
          ? undefined
          : `Selected range exceeds the server limit of 10,000 ${trimmedTimeframe} intervals.`,
    })

    return { errors, rangeErrors, start, end }
  }

  function applyScopeToFilters(scope: ListDataCandlesParams) {
    venue = scope.venue
    symbol = scope.symbol
    assetClass = scope.assetClass
    timeframe = scope.timeframe
    utcStart = new Date(scope.start)
    utcEnd = new Date(scope.end)
  }

  function availabilityKey(item: Pick<CandleAvailabilityItem, 'venue' | 'symbol' | 'assetClass'>): string {
    return `${item.venue}::${item.symbol}::${item.assetClass}`
  }

  function availabilityKeyFromSelection(
    item: Pick<CandleAvailabilityDefaultSelection, 'venue' | 'symbol' | 'assetClass'>,
  ): string {
    return availabilityKey(item)
  }

  function findAvailabilityKeyForScope(
    scope: Pick<ListDataCandlesParams, 'venue' | 'symbol' | 'assetClass'>,
    items: Pick<CandleAvailabilityItem, 'venue' | 'symbol' | 'assetClass'>[] = availabilityItems,
  ) {
    const match = items.find(
      (item) =>
        item.venue === scope.venue &&
        item.symbol === scope.symbol &&
        item.assetClass === scope.assetClass,
    )

    return match ? availabilityKey(match) : null
  }

  function mapDefaultSelectionToScope(selection: CandleAvailabilityDefaultSelection): ListDataCandlesParams {
    return {
      venue: selection.venue,
      symbol: selection.symbol,
      assetClass: selection.assetClass,
      timeframe: selection.timeframe,
      start: selection.start,
      end: selection.end,
    }
  }

  function mapAvailabilityItemToScope(item: CandleAvailabilityItem): ListDataCandlesParams {
    return {
      venue: item.venue,
      symbol: item.symbol,
      assetClass: item.assetClass,
      timeframe: item.defaultSlice.timeframe,
      start: item.defaultSlice.start,
      end: item.defaultSlice.end,
    }
  }

  function readRouteScopeFromHash(): ListDataCandlesParams | null {
    if (typeof window === 'undefined') {
      return null
    }

    const hash = window.location.hash.startsWith('#')
      ? window.location.hash.slice(1)
      : window.location.hash
    const queryIndex = hash.indexOf('?')
    if (queryIndex < 0) {
      return null
    }

    const routePath = hash.slice(0, queryIndex)
    if (routePath !== '/data') {
      return null
    }

    const searchParams = new URLSearchParams(hash.slice(queryIndex + 1))
    const venueValue = searchParams.get('venue')?.trim() ?? ''
    const symbolValue = searchParams.get('symbol')?.trim() ?? ''
    const assetClassValue = searchParams.get('assetClass')?.trim() ?? ''
    const timeframeValue = searchParams.get('timeframe')?.trim() ?? ''
     const start = parseTimestamp(searchParams.get('start') ?? '')
     const end = parseTimestamp(searchParams.get('end') ?? '')

    if (!venueValue || !symbolValue || !assetClassValue || !timeframeValue || !start || !end) {
      return null
    }

    if (!(timeframeValue in timeframeDurationsMs) || start >= end) {
      return null
    }

    return {
      venue: venueValue,
      symbol: symbolValue,
      assetClass: assetClassValue,
      timeframe: timeframeValue,
      start,
      end,
    }
  }

  function formatAvailabilityRange(item: { start: Date; end: Date }): string {
    return `${formatLocalDateTime(item.start)} → ${formatLocalDateTime(item.end)}`
  }

  function formatSelectedCandleLabel(candle: DataCandle | null): string {
    if (!candle) {
      return 'No normalized candle selected yet.'
    }

    return `${formatLocalDateTime(candle.start)} · ${candle.timeframe} · O ${candle.open} · C ${candle.close}`
  }

  function mapAssetClassForBackfill(assetClassValue: string, venueValue: string): string {
    if (venueValue === 'hyperliquid-perps' && assetClassValue === 'crypto') {
      return 'future'
    }
    return assetClassValue
  }
</script>

<section class="page" aria-labelledby="data-heading">
  <header class="page-header">
    <div>
      <h1 id="data-heading">Historical data</h1>
      <p class="page-copy">
        Browse persisted normalized candle availability first, then drill into exact candles,
        linked evidence, and optional raw payload metadata for the current candle scope.
      </p>
    </div>
  </header>

  <section class="panel" aria-labelledby="availability-heading">
    <div class="panel-header">
      <div>
        <h2 id="availability-heading">Available normalized candle entries</h2>
        <p>Open the route and start from persisted venue, symbol, and asset class availability.</p>
      </div>
    </div>

    {#if availabilityLoading}
      <p class="status">Loading candle availability…</p>
    {:else if availabilityError}
      <p class="alert" role="alert">{availabilityError}</p>
    {:else if availabilityCompatibilityNote}
      <p class="note">{availabilityCompatibilityNote}</p>
    {:else if availabilityItems.length === 0}
      <p class="empty">No normalized candle availability was found yet.</p>
    {:else}
      <div class="availability-list" aria-label="Candle availability entries">
        {#each availabilityItems as item (availabilityKey(item))}
          <button
            class:selected={selectedAvailabilityKey === availabilityKey(item)}
            class="availability-card"
            type="button"
            onclick={() => handleAvailabilitySelection(item)}
          >
            <div class="availability-card__header">
              <strong>{item.venue}</strong>
              <span>{item.symbol}</span>
              <span>{item.assetClass}</span>
            </div>
            <p class="availability-card__default">
              Default slice: {item.defaultSlice.timeframe} · {formatAvailabilityRange(item.defaultSlice)}
            </p>
            <ul class="availability-timeframes">
              {#each item.timeframes as timeframeSummary (`${item.symbol}-${timeframeSummary.timeframe}`)}
                <li>
                  <strong>{timeframeSummary.timeframe}</strong>
                  <span>{timeframeSummary.count} candles</span>
                  <span>{formatAvailabilityRange(timeframeSummary)}</span>
                </li>
              {/each}
            </ul>
          </button>
        {/each}
      </div>
    {/if}
  </section>

  <form class="filter-form" aria-label="Historical data filters" onsubmit={handleSubmit}>
    <label>
      <span>Venue</span>
      <select bind:value={venue}>
        <option value="">Select venue</option>
        <option value="hyperliquid-perps">hyperliquid-perps</option>
      </select>
    </label>
    <label>
      <span>Symbol</span>
      <input bind:value={symbol} placeholder="e.g. BTCUSD" />
    </label>
    <label>
      <span>Asset class</span>
      <select bind:value={assetClass}>
        <option value="">Select asset class</option>
        <option value="crypto">crypto</option>
        <option value="future">future</option>
      </select>
    </label>
    <label>
      <span>Timeframe</span>
      <select bind:value={timeframe}>
        <option value="">Select timeframe</option>
        <option value="1m">1m</option>
        <option value="5m">5m</option>
        <option value="15m">15m</option>
        <option value="1h">1h</option>
        <option value="4h">4h</option>
        <option value="1d">1d</option>
      </select>
    </label>
    <div class="filter-form__range">
       <p class="filter-form__range-label">Date range</p>
       <DateRangePicker
        bind:startValue={utcStart}
        bind:endValue={utcEnd}
        showValidation={showRangeValidation}
        disabled={candlesLoading}
        showPresets={selectedAvailabilityTimeframe !== null}
        presetAnchor={selectedAvailabilityTimeframe?.end ?? null}
        min={selectedAvailabilityTimeframe?.start ?? null}
        max={selectedAvailabilityTimeframe?.end ?? null}
        timeframeDurationMs={selectedTimeframeDurationMs}
        maxIntervals={selectedTimeframeDurationMs === null ? null : maxIntervals}
        outOfBoundsMessage={selectedAvailabilityTimeframe
           ? 'Range must stay within the selected availability window.'
          : undefined}
        maxIntervalsMessage={selectedTimeframeMaxIntervalsMessage}
      />
      {#if selectedAvailabilityTimeframe}
        <p class="filter-form__range-note">
          Presets anchor to the latest persisted {selectedAvailabilityTimeframe.timeframe} candle end for
          the selected availability entry.
        </p>
      {/if}
    </div>
    <label>
      <span>Ingestion run ID</span>
      <input bind:value={ingestionRunId} placeholder="Optional" spellcheck="false" />
    </label>
    <div class="actions-row">
      <p class="actions-copy">
        Use the exact filters below any time. Broad raw payload metadata stays optional for the current candle scope.
      </p>
      <div class="form-actions">
        <button class="primary" type="submit" disabled={candlesLoading}>Load candles</button>
        <button
          class="secondary"
          type="button"
          disabled={!currentScope || candlesLoading || rawPayloadsLoading}
          onclick={loadRawPayloadsForCurrentScope}
        >
          Load raw payload metadata
        </button>
      </div>
    </div>
  </form>

  <section class="panel" aria-labelledby="backfill-heading">
    <div class="panel-header">
      <div>
        <h2 id="backfill-heading">Start historical backfill</h2>
        <p>Explicitly create a durable job from the current data scope. Browsing, loading, and selecting candles stay read-only unless you use this action.</p>
      </div>
    </div>

    <div class="backfill-panel">
      <p class="note">Current backfill request uses the current form scope. For `hyperliquid-perps`, the backend currently expects futures job scope even though read browsing uses the existing `crypto` label.</p>
      <div class="backfill-fields">
        <label>
          <span>Idempotency key</span>
          <input bind:value={backfillIdempotencyKey} placeholder="Optional" spellcheck="false" />
        </label>
        <label>
          <span>Backfill page size</span>
          <input bind:value={backfillPageSize} inputmode="numeric" placeholder="0 uses backend default" />
        </label>
      </div>
      <div class="form-actions backfill-actions">
        <button class="warning" type="button" disabled={jobSubmitting} onclick={startHistoricalBackfill}>
          Start historical backfill
        </button>
      </div>
    </div>

    {#if jobError}
      <p class="alert" role="alert">{jobError}</p>
    {/if}

    {#if createdJob}
      <div class="success success-panel" aria-live="polite">
        <p>
          Created job {createdJob.id} with status {createdJob.status}.
          <a href={`/jobs/${encodeURIComponent(createdJob.id)}`} use:link>Open created job</a>
        </p>
        <button
          class="secondary"
          type="button"
          onclick={() => void loadAvailability()}
          disabled={availabilityLoading}
        >
          {availabilityLoading ? 'Reloading availability…' : 'Reload availability'}
        </button>
      </div>
    {/if}
  </section>

  {#if validationErrors.length > 0}
    <div class="alert" role="alert">
      <ul>
        {#each validationErrors as error (error)}
          <li>{error}</li>
        {/each}
      </ul>
    </div>
  {/if}

  {#if candlesLoading}
    <p class="status">Loading normalized candles…</p>
  {/if}

  {#if candlesError}
    <p class="alert" role="alert">{candlesError}</p>
  {/if}

  {#if rawPayloadsError}
    <p class="alert" role="alert">{rawPayloadsError}</p>
  {/if}

  {#if currentScope}
    <section class="summary" aria-label="Data summary">
      <div class="summary-card">
        <h2>Summary</h2>
        <p>{candles.length} normalized candles</p>
        <p>{rawPayloadsLoaded ? `${rawPayloads.length} raw payload rows` : 'Raw payload metadata not loaded yet'}</p>
      </div>
      <div class="summary-card">
        <h2>Selected candle</h2>
        <p>{formatSelectedCandleLabel(selectedCandle)}</p>
        <p>
          {#if !selectedCandle}
            Select a normalized candle to inspect linked evidence.
          {:else if linkedEvidenceLoading}
            Loading linked evidence…
          {:else}
            {linkedEvidence.length} linked raw payload rows
          {/if}
        </p>
      </div>
      {#if selectedAvailability}
        <div class="summary-card">
          <h2>Selected availability entry</h2>
          <p>{selectedAvailability.venue} / {selectedAvailability.symbol} / {selectedAvailability.assetClass}</p>
          <p>{selectedAvailability.defaultSlice.timeframe} default · {formatAvailabilityRange(selectedAvailability.defaultSlice)}</p>
        </div>
      {/if}
    </section>

    <section class="panel" aria-labelledby="candles-heading">
      <div class="panel-header">
        <div>
          <h2 id="candles-heading">Normalized candles</h2>
          <p>Returned candles default-select the first row and load linked evidence by provenance.</p>
        </div>
      </div>

      {#if candles.length > 0}
        <p class="selection-banner" aria-live="polite">
          <strong>Selected candle:</strong>
          {formatSelectedCandleLabel(selectedCandle)}
        </p>

        <DataCandlestickChart rows={chartRows} />

        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th scope="col">Start</th>
                <th scope="col">Open</th>
                <th scope="col">Close</th>
                <th scope="col">Quality</th>
                <th scope="col">Evidence</th>
              </tr>
            </thead>
            <tbody>
              {#each candles as candle (candle.identity)}
                <tr class:selected={selectedCandleIdentity === candle.identity}>
                   <td>{formatLocalDateTime(candle.start)}</td>
                  <td>{candle.open}</td>
                  <td>{candle.close}</td>
                  <td>{candle.quality}</td>
                  <td>
                    <button
                      class="secondary table-button"
                      type="button"
                      disabled={selectedCandleIdentity === candle.identity}
                      aria-pressed={selectedCandleIdentity === candle.identity}
                      onclick={() => selectCandle(candle)}
                    >
                      {selectedCandleIdentity === candle.identity ? 'Selected' : 'Select'}
                    </button>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {:else if !candlesError && !candlesLoading}
        <p class="empty">No normalized candles matched these filters.</p>
      {/if}
    </section>

    <section class="panel" aria-labelledby="evidence-heading">
      <div class="panel-header">
        <h2 id="evidence-heading">Linked raw evidence</h2>
        {#if selectedCandle}
          <p>{selectedCandle.provenanceSource} / {selectedCandle.provenanceIdentity}</p>
        {/if}
      </div>

      {#if !selectedCandle}
        <p class="empty">Select a normalized candle row to load linked raw evidence.</p>
      {:else if linkedEvidenceLoading}
        <p class="status">Loading linked raw evidence…</p>
      {:else if linkedEvidenceError}
        <p class="alert" role="alert">{linkedEvidenceError}</p>
      {:else if linkedEvidence.length === 0}
        <p class="empty">No linked raw evidence was found for this candle.</p>
      {:else}
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th scope="col">ID</th>
                <th scope="col">Endpoint</th>
                <th scope="col">Entity hint</th>
              </tr>
            </thead>
            <tbody>
              {#each linkedEvidence as item (item.id)}
                <tr>
                  <td>{item.id}</td>
                  <td>{item.endpoint}</td>
                  <td>{item.entityHint}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>

    <section class="panel" aria-labelledby="raw-payloads-heading">
      <div class="panel-header">
        <div>
          <h2 id="raw-payloads-heading">Raw payload metadata</h2>
          <p>Broad raw payload browsing is explicit and secondary to the current normalized candle scope.</p>
        </div>
      </div>

      {#if rawPayloadsLoading}
        <p class="status">Loading raw payload metadata…</p>
      {:else if !rawPayloadsLoaded}
        <p class="empty">Load raw payload metadata when you want broader browsing for this candle scope.</p>
      {:else if rawPayloads.length > 0}
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th scope="col">ID</th>
                <th scope="col">Endpoint</th>
                <th scope="col">Request type</th>
                <th scope="col">Received at</th>
                <th scope="col">Detail</th>
              </tr>
            </thead>
            <tbody>
              {#each rawPayloads as item (item.id)}
                <tr>
                  <td>{item.id}</td>
                  <td>{item.endpoint}</td>
                  <td>{item.requestType}</td>
                   <td>{formatLocalDateTime(item.receivedAt)}</td>
                  <td>
                    <button class="secondary table-button" type="button" onclick={() => openRawPayloadDetail(item.id)}>
                      View detail
                    </button>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {:else if !rawPayloadsError}
        <p class="empty">No raw payload metadata matched these filters.</p>
      {/if}
    </section>
  {/if}

  {#if detailDrawerOpen}
    <div class="drawer-backdrop" aria-hidden="true" onclick={closeRawPayloadDetail}></div>
    <div class="drawer" role="dialog" aria-modal="true" aria-label="Raw payload detail">
      <div class="drawer-header">
        <div>
          <h2>Raw payload detail</h2>
          <p class="drawer-copy">Inspect the bounded payload preview and copy the storage ref for the full body when needed.</p>
        </div>
        <button class="secondary" type="button" onclick={closeRawPayloadDetail}>Close</button>
      </div>

      {#if detailLoading}
        <p class="status">Loading raw payload detail…</p>
      {:else if detailError}
        <p class="alert" role="alert">{detailError}</p>
      {:else if rawPayloadDetail && selectedRawPayloadId}
        <div class="detail-actions">
          <p class:detail-warning={rawPayloadDetail.responseBodyPreviewTruncated} class="detail-note">
            {#if rawPayloadDetail.responseBodyPreviewTruncated}
              This API exposes only a truncated preview. Use the body ref below to inspect the full payload in storage.
            {:else}
              This API exposes the available payload preview and storage ref for follow-up inspection.
            {/if}
          </p>

          <div class="detail-actions__buttons">
            <button
              class="secondary"
              type="button"
              onclick={() => copyDetailValue(rawPayloadDetail!.responseBodyPreview, 'Preview')}
            >
              Copy preview
            </button>
            <button
              class="secondary"
              type="button"
              onclick={() => copyDetailValue(rawPayloadDetail!.metadata.payloadBodyRef, 'Body ref')}
            >
              Copy body ref
            </button>
          </div>
        </div>

        {#if detailFeedback}
          <p class="status detail-feedback" aria-live="polite">{detailFeedback}</p>
        {/if}

        <dl class="detail-grid">
          <div><dt>ID</dt><dd>{selectedRawPayloadId}</dd></div>
          <div><dt>Endpoint</dt><dd>{rawPayloadDetail.metadata.endpoint}</dd></div>
          <div><dt>Request hash</dt><dd>{rawPayloadDetail.metadata.requestPayloadHash}</dd></div>
          <div><dt>Response hash</dt><dd>{rawPayloadDetail.metadata.responseBodyHash}</dd></div>
          <div><dt>Body ref</dt><dd>{rawPayloadDetail.metadata.payloadBodyRef}</dd></div>
          <div><dt>Instrument hint</dt><dd>{rawPayloadDetail.metadata.symbol ?? '—'}</dd></div>
          <div><dt>Timeframe</dt><dd>{rawPayloadDetail.metadata.timeframe ?? '—'}</dd></div>
           <div><dt>Range</dt><dd>{formatLocalDateTime(rawPayloadDetail.metadata.start)} → {formatLocalDateTime(rawPayloadDetail.metadata.end)}</dd></div>
          <div><dt>Body bytes</dt><dd>{rawPayloadDetail.responseBodySizeBytes}</dd></div>
          <div><dt>Truncated</dt><dd>{rawPayloadDetail.responseBodyPreviewTruncated ? 'Yes' : 'No'}</dd></div>
        </dl>

        <pre class="preview">{rawPayloadDetail.responseBodyPreview}</pre>
      {/if}
    </div>
  {/if}
</section>

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: var(--space-24);
  }

  .page-header {
    display: flex;
    justify-content: space-between;
    gap: var(--space-16);
    align-items: flex-start;
  }

  .page-copy {
    max-width: 70ch;
  }

  .filter-form {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
    gap: var(--space-16);
    padding: var(--space-16);
    border: 1px solid var(--border);
    border-radius: var(--radius-default);
    background: var(--surface-raised);
  }

  .actions-row {
    grid-column: 1 / -1;
  }

  .backfill-panel {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
  }

  .backfill-fields {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: var(--space-16);
  }

  .backfill-fields label {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }

  .backfill-actions {
    justify-content: flex-start;
  }

  .success {
    color: var(--color-success-green);
  }

  .success-panel {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    align-items: center;
    gap: var(--space-12);
  }

  .success-panel p {
    margin: 0;
  }

  .success a {
    color: inherit;
    font-weight: 500;
  }

  .filter-form__range {
    grid-column: 1 / -1;
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
    color: var(--text-h);
  }

  .filter-form__range-label {
    margin: 0;
    font-weight: 700;
  }

  .filter-form__range-note {
    margin: 0;
    color: var(--text);
  }

  .filter-form label {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    color: var(--text-h);
    font-weight: 500;
  }

  .filter-form input,
  .filter-form select {
    width: 100%;
    box-sizing: border-box;
    padding: var(--space-12);
    border-radius: var(--radius-input);
    border: 1px solid var(--border);
    font: inherit;
    color: var(--color-input-text);
    background: var(--color-input-bg);
  }

  .form-actions {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-8);
    align-items: center;
    justify-content: flex-end;
  }

  .actions-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-16);
    padding-top: var(--space-8);
    border-top: 1px solid var(--border);
  }

  .actions-copy {
    margin: 0;
    color: var(--text);
    max-width: 44ch;
  }

  .summary {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(16rem, 1fr));
    gap: var(--space-16);
  }

  .summary-card,
  .panel,
  .drawer,
  .availability-card {
    border: 1px solid var(--border);
    border-radius: var(--radius-default);
    background: var(--surface-raised);
  }

  .summary-card,
  .panel {
    padding: var(--space-16);
  }

  .panel-header {
    display: flex;
    justify-content: space-between;
    gap: var(--space-16);
    align-items: baseline;
    margin-bottom: var(--space-16);
  }

  .availability-list {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(18rem, 1fr));
    gap: var(--space-16);
  }

  .availability-card {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
    padding: var(--space-16);
    text-align: left;
    color: inherit;
  }

  .availability-card.selected {
    border-color: var(--accent);
    background: var(--accent-bg);
  }

  .availability-card__header {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-8);
  }

  .availability-card__default {
    margin: 0;
  }

  .availability-timeframes {
    margin: 0;
    padding-left: var(--space-20);
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }

  .availability-timeframes li {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .table-wrap {
    overflow-x: auto;
  }

  .selection-banner {
    margin: 0 0 var(--space-16);
    padding: var(--space-12) var(--space-16);
    border: 1px solid var(--border);
    border-radius: var(--radius-default);
    background: var(--bg-subtle, var(--bg));
  }

  table {
    width: 100%;
    border-collapse: collapse;
  }

  th,
  td {
    padding: var(--space-8);
    border-top: 1px solid var(--border);
    text-align: left;
    vertical-align: top;
  }

  tr.selected {
    background: var(--accent-bg);
  }

  .table-button {
    padding-inline: var(--space-12);
  }

  .alert {
    padding: var(--space-12) var(--space-16);
    border: 1px solid var(--danger-border);
    border-radius: var(--radius-default);
    background: var(--danger-bg);
    color: var(--text-h);
  }

  .alert ul {
    margin: 0;
    padding-left: var(--space-20);
  }

  .status,
  .empty {
    color: var(--text);
  }

  .note {
    margin: 0;
    padding: var(--space-12) var(--space-16);
    border: 1px solid var(--border);
    border-radius: var(--radius-default);
    background: var(--bg-subtle, var(--bg));
    color: var(--text);
  }

  .drawer {
    position: fixed;
    inset: var(--space-20);
    width: min(72rem, calc(100vw - 2 * var(--space-20)));
    height: min(85vh, calc(100vh - 2 * var(--space-20)));
    margin: auto;
    padding: var(--space-16);
    overflow: auto;
    z-index: 20;
  }

  .drawer-backdrop {
    position: fixed;
    inset: 0;
    background: color-mix(in srgb, var(--bg) 75%, transparent);
    z-index: 19;
  }

  .drawer-header {
    display: flex;
    justify-content: space-between;
    gap: var(--space-16);
    align-items: flex-start;
    margin-bottom: var(--space-16);
  }

  .drawer-copy {
    margin: var(--space-8) 0 0;
    color: var(--text);
    max-width: 56ch;
  }

  .detail-actions {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    gap: var(--space-12);
    margin-bottom: var(--space-16);
  }

  .detail-note {
    margin: 0;
    max-width: 56ch;
  }

  .detail-warning {
    color: var(--warning-text, var(--text-h));
  }

  .detail-actions__buttons {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-8);
  }

  .detail-feedback {
    margin-top: 0;
  }

  .detail-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
    gap: var(--space-12);
    margin: 0 0 var(--space-16);
  }

  .detail-grid dt {
    font-weight: 700;
    color: var(--text-h);
  }

  .detail-grid dd {
    margin: var(--space-4) 0 0;
  }

  .preview {
    overflow: auto;
    padding: var(--space-12);
    border-radius: var(--radius-default);
    background: var(--bg);
    color: var(--text-h);
  }

  @media (max-width: 767px) {
    .drawer {
      inset: var(--space-16);
      width: auto;
      height: auto;
    }

    .panel-header,
    .page-header {
      flex-direction: column;
      align-items: flex-start;
    }

    .actions-row,
    .form-actions,
    .detail-actions,
    .detail-actions__buttons {
      flex-direction: column;
      align-items: stretch;
    }
  }
</style>
