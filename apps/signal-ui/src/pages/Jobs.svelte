<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { formatCompactIdentifier } from '../lib/compact-identifier'
  import { createSignalJobsApiForAuth, type JobSummary } from '../lib/jobs/api'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'

  const jobsApi = $derived.by(() => createSignalJobsApiForAuth({ baseUrl: appBaseUrl, authStore }))

  let loading = $state(true)
  let error = $state<string | null>(null)
  let jobs = $state<JobSummary[]>([])
  let nextCursor = $state('')

  let statusFilter = $state('')
  let jobTypeFilter = $state('')
  let sourceFilter = $state('')

  onMount(() => {
    void loadJobs()
  })

  async function loadJobs() {
    loading = true
    error = null
    try {
      const response = await jobsApi.listJobs({
        status: statusFilter ? [statusFilter] : [],
        jobType: jobTypeFilter ? [jobTypeFilter] : [],
        source: sourceFilter ? [sourceFilter] : [],
        limit: 25,
        cursor: '',
      })
      jobs = response.items
      nextCursor = response.nextCursor
    } catch (loadError) {
      jobs = []
      nextCursor = ''
      error = loadError instanceof Error ? loadError.message : 'Failed to load jobs'
    } finally {
      loading = false
    }
  }

  async function applyFilters(event: SubmitEvent) {
    event.preventDefault()
    await loadJobs()
  }

  function formatDate(value: Date | null): string {
    return value ? value.toISOString() : '—'
  }

  function formatScope(item: JobSummary): string {
    return `${item.input.venue} / ${item.input.symbol} / ${item.input.assetClass} / ${item.input.timeframe}`
  }
</script>

<section class="page" aria-labelledby="jobs-heading">
  <header class="page-header">
    <div>
      <h1 id="jobs-heading">Jobs</h1>
      <p class="muted">Review durable historical ingestion jobs, filter the queue, and open detail on a separate route.</p>
    </div>
    <button class="secondary" type="button" onclick={loadJobs} disabled={loading}>Refresh jobs</button>
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
          <option value="historical_raw_candle_backfill">historical_raw_candle_backfill</option>
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
              <dd>{formatDate(item.createdAt)}</dd>
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
      <p class="muted">Next cursor available: {nextCursor}</p>
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
