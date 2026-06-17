<script lang="ts">
  import { link } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { createSignalJobsApiForAuth, type JobDetail as JobDetailModel } from '../lib/jobs/api'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'

  const jobsApi = $derived.by(() => createSignalJobsApiForAuth({ baseUrl: appBaseUrl, authStore }))

  let { params = {} } = $props<{ params?: { jobId?: string } }>()

  let loading = $state(true)
  let error = $state<string | null>(null)
  let detail = $state<JobDetailModel | null>(null)
  let requestToken = 0

  $effect(() => {
    void loadDetail(params.jobId)
  })

  async function loadDetail(jobId: string | undefined) {
    const token = ++requestToken
    detail = null
    if (!jobId) {
      loading = false
      error = 'Job id is required.'
      return
    }

    loading = true
    error = null
    try {
      const loaded = await jobsApi.getJob({ jobId })
      if (token !== requestToken) {
        return
      }
      detail = loaded
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

  function formatDate(value: Date | null): string {
    return value ? value.toISOString() : '—'
  }
</script>

<section class="page" aria-labelledby="job-detail-heading">
  <header class="page-header">
    <div>
      <h1 id="job-detail-heading">Job detail</h1>
      <p class="muted">Inspect one durable historical backfill request, its worker metadata, and terminal result or safe error.</p>
    </div>
    <div class="page-links">
      <a href="/jobs" use:link>Back to jobs</a>
      <a href="/data" use:link>Back to data</a>
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

    <section class="panel">
      <h2>Input</h2>
      <dl class="summary-grid">
        <div><dt>ingestionRunId</dt><dd>{detail.input.ingestionRunId}</dd></div>
        <div><dt>Venue</dt><dd>{detail.input.venue}</dd></div>
        <div><dt>Symbol</dt><dd>{detail.input.symbol}</dd></div>
        <div><dt>Asset class</dt><dd>{detail.input.assetClass}</dd></div>
        <div><dt>Timeframe</dt><dd>{detail.input.timeframe}</dd></div>
        <div><dt>Page size</dt><dd>{detail.input.pageSize}</dd></div>
        <div><dt>Start</dt><dd>{formatDate(detail.input.start)}</dd></div>
        <div><dt>End</dt><dd>{formatDate(detail.input.end)}</dd></div>
      </dl>
    </section>

    <section class="panel">
      <h2>Timeline and worker</h2>
      <dl class="summary-grid">
        <div><dt>Created</dt><dd>{formatDate(detail.createdAt)}</dd></div>
        <div><dt>Updated</dt><dd>{formatDate(detail.updatedAt)}</dd></div>
        <div><dt>Started</dt><dd>{formatDate(detail.startedAt)}</dd></div>
        <div><dt>Completed</dt><dd>{formatDate(detail.completedAt)}</dd></div>
        <div><dt>Last attempt</dt><dd>{formatDate(detail.lastAttemptAt)}</dd></div>
      </dl>
    </section>

    {#if detail.result}
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
                <li>{formatDate(item.start)} → {formatDate(item.end)}</li>
              {/each}
            </ul>
          </div>
        {/if}
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
