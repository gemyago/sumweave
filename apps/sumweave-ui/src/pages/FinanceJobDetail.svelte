<script lang="ts">
  import { onDestroy } from 'svelte'
  import { link, replace } from 'svelte-spa-router'
  import { createSignalJobsApiForAuth, JobsApiError, type JobDetail } from '../lib/jobs/api'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { isRememberedObservedDispatch } from '../lib/jobs/observed-dispatch'

  let { jobId, params = {} } = $props<{ jobId?: string; params?: { jobId?: string } }>()
  let detail = $state<JobDetail | null>(null)
  let error = $state('')
  let pending = $state(false)
  let refreshTimer: ReturnType<typeof setTimeout> | undefined
  let requestToken = 0
  const pollingIntervalMs = 2_000
  const pollingStatuses = new Set(['queued', 'running'])

  $effect(() => {
    const resolvedJobId = jobId ?? params.jobId ?? ''
    detail = null
    error = ''
    pending = false
    clearRefreshTimer()
    if (resolvedJobId) void loadJob(resolvedJobId)
  })

  onDestroy(() => {
    requestToken += 1
    clearRefreshTimer()
  })

  async function loadJob(resolvedJobId: string) {
    const token = ++requestToken
    clearRefreshTimer()
    pending = false
    error = ''
    try {
      const loaded = await createSignalJobsApiForAuth({ baseUrl: '/api/v1', authStore }).getJob({ jobId: resolvedJobId })
      if (token !== requestToken || resolvedJobId !== (jobId ?? params.jobId ?? '')) return
      detail = loaded
      if (pollingStatuses.has(loaded.status)) {
        refreshTimer = setTimeout(() => void loadJob(resolvedJobId), pollingIntervalMs)
      }
    } catch (cause) {
      if (token !== requestToken || resolvedJobId !== (jobId ?? params.jobId ?? '')) return
      if (cause instanceof JobsApiError && cause.status === 404 && isRememberedObservedDispatch(resolvedJobId)) {
        pending = true
        refreshTimer = setTimeout(() => void loadJob(resolvedJobId), pollingIntervalMs)
        return
      }
      error = cause instanceof Error ? cause.message : 'Unable to load job.'
    }
  }

  function clearRefreshTimer() {
    if (refreshTimer) {
      clearTimeout(refreshTimer)
      refreshTimer = undefined
    }
  }
</script>

<section class="container py-4">
  <p><a href="/finance" use:link>Finance</a> / <a href="/admin/jobs" use:link>Jobs</a> / Job</p>
  <h1>Finance job</h1>
  {#if error}<div class="alert alert-danger" role="alert">{error}</div>{:else if pending}<p class="text-body-secondary" role="status">Waiting for a worker to receive this job…</p>{:else if !detail}<p>Loading job…</p>{:else}
    <dl class="row"><dt class="col-sm-3">Type</dt><dd class="col-sm-9">{detail.jobType}</dd><dt class="col-sm-3">Status</dt><dd class="col-sm-9">{detail.status}</dd><dt class="col-sm-3">Attempts</dt><dd class="col-sm-9">{detail.attemptCount}</dd><dt class="col-sm-3">Worker</dt><dd class="col-sm-9">{detail.workerId || '—'}</dd></dl>
    {#if detail.error}<div class="alert alert-warning"><strong>{detail.error.summary}</strong></div>{/if}
  {/if}
  <button class="btn btn-outline-secondary" onclick={() => replace('/admin/jobs')}>Back to jobs</button>
</section>
