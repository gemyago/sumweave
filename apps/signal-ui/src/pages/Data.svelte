<script lang="ts">
  import DataCandlestickChart from '../components/DataCandlestickChart.svelte'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { toChartCandleRows } from '../lib/data/charting'
  import {
    createSignalDataApiForAuth,
    type DataCandle,
    type DataTimeframe,
    type RawPayloadDetailResponse,
    type RawPayloadMetadata,
  } from '../lib/data/data-api'

  const dataBaseUrl = import.meta.env.VITE_DATA_API_BASE_URL ?? '/api/v1/data'

  const dataApi = $derived.by(() =>
    createSignalDataApiForAuth({ baseUrl: dataBaseUrl, authStore }),
  )

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
  let utcStart = $state('')
  let utcEnd = $state('')
  let ingestionRunId = $state('')

  let validationErrors = $state<string[]>([])
  let hasSubmitted = $state(false)
  let loading = $state(false)

  let candles = $state<DataCandle[]>([])
  let rawPayloads = $state<RawPayloadMetadata[]>([])
  let candlesError = $state<string | null>(null)
  let rawPayloadsError = $state<string | null>(null)
  let searchRequestToken = 0

  let selectedCandleIdentity = $state<number | null>(null)
  let linkedEvidence = $state<RawPayloadMetadata[]>([])
  let linkedEvidenceLoading = $state(false)
  let linkedEvidenceError = $state<string | null>(null)
  let linkedEvidenceRequestToken = 0

  let detailDrawerOpen = $state(false)
  let detailLoading = $state(false)
  let detailError = $state<string | null>(null)
  let selectedRawPayloadId = $state<string | null>(null)
  let rawPayloadDetail = $state<RawPayloadDetailResponse | null>(null)
  let detailRequestToken = 0

  const chartRows = $derived(toChartCandleRows(candles))
  const selectedCandle = $derived(
    selectedCandleIdentity === null
      ? null
      : candles.find((candle) => candle.identity === selectedCandleIdentity) ?? null,
  )

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    const parsed = validateAndBuildQuery()
    validationErrors = parsed.errors
    if (parsed.errors.length > 0 || !parsed.start || !parsed.end) {
      return
    }

    hasSubmitted = true
    const requestToken = ++searchRequestToken
    loading = true
    candles = []
    rawPayloads = []
    candlesError = null
    rawPayloadsError = null
    selectedCandleIdentity = null
    linkedEvidenceRequestToken += 1
    linkedEvidence = []
    linkedEvidenceLoading = false
    linkedEvidenceError = null
    detailRequestToken += 1
    detailDrawerOpen = false
    detailLoading = false
    selectedRawPayloadId = null
    rawPayloadDetail = null
    detailError = null

    const api = dataApi
    const [candlesResult, rawPayloadsResult] = await Promise.allSettled([
      api.listCandles({
        venue: venue.trim(),
        symbol: symbol.trim(),
        assetClass: assetClass.trim(),
        timeframe: timeframe.trim(),
        start: parsed.start,
        end: parsed.end,
      }),
      api.listRawPayloads({
        venue: venue.trim(),
        symbol: symbol.trim(),
        assetClass: assetClass.trim(),
        timeframe: timeframe.trim(),
        start: parsed.start,
        end: parsed.end,
        ingestionRunId: ingestionRunId.trim(),
      }),
    ])

    if (requestToken !== searchRequestToken) {
      return
    }

    if (candlesResult.status === 'fulfilled') {
      candles = candlesResult.value.items
    } else {
      candlesError = candlesResult.reason instanceof Error ? candlesResult.reason.message : 'Failed to load normalized candles'
    }

    if (rawPayloadsResult.status === 'fulfilled') {
      rawPayloads = rawPayloadsResult.value.items
    } else {
      rawPayloadsError = rawPayloadsResult.reason instanceof Error ? rawPayloadsResult.reason.message : 'Failed to load raw payload metadata'
    }

    loading = false
  }

  async function selectCandle(candle: DataCandle) {
    selectedCandleIdentity = candle.identity
    const requestToken = ++linkedEvidenceRequestToken
    linkedEvidenceLoading = true
    linkedEvidenceError = null

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
    selectedRawPayloadId = null
    rawPayloadDetail = null
  }

  function validateAndBuildQuery(): { errors: string[]; start: Date | null; end: Date | null } {
    const errors: string[] = []
    const trimmedVenue = venue.trim()
    const trimmedSymbol = symbol.trim()
    const trimmedAssetClass = assetClass.trim()
    const trimmedTimeframe = timeframe.trim()
    const start = parseUtcTimestamp(utcStart)
    const end = parseUtcTimestamp(utcEnd)

    if (!trimmedVenue) errors.push('Venue is required.')
    if (!trimmedSymbol) errors.push('Symbol is required.')
    if (!trimmedAssetClass) errors.push('Asset class is required.')
    if (!trimmedTimeframe) errors.push('Timeframe is required.')
    if (!utcStart.trim()) errors.push('UTC start is required.')
    if (!utcEnd.trim()) errors.push('UTC end is required.')
    if (utcStart.trim() && !start) errors.push('UTC start must be a valid ISO-8601 timestamp.')
    if (utcEnd.trim() && !end) errors.push('UTC end must be a valid ISO-8601 timestamp.')

    if (start && end && start >= end) {
      errors.push('UTC start must be earlier than UTC end.')
    }

    if (start && end && trimmedTimeframe && trimmedTimeframe in timeframeDurationsMs) {
      const timeframeKey = trimmedTimeframe as DataTimeframe
      const maxRange = timeframeDurationsMs[timeframeKey] * maxIntervals
      if (end.getTime() - start.getTime() > maxRange) {
        errors.push(`Selected range exceeds the server limit of 10,000 ${trimmedTimeframe} intervals.`)
      }
    }

    return { errors, start, end }
  }

  function parseUtcTimestamp(value: string): Date | null {
    const trimmed = value.trim()
    if (!trimmed) {
      return null
    }
    if (!/(Z|[+-]\d{2}:\d{2})$/.test(trimmed)) {
      return null
    }
    const date = new Date(trimmed)
    return Number.isNaN(date.getTime()) ? null : date
  }

  function formatDateTime(value: Date | null): string {
    return value ? value.toISOString() : '—'
  }
