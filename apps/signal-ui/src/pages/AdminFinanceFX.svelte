<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import AdminSubnav from '../components/AdminSubnav.svelte'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { createSignalFinanceApiForAuth, type FinanceFXDiagnostics } from '../lib/finance/api'
  import { formatFinanceDate } from '../lib/finance/format'
  import { dateInputValue, withDateInput } from '../lib/date-range'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  let loading = $state(true)
  let error = $state<string | null>(null)
  let diagnostics = $state<FinanceFXDiagnostics | null>(null)
  let provider = $state('')
  let baseCurrencies = $state('USD,EUR')
  let quoteCurrency = $state('PLN')
  let startDate = $state<Date | undefined>(withDateInput(undefined, '2026-01-01'))
  let endDate = $state<Date | undefined>(withDateInput(undefined, '2026-01-31'))
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
    if (!startDate || !endDate || Number.isNaN(startDate.getTime()) || Number.isNaN(endDate.getTime())) {
      error = 'Choose valid FX timestamps.'
      return
    }
    const result = await financeApi.triggerFXSync({
      provider,
      baseCurrencies: baseCurrencies.split(',').map((item) => item.trim()).filter(Boolean),
      quoteCurrency,
      startDate,
      endDate,
    })
    lastJobId = result.jobId
  }
</script>

<section class="page" aria-labelledby="admin-fx-heading">
  <header><h1 id="admin-fx-heading">Admin finance FX</h1><p class="muted">Sanitized FX diagnostics with manual sync affordance.</p></header>
  <AdminSubnav current="/admin/finance/fx" />
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if loading}
    <p class="muted" role="status">Loading FX diagnostics…</p>
  {:else if diagnostics}
    <section class="panel">
      <h2>Stored diagnostics</h2>
      <p class="muted">Default provider {diagnostics.defaultProvider || '—'} · stored rates {diagnostics.storedRatesCount}</p>
      <div class="stack">
        {#each diagnostics.providers as providerState (providerState.name)}
          <article class="card"><strong>{providerState.name}</strong><span>default {String(providerState.default)}</span><span>ready {String(providerState.ready)}</span></article>
        {/each}
      </div>
    </section>
    <form class="panel form" onsubmit={triggerSync}>
      <h2>Trigger FX sync</h2>
      <label><span>Provider</span><input bind:value={provider} aria-label="FX provider" /></label>
      <label><span>Base currencies</span><input bind:value={baseCurrencies} aria-label="Base currencies" /></label>
      <label><span>Quote currency</span><input bind:value={quoteCurrency} aria-label="Quote currency" required /></label>
      <label>
        <span>Start date</span>
        <input
          value={dateInputValue(startDate)}
          oninput={(event) => startDate = withDateInput(startDate, event.currentTarget.value)}
          type="date"
          aria-label="FX start date"
          required
        />
      </label>
      <label>
        <span>End date</span>
        <input
          value={dateInputValue(endDate)}
          oninput={(event) => endDate = withDateInput(endDate, event.currentTarget.value)}
          type="date"
          aria-label="FX end date"
          required
        />
      </label>
      <button class="primary" type="submit">Trigger FX sync</button>
      {#if lastJobId}<p class="muted">Last sync job {lastJobId} · <a href={`/admin/jobs/${encodeURIComponent(lastJobId)}`} use:link>open admin job detail</a></p>{/if}
      <p class="muted">Selected range starts {formatFinanceDate(startDate)}</p>
    </form>
  {/if}
</section>

<style>.page,.stack{display:flex;flex-direction:column;gap:var(--space-16)}.panel{display:flex;flex-direction:column;gap:var(--space-12);padding:var(--space-16);border:1px solid var(--border);border-radius:4px;background:var(--bg-elevated,var(--bg))}.form{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:var(--space-16);align-items:end}.card{display:flex;justify-content:space-between;gap:var(--space-12);padding:var(--space-12);border:1px solid var(--border);border-radius:4px}.panel h2,header h1{margin:0}.muted{margin:0;color:var(--text-muted)}.error{color:var(--color-danger-red)}</style>
