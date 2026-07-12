<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { formatFinanceDateTime } from '../lib/finance/format'
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'
  import {
    createSignalJobsApiForAuth,
    isHistoricalDataBackfillJob,
    type JobDetail as JobDetailModel,
  } from '../lib/jobs/api'

  let { params = {} } = $props<{ params?: { jobId?: string } }>()

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const jobsApi = $derived.by(() => createSignalJobsApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const financeShell = useFinanceShellState()

  let loadingWorkspace = $state(true)
  let loadingDetail = $state(false)
  let error = $state<string | null>(null)
  let detail = $state<JobDetailModel | null>(null)
  let requestToken = 0
  let refreshTimer: ReturnType<typeof setTimeout> | null = null

  const autoRefreshStatuses = new Set(['queued', 'running'])
  const autoRefreshMs = 2_000

  onMount(() => {
    void loadWorkspace()
  })

  onDestroy(() => {
    clearRefreshTimer()
  })

  async function loadWorkspace() {
    loadingWorkspace = true
    error = null

    try {
      await financeShell.initialize()
      if (!financeShell.needsTenantSelection && financeShell.selectedTenantId) {
        await loadDetail(params.jobId)
      }
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load finance workspace'
    } finally {
      loadingWorkspace = false
    }
  }

  async function loadDetail(jobId: string | undefined, options: { preserveDetail?: boolean } = {}) {
    const token = ++requestToken
    clearRefreshTimer()

    if (!jobId) {
      detail = null
      loadingDetail = false
      error = 'Job id is required.'
      return
    }

    if (!options.preserveDetail) {
      detail = null
      loadingDetail = true
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

      error = loadError instanceof Error ? loadError.message : 'Failed to load finance job detail'
      detail = null
    } finally {
      if (token === requestToken) {
        loadingDetail = false
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

  function statusBadgeClass(status: string): string {
    if (status === 'succeeded') return 'text-bg-success'
    if (status === 'failed') return 'text-bg-danger'
    if (status === 'running') return 'text-bg-primary'
    return 'text-bg-secondary'
  }

  $effect(() => {
    void params.jobId
    void financeShell.selectedTenantId
    if (loadingWorkspace || financeShell.needsTenantSelection || !financeShell.selectedTenantId) {
      clearRefreshTimer()
      return
    }
    void loadDetail(params.jobId)
  })
</script>

<section class="container-fluid px-0" aria-labelledby="finance-job-detail-heading">
  <div class="d-grid gap-4">
    <header class="card border-0 shadow-sm">
      <div class="card-body p-4 p-xl-5 d-flex flex-column flex-lg-row justify-content-between gap-3 align-items-lg-center">
        <div>
          <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Durable finance job</p>
          <h1 id="finance-job-detail-heading" class="h3 mb-2">Finance job detail</h1>
          <p class="text-body-secondary mb-0">
            Inspect imports, sync work, or FX follow-up without leaving the finance workspace.
          </p>
        </div>

        <div class="d-flex flex-wrap gap-2">
          <a class="btn btn-outline-secondary btn-sm" href="/finance/connections" use:link>Back to finance connections</a>
          <a class="btn btn-outline-secondary btn-sm" href="/finance/imports" use:link>Back to finance imports</a>
          {#if detail && isHistoricalDataBackfillJob(detail)}
            <a class="btn btn-outline-secondary btn-sm" href={buildDataScopeHref(detail)} use:link>Open data scope</a>
          {/if}
        </div>
      </div>
    </header>

    {#if error}
      <div class="alert alert-danger mb-0" role="alert">{error}</div>
    {/if}

    {#if loadingWorkspace}
      <div class="alert alert-secondary mb-0" role="status">Loading finance workspace…</div>
    {:else if financeShell.needsTenantSelection}
      <section class="card shadow-sm">
        <div class="card-body p-4 d-grid gap-3">
          {#if !financeShell.embedded}
            <div class="col-12 col-lg-5 px-0">
              <label class="form-label" for="finance-job-detail-tenant">Tenant</label>
              <select
                id="finance-job-detail-tenant"
                class="form-select"
                value={financeShell.selectedTenantId}
                onchange={(event) => financeShell.selectTenant((event.currentTarget as HTMLSelectElement).value)}
                aria-label="Tenant"
              >
                <option value="">Select tenant</option>
                {#each financeShell.tenants as tenant (tenant.id)}
                  <option value={tenant.id}>{tenant.name}</option>
                {/each}
              </select>
            </div>
          {/if}

          <div class="alert alert-warning mb-0" role="status">Select an active tenant to continue on this finance route.</div>
        </div>
      </section>
    {:else if !financeShell.selectedTenantId}
      <div class="alert alert-light border mb-0" role="status">
        Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a> before opening this finance job detail route.
      </div>
    {:else if loadingDetail}
      <div class="alert alert-secondary mb-0" role="status">Loading finance job detail…</div>
    {:else if detail}
      <div class="row g-4">
        <div class="col-12">
          <section class="card shadow-sm">
            <div class="card-body p-4 d-grid gap-3">
              <div class="d-flex flex-column flex-lg-row justify-content-between gap-3 align-items-lg-center">
                <div>
                  <h2 class="h5 mb-1">Summary</h2>
                  <p class="text-body-secondary mb-0">{detail.id}</p>
                </div>

                <span class={`badge ${statusBadgeClass(detail.status)} align-self-start align-self-lg-center`}>
                  {detail.status}
                </span>
              </div>

              <div class="row g-3">
                <div class="col-12 col-md-6 col-xl-4"><strong>Job type</strong><div>{detail.jobType}</div></div>
                <div class="col-12 col-md-6 col-xl-4"><strong>Requester source</strong><div>{detail.requester.source}</div></div>
                <div class="col-12 col-md-6 col-xl-4"><strong>Requester user</strong><div>{detail.requester.userId || '—'}</div></div>
                <div class="col-12 col-md-6 col-xl-4"><strong>Worker ID</strong><div>{detail.workerId || '—'}</div></div>
                <div class="col-12 col-md-6 col-xl-4"><strong>Attempt count</strong><div>{detail.attemptCount}</div></div>
              </div>
            </div>
          </section>
        </div>

        <div class="col-12 col-xl-6">
          <section class="card shadow-sm h-100">
            <div class="card-body p-4 d-grid gap-3">
              <div>
                <h2 class="h5 mb-1">Input</h2>
                <p class="text-body-secondary mb-0">
                  {#if isHistoricalDataBackfillJob(detail)}
                    Request scope kept on the finance-local job route.
                  {:else}
                    Input details are not available for this job type in the current API surface.
                  {/if}
                </p>
              </div>

              {#if isHistoricalDataBackfillJob(detail)}
                <div class="row g-3">
                  <div class="col-12 col-md-6"><strong>ingestionRunId</strong><div>{detail.input.ingestionRunId || '—'}</div></div>
                  <div class="col-12 col-md-6"><strong>Venue</strong><div>{detail.input.venue || '—'}</div></div>
                  <div class="col-12 col-md-6"><strong>Symbol</strong><div>{detail.input.symbol || '—'}</div></div>
                  <div class="col-12 col-md-6"><strong>Asset class</strong><div>{detail.input.assetClass || '—'}</div></div>
                  <div class="col-12 col-md-6"><strong>Timeframe</strong><div>{detail.input.timeframe || '—'}</div></div>
                  <div class="col-12 col-md-6"><strong>Page size</strong><div>{detail.input.pageSize}</div></div>
                  <div class="col-12 col-md-6"><strong>Start</strong><div>{formatFinanceDateTime(detail.input.start)}</div></div>
                  <div class="col-12 col-md-6"><strong>End</strong><div>{formatFinanceDateTime(detail.input.end)}</div></div>
                </div>
              {/if}
            </div>
          </section>
        </div>

        <div class="col-12 col-xl-6">
          <section class="card shadow-sm h-100">
            <div class="card-body p-4 d-grid gap-3">
              <div>
                <h2 class="h5 mb-1">Timeline and worker</h2>
                <p class="text-body-secondary mb-0">Durable lifecycle timestamps and worker assignment.</p>
              </div>

              <div class="row g-3">
                <div class="col-12 col-md-6"><strong>Created</strong><div>{formatFinanceDateTime(detail.createdAt)}</div></div>
                <div class="col-12 col-md-6"><strong>Updated</strong><div>{formatFinanceDateTime(detail.updatedAt)}</div></div>
                <div class="col-12 col-md-6"><strong>Started</strong><div>{formatFinanceDateTime(detail.startedAt)}</div></div>
                <div class="col-12 col-md-6"><strong>Completed</strong><div>{formatFinanceDateTime(detail.completedAt)}</div></div>
                <div class="col-12 col-md-6"><strong>Last attempt</strong><div>{formatFinanceDateTime(detail.lastAttemptAt)}</div></div>
              </div>
            </div>
          </section>
        </div>

        <div class="col-12 col-xl-6">
          <section class="card shadow-sm h-100">
            <div class="card-body p-4 d-grid gap-3">
              <div>
                <h2 class="h5 mb-1">Result</h2>
                <p class="text-body-secondary mb-0">
                  {#if hasHistoricalResult(detail) && detail.result}
                    Current durable result payload.
                  {:else if !detail.jobType.includes('historical')}
                    Result details are not yet specialized for this job type.
                  {:else}
                    No terminal result yet.
                  {/if}
                </p>
              </div>

              {#if hasHistoricalResult(detail) && detail.result}
                <div class="row g-3">
                  <div class="col-12 col-md-6"><strong>persistedCount</strong><div>{detail.result.persistedCount}</div></div>
                  <div class="col-12 col-md-6"><strong>expectedCount</strong><div>{detail.result.expectedCount}</div></div>
                  <div class="col-12 col-md-6"><strong>missingIntervalCount</strong><div>{detail.result.missingIntervalCount}</div></div>
                  <div class="col-12 col-md-6"><strong>duplicateNaturalKeyCount</strong><div>{detail.result.duplicateNaturalKeyCount}</div></div>
                  <div class="col-12 col-md-6"><strong>rawPayloadCount</strong><div>{detail.result.rawPayloadCount ?? '—'}</div></div>
                  <div class="col-12 col-md-6"><strong>missingIntervalPreviewCap</strong><div>{detail.result.missingIntervalPreviewCap}</div></div>
                </div>

                {#if detail.result.missingIntervalPreview.length > 0}
                  <div class="border rounded-3 p-3 bg-body-tertiary">
                    <h3 class="h6 mb-2">Missing interval preview</h3>
                    <ul class="mb-0 ps-3 d-grid gap-1">
                      {#each detail.result.missingIntervalPreview as item (`${item.start.toISOString()}-${item.end.toISOString()}`)}
                        <li>{formatFinanceDateTime(item.start)} → {formatFinanceDateTime(item.end)}</li>
                      {/each}
                    </ul>
                  </div>
                {/if}
              {/if}
            </div>
          </section>
        </div>

        {#if detail.error}
          <div class="col-12 col-xl-6">
            <section class="card border-danger shadow-sm h-100">
              <div class="card-body p-4 d-grid gap-3">
                <div>
                  <h2 class="h5 mb-1">Error</h2>
                  <p class="text-danger mb-0">{detail.error.summary}</p>
                </div>
                <div><strong>Code</strong><div>{detail.error.code}</div></div>
                <div><strong>Details</strong><div>{detail.error.details}</div></div>
              </div>
            </section>
          </div>
        {/if}
      </div>
    {/if}
  </div>
</section>
