<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import { createSignalJobsApiForAuth, type JobSummary } from '../lib/jobs/api'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { documentTitle } from '../lib/document-title'

  const financeJobTypes = ['finance.csv_import', 'finance.account_import', 'finance.bank_connection_sync', 'finance.fx_rates_refresh']
  const jobsApi = createSignalJobsApiForAuth({ baseUrl: '/api/v1', authStore })
  let jobs = $state<JobSummary[]>([])
  let error = $state('')
  let nextCursor = $state('')
  let loading = $state(true)

  onMount(() => {
    document.title = documentTitle('Jobs', 'Admin')
    void loadJobs()
  })

  async function loadJobs(cursor = '') {
    loading = true
    error = ''
    try {
      const page = await jobsApi.listJobs(cursor ? { jobType: financeJobTypes, cursor } : { jobType: financeJobTypes })
      jobs = cursor ? [...jobs, ...page.items] : page.items
      nextCursor = page.nextCursor
    } catch (cause) {
      error = cause instanceof Error ? cause.message : 'Unable to load jobs.'
    } finally {
      loading = false
    }
  }
</script>
<section class="container py-4">
  <h1>Admin jobs</h1>
  {#if error}
    <div class="alert alert-danger" role="alert">{error}</div>
  {:else if loading && jobs.length === 0}
    <p role="status">Loading finance jobs…</p>
  {:else if jobs.length === 0}
    <p>No finance jobs found.</p>
  {:else}
    <ul class="list-group">
      {#each jobs as job (job.id)}
        <li class="list-group-item"><a href={`/admin/jobs/${job.id}`} use:link>{job.jobType}</a> — {job.status}</li>
      {/each}
    </ul>
    {#if nextCursor}
      <button class="btn btn-outline-secondary mt-3" disabled={loading} onclick={() => void loadJobs(nextCursor)}>Load more</button>
    {/if}
  {/if}
</section>
