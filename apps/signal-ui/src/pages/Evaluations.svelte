<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import DateRangePicker from '../components/DateRangePicker.svelte'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { formatCompactIdentifier } from '../lib/compact-identifier'
  import {
    createSignalStrategyWorkspaceApiForAuth,
    type EvaluationDetail,
    type EvaluationRow,
    type StrategyVersionRow,
  } from '../lib/strategy-workspace/api'
  import { validateRange } from '../lib/date-range'
  import { formatLocalDateTime } from '../lib/timestamp'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'

  const workspaceApi = $derived.by(() =>
    createSignalStrategyWorkspaceApiForAuth({ baseUrl: appBaseUrl, authStore }),
  )

  let { params = {} } = $props<{ params?: { strategyId?: string; version?: string } }>()

  let strategiesLoading = $state(true)
  let strategiesError = $state<string | null>(null)
  let strategies = $state<StrategyVersionRow[]>([])

  let historyLoading = $state(true)
  let historyError = $state<string | null>(null)
  let history = $state<EvaluationRow[]>([])

  let runSelection = $state('')
  let rangeStart = $state<Date | undefined>()
  let rangeEnd = $state<Date | undefined>()
  let quantity = $state('1')
  let note = $state('')
  let runErrors = $state<string[]>([])
  let showRangeValidation = $state(false)
  let runSubmitting = $state(false)
  let createdRun = $state<EvaluationDetail | null>(null)
  let createdRunError = $state<string | null>(null)
  let lastRouteDrivenSelection = $state<string | null>(null)

  let historyFilterStrategyId = $state('')
  let historyFilterStatus = $state('')

  onMount(() => {
    void loadStrategies()
    void loadHistory()
  })

  $effect(() => {
    syncRunSelectionFromRoute()
  })

  async function loadStrategies() {
    strategiesLoading = true
    strategiesError = null
    try {
      const items = await workspaceApi.listStrategies()
      strategies = items.filter((item) => item.status === 'ready')
      syncRunSelectionFromRoute()
    } catch (error) {
      strategies = []
      strategiesError = error instanceof Error ? error.message : 'Failed to load ready strategy versions'
    } finally {
      strategiesLoading = false
    }
  }

  async function loadHistory() {
    historyLoading = true
    historyError = null
    try {
      history = await workspaceApi.listEvaluationBacktests({
        strategyId: historyFilterStrategyId.trim() || undefined,
        status: historyFilterStatus.trim() || undefined,
      })
    } catch (error) {
      history = []
      historyError = error instanceof Error ? error.message : 'Failed to load evaluation history'
    } finally {
      historyLoading = false
    }
  }

  async function applyHistoryFilters(event: SubmitEvent) {
    event.preventDefault()
    await loadHistory()
  }

  function strategyKey(item: { strategyId: string; version: string }): string {
    return JSON.stringify([item.strategyId, item.version])
  }

  function parseSelection(selection: string): { strategyId: string; version: string } | null {
    try {
      const parsed = JSON.parse(selection)
      return Array.isArray(parsed) && typeof parsed[0] === 'string' && typeof parsed[1] === 'string'
        ? { strategyId: parsed[0], version: parsed[1] }
        : null
    } catch {
      return null
    }
  }

  function routeSelectionCandidate(): string | null {
    const strategyId = params.strategyId?.trim() ?? ''
    const version = params.version?.trim() ?? ''
    return strategyId && version ? strategyKey({ strategyId, version }) : null
  }

  function syncRunSelectionFromRoute() {
    const candidate = routeSelectionCandidate()
    if (!candidate) {
      lastRouteDrivenSelection = null
      return
    }

    if (strategies.some((item) => strategyKey(item) === candidate)) {
      runSelection = candidate
      lastRouteDrivenSelection = candidate
      return
    }

    if (runSelection === lastRouteDrivenSelection) {
      runSelection = ''
    }
    lastRouteDrivenSelection = candidate
  }

  function validateRunForm(): { start: Date; end: Date } | null {
    const errors: string[] = []
    const selected = parseSelection(runSelection)
    const quantityText = String(quantity)
    if (!selected) {
      errors.push('Select a ready strategy version before starting an evaluation.')
    }
    const rangeErrors = validateRange({
      start: rangeStart,
      end: rangeEnd,
       requiredStartMessage: 'Enter a valid start timestamp.',
       requiredEndMessage: 'Enter a valid end timestamp.',
       invalidStartMessage: 'Enter a valid start timestamp.',
       invalidEndMessage: 'Enter a valid end timestamp.',
       notEarlierMessage: 'Start must be earlier than end.',
    })
    const start = rangeErrors.length === 0 ? rangeStart ?? null : null
    const end = rangeErrors.length === 0 ? rangeEnd ?? null : null
    errors.push(...rangeErrors)
    if (!quantityText.trim() || Number(quantityText) <= 0) {
      errors.push('Quantity must be a positive number.')
    }
    runErrors = errors
    return errors.length === 0 && start && end ? { start, end } : null
  }

  async function submitRun(event: SubmitEvent) {
    event.preventDefault()
    showRangeValidation = true
    createdRun = null
    createdRunError = null
    const selected = parseSelection(runSelection)
    const validated = validateRunForm()
    if (!selected || !validated) {
      return
    }
    const { start, end } = validated

    runSubmitting = true
    try {
      createdRun = await workspaceApi.createEvaluationBacktest({
        body: {
          strategyId: selected.strategyId,
          strategyVersion: selected.version,
          start,
          end,
          quantity: Number(String(quantity)),
          ...(note.trim() ? { note: note.trim() } : {}),
        },
      })
      await loadHistory()
    } catch (error) {
      createdRunError = error instanceof Error ? error.message : 'Failed to create evaluation run'
    } finally {
      runSubmitting = false
    }
  }

  function formatInstrument(item: EvaluationRow): string {
    return `${item.instrument.venue}/${item.instrument.symbol}/${item.instrument.assetClass}`
  }

  function formatMetricValue(value: number | undefined): string {
    return value === undefined ? '—' : String(value)
  }
