<script lang="ts">
  import { onDestroy } from 'svelte'
  import { link } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalJobsApiForAuth,
    isHistoricalDataBackfillJob,
    type JobDetail as JobDetailModel,
  } from '../lib/jobs/api'
  import { formatLocalDateTime } from '../lib/timestamp'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'

  const jobsApi = $derived.by(() => createSignalJobsApiForAuth({ baseUrl: appBaseUrl, authStore }))

  let {
    params = {},
    heading = 'Job detail',
    description = 'Inspect one durable historical backfill request, its worker metadata, and terminal result or safe error.',
    primaryBackHref = '/jobs',
    primaryBackLabel = 'Back to jobs',
    secondaryBackHref = '/data',
    secondaryBackLabel = 'Back to data',
    formatDateValue = formatLocalDateTime,
  } = $props<{
    params?: { jobId?: string }
    heading?: string
    description?: string
    primaryBackHref?: string
    primaryBackLabel?: string
    secondaryBackHref?: string
    secondaryBackLabel?: string
    formatDateValue?: (value: Date | null | undefined) => string
  }>()

  let loading = $state(true)
  let error = $state<string | null>(null)
  let detail = $state<JobDetailModel | null>(null)
  let requestToken = 0
  let refreshTimer: ReturnType<typeof setTimeout> | null = null

  const autoRefreshStatuses = new Set(['queued', 'running'])
  const autoRefreshMs = 2_000

  $effect(() => {
    clearRefreshTimer()
    void loadDetail(params.jobId)
  })

  onDestroy(() => {
    clearRefreshTimer()
  })

  async function loadDetail(jobId: string | undefined, options: { preserveDetail?: boolean } = {}) {
    const token = ++requestToken
    clearRefreshTimer()
    if (!jobId) {
      detail = null
      loading = false
      error = 'Job id is required.'
      return
    }

    if (!options.preserveDetail) {
      detail = null
      loading = true
    }
    error = null
    try {
      const loaded = await jobsApi.getJob({ jobId })
      if (token !== requestToken) {
        return
      }
      detail = loaded
      if (autoRefreshStatuses.has(loaded.status)) {
        refreshTimer = setTimeout(() => {
          void loadDetail(jobId, { preserveDetail: true })
        }, autoRefreshMs)
      }
    } catch (loadError) {
      if (token !== requestToken) {
        return
      }
      error = loadError instanceof Error ? loadError.message : 'Failed to load job detail'
      detail = null
    } finally {
      if (token === requestToken) {
        loading = false
      }
    }
  }

  function clearRefreshTimer() {
    if (refreshTimer !== null) {
      clearTimeout(refreshTimer)
      refreshTimer = null
    }
  }

  function buildDataScopeHref(item: JobDetailModel): string {
    if (!isHistoricalDataBackfillJob(item)) {
      throw new Error('Historical data scope is only available for historical backfill jobs.')
    }
    const query = new URLSearchParams({
      venue: item.input.venue,
      symbol: item.input.symbol,
      assetClass: item.input.assetClass,
      timeframe: item.input.timeframe,
      start: item.input.start.toISOString(),
      end: item.input.end.toISOString(),
    })
    return `/data?${query.toString()}`
  }

  function hasHistoricalResult(item: JobDetailModel): boolean {
    return isHistoricalDataBackfillJob(item) && item.result !== undefined
  }
</script>

