<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import AdminSubnav from '../components/AdminSubnav.svelte'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { createSignalFinanceApiForAuth, type FinanceFXDiagnostics } from '../lib/finance/api'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  let loading = $state(true)
  let error = $state<string | null>(null)
  let diagnostics = $state<FinanceFXDiagnostics | null>(null)
  let provider = $state('')
  let baseCurrencies = $state('USD,EUR')
  let quoteCurrency = $state('PLN')
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
        provider,
        baseCurrencies: baseCurrencies.split(',').map((item) => item.trim()).filter(Boolean),
        quoteCurrency,
      })
      lastJobId = result.jobId
    } catch (triggerError) {
      error = triggerError instanceof Error ? triggerError.message : 'Failed to trigger current FX refresh'
    }
  }
</script>

<section class="container py-4" aria-labelledby="admin-fx-heading">
  <header class="mb-4">
    <h1 id="admin-fx-heading" class="h3">Admin finance FX</h1>
    <p class="text-body-secondary mb-0">Sanitized diagnostics and manual current-rate refresh.</p>
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
            <h2 class="h5">Refresh current FX rates</h2>
            <p class="text-body-secondary">Fetches only the latest rates. Transaction dates and historical ranges are not used.</p>
            <div class="row g-3">
              <div class="col-12 col-md-4"><label class="form-label" for="fx-provider">Provider</label><input id="fx-provider" class="form-control" bind:value={provider} aria-label="FX provider" /></div>
              <div class="col-12 col-md-4"><label class="form-label" for="fx-base-currencies">Base currencies</label><input id="fx-base-currencies" class="form-control" bind:value={baseCurrencies} aria-label="Base currencies" /></div>
              <div class="col-12 col-md-4"><label class="form-label" for="fx-quote-currency">Quote currency</label><input id="fx-quote-currency" class="form-control" bind:value={quoteCurrency} aria-label="Quote currency" required /></div>
              <div class="col-12"><button class="btn btn-primary" type="submit">Refresh current FX rates</button></div>
            </div>
            {#if lastJobId}<p class="text-body-secondary small mt-3 mb-0">Latest refresh job {lastJobId} · <a href={`/admin/jobs/${encodeURIComponent(lastJobId)}`} use:link>open admin job detail</a></p>{/if}
          </div>
        </form>
      </div>
    </div>
  {/if}
</section>
