<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { formatCompactIdentifier } from '../lib/compact-identifier'
  import {
    createSignalJobsApiForAuth,
    isHistoricalDataBackfillJob,
    type JobSummary,
  } from '../lib/jobs/api'
  import { formatLocalDateTime } from '../lib/timestamp'

  let {
    heading = 'Jobs',
    description = 'Review durable historical ingestion jobs, filter the queue, and open detail on a separate route.',
  } = $props<{ heading?: string; description?: string }>()

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'

  const jobsApi = $derived.by(() => createSignalJobsApiForAuth({ baseUrl: appBaseUrl, authStore }))

  let loading = $state(true)
  let loadingMore = $state(false)
  let error = $state<string | null>(null)
  let jobs = $state<JobSummary[]>([])
  let nextCursor = $state('')

  let statusFilter = $state('')
  let jobTypeFilter = $state('')
  let sourceFilter = $state('')

  onMount(() => {
    void loadJobs()
  })

  async function loadJobs(options: { cursor?: string; append?: boolean } = {}) {
    const append = options.append ?? false
    const cursor = options.cursor ?? ''
    if (append) {
      loadingMore = true
    } else {
      loading = true
      error = null
    }
    try {
      const response = await jobsApi.listJobs({
        status: statusFilter ? [statusFilter] : [],
        jobType: jobTypeFilter ? [jobTypeFilter] : [],
        source: sourceFilter ? [sourceFilter] : [],
        limit: 25,
        cursor,
      })
      jobs = append ? [...jobs, ...response.items] : response.items
      nextCursor = response.nextCursor
    } catch (loadError) {
      if (!append) {
        jobs = []
        nextCursor = ''
      }
      error = loadError instanceof Error ? loadError.message : 'Failed to load jobs'
    } finally {
      if (append) {
        loadingMore = false
      } else {
        loading = false
      }
    }
  }

  async function applyFilters(event: SubmitEvent) {
    event.preventDefault()
    await loadJobs()
  }

  async function loadMore() {
    if (!nextCursor) {
      return
    }
    await loadJobs({ cursor: nextCursor, append: true })
  }

  function formatScope(item: JobSummary): string {
    if (!isHistoricalDataBackfillJob(item)) {
      return 'Not available for this job type'
    }
    return `${item.input.venue} / ${item.input.symbol} / ${item.input.assetClass} / ${item.input.timeframe}`
  }
</script>

<section class="page" aria-labelledby="jobs-heading">
  <header class="page-header">
    <div>
      <h1 id="jobs-heading">{heading}</h1>
      <p class="muted">{description}</p>
    </div>
    <button class="secondary" type="button" onclick={() => void loadJobs()} disabled={loading}>Refresh jobs</button>
  </header>

  <section class="panel">
    <form class="filters" onsubmit={applyFilters}>
      <label>
        <span>Status</span>
        <select bind:value={statusFilter} aria-label="Status">
          <option value="">Any status</option>
          <option value="queued">queued</option>
          <option value="running">running</option>
          <option value="succeeded">succeeded</option>
          <option value="failed">failed</option>
        </select>
      </label>
      <label>
        <span>Job type</span>
        <select bind:value={jobTypeFilter} aria-label="Job type">
          <option value="">Any job type</option>
           <option value="data.historical_raw_candle_backfill">data.historical_raw_candle_backfill</option>
           <option value="finance.bank_connection_sync">finance.bank_connection_sync</option>
           <option value="finance.fx_rates_sync">finance.fx_rates_sync</option>
           <option value="finance.csv_import">finance.csv_import</option>
           <option value="finance.account_import">finance.account_import</option>
        </select>
      </label>
      <label>
        <span>Source</span>
        <select bind:value={sourceFilter} aria-label="Source">
          <option value="">Any source</option>
          <option value="operator">operator</option>
          <option value="agent">agent</option>
        </select>
      </label>
      <div class="filters__actions">
        <button class="primary" type="submit" disabled={loading}>Apply filters</button>
      </div>
    </form>
  </section>

  {#if error}
    <p class="error" role="alert">{error}</p>
  {:else if loading}
    <p class="muted" role="status">Loading jobs…</p>
  {:else if jobs.length === 0}
    <p class="muted">No durable jobs matched the current filters.</p>
  {:else}
    <div class="job-list" aria-label="Job summaries">
      {#each jobs as item (item.id)}
        <article class="panel job-card">
          <div class="job-card__header">
            <div>
              <p class="job-card__eyebrow">{item.status} · {item.jobType}</p>
              <h2>{item.id}</h2>
            </div>
            <a class="button-link" href={`/jobs/${encodeURIComponent(item.id)}`} use:link>Open job detail</a>
          </div>

          <dl class="summary-grid">
            <div>
              <dt>Scope</dt>
              <dd>{formatScope(item)}</dd>
            </div>
            <div>
              <dt>Requested by</dt>
              <dd>{item.requester.source} · {formatCompactIdentifier(item.requester.userId)}</dd>
            </div>
            <div>
              <dt>Created</dt>
              <dd>{formatLocalDateTime(item.createdAt)}</dd>
            </div>
            <div>
              <dt>Attempts</dt>
              <dd>{item.attemptCount}</dd>
            </div>
          </dl>

          {#if item.result}
            <p class="muted">
              Result: {item.result.persistedCount} persisted / {item.result.expectedCount} expected · {item.result.missingIntervalCount} missing intervals
            </p>
          {/if}

          {#if item.error}
            <p class="error-inline">{item.error.summary}</p>
          {/if}
        </article>
      {/each}
    </div>

    {#if nextCursor}
      <div class="load-more">
        <button class="secondary" type="button" onclick={loadMore} disabled={loadingMore}>
          {loadingMore ? 'Loading more…' : 'Load more'}
        </button>
      </div>
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
    align-items: flex-start;
    gap: var(--space-16);
  }

  .panel {
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: var(--space-16);
    background: var(--bg-elevated, var(--bg));
  }

  .filters {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    gap: var(--space-16);
  }

  .filters label {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }

  .filters__actions {
    display: flex;
    align-items: end;
  }

  .job-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
  }

  .job-card {
    gap: var(--space-12);
  }

  .job-card__header {
    display: flex;
    justify-content: space-between;
    gap: var(--space-16);
    align-items: flex-start;
  }

  .job-card__header h2,
  .page-header h1 {
    margin: 0;
  }

  .job-card__eyebrow,
  .muted {
    margin: 0;
    color: var(--text-muted);
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
  }

  .button-link {
    color: var(--link);
    text-decoration: underline;
    font-weight: 500;
  }

  .load-more {
    display: flex;
    justify-content: flex-start;
  }

  .error,
  .error-inline {
    color: var(--color-danger-red);
  }

  .error-inline {
    margin: 0;
  }

  @media (max-width: 700px) {
    .page-header,
    .job-card__header {
      flex-direction: column;
    }
  }
</style>
