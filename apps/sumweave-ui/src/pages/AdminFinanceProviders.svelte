<script lang="ts">
  import { onMount } from 'svelte'
  import { documentTitle } from '../lib/document-title'
  import DocumentTitle from '../components/DocumentTitle.svelte'
  import AdminSubnav from '../components/AdminSubnav.svelte'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { createSignalFinanceApiForAuth } from '../lib/finance/api'
  import { createSignalJobsApiForAuth, type JobSummary } from '../lib/jobs/api'
  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const jobsApi = $derived.by(() => createSignalJobsApiForAuth({ baseUrl: appBaseUrl, authStore }))
  let loading = $state(true); let error = $state<string | null>(null); let providers = $state<{ name: string; default: boolean; ready: boolean }[]>([]); let jobs = $state<JobSummary[]>([])
  onMount(() => { void loadPage() })
  async function loadPage() { loading = true; error = null; try { const [diagnostics, financeJobs] = await Promise.all([financeApi.getFXDiagnostics(), jobsApi.listJobs({ jobType: ['finance.bank_connection_sync', 'finance.fx_rates_refresh', 'finance.csv_import', 'finance.account_import'], limit: 10 })]); providers = diagnostics.providers; jobs = financeJobs.items } catch (loadError) { error = loadError instanceof Error ? loadError.message : 'Failed to load provider diagnostics' } finally { loading = false } }
</script>

<DocumentTitle title={documentTitle('Provider diagnostics', 'Admin')} />

<section class="page" aria-labelledby="admin-providers-heading"><header><h1 id="admin-providers-heading">Admin finance providers</h1><p class="muted">Sanitized provider readiness and recent finance job activity without showing secrets or source documents.</p></header><AdminSubnav current="/admin/finance/providers" />{#if error}<p class="error" role="alert">{error}</p>{/if}{#if loading}<p class="muted" role="status">Loading provider diagnostics…</p>{:else}<section class="panel"><h2>Provider readiness</h2><div class="stack">{#each providers as provider (provider.name)}<article class="card"><strong>{provider.name}</strong><span>default {String(provider.default)}</span><span>ready {String(provider.ready)}</span></article>{:else}<p class="muted">No provider diagnostics available.</p>{/each}</div></section><section class="panel"><h2>Recent finance jobs</h2><div class="stack">{#each jobs as job (job.id)}<article class="card"><strong>{job.jobType}</strong><span>{job.status}</span><span>{job.id}</span></article>{:else}<p class="muted">No recent finance jobs were returned.</p>{/each}</div></section>{/if}</section>

<style>.page,.stack{display:flex;flex-direction:column;gap:var(--space-16)}.panel{display:flex;flex-direction:column;gap:var(--space-12);padding:var(--space-16);border:1px solid var(--border);border-radius:4px;background:var(--bg-elevated,var(--bg))}.card{display:flex;justify-content:space-between;gap:var(--space-12);padding:var(--space-12);border:1px solid var(--border);border-radius:4px}.panel h2,header h1{margin:0}.muted{margin:0;color:var(--text-muted)}.error{color:var(--color-danger-red)}</style>
