<script lang="ts">
  import { onDestroy } from 'svelte'
  import { link } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { createSignalJobsApiForAuth, JobsApiError, type JobDetail } from '../lib/jobs/api'
  import { rememberObservedDispatch } from '../lib/jobs/observed-dispatch'

  let { jobId, openHref, label = 'Job', observedDispatch = false } = $props<{
    jobId: string
    openHref: string
    label?: string
    observedDispatch?: boolean
  }>()

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const jobsApi = $derived.by(() => createSignalJobsApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const pollingStatuses = new Set(['queued', 'running'])
  const pollingIntervalMs = 2_000

  let job = $state<JobDetail | null>(null)
  let refreshing = $state(false)
  let waitingForMaterialization = $state(false)
  let error = $state<string | null>(null)
  let requestToken = 0
  let refreshTimer: ReturnType<typeof setTimeout> | null = null

  $effect(() => {
    const activeJobId = jobId
    job = null
    error = null
    waitingForMaterialization = false
    clearRefreshTimer()
    if (activeJobId) {
      if (observedDispatch) rememberObservedDispatch(activeJobId)
      void loadJob(activeJobId)
    }
  })

  onDestroy(() => {
    requestToken += 1
    clearRefreshTimer()
  })

  async function loadJob(activeJobId: string) {
    const token = ++requestToken
    clearRefreshTimer()
    refreshing = true
    error = null
    waitingForMaterialization = false

    try {
      const loaded = await jobsApi.getJob({ jobId: activeJobId })
      if (token !== requestToken || activeJobId !== jobId) return

      job = loaded
      if (pollingStatuses.has(loaded.status)) {
        refreshTimer = setTimeout(() => void loadJob(activeJobId), pollingIntervalMs)
      }
    } catch (loadError) {
      if (token !== requestToken || activeJobId !== jobId) return
      if (observedDispatch && loadError instanceof JobsApiError && loadError.status === 404) {
        waitingForMaterialization = true
        refreshTimer = setTimeout(() => void loadJob(activeJobId), pollingIntervalMs)
        return
      }
      error = loadError instanceof Error ? loadError.message : `Could not load ${label.toLowerCase()} status.`
    } finally {
      if (token === requestToken) refreshing = false
    }
  }

  function clearRefreshTimer() {
    if (refreshTimer !== null) {
      clearTimeout(refreshTimer)
      refreshTimer = null
    }
  }

  function statusBadgeClass(status: string) {
    if (status === 'succeeded') return 'text-bg-success'
    if (status === 'failed') return 'text-bg-danger'
    if (status === 'running') return 'text-bg-primary'
    return 'text-bg-secondary'
  }

  function statusMessage(item: JobDetail) {
    if (item.status === 'queued') return 'Queued — waiting for a worker.'
    if (item.status === 'running') return 'Running now.'
    if (item.status === 'succeeded') return 'Completed.'
    if (item.status === 'failed') return item.error?.summary || 'Failed.'
    return item.status
  }
</script>

<div class="border rounded p-2 small d-flex flex-column flex-sm-row align-items-sm-center justify-content-between gap-2" aria-live="polite">
  <div class="d-flex flex-wrap align-items-center gap-2">
    {#if job}
      <span class={`badge ${statusBadgeClass(job.status)}`}>{job.status}</span>
      <span>{statusMessage(job)}</span>
    {:else if waitingForMaterialization}
      <span class="text-body-secondary">Waiting for a worker to receive this {label.toLowerCase()}…</span>
    {:else if refreshing}
      <span class="text-body-secondary">Checking {label.toLowerCase()} status…</span>
    {:else if error}
      <span class="text-danger">{error}</span>
    {/if}
  </div>
  <div class="d-flex flex-wrap gap-2 align-items-center">
    {#if error}
      <button class="btn btn-outline-secondary btn-sm" type="button" onclick={() => void loadJob(jobId)}>Retry status</button>
    {/if}
    <a href={openHref} use:link>Open job</a>
  </div>
</div>
