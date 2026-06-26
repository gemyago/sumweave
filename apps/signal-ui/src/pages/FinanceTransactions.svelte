<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import FinanceSubnav from '../components/FinanceSubnav.svelte'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalFinanceApiForAuth,
    type FinanceAccount,
    type FinanceTenantSummary,
    type FinanceTransaction,
  } from '../lib/finance/api'
  import { formatFinanceDateTime } from '../lib/finance/format'
  import { chooseFinanceTenantId, setPreferredFinanceTenantId } from '../lib/finance/tenant-selection'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))

  let loading = $state(true)
  let error = $state<string | null>(null)
  let tenants = $state<FinanceTenantSummary[]>([])
  let selectedTenantId = $state('')
  let accounts = $state<FinanceAccount[]>([])
  let transactions = $state<FinanceTransaction[]>([])
  let accountFilter = $state('')
  let statusFilter = $state('')
  let sourceFilter = $state('')
  let sortOrder = $state('desc')

  const selectedTenant = $derived(tenants.find((item) => item.id === selectedTenantId) ?? null)
  const visibleTransactions = $derived(
    [...transactions].sort((a, b) =>
      sortOrder === 'asc'
        ? a.effectiveAt.getTime() - b.effectiveAt.getTime()
        : b.effectiveAt.getTime() - a.effectiveAt.getTime(),
    ),
  )

  onMount(() => {
    void loadPage()
  })

  async function loadPage() {
    loading = true
    error = null
    try {
      tenants = await financeApi.listTenants()
      selectedTenantId = chooseFinanceTenantId(tenants)
      if (selectedTenantId) {
        setPreferredFinanceTenantId(selectedTenantId)
        await loadTenantData()
      } else {
        accounts = []
        transactions = []
      }
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load transactions'
    } finally {
      loading = false
    }
  }

  async function loadTenantData() {
    if (!selectedTenantId) {
      accounts = []
      transactions = []
      return
    }
    ;[accounts, transactions] = await Promise.all([
      financeApi.listAccounts({ tenantId: selectedTenantId }),
      financeApi.listTransactions({
        tenantId: selectedTenantId,
        accountId: accountFilter,
        status: statusFilter,
        source: sourceFilter,
      }),
    ])
  }

  async function onTenantChange() {
    setPreferredFinanceTenantId(selectedTenantId)
    accountFilter = ''
    statusFilter = ''
    sourceFilter = ''
    await loadTenantData()
  }

  function transactionFlags(item: FinanceTransaction): string[] {
    return [
      item.status === 'pending' ? 'pending' : '',
      item.hiddenAt ? 'hidden' : '',
      item.transferGroupId ? 'transfer' : '',
      item.kind === 'refund' ? 'refund' : '',
      item.kind === 'reconciliation' ? 'reconciliation' : '',
    ].filter(Boolean)
  }
</script>

<section class="page" aria-labelledby="finance-transactions-heading">
  <header class="hero">
    <div>
      <h1 id="finance-transactions-heading">Finance transactions</h1>
      <p class="muted">
        Browse transaction cards, keep state cues visible, and jump into dedicated create or edit routes.
      </p>
    </div>
    <a class="primary action-link" href="/finance/transactions/new" use:link>Create transaction</a>
  </header>

  <FinanceSubnav current="/finance/transactions" tenantName={selectedTenant?.name ?? ''} />

  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}

  {#if loading}
    <p class="muted" role="status">Loading transactions…</p>
  {:else}
    <section class="panel filters">
      <label>
        <span>Tenant</span>
        <select bind:value={selectedTenantId} onchange={() => void onTenantChange()} aria-label="Tenant">
          <option value="">Select tenant</option>
          {#each tenants as tenant (tenant.id)}
            <option value={tenant.id}>{tenant.name}</option>
          {/each}
        </select>
      </label>
      <label>
        <span>Account</span>
        <select bind:value={accountFilter} onchange={() => void loadTenantData()} aria-label="Account filter">
          <option value="">Any account</option>
          {#each accounts as account (account.id)}
            <option value={account.id}>{account.name}</option>
          {/each}
        </select>
      </label>
      <label>
        <span>Status</span>
        <select bind:value={statusFilter} onchange={() => void loadTenantData()} aria-label="Transaction status filter">
          <option value="">Any status</option>
          <option value="pending">pending</option>
          <option value="booked">booked</option>
        </select>
      </label>
      <label>
        <span>Source</span>
        <select bind:value={sourceFilter} onchange={() => void loadTenantData()} aria-label="Transaction source filter">
          <option value="">Any source</option>
          <option value="manual">manual</option>
          <option value="provider">provider</option>
          <option value="csv">csv</option>
          <option value="system">system</option>
        </select>
      </label>
      <label>
        <span>Sort</span>
        <select bind:value={sortOrder} aria-label="Sort order">
          <option value="desc">Newest first</option>
          <option value="asc">Oldest first</option>
        </select>
      </label>
    </section>

    {#if !selectedTenantId}
      <section class="panel">
        <p class="muted">Select a finance tenant to load transaction history and editor links.</p>
      </section>
    {:else}
      <div class="stack">
        {#each visibleTransactions as item (item.id)}
          <article class="panel transaction-card">
            <div class="row">
              <div>
                <h2>{item.description || item.kind}</h2>
                <p class="muted">{item.source} · {item.status} · {item.currency} {item.amountMinor}</p>
              </div>
              <div class="actions">
                <div class="flags" aria-label="Transaction state flags">
                  {#each transactionFlags(item) as flag (flag)}
                    <span>{flag}</span>
                  {/each}
                </div>
                <a href={`/finance/transactions/${item.id}`} use:link>Open transaction</a>
              </div>
            </div>
            <p class="muted">
              Effective {formatFinanceDateTime(item.effectiveAt)} · category {item.categoryId || '—'}
            </p>
          </article>
        {:else}
          <p class="muted">No transactions matched the current filters.</p>
        {/each}
      </div>
    {/if}
  {/if}
</section>

<style>
  .page,
  .stack {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
  }

  .hero,
  .row,
  .actions {
    display: flex;
    gap: var(--space-16);
  }

  .hero,
  .row {
    justify-content: space-between;
    align-items: flex-start;
  }

  .actions {
    flex-direction: column;
    align-items: flex-end;
  }

  .panel {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
    padding: var(--space-16);
    border: 1px solid var(--border);
    border-radius: 4px;
    background: var(--bg-elevated, var(--bg));
  }

  .filters {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    gap: var(--space-16);
    align-items: end;
  }

  .flags {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-8);
    justify-content: flex-end;
  }

  .flags span {
    padding: 2px 6px;
    border: 1px solid var(--border);
    border-radius: 4px;
  }

  .action-link {
    text-decoration: none;
  }

  .panel h2,
  .hero h1 {
    margin: 0;
  }

  .muted {
    margin: 0;
    color: var(--text-muted);
  }

  .error {
    color: var(--color-danger-red);
  }

  @media (max-width: 640px) {
    .hero,
    .row {
      flex-direction: column;
    }

    .actions {
      width: 100%;
      align-items: flex-start;
    }

    .flags {
      justify-content: flex-start;
    }
  }
</style>
