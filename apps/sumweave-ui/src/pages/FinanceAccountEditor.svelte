<script lang="ts">
  import { onMount } from 'svelte'
  import { documentTitle } from '../lib/document-title'
  import DocumentTitle from '../components/DocumentTitle.svelte'
  import { link } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { createSignalFinanceApiForAuth } from '../lib/finance/api'
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'
  import { supportedFinanceTenantDisplayCurrencies } from '../lib/finance/tenant-display-currencies'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const financeShell = useFinanceShellState()

  let loading = $state(true)
  let saving = $state(false)
  let error = $state<string | null>(null)
  let success = $state<string | null>(null)
  let name = $state('')
  let currency = $state('USD')
  let kind = $state('manual')

  onMount(() => {
    void initialize()
  })

  async function initialize() {
    loading = true
    error = null
    try {
      await financeShell.initialize()
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load account editor'
    } finally {
      loading = false
    }
  }

  async function createAccount(event: SubmitEvent) {
    event.preventDefault()
    if (!financeShell.selectedTenantId) return
    saving = true
    error = null
    success = null
    try {
      await financeApi.createAccount({ tenantId: financeShell.selectedTenantId, name, currency, kind })
      success = 'Account created. You can now record transactions with it.'
      name = ''
    } catch (createError) {
      error = createError instanceof Error ? createError.message : 'Failed to create account'
    } finally {
      saving = false
    }
  }
</script>

<DocumentTitle title={documentTitle('New account', 'Accounts')} />

<section class="container-fluid px-0" aria-labelledby="finance-account-editor-heading">
  <div class="d-grid gap-4">
    <header class="card border-0 shadow-sm">
      <div class="card-body p-4 p-xl-5 d-flex flex-column flex-lg-row justify-content-between gap-3 align-items-lg-center">
        <div>
          <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Accounts</p>
          <h1 id="finance-account-editor-heading" class="h3 mb-2">Create account</h1>
          <p class="text-body-secondary mb-0">Set the account identity and reporting currency before recording activity.</p>
        </div>
        <a class="btn btn-outline-secondary text-nowrap align-self-start align-self-lg-center" href="/finance/accounts" use:link>Back to accounts</a>
      </div>
    </header>

    {#if error}<div class="alert alert-danger mb-0" role="alert">{error}</div>{/if}
    {#if success}<div class="alert alert-success mb-0" role="status">{success}</div>{/if}

    {#if loading}
      <div class="alert alert-secondary mb-0" role="status">Loading account editor…</div>
    {:else if financeShell.needsTenantSelection}
      <div class="alert alert-warning mb-0" role="status">Select an active tenant to continue on this finance route.</div>
    {:else if !financeShell.selectedTenantId}
      <div class="alert alert-light border mb-0" role="status">Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a> before creating an account.</div>
    {:else}
      <form class="card shadow-sm" onsubmit={createAccount} oninput={() => { error = null; success = null }}>
        <div class="card-body p-4 d-grid gap-4">
          <h2 class="h5 mb-0">Account details</h2>
          <div class="row g-3">
            <div class="col-12 col-md-6"><label class="form-label" for="finance-account-name">Name</label><input id="finance-account-name" class="form-control" bind:value={name} aria-label="Account name" required /></div>
            <div class="col-12 col-md-3"><label class="form-label" for="finance-account-currency">Currency</label><select id="finance-account-currency" class="form-select" bind:value={currency} aria-label="Account currency">{#each supportedFinanceTenantDisplayCurrencies as code (code)}<option value={code}>{code}</option>{/each}</select></div>
            <div class="col-12 col-md-3"><label class="form-label" for="finance-account-kind">Kind</label><select id="finance-account-kind" class="form-select" bind:value={kind} aria-label="Account kind"><option value="manual">manual</option><option value="linked">linked</option><option value="imported">imported</option><option value="reconciliation">reconciliation</option></select></div>
          </div>
          <div class="d-flex flex-wrap gap-2"><button class="btn btn-primary" type="submit" disabled={saving}>{saving ? 'Creating…' : 'Create account'}</button><a class="btn btn-outline-secondary" href="/finance/accounts" use:link>Cancel</a></div>
        </div>
      </form>
    {/if}
  </div>
</section>
