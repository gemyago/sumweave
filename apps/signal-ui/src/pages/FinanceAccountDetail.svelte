<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import FinanceSubnav from '../components/FinanceSubnav.svelte'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { createSignalFinanceApiForAuth, type FinanceAccount, type FinanceTenantSummary, type FinanceTransaction } from '../lib/finance/api'
  import { chooseFinanceTenantId, setPreferredFinanceTenantId } from '../lib/finance/tenant-selection'
  import { formatFinanceDateTime } from '../lib/finance/format'
  let { params = {} } = $props<{ params?: { accountId?: string } }>()
  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  let loading = $state(true)
  let error = $state<string | null>(null)
  let tenants = $state<FinanceTenantSummary[]>([])
  let selectedTenantId = $state('')
  let account = $state<FinanceAccount | null>(null)
  let transactions = $state<FinanceTransaction[]>([])
  const selectedTenant = $derived(tenants.find((item) => item.id === selectedTenantId) ?? null)
  const needsTenantSelection = $derived(tenants.length > 1 && !selectedTenantId)
  onMount(() => { void loadPage() })

  async function loadPage() {
    loading = true
    error = null
    account = null
    transactions = []
    try {
      tenants = await financeApi.listTenants()
      selectedTenantId = chooseFinanceTenantId(tenants)
      if (!selectedTenantId || !params.accountId) {
        return
      }

      setPreferredFinanceTenantId(selectedTenantId)
      await loadAccountDetail()
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load account detail'
    } finally {
      loading = false
    }
  }

  async function loadAccountDetail() {
    account = null
    transactions = []
    if (!selectedTenantId || !params.accountId) {
      return
    }

    const [accounts, tx] = await Promise.all([
      financeApi.listAccounts({ tenantId: selectedTenantId }),
      financeApi.listTransactions({ tenantId: selectedTenantId, accountId: params.accountId }),
    ])
    account = accounts.find((item) => item.id === params.accountId) ?? null
    transactions = tx
  }

  async function onTenantChange() {
    setPreferredFinanceTenantId(selectedTenantId)
    await loadAccountDetail()
  }
</script>

<section class="page" aria-labelledby="finance-account-detail-heading">
  <header class="row"><div><h1 id="finance-account-detail-heading">Finance account detail</h1><p class="muted">Focused route for one account instead of a split-pane workspace.</p></div><div class="row-links"><a href="/finance/accounts" use:link>Back to accounts</a><a href="/finance/transactions" use:link>Open transactions</a></div></header>
  <FinanceSubnav current="/finance/accounts" tenantName={selectedTenant?.name ?? ''} />
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if loading}<p class="muted" role="status">Loading account detail…</p>{:else}<section class="panel"><label><span>Tenant</span><select bind:value={selectedTenantId} onchange={() => void onTenantChange()} aria-label="Tenant"><option value="">Select tenant</option>{#each tenants as tenant (tenant.id)}<option value={tenant.id}>{tenant.name}</option>{/each}</select></label></section>{#if needsTenantSelection}<section class="panel"><p>Select an active tenant to continue on this finance route.</p></section>{:else if !selectedTenantId}<section class="panel"><p>Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a> before opening this account detail route.</p></section>{:else if account}<section class="panel"><h2>{account.name}</h2><p class="muted">{account.kind} · {account.currency} · Provider {account.provider || 'manual'}</p><p class="muted">Created {formatFinanceDateTime(account.createdAt)} · Updated {formatFinanceDateTime(account.updatedAt)}</p></section><section class="panel"><h2>Recent transactions</h2>{#if transactions.length === 0}<p class="muted">No transactions yet.</p>{:else}<div class="stack">{#each transactions as item (item.id)}<article class="card"><strong>{item.description || item.kind}</strong><span>{item.status}</span><span>{item.amountMinor}</span></article>{/each}</div>{/if}</section>{:else}<p class="muted">Account not found for the selected tenant.</p>{/if}{/if}
</section>

<style>.page,.stack{display:flex;flex-direction:column;gap:var(--space-16)}.row{display:flex;justify-content:space-between;gap:var(--space-16);align-items:flex-start}.row-links{display:flex;flex-wrap:wrap;gap:var(--space-12)}.panel{display:flex;flex-direction:column;gap:var(--space-12);padding:var(--space-16);border:1px solid var(--border);border-radius:4px;background:var(--bg-elevated,var(--bg))}.panel h2,header h1{margin:0}.card{display:flex;justify-content:space-between;gap:var(--space-12);padding:var(--space-12);border:1px solid var(--border);border-radius:4px}.muted{margin:0;color:var(--text-muted)}.error{color:var(--color-danger-red)}</style>
