<script lang="ts">
  import { onMount } from 'svelte'
  import { link, replace } from 'svelte-spa-router'
  import { createSignalJobsApiForAuth, type JobDetail } from '../lib/jobs/api'
  import { authStore } from '../lib/auth/auth-store.svelte'

  let { jobId, params = {} } = $props<{ jobId?: string; params?: { jobId?: string } }>()
  let detail = $state<JobDetail | null>(null)
  let error = $state('')
  onMount(async () => { const resolvedJobId = jobId ?? params.jobId ?? ''; try { detail = await createSignalJobsApiForAuth({ baseUrl: '/api/v1', authStore }).getJob({ jobId: resolvedJobId }) } catch (cause) { error = cause instanceof Error ? cause.message : 'Unable to load job.' } })
</script>

<section class="container py-4">
  <p><a href="/finance" use:link>Finance</a> / <a href="/admin/jobs" use:link>Jobs</a> / Job</p>
  <h1>Finance job</h1>
  {#if error}<div class="alert alert-danger" role="alert">{error}</div>{:else if !detail}<p>Loading job…</p>{:else}
    <dl class="row"><dt class="col-sm-3">Type</dt><dd class="col-sm-9">{detail.jobType}</dd><dt class="col-sm-3">Status</dt><dd class="col-sm-9">{detail.status}</dd><dt class="col-sm-3">Attempts</dt><dd class="col-sm-9">{detail.attemptCount}</dd><dt class="col-sm-3">Worker</dt><dd class="col-sm-9">{detail.workerId || '—'}</dd></dl>
    {#if detail.error}<div class="alert alert-warning"><strong>{detail.error.summary}</strong></div>{/if}
  {/if}
  <button class="btn btn-outline-secondary" onclick={() => replace('/admin/jobs')}>Back to jobs</button>
</section>
