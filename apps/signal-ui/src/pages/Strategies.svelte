<script lang="ts">
  import { onMount } from 'svelte'
  import { link, push } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { formatCompactIdentifier } from '../lib/compact-identifier'
  import {
    createSignalStrategyWorkspaceApiForAuth,
    type CreateStrategyVersionRequest,
    type StrategyDefinition,
    type StrategyFieldError,
    type StrategyValidationResponse,
    type StrategyVersionCandidate,
    type StrategyVersionDetail,
    type StrategyVersionRow,
  } from '../lib/strategy-workspace/api'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'

  const workspaceApi = $derived.by(() =>
    createSignalStrategyWorkspaceApiForAuth({ baseUrl: appBaseUrl, authStore }),
  )

  let { params = {} } = $props<{ params?: { strategyId?: string; version?: string } }>()

  type DraftMode = 'new' | 'duplicate'

  interface StrategyDraft {
    strategyId: string
    version: string
    displayName: string
    notes: string
    venue: string
    symbol: string
    assetClass: string
    active: boolean
    timeframe: string
    fastWindow: number
    slowWindow: number
    parentStrategyId: string
    parentVersion: string
  }

  const supportedAssetClasses = ['crypto']
  const supportedTimeframes = ['1m', '5m', '15m', '1h', '4h', '1d']

  let listLoading = $state(true)
  let listError = $state<string | null>(null)
  let strategies = $state<StrategyVersionRow[]>([])

  let detailLoading = $state(false)
  let detailError = $state<string | null>(null)
  let selectedDetail = $state<StrategyVersionDetail | null>(null)

  let draftMode = $state<DraftMode>('new')
  let draft = $state(createEmptyDraft())
  let draftDirty = $state(false)
  let draftNotice = $state('Draft fields stay local until you validate or save a new ready version.')

  let validationLoading = $state(false)
  let validationResult = $state<StrategyValidationResponse | null>(null)
  let saveLoading = $state(false)
  let saveError = $state<string | null>(null)
  const hasSelectedRoute = $derived(Boolean(params.strategyId?.trim() && params.version?.trim()))

  let activeRouteKey = ''

  onMount(() => {
    void loadStrategies()
  })

  $effect(() => {
    const strategyId = params.strategyId?.trim() ?? ''
    const version = params.version?.trim() ?? ''
    const routeKey = strategyId && version ? `${strategyId}/${version}` : ''
    if (routeKey === activeRouteKey) {
      return
    }
    activeRouteKey = routeKey
    void syncRouteSelection(strategyId, version)
  })

  async function loadStrategies() {
    listLoading = true
    listError = null
    try {
      strategies = await workspaceApi.listStrategies()
    } catch (error) {
      listError = error instanceof Error ? error.message : 'Failed to load strategy workspace'
      strategies = []
    } finally {
      listLoading = false
    }
  }

  async function syncRouteSelection(strategyId: string, version: string) {
    detailError = null
    if (!strategyId || !version) {
      selectedDetail = null
      detailLoading = false
      return
    }

    detailLoading = true
    try {
      selectedDetail = await workspaceApi.getStrategyVersion({ strategyId, version })
    } catch (error) {
      detailError = error instanceof Error ? error.message : 'Failed to load strategy version'
      selectedDetail = null
    } finally {
      detailLoading = false
    }
  }

  function createEmptyDraft(): StrategyDraft {
    return {
      strategyId: '',
      version: '',
      displayName: '',
      notes: '',
      venue: 'binance',
      symbol: 'BTCUSDT',
      assetClass: 'crypto',
      active: true,
      timeframe: '1h',
      fastWindow: 9,
      slowWindow: 21,
      parentStrategyId: '',
      parentVersion: '',
    }
  }

  function updateDraft<K extends keyof StrategyDraft>(field: K, value: StrategyDraft[K]) {
    draft = { ...draft, [field]: value }
    draftDirty = true
    saveError = null
  }

  function startNewDraft() {
    draftMode = 'new'
    draft = createEmptyDraft()
    draftDirty = false
    validationResult = null
    saveError = null
    draftNotice = 'Draft fields stay local until you validate or save a new ready version.'
    push('/strategies')
  }

  async function duplicateSelectedVersion() {
    if (!selectedDetail) {
      return
    }
    saveError = null
    try {
      const candidate = await workspaceApi.duplicateStrategyVersion({
        strategyId: selectedDetail.strategyId,
        version: selectedDetail.version,
      })
      applyCandidate(candidate)
      push('/strategies')
    } catch (error) {
      saveError = error instanceof Error ? error.message : 'Failed to duplicate strategy version'
    }
  }

  function applyCandidate(candidate: StrategyVersionCandidate) {
    draftMode = 'duplicate'
    draft = {
      strategyId: candidate.strategyId,
      version: candidate.version,
      displayName: candidate.displayName,
      notes: candidate.notes,
      venue: candidate.definition.instrument.venue,
      symbol: candidate.definition.instrument.symbol,
      assetClass: candidate.definition.instrument.assetClass,
      active: candidate.definition.instrument.active,
      timeframe: candidate.definition.timeframe,
      fastWindow: candidate.definition.parameters.fastWindow,
      slowWindow: candidate.definition.parameters.slowWindow,
      parentStrategyId: candidate.parentStrategyId,
      parentVersion: candidate.parentVersion,
    }
    draftDirty = false
    validationResult = null
    draftNotice =
      'Saved versions are immutable. This duplicate is a local draft until you validate and save a new ready version.'
  }

  function buildDefinition(): StrategyDefinition {
    return {
      kind: 'moving-average-crossover',
      instrument: {
        venue: draft.venue.trim(),
        symbol: draft.symbol.trim(),
        assetClass: draft.assetClass,
        active: draft.active,
      },
      timeframe: draft.timeframe,
      parameters: {
        fastWindow: Number(draft.fastWindow),
        slowWindow: Number(draft.slowWindow),
      },
    }
  }

  async function validateDraft() {
    validationLoading = true
    saveError = null
    try {
      validationResult = await workspaceApi.validateStrategy({ definition: buildDefinition() })
    } catch (error) {
      saveError = error instanceof Error ? error.message : 'Failed to validate strategy draft'
      validationResult = null
    } finally {
      validationLoading = false
    }
  }

  async function saveDraft(event: SubmitEvent) {
    event.preventDefault()
    saveLoading = true
    saveError = null
    try {
      const body: CreateStrategyVersionRequest = {
        strategyId: draft.strategyId.trim(),
        version: draft.version.trim(),
        displayName: draft.displayName.trim(),
        definition: buildDefinition(),
        ...(draft.notes.trim() ? { notes: draft.notes.trim() } : {}),
        ...(draft.parentStrategyId.trim() ? { parentStrategyId: draft.parentStrategyId.trim() } : {}),
        ...(draft.parentVersion.trim() ? { parentVersion: draft.parentVersion.trim() } : {}),
      }

      const saved = await workspaceApi.createStrategyVersion({ body })
      await loadStrategies()
      validationResult = null
      draftDirty = false
      push(`/strategies/${encodeURIComponent(saved.strategyId)}/${encodeURIComponent(saved.version)}`)
    } catch (error) {
      saveError = error instanceof Error ? error.message : 'Failed to save strategy version'
    } finally {
      saveLoading = false
    }
  }

  function formatDate(value: Date): string {
    return value.toISOString().replace('T', ' ').slice(0, 16) + 'Z'
  }

  function formatInstrument(row: { instrument: { venue: string; symbol: string; assetClass: string } }): string {
    return `${row.instrument.venue} / ${row.instrument.symbol} / ${row.instrument.assetClass}`
  }

  function validationErrors(result: StrategyValidationResponse | null): StrategyFieldError[] {
    return result?.errors ?? []
  }