</script>

<section class="page" aria-labelledby="data-heading">
  <header class="page-header">
    <div>
      <h1 id="data-heading">Historical data</h1>
      <p class="page-copy">
        Browse normalized candles and linked raw payload evidence. Load is manual so the route never auto-queries large ranges.
      </p>
    </div>
  </header>

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
    <label>
      <span>UTC start</span>
      <input bind:value={utcStart} placeholder="2026-06-15T12:00:00Z" spellcheck="false" />
    </label>
    <label>
      <span>UTC end</span>
      <input bind:value={utcEnd} placeholder="2026-06-15T13:00:00Z" spellcheck="false" />
    </label>
    <label>
      <span>Ingestion run ID</span>
      <input bind:value={ingestionRunId} placeholder="Optional" spellcheck="false" />
    </label>
    <div class="form-actions">
      <button class="primary" type="submit" disabled={loading}>Load</button>
    </div>
  </form>

  {#if validationErrors.length > 0}
    <div class="alert" role="alert">
      <ul>
        {#each validationErrors as error (error)}
          <li>{error}</li>
        {/each}
      </ul>
    </div>
  {/if}

  {#if loading}
    <p class="status">Loading normalized candles and raw payload metadata…</p>
  {/if}

  {#if candlesError}
    <p class="alert" role="alert">{candlesError}</p>
  {/if}

  {#if rawPayloadsError}
    <p class="alert" role="alert">{rawPayloadsError}</p>
  {/if}

  {#if hasSubmitted}
    <section class="summary" aria-label="Data summary">
      <div class="summary-card">
        <h2>Summary</h2>
        <p>{candles.length} normalized candles</p>
        <p>{rawPayloads.length} raw payload rows</p>
      </div>
    </section>

    <div class="results-grid">
      <section class="panel" aria-labelledby="candles-heading">
        <div class="panel-header">
          <h2 id="candles-heading">Normalized candles</h2>
          <p>Select a table row to load linked evidence.</p>
        </div>

        {#if candles.length > 0}
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
                    <td>{formatDateTime(candle.start)}</td>
                    <td>{candle.open}</td>
                    <td>{candle.close}</td>
                    <td>{candle.quality}</td>
                    <td>
                      <button class="secondary table-button" type="button" onclick={() => selectCandle(candle)}>
                        Select
                      </button>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {:else if !candlesError && !loading}
          <p class="empty">No normalized candles matched these filters.</p>
        {/if}
      </section>

      <section class="panel" aria-labelledby="raw-payloads-heading">
        <div class="panel-header">
          <h2 id="raw-payloads-heading">Raw payload metadata</h2>
          <p>Response bodies stay bounded until you open detail.</p>
        </div>

        {#if rawPayloads.length > 0}
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
                    <td>{formatDateTime(item.receivedAt)}</td>
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
        {:else if !rawPayloadsError && !loading}
          <p class="empty">No raw payload metadata matched these filters.</p>
        {/if}
      </section>
    </div>

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
  {/if}

  {#if detailDrawerOpen}
    <div class="drawer" role="dialog" aria-modal="false" aria-label="Raw payload detail">
      <div class="drawer-header">
        <h2>Raw payload detail</h2>
        <button class="secondary" type="button" onclick={closeRawPayloadDetail}>Close</button>
      </div>

      {#if detailLoading}
        <p class="status">Loading raw payload detail…</p>
      {:else if detailError}
        <p class="alert" role="alert">{detailError}</p>
      {:else if rawPayloadDetail && selectedRawPayloadId}
        <dl class="detail-grid">
          <div><dt>ID</dt><dd>{selectedRawPayloadId}</dd></div>
          <div><dt>Endpoint</dt><dd>{rawPayloadDetail.metadata.endpoint}</dd></div>
          <div><dt>Request hash</dt><dd>{rawPayloadDetail.metadata.requestPayloadHash}</dd></div>
          <div><dt>Response hash</dt><dd>{rawPayloadDetail.metadata.responseBodyHash}</dd></div>
          <div><dt>Body ref</dt><dd>{rawPayloadDetail.metadata.payloadBodyRef}</dd></div>
          <div><dt>Instrument hint</dt><dd>{rawPayloadDetail.metadata.symbol ?? '—'}</dd></div>
          <div><dt>Timeframe</dt><dd>{rawPayloadDetail.metadata.timeframe ?? '—'}</dd></div>
          <div><dt>Range</dt><dd>{formatDateTime(rawPayloadDetail.metadata.start)} → {formatDateTime(rawPayloadDetail.metadata.end)}</dd></div>
          <div><dt>Preview bytes</dt><dd>{rawPayloadDetail.responseBodySizeBytes}</dd></div>
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
    align-items: end;
  }

  .summary-card,
  .panel,
  .drawer {
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

  .results-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(20rem, 1fr));
    gap: var(--space-24);
  }

  .table-wrap {
    overflow-x: auto;
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

  .drawer {
    position: fixed;
    top: var(--space-20);
    right: var(--space-20);
    bottom: var(--space-20);
    width: min(32rem, calc(100vw - 2 * var(--space-20)));
    padding: var(--space-16);
    overflow: auto;
    z-index: 20;
  }

  .drawer-header {
    display: flex;
    justify-content: space-between;
    gap: var(--space-16);
    align-items: center;
    margin-bottom: var(--space-16);
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
      inset: auto var(--space-16) var(--space-16) var(--space-16);
      top: var(--space-16);
      width: auto;
    }

    .panel-header,
    .page-header {
      flex-direction: column;
      align-items: flex-start;
    }
  }
</style>