</script>

<section class="page" aria-labelledby="evaluations-heading">
  <header class="page-header">
    <div>
      <h1 id="evaluations-heading">Evaluations</h1>
      <p class="muted">Run deterministic backtests from saved ready strategy versions and inspect the persisted history.</p>
    </div>
  </header>

  <section class="panel" aria-labelledby="run-heading">
    <div class="panel-header">
      <h2 id="run-heading">Run evaluation</h2>
      <p class="muted">Only saved ready versions are eligible for v0 evaluation.</p>
    </div>

    {#if strategiesError}
      <p class="error" role="alert">{strategiesError}</p>
    {/if}

    <form class="run-form" onsubmit={submitRun}>
      <label>
        Strategy version
        <select bind:value={runSelection} disabled={strategiesLoading}>
          <option value="">Select a ready version</option>
          {#each strategies as item (strategyKey(item))}
            <option value={strategyKey(item)}>{item.displayName} — {item.strategyId}/{item.version}</option>
          {/each}
        </select>
      </label>

      <div class="field-row field-row--range">
        <div class="run-form__range">
           <p class="run-form__range-label">Date range</p>
           <DateRangePicker
            bind:startValue={rangeStart}
            bind:endValue={rangeEnd}
            disabled={runSubmitting}
            showValidation={showRangeValidation}
            presetAnchor={new Date()}
             requiredStartMessage="Enter a valid start timestamp."
             requiredEndMessage="Enter a valid end timestamp."
             invalidStartMessage="Enter a valid start timestamp."
             invalidEndMessage="Enter a valid end timestamp."
             notEarlierMessage="Start must be earlier than end."
          />
        </div>
        <label>
          Quantity
          <input bind:value={quantity} type="number" min="0.000001" step="any" />
        </label>
      </div>

      <label>
        Note
        <textarea bind:value={note} rows="3" placeholder="Operator note for later review"></textarea>
      </label>

      {#if runErrors.length > 0}
        <div role="alert">
          <p class="error-title">Run request needs fixes</p>
          <ul>
            {#each runErrors as error (error)}
              <li>{error}</li>
            {/each}
          </ul>
        </div>
      {/if}

      {#if createdRunError}
        <p class="error" role="alert">{createdRunError}</p>
      {/if}

      {#if createdRun}
        <div class="result-panel" role="status">
          <p>
            Created run
            <code title={createdRun.runId} aria-label={createdRun.runId}>
              {formatCompactIdentifier(createdRun.runId)}
            </code>
            with status <strong>{createdRun.status}</strong>.
          </p>
          <p class="muted">Open detail to inspect summary, report, and evidence tables.</p>
          {#if createdRun.status === 'failed' && createdRun.failureReason === 'replay-data-unavailable'}
            <p class="warning-copy">
              Next action: inspect local data on
              <a class="button-link" href="/data" use:link>Historical data</a>
              and start a bounded historical backfill before retrying this run.
            </p>
          {/if}
          <a class="button-link" href={`/evaluations/${encodeURIComponent(createdRun.runId)}`} use:link>
            Open evaluation detail
          </a>
        </div>
      {/if}

      <div class="actions">
        <button type="submit" class="primary" disabled={runSubmitting}>
          {runSubmitting ? 'Running…' : 'Start evaluation'}
        </button>
      </div>
    </form>
  </section>

  <section class="panel" aria-labelledby="history-heading">
    <div class="panel-header">
      <h2 id="history-heading">History</h2>
      <p class="muted">Filter by strategy id and run status.</p>
    </div>

    <form class="filters" onsubmit={applyHistoryFilters}>
      <label>
        Strategy id
        <input bind:value={historyFilterStrategyId} type="text" placeholder="strategy-a" />
      </label>
      <label>
        Status
        <select bind:value={historyFilterStatus}>
          <option value="">All</option>
          <option value="pending">pending</option>
          <option value="running">running</option>
          <option value="completed">completed</option>
          <option value="failed">failed</option>
        </select>
      </label>
      <button type="submit" class="secondary">Apply filters</button>
    </form>

    {#if historyError}
      <p class="error" role="alert">{historyError}</p>
    {:else if historyLoading}
      <p class="muted" role="status">Loading evaluation history…</p>
    {:else if history.length === 0}
      <p class="muted">No evaluation runs matched the current filters.</p>
    {:else}
      <div class="history-list">
        {#each history as item (item.runId)}
          <article class="history-item">
            <div class="history-item__header">
              <div>
                <p class="history-item__eyebrow">Run</p>
                <a href={`/evaluations/${encodeURIComponent(item.runId)}`} use:link>
                  <code title={item.runId} aria-label={item.runId}>
                    {formatCompactIdentifier(item.runId)}
                  </code>
                </a>
              </div>
              <div class="history-item__header-meta">
                <span><strong>Status:</strong> {item.status}</span>
                <span><strong>Decision:</strong> {item.decision ?? '—'}</span>
              </div>
            </div>

            <dl class="history-item__grid">
              <div>
                <dt>Strategy</dt>
                <dd>{item.strategyId}/{item.strategyVersion}</dd>
              </div>
              <div>
                <dt>Artifact hash</dt>
                <dd>
                  <code title={item.strategyArtifactHash} aria-label={item.strategyArtifactHash}>
                    {formatCompactIdentifier(item.strategyArtifactHash)}
                  </code>
                </dd>
              </div>
              <div>
                <dt>Instrument</dt>
                <dd>{formatInstrument(item)}</dd>
              </div>
              <div>
                <dt>Timeframe</dt>
                <dd>{item.timeframe}</dd>
              </div>
              <div class="history-item__wide">
                <dt>Tested range</dt>
                <dd>{formatLocalDateTime(item.testedRangeStart)} → {formatLocalDateTime(item.testedRangeEnd)}</dd>
              </div>
              <div>
                <dt>Metrics</dt>
                <dd class="history-metrics">
                  <span>Trades: {formatMetricValue(item.metrics?.tradeCount)}</span>
                  <span>Blocked: {formatMetricValue(item.metrics?.blockedGovernorDecisionCount)}</span>
                  <span>Rejected: {formatMetricValue(item.metrics?.rejectedGovernorDecisionCount)}</span>
                </dd>
              </div>
              <div class="history-item__wide">
                <dt>Lifecycle</dt>
                <dd class="history-lifecycle">
                   <span>Created: {formatLocalDateTime(item.createdAt)}</span>
                   <span>Updated: {formatLocalDateTime(item.updatedAt)}</span>
                </dd>
              </div>
            </dl>

            <a class="history-item__link" href={`/evaluations/${encodeURIComponent(item.runId)}`} use:link>
              Open evaluation detail
            </a>
          </article>
        {/each}
      </div>
    {/if}
  </section>
</section>

<style>
  .page,
  .run-form,
  .filters,
  .field-row {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
  }

  .page {
    gap: var(--space-24);
  }

  .page-header,
  .panel-header,
  .actions {
    display: flex;
    gap: var(--space-16);
    justify-content: space-between;
    align-items: flex-start;
  }

  .panel {
    border: 1px solid var(--border);
    border-radius: var(--radius-default);
    padding: var(--space-16);
  }

  .filters {
    display: flex;
    flex-direction: row;
    gap: var(--space-16);
    justify-content: flex-start;
    align-items: flex-end;
    flex-wrap: wrap;
  }

  .field-row {
    flex-direction: row;
    flex-wrap: wrap;
  }

  label {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    font-weight: 500;
  }

  .field-row > label {
    flex: 1 1 12rem;
    min-width: 0;
  }

  .field-row--range {
    align-items: flex-start;
  }

  .run-form__range {
    flex: 1 1 100%;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
  }

  .run-form__range-label {
    margin: 0;
    font-weight: 700;
  }

  .filters > label {
    flex: 0 1 14rem;
    min-width: min(14rem, 100%);
  }

  .filters > button {
    align-self: flex-end;
  }

  input,
  select,
  textarea {
    font: inherit;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-default);
    padding: var(--space-12);
  }

  .history-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
  }

  .history-item {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
    border-top: 1px solid var(--border);
    padding-top: var(--space-16);
  }

  .history-item:first-child {
    border-top: 0;
    padding-top: 0;
  }

  .history-item__header,
  .history-item__header-meta,
  .history-lifecycle,
  .history-metrics {
    display: flex;
    gap: var(--space-8);
    flex-wrap: wrap;
  }

  .history-item__header {
    justify-content: space-between;
    align-items: flex-start;
    gap: var(--space-16);
  }

  .history-item__header-meta {
    color: var(--text-muted);
    justify-content: flex-end;
  }

  .history-item__eyebrow {
    margin: 0 0 var(--space-4);
    color: var(--text-muted);
  }

  .history-item__grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--space-12) var(--space-16);
    margin: 0;
  }

  .history-item__grid dt {
    color: var(--text-muted);
  }

  .history-item__grid dd {
    margin: 0;
    overflow-wrap: anywhere;
  }

  .history-item__grid code {
    display: inline-block;
    max-width: 100%;
    vertical-align: top;
  }

  .history-item__wide {
    grid-column: 1 / -1;
  }

  .history-item__link {
    align-self: flex-start;
  }

  .muted {
    color: var(--text-muted);
  }

  .error,
  .error-title {
    color: var(--color-danger-red);
  }

  .warning-copy {
    margin: 0;
    color: var(--color-warning-orange);
  }

  .result-panel {
    border: 1px solid var(--border);
    border-radius: var(--radius-default);
    padding: var(--space-12);
  }

  .button-link {
    display: inline-flex;
    align-items: center;
    padding: 4px 20px;
    border: 1px solid var(--border);
    border-radius: var(--radius-default);
    text-decoration: none;
    color: var(--text);
  }

  @media (max-width: 900px) {
    .page-header,
    .panel-header,
    .filters,
    .field-row,
    .history-item__header {
      flex-direction: column;
      align-items: stretch;
    }

    .history-item__header-meta {
      justify-content: flex-start;
    }

    .history-item__grid {
      grid-template-columns: 1fr;
    }

    .history-item__wide {
      grid-column: auto;
    }
  }
</style>