<section class="page" aria-labelledby="job-detail-heading">
  <header class="page-header">
    <div>
        <h1 id="job-detail-heading">{heading}</h1>
        <p class="muted">{description}</p>
      </div>
      <div class="page-links">
        <a href={primaryBackHref} use:link>{primaryBackLabel}</a>
        <a href={secondaryBackHref} use:link>{secondaryBackLabel}</a>
        {#if detail && isHistoricalDataBackfillJob(detail)}
          <a href={buildDataScopeHref(detail)} use:link>Open data scope</a>
        {/if}
      </div>
  </header>

  {#if error}
    <p class="error" role="alert">{error}</p>
  {:else if loading}
    <p class="muted" role="status">Loading job detail…</p>
  {:else if detail}
    <section class="panel">
      <h2>{detail.id}</h2>
      <dl class="summary-grid">
        <div><dt>Status</dt><dd>{detail.status}</dd></div>
        <div><dt>Job type</dt><dd>{detail.jobType}</dd></div>
        <div><dt>Requester source</dt><dd>{detail.requester.source}</dd></div>
        <div><dt>Requester user</dt><dd>{detail.requester.userId || '—'}</dd></div>
        <div><dt>Worker ID</dt><dd>{detail.workerId || '—'}</dd></div>
        <div><dt>Attempt count</dt><dd>{detail.attemptCount}</dd></div>
      </dl>
    </section>

    {#if isHistoricalDataBackfillJob(detail)}
      <section class="panel">
        <h2>Input</h2>
        <dl class="summary-grid">
          <div><dt>ingestionRunId</dt><dd>{detail.input.ingestionRunId}</dd></div>
          <div><dt>Venue</dt><dd>{detail.input.venue}</dd></div>
          <div><dt>Symbol</dt><dd>{detail.input.symbol}</dd></div>
          <div><dt>Asset class</dt><dd>{detail.input.assetClass}</dd></div>
          <div><dt>Timeframe</dt><dd>{detail.input.timeframe}</dd></div>
          <div><dt>Page size</dt><dd>{detail.input.pageSize}</dd></div>
          <div><dt>Start</dt><dd>{formatDateValue(detail.input.start)}</dd></div>
          <div><dt>End</dt><dd>{formatDateValue(detail.input.end)}</dd></div>
        </dl>
      </section>
    {:else}
      <section class="panel">
        <h2>Input</h2>
        <p class="muted">Input details are not available for this job type in the current API surface.</p>
      </section>
    {/if}

    <section class="panel">
      <h2>Timeline and worker</h2>
      <dl class="summary-grid">
        <div><dt>Created</dt><dd>{formatDateValue(detail.createdAt)}</dd></div>
        <div><dt>Updated</dt><dd>{formatDateValue(detail.updatedAt)}</dd></div>
        <div><dt>Started</dt><dd>{formatDateValue(detail.startedAt)}</dd></div>
        <div><dt>Completed</dt><dd>{formatDateValue(detail.completedAt)}</dd></div>
        <div><dt>Last attempt</dt><dd>{formatDateValue(detail.lastAttemptAt)}</dd></div>
      </dl>
    </section>

    {#if hasHistoricalResult(detail) && detail.result}
      <section class="panel">
        <h2>Result</h2>
        <dl class="summary-grid">
          <div><dt>persistedCount</dt><dd>{detail.result.persistedCount}</dd></div>
          <div><dt>expectedCount</dt><dd>{detail.result.expectedCount}</dd></div>
          <div><dt>missingIntervalCount</dt><dd>{detail.result.missingIntervalCount}</dd></div>
          <div><dt>duplicateNaturalKeyCount</dt><dd>{detail.result.duplicateNaturalKeyCount}</dd></div>
          <div><dt>rawPayloadCount</dt><dd>{detail.result.rawPayloadCount ?? '—'}</dd></div>
          <div><dt>missingIntervalPreviewCap</dt><dd>{detail.result.missingIntervalPreviewCap}</dd></div>
        </dl>

        {#if detail.result.missingIntervalPreview.length > 0}
          <div class="missing-preview">
            <h3>Missing interval preview</h3>
            <ul>
              {#each detail.result.missingIntervalPreview as item (`${item.start.toISOString()}-${item.end.toISOString()}`)}
                <li>{formatDateValue(item.start)} → {formatDateValue(item.end)}</li>
              {/each}
            </ul>
          </div>
        {/if}
      </section>
    {:else if !isHistoricalDataBackfillJob(detail)}
      <section class="panel">
        <h2>Result</h2>
        <p class="muted">Result details are not yet specialized for this job type.</p>
      </section>
    {/if}

    {#if detail.error}
      <section class="panel">
        <h2>Error</h2>
        <p class="error-inline">{detail.error.summary}</p>
        <p class="muted">{detail.error.code}</p>
        <p>{detail.error.details}</p>
      </section>
    {/if}
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

  .page-header h1,
  .panel h2,
  .panel h3 {
    margin: 0;
  }

  .page-links {
    display: flex;
    gap: var(--space-16);
    flex-wrap: wrap;
  }

  .page-links a {
    color: var(--link);
    text-decoration: underline;
    font-weight: 500;
  }

  .panel {
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: var(--space-16);
    background: var(--bg-elevated, var(--bg));
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
  }

  .summary-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: var(--space-12);
    margin: 0;
  }

  .summary-grid dt {
    font-weight: 700;
  }

  .summary-grid dd {
    margin: var(--space-4) 0 0;
    overflow-wrap: anywhere;
  }

  .muted {
    margin: 0;
    color: var(--text-muted);
  }

  .error,
  .error-inline {
    color: var(--color-danger-red);
  }

  .error-inline {
    margin: 0;
  }

  .missing-preview ul {
    margin: var(--space-8) 0 0;
    padding-left: var(--space-20);
  }

  @media (max-width: 700px) {
    .page-header {
      flex-direction: column;
    }
  }
</style>