</script>

{#if hasSelectedRoute}
  <section class="page" aria-labelledby="strategy-detail-heading">
    <header class="page-header">
      <div>
        <a class="meta" href="/strategies" use:link>Back to strategies</a>
        <h1 id="strategy-detail-heading">{selectedDetail?.displayName ?? 'Strategy version'}</h1>
        <p class="muted">Immutable saved version for review, duplication, and evaluation.</p>
      </div>
      <button type="button" class="secondary" onclick={startNewDraft}>New draft</button>
    </header>

    {#if detailError}
      <p class="error" role="alert">{detailError}</p>
    {:else if detailLoading}
      <p class="muted" role="status">Loading strategy detail…</p>
    {:else if selectedDetail}
      <section class="panel" aria-labelledby="detail-heading">
        <div class="panel-header">
          <div>
            <h2 id="detail-heading">{selectedDetail.strategyId} / {selectedDetail.version}</h2>
            <p class="muted">Saved versions are immutable. Duplicate this one to continue editing.</p>
          </div>
          <div class="actions detail-actions">
            <button type="button" class="secondary" onclick={duplicateSelectedVersion}>Duplicate to draft</button>
            {#if selectedDetail.status === 'ready'}
              <a
                class="button-link"
                href={`/evaluations/run/${encodeURIComponent(selectedDetail.strategyId)}/${encodeURIComponent(selectedDetail.version)}`}
                use:link
              >Run evaluation</a>
            {/if}
          </div>
        </div>

        {#if saveError}
          <p class="error" role="alert">{saveError}</p>
        {/if}

        <dl class="preview-grid">
          <div>
            <dt>Status</dt>
            <dd>{selectedDetail.status}</dd>
          </div>
          <div>
            <dt>Source</dt>
            <dd>{selectedDetail.sourceLabel}</dd>
          </div>
          <div>
            <dt>Artifact hash</dt>
            <dd>
              <code title={selectedDetail.artifactHash} aria-label={selectedDetail.artifactHash}>
                {formatCompactIdentifier(selectedDetail.artifactHash)}
              </code>
            </dd>
          </div>
          <div>
            <dt>Timeframe</dt>
            <dd>{selectedDetail.timeframe}</dd>
          </div>
        </dl>

        {#if selectedDetail.sourceType === 'demo'}
          <p class="demo-note">
            Demo example only, not a recommendation. Evaluation succeeds only when matching local historical data is present.
          </p>
        {/if}

        <pre>{JSON.stringify(selectedDetail.definition, null, 2)}</pre>
      </section>
    {:else}
      <p class="muted">Select a saved version from the workspace to inspect its immutable detail.</p>
    {/if}
  </section>
{:else}
  <section class="page" aria-labelledby="strategies-heading">
    <header class="page-header">
      <div>
        <h1 id="strategies-heading">Strategies</h1>
        <p class="muted">Constrained v0 workspace for moving-average crossover strategy versions.</p>
      </div>
      <button type="button" class="secondary" onclick={startNewDraft}>New draft</button>
    </header>

    {#if listError}
      <p class="error" role="alert">{listError}</p>
    {/if}

    <section class="panel" aria-labelledby="strategy-list-heading">
      <div class="panel-header">
        <h2 id="strategy-list-heading">Saved versions</h2>
        <p class="muted">Open a saved version on its own screen when you need immutable detail.</p>
      </div>

      {#if listLoading}
        <p class="muted" role="status">Loading strategy versions…</p>
      {:else if strategies.length === 0}
        <p class="muted">No strategy versions saved yet.</p>
      {:else}
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Strategy</th>
                <th>Status</th>
                <th>Source</th>
                <th>Artifact hash</th>
                <th>Instrument</th>
                <th>Timeframe</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {#each strategies as strategy (`${strategy.strategyId}/${strategy.version}`)}
                <tr>
                  <td>
                    <a
                      class="strategy-link"
                      href={`/strategies/${encodeURIComponent(strategy.strategyId)}/${encodeURIComponent(strategy.version)}`}
                      use:link
                    >
                      <strong>{strategy.displayName}</strong><br />
                      <span class="meta strategy-meta">{strategy.strategyId} / {strategy.version}</span>
                    </a>
                  </td>
                  <td>{strategy.status}</td>
                  <td>
                    {strategy.sourceLabel}
                    {#if strategy.sourceType === 'demo'}
                      <div class="demo-note">Example only, not a recommendation.</div>
                    {/if}
                  </td>
                  <td>
                    <code title={strategy.artifactHash} aria-label={strategy.artifactHash}>
                      {formatCompactIdentifier(strategy.artifactHash)}
                    </code>
                  </td>
                  <td>{formatInstrument(strategy)}</td>
                  <td>{strategy.timeframe}</td>
                  <td>{formatDate(strategy.createdAt)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>

    <section class="panel" aria-labelledby="editor-heading">
      <div class="panel-header">
        <h2 id="editor-heading">Strategy editor</h2>
        <div>
          <p class="muted">{draftNotice}</p>
          <p class="meta">Draft mode: {draftMode}</p>
        </div>
      </div>

      <form class="editor-form" onsubmit={saveDraft}>
        <div class="field-row field-row--3">
          <label>
            Strategy ID
            <input
              type="text"
              value={draft.strategyId}
              oninput={(event) => updateDraft('strategyId', event.currentTarget.value)}
              required
              placeholder="strategy-id"
            />
          </label>
          <label>
            Version
            <input
              type="text"
              value={draft.version}
              oninput={(event) => updateDraft('version', event.currentTarget.value)}
              required
              placeholder="v1"
            />
          </label>
          <label>
            Display name
            <input
              type="text"
              value={draft.displayName}
              oninput={(event) => updateDraft('displayName', event.currentTarget.value)}
              required
              placeholder="Example crossover"
            />
          </label>
        </div>

        <div class="field-row field-row--4">
          <label>
            Kind
            <input type="text" value="moving-average-crossover" disabled />
          </label>
          <label>
            Venue
            <input
              type="text"
              value={draft.venue}
              oninput={(event) => updateDraft('venue', event.currentTarget.value)}
              required
            />
          </label>
          <label>
            Symbol
            <input
              type="text"
              value={draft.symbol}
              oninput={(event) => updateDraft('symbol', event.currentTarget.value)}
              required
            />
          </label>
          <label>
            Asset class
            <select value={draft.assetClass} onchange={(event) => updateDraft('assetClass', event.currentTarget.value)}>
              {#each supportedAssetClasses as item (item)}
                <option value={item}>{item}</option>
              {/each}
            </select>
          </label>
        </div>

        <div class="field-row field-row--4">
          <label>
            Timeframe
            <select value={draft.timeframe} onchange={(event) => updateDraft('timeframe', event.currentTarget.value)}>
              {#each supportedTimeframes as item (item)}
                <option value={item}>{item}</option>
              {/each}
            </select>
          </label>
          <label>
            Fast window
            <input
              type="number"
              min="1"
              value={draft.fastWindow}
              oninput={(event) => updateDraft('fastWindow', Number(event.currentTarget.value))}
              required
            />
          </label>
          <label>
            Slow window
            <input
              type="number"
              min="1"
              value={draft.slowWindow}
              oninput={(event) => updateDraft('slowWindow', Number(event.currentTarget.value))}
              required
            />
          </label>
          <label class="checkbox-field">
            <span>Active instrument</span>
            <input
              type="checkbox"
              checked={draft.active}
              onchange={(event) => updateDraft('active', event.currentTarget.checked)}
            />
          </label>
        </div>

        <label>
          Notes
          <textarea
            rows="4"
            value={draft.notes}
            oninput={(event) => updateDraft('notes', event.currentTarget.value)}
            placeholder="Draft notes or evaluation context"
          ></textarea>
        </label>

        {#if draft.parentStrategyId}
          <p class="muted">
            Parent version: <code>{draft.parentStrategyId}</code> / <code>{draft.parentVersion}</code>
          </p>
        {/if}

        {#if saveError}
          <p class="error" role="alert">{saveError}</p>
        {/if}

        <div class="actions">
          <button type="button" class="secondary" onclick={validateDraft} disabled={validationLoading}>
            {validationLoading ? 'Validating…' : 'Validate'}
          </button>
          <button type="submit" class="primary" disabled={saveLoading || !draftDirty}>
            {saveLoading ? 'Saving…' : 'Save version'}
          </button>
        </div>
      </form>

      <section class="validation-panel" aria-labelledby="validation-heading">
        <h3 id="validation-heading">Validation</h3>
        {#if validationResult === null}
          <p class="muted">Run backend validation to inspect canonical strategy JSON and field errors.</p>
        {:else if !validationResult.valid}
          <div role="alert">
            <p class="error-title">Validation failed</p>
            <ul>
              {#each validationErrors(validationResult) as fieldError (`${fieldError.path}:${fieldError.message}`)}
                <li><code>{fieldError.path}</code>: {fieldError.message}</li>
              {/each}
            </ul>
          </div>
        {:else if validationResult.preview}
          <div>
            <p class="success">Definition is valid for the supported v0 schema.</p>
            <dl class="preview-grid">
              <div>
                <dt>Schema</dt>
                <dd>{validationResult.preview.schemaVersion}</dd>
              </div>
              <div>
                <dt>Artifact hash</dt>
                <dd>
                  <code
                    title={validationResult.preview.artifactHash}
                    aria-label={validationResult.preview.artifactHash}
                  >
                    {formatCompactIdentifier(validationResult.preview.artifactHash)}
                  </code>
                </dd>
              </div>
              <div>
                <dt>Existing artifact</dt>
                <dd>{validationResult.preview.existingArtifact ? 'yes' : 'no'}</dd>
              </div>
              <div>
                <dt>Parameters</dt>
                <dd>
                  fast={validationResult.preview.parameterSummary.fastWindow}, slow={validationResult.preview.parameterSummary.slowWindow}
                </dd>
              </div>
            </dl>
            <pre>{validationResult.preview.canonicalJson}</pre>
          </div>
        {/if}
      </section>
    </section>
  </section>
{/if}

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: var(--space-24);
  }

  .page-header,
  .panel-header,
  .actions,
  .field-row {
    display: flex;
    gap: var(--space-16);
  }

  .page-header,
  .panel-header {
    justify-content: space-between;
    align-items: flex-start;
  }

  .panel {
    border: 1px solid var(--border);
    border-radius: var(--radius-default);
    padding: var(--space-16);
    background: var(--bg);
  }

  .muted,
  .meta {
    color: var(--text-muted);
  }

  .error,
  .error-title,
  .demo-note {
    color: var(--color-danger-red);
  }

  .success {
    color: var(--color-success-green);
  }

  .table-wrap {
    overflow-x: auto;
  }

  table {
    width: 100%;
    border-collapse: collapse;
  }

  td code {
    display: inline-block;
    max-width: 100%;
    vertical-align: top;
  }

  th,
  td {
    padding: var(--space-12) var(--space-8);
    border-top: 1px solid var(--border);
    text-align: left;
    vertical-align: top;
  }

  th {
    color: var(--text-muted);
  }

  th:nth-child(2),
  td:nth-child(2),
  th:nth-child(6),
  td:nth-child(6),
  th:nth-child(7),
  td:nth-child(7) {
    white-space: nowrap;
  }

  th:nth-child(1),
  td:nth-child(1) {
    width: 28%;
  }

  th:nth-child(4),
  td:nth-child(4) {
    width: 17%;
  }

  .strategy-link {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .strategy-link strong,
  .strategy-meta {
    overflow-wrap: anywhere;
  }

  .editor-form {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
  }

  .field-row {
    flex-wrap: wrap;
  }

  .field-row--3 > label {
    flex: 1 1 12rem;
  }

  .field-row--4 > label {
    flex: 1 1 10rem;
  }

  label {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    font-weight: 500;
  }

  input,
  select,
  textarea,
  pre {
    font: inherit;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-default);
    padding: var(--space-12);
    box-sizing: border-box;
  }

  textarea,
  pre {
    width: 100%;
  }

  pre {
    overflow-x: auto;
    white-space: pre-wrap;
  }

  .checkbox-field {
    justify-content: flex-end;
  }

  .checkbox-field input {
    width: auto;
    align-self: flex-start;
  }

  .actions {
    flex-wrap: wrap;
    align-items: center;
  }

  .detail-actions {
    justify-content: flex-end;
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

  .validation-panel {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
  }

  .preview-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--space-12) var(--space-16);
    margin: 0;
  }

  .preview-grid dt {
    color: var(--text-muted);
  }

  .preview-grid dd {
    margin: 0;
  }

  @media (max-width: 900px) {
    .page-header,
    .panel-header {
      flex-direction: column;
    }

    .preview-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
