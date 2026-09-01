<script lang="ts">
  import { onMount } from 'svelte'
  import { documentTitle } from '../lib/document-title'
  import DocumentTitle from '../components/DocumentTitle.svelte'
  import AdminSubnav from '../components/AdminSubnav.svelte'
  import JobStatus from '../components/JobStatus.svelte'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { createSignalFinanceApiForAuth, type FinanceFXDiagnostics } from '../lib/finance/api'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  let loading = $state(true)
  let error = $state<string | null>(null)
  let diagnostics = $state<FinanceFXDiagnostics | null>(null)
  let provider = $state('')
  let lastJobId = $state('')

  onMount(() => { void loadDiagnostics() })

  async function loadDiagnostics() {
    loading = true
    error = null
    try {
      diagnostics = await financeApi.getFXDiagnostics()
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load FX diagnostics'
    } finally {
      loading = false
    }
  }

  async function triggerSync(event: SubmitEvent) {
    event.preventDefault()
    error = null
    try {
      const result = await financeApi.triggerFXSync({
        ...(provider ? { provider } : {}),
      })
      lastJobId = result.jobId
    } catch (triggerError) {
      error = triggerError instanceof Error ? triggerError.message : 'Failed to trigger current FX refresh'
    }
  }
</script>

<DocumentTitle title={documentTitle('FX diagnostics', 'Admin')} />

<section class="container py-4" aria-labelledby="admin-fx-heading">
  <header class="mb-4">
    <h1 id="admin-fx-heading" class="h3">Admin finance FX</h1>
    <p class="text-body-secondary mb-0">Sanitized diagnostics and required-rate refresh.</p>
  </header>
  <AdminSubnav current="/admin/finance/fx" />
  {#if error}<div class="alert alert-danger mt-3" role="alert">{error}</div>{/if}
  {#if loading}
    <p class="text-body-secondary mt-3" role="status">Loading FX diagnostics…</p>
  {:else if diagnostics}
    <div class="row g-4 mt-1">
      <div class="col-12 col-lg-5">
        <section class="card h-100">
          <div class="card-body">
            <h2 class="h5">Stored diagnostics</h2>
            <p class="text-body-secondary">Default provider {diagnostics.defaultProvider || '—'} · current rates {diagnostics.storedRatesCount}</p>
            <div class="list-group list-group-flush">
              {#each diagnostics.providers as providerState (providerState.name)}
                <div class="list-group-item px-0 d-flex justify-content-between gap-2">
                  <strong>{providerState.name}</strong>
                  <span class="text-body-secondary small">default {String(providerState.default)} · ready {String(providerState.ready)}</span>
                </div>
              {/each}
            </div>
          </div>
        </section>
      </div>
      <div class="col-12 col-lg-7">
        <form class="card" onsubmit={triggerSync}>
          <div class="card-body">
            <h2 class="h5">Refresh required rates</h2>
            <p class="text-body-secondary">Discovers active-tenant account and transaction currency pairs when the job runs, then fetches only the latest required rates.</p>
            <div class="row g-3">
              <div class="col-12 col-md-6">
                <label class="form-label" for="fx-provider">Provider</label>
                <select id="fx-provider" class="form-select" bind:value={provider} aria-label="FX provider">
                  <option value="">Configured default ({diagnostics.defaultProvider || 'none'})</option>
                  {#each diagnostics.providers as providerState (providerState.name)}
                    <option value={providerState.name}>{providerState.name}{providerState.ready ? '' : ' (not ready)'}</option>
                  {/each}
                </select>
              </div>
              <div class="col-12"><button class="btn btn-primary" type="submit">Refresh required rates</button></div>
            </div>
            {#if lastJobId}
              <div class="mt-3">
                <JobStatus jobId={lastJobId} openHref={`/admin/jobs/${encodeURIComponent(lastJobId)}`} label="FX refresh" observedDispatch />
              </div>
            {/if}
          </div>
        </form>
      </div>
    </div>
  {/if}
</section>
