<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { createSignalFinanceApiForAuth, type FinanceAccount } from '../lib/finance/api'
  import { formatFinanceDateTime } from '../lib/finance/format'
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const financeShell = useFinanceShellState()
  let loading = $state(true)
  let error = $state<string | null>(null)
  let accounts = $state<FinanceAccount[]>([])
  let includeHidden = $state(false)
  let accountName = $state('')
  let accountCurrency = $state('USD')
  let accountKind = $state('manual')
  let reactiveReady = $state(false)
  let skipNextReactiveLoad = false
  onMount(() => { void loadPage() })
  async function loadPage() { loading = true; reactiveReady = false; error = null; try { await financeShell.initialize(); await loadAccounts() } catch (loadError) { error = loadError instanceof Error ? loadError.message : 'Failed to load accounts' } finally { skipNextReactiveLoad = true; reactiveReady = true; loading = false } }
  async function loadAccounts() { if (!financeShell.selectedTenantId) { accounts = []; return } accounts = await financeApi.listAccounts({ tenantId: financeShell.selectedTenantId, includeHidden }) }
  async function createAccount(event: SubmitEvent) { event.preventDefault(); await financeApi.createAccount({ tenantId: financeShell.selectedTenantId, name: accountName, currency: accountCurrency, kind: accountKind }); accountName = ''; await loadAccounts() }

  $effect(() => {
    if (financeShell.loading || !reactiveReady) return
    void financeShell.selectedTenantId
    void includeHidden
    if (skipNextReactiveLoad) {
      skipNextReactiveLoad = false
      return
    }
    void loadAccounts()
  })
</script>

<section class="page" aria-labelledby="finance-accounts-heading">
  <header><h1 id="finance-accounts-heading">Finance accounts</h1><p class="muted">Stacked account summaries with a separate detail route for each account.</p></header>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if loading}<p class="muted" role="status">Loading accounts…</p>{:else}
    <section class="panel controls">{#if !financeShell.embedded}<label><span>Tenant</span><select value={financeShell.selectedTenantId} onchange={(event) => financeShell.selectTenant((event.currentTarget as HTMLSelectElement).value)} aria-label="Tenant"><option value="">Select tenant</option>{#each financeShell.tenants as tenant (tenant.id)}<option value={tenant.id}>{tenant.name}</option>{/each}</select></label>{/if}<label class="checkbox"><input type="checkbox" bind:checked={includeHidden} /> Include hidden</label></section>
    {#if financeShell.needsTenantSelection}<section class="panel"><p>Select an active tenant to continue on this finance route.</p></section>{:else if !financeShell.selectedTenantId}<section class="panel"><p>Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a> before managing accounts.</p></section>{/if}
    <form class="panel create-form" onsubmit={createAccount}>
      <h2>Create account</h2>
      <label><span>Name</span><input bind:value={accountName} aria-label="Account name" required /></label>
      <label><span>Currency</span><input bind:value={accountCurrency} aria-label="Account currency" required /></label>
      <label><span>Kind</span><select bind:value={accountKind} aria-label="Account kind"><option value="manual">manual</option><option value="linked">linked</option><option value="imported">imported</option><option value="reconciliation">reconciliation</option></select></label>
      <button class="primary" type="submit" disabled={!financeShell.selectedTenantId}>Create account</button>
    </form>
    <div class="stack">{#each accounts as account (account.id)}<article class="panel"><div class="row"><div><h2>{account.name}</h2><p class="muted">{account.kind} · {account.currency}</p></div><a href={`/finance/accounts/${encodeURIComponent(account.id)}`} use:link>Open account detail</a></div><p class="muted">Provider {account.provider || 'manual'} · Updated {formatFinanceDateTime(account.updatedAt)}</p></article>{:else}<p class="muted">No accounts yet.</p>{/each}</div>
  {/if}
</section>

<style>.page,.stack{display:flex;flex-direction:column;gap:var(--space-16)}.panel{display:flex;flex-direction:column;gap:var(--space-12);padding:var(--space-16);border:1px solid var(--border);border-radius:4px;background:var(--bg-elevated,var(--bg))}.controls,.create-form{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:var(--space-16);align-items:end}.row{display:flex;justify-content:space-between;gap:var(--space-12);align-items:flex-start}.panel h2,header h1{margin:0}.muted{margin:0;color:var(--text-muted)}.checkbox{display:flex;gap:var(--space-8);align-items:center}label{display:flex;flex-direction:column;gap:var(--space-8)}.error{color:var(--color-danger-red)}</style>
