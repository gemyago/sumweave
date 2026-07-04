<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalFinanceApiForAuth,
    type FinanceAccount,
    type FinanceTransaction,
  } from '../lib/finance/api'
  import { formatFinanceDateTime, formatFinanceMoney } from '../lib/finance/format'
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const financeShell = useFinanceShellState()

  let loading = $state(true)
  let error = $state<string | null>(null)
  let accounts = $state<FinanceAccount[]>([])
  let transactions = $state<FinanceTransaction[]>([])
  let accountFilter = $state('')
  let statusFilter = $state('')
  let sourceFilter = $state('')
  let sortOrder = $state('desc')
  let selectedTransactionId = $state('')
  let loadingList = $state(false)
  let reactiveReady = $state(false)
  let skipNextReactiveLoad = false
  const visibleTransactions = $derived(
    [...transactions].sort((a, b) =>
      sortOrder === 'asc'
        ? a.effectiveAt.getTime() - b.effectiveAt.getTime()
        : b.effectiveAt.getTime() - a.effectiveAt.getTime(),
    ),
  )
  const accountNameById = $derived.by(
    () => new Map(accounts.map((account) => [account.id, account.name])),
  )
  const selectedTransaction = $derived.by(
    () => visibleTransactions.find((item) => item.id === selectedTransactionId) ?? visibleTransactions[0] ?? null,
  )
  const visiblePendingCount = $derived(
    visibleTransactions.filter((item) => item.status === 'pending').length,
  )
  const visibleHiddenCount = $derived(
    visibleTransactions.filter((item) => item.hiddenAt !== null).length,
  )
  const activeFilterCount = $derived(
    [accountFilter, statusFilter, sourceFilter].filter(Boolean).length,
  )

  onMount(() => {
    void loadPage()
  })

  async function loadPage() {
    loading = true
    error = null
    reactiveReady = false
    try {
      await financeShell.initialize()
      if (financeShell.selectedTenantId) {
        await loadTenantData()
      } else {
        accounts = []
        transactions = []
      }
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load transactions'
    } finally {
      skipNextReactiveLoad = true
      reactiveReady = true
      loading = false
    }
  }

  async function loadTenantData() {
    if (!financeShell.selectedTenantId) {
      accounts = []
      transactions = []
      selectedTransactionId = ''
      return
    }
    loadingList = true
    error = null
    try {
      const [loadedAccounts, loadedTransactions] = await Promise.all([
        financeApi.listAccounts({ tenantId: financeShell.selectedTenantId }),
        financeApi.listTransactions({
          tenantId: financeShell.selectedTenantId,
          accountId: accountFilter,
          status: statusFilter,
          source: sourceFilter,
        }),
      ])

      accounts = loadedAccounts
      transactions = loadedTransactions
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load transactions'
    } finally {
      loadingList = false
    }
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

  function accountName(accountId: string): string {
    return accountNameById.get(accountId) ?? 'Unknown account'
  }

  function selectTransaction(id: string): void {
    selectedTransactionId = id
  }

  $effect(() => {
    const ids = visibleTransactions.map((item) => item.id)
    if (ids.length === 0) {
      if (selectedTransactionId) {
        selectedTransactionId = ''
      }
      return
    }

    if (!ids.includes(selectedTransactionId)) {
      selectedTransactionId = ids[0]
    }
  })

  $effect(() => {
    if (financeShell.loading || !reactiveReady) return
    void financeShell.selectedTenantId
    if (skipNextReactiveLoad) {
      skipNextReactiveLoad = false
      return
    }
    void loadTenantData()
  })
</script>

<section class="page" aria-labelledby="finance-transactions-heading">
  <header class="hero panel">
    <div class="hero-copy">
      <p class="eyebrow">Transactions workspace</p>
      <h1 id="finance-transactions-heading">Finance transactions</h1>
      <p class="muted">
        Review a wider ledger table, keep filter context visible, and open the selected record in a dedicated editor route.
      </p>
    </div>

    <div class="hero-actions">
      <a href="/finance/imports" use:link>Import CSV</a>
      <a class="primary action-link" href="/finance/transactions/new" use:link>Create transaction</a>
    </div>
  </header>

  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}

  {#if loading}
    <p class="muted" role="status">Loading transactions…</p>
  {:else if financeShell.needsTenantSelection}
    <section class="panel stack">
      {#if !financeShell.embedded}
        <label>
          <span>Tenant</span>
          <select value={financeShell.selectedTenantId} onchange={(event) => financeShell.selectTenant((event.currentTarget as HTMLSelectElement).value)} aria-label="Tenant">
            <option value="">Select tenant</option>
            {#each financeShell.tenants as tenant (tenant.id)}
              <option value={tenant.id}>{tenant.name}</option>
            {/each}
          </select>
        </label>
      {/if}
      <p>Select an active tenant to continue on this finance route.</p>
    </section>
  {:else if !financeShell.selectedTenantId}
    <section class="panel"><p>Select a finance tenant to load transaction history and editor links.</p></section>
  {:else}
    <section class="panel filters-panel">
      <div class="filters-header">
        <div>
          <p class="eyebrow">Filters</p>
          <p class="muted">Keep browse context visible while switching between the ledger and the selected transaction inspector.</p>
        </div>

        <div class="summary-chips" aria-label="Transaction summaries">
          <span>{visibleTransactions.length} visible</span>
          <span>{visiblePendingCount} pending</span>
          <span>{visibleHiddenCount} hidden</span>
          <span>{activeFilterCount} filters</span>
        </div>
      </div>

      <div class="filters">
        {#if !financeShell.embedded}
          <label>
            <span>Tenant</span>
            <select value={financeShell.selectedTenantId} onchange={(event) => financeShell.selectTenant((event.currentTarget as HTMLSelectElement).value)} aria-label="Tenant">
              <option value="">Select tenant</option>
              {#each financeShell.tenants as tenant (tenant.id)}
                <option value={tenant.id}>{tenant.name}</option>
              {/each}
            </select>
          </label>
        {/if}
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
      </div>
    </section>

    <section class="transactions-layout">
      <article class="panel ledger-panel">
        <div class="ledger-header">
          <div>
            <p class="eyebrow">Ledger</p>
            <h2>Transactions</h2>
          </div>
          {#if loadingList}
            <p class="muted" role="status">Refreshing transactions…</p>
          {/if}
        </div>

        {#if visibleTransactions.length === 0}
          <p class="muted">No transactions matched the current filters.</p>
        {:else}
          <div class="ledger-table-scroll">
            <table class="ledger-table" aria-label="Transactions ledger">
              <thead>
                <tr>
                  <th scope="col">Description</th>
                  <th scope="col">Effective</th>
                  <th scope="col">Account</th>
                  <th scope="col">Category</th>
                  <th scope="col">Source</th>
                  <th scope="col">Amount</th>
                  <th scope="col">State</th>
                  <th scope="col">Open</th>
                </tr>
              </thead>
              <tbody>
                {#each visibleTransactions as item (item.id)}
                  <tr class:selected={selectedTransaction?.id === item.id}>
                    <td data-label="Description">
                      <button
                        type="button"
                        class="row-button"
                        aria-pressed={selectedTransaction?.id === item.id}
                        onclick={() => selectTransaction(item.id)}
                      >
                        <span class="row-title">{item.description || item.kind}</span>
                        <span class="row-subtitle">{item.kind} · {item.status}</span>
                      </button>
                    </td>
                    <td data-label="Effective">{formatFinanceDateTime(item.effectiveAt)}</td>
                    <td data-label="Account">{accountName(item.accountId)}</td>
                    <td data-label="Category">{item.categoryId || '—'}</td>
                    <td data-label="Source">{item.source}</td>
                    <td data-label="Amount" class="amount-cell">{formatFinanceMoney(item.amountMinor, item.currency)}</td>
                    <td data-label="State">
                      <div class="flags" aria-label="Transaction state flags">
                        {#each transactionFlags(item) as flag (flag)}
                          <span>{flag}</span>
                        {/each}
                        {#if transactionFlags(item).length === 0}
                          <span class="flag-empty">clear</span>
                        {/if}
                      </div>
                    </td>
                    <td data-label="Open">
                      <a href={`/finance/transactions/${item.id}`} use:link>Open record</a>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </article>

      <aside class="panel inspector-panel" aria-label="Selected transaction details">
        {#if selectedTransaction}
          <div class="inspector-header">
            <div>
              <p class="eyebrow">Selected transaction</p>
              <h2>{selectedTransaction.description || selectedTransaction.kind}</h2>
            </div>
            <a href={`/finance/transactions/${selectedTransaction.id}`} use:link>Open transaction</a>
          </div>

          <p class="inspector-amount">{formatFinanceMoney(selectedTransaction.amountMinor, selectedTransaction.currency)}</p>

          <div class="summary-chips summary-chips--compact">
            <span>{selectedTransaction.status}</span>
            <span>{selectedTransaction.source}</span>
            <span>{selectedTransaction.kind}</span>
          </div>

          <dl class="inspector-meta">
            <div>
              <dt>Effective</dt>
              <dd>{formatFinanceDateTime(selectedTransaction.effectiveAt)}</dd>
            </div>
            <div>
              <dt>Account</dt>
              <dd>{accountName(selectedTransaction.accountId)}</dd>
            </div>
            <div>
              <dt>Category</dt>
              <dd>{selectedTransaction.categoryId || '—'}</dd>
            </div>
            <div>
              <dt>Transfer group</dt>
              <dd>{selectedTransaction.transferGroupId || '—'}</dd>
            </div>
            <div>
              <dt>Hidden</dt>
              <dd>{selectedTransaction.hiddenAt ? 'Yes' : 'No'}</dd>
            </div>
            <div>
              <dt>Updated</dt>
              <dd>{formatFinanceDateTime(selectedTransaction.updatedAt)}</dd>
            </div>
          </dl>

          {#if selectedTransaction.providerOriginal}
            <section class="provider-original">
              <p class="eyebrow">Provider original</p>
              <dl class="inspector-meta">
                <div>
                  <dt>Amount</dt>
                  <dd>{formatFinanceMoney(selectedTransaction.providerOriginal.amountMinor, selectedTransaction.providerOriginal.currency)}</dd>
                </div>
                <div>
                  <dt>Description</dt>
                  <dd>{selectedTransaction.providerOriginal.description || '—'}</dd>
                </div>
                <div>
                  <dt>Effective</dt>
                  <dd>{formatFinanceDateTime(selectedTransaction.providerOriginal.effectiveAt)}</dd>
                </div>
              </dl>
            </section>
          {/if}

          <div class="inspector-actions">
            <a href={`/finance/accounts/${selectedTransaction.accountId}`} use:link>Open account</a>
            <a href={`/finance/transactions/${selectedTransaction.id}`} use:link>Edit transaction</a>
          </div>
        {:else}
          <p class="muted">Select a transaction row to inspect it here.</p>
        {/if}
      </aside>
    </section>
  {/if}
</section>

<style>
  .page,
  .filters-panel,
  .hero-copy,
  .filters-header,
  .ledger-panel,
  .inspector-panel,
  .provider-original {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
  }

  .hero,
  .hero-actions,
  .filters,
  .transactions-layout,
  .ledger-header,
  .inspector-header,
  .inspector-actions,
  .summary-chips,
  .flags {
    display: flex;
    gap: var(--space-16);
  }

  .hero,
  .ledger-header,
  .inspector-header {
    justify-content: space-between;
    align-items: flex-start;
  }

  .panel {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
    padding: var(--space-18);
    border: 1px solid var(--border);
    border-radius: 4px;
    background: var(--surface-raised);
  }

  .hero {
    flex-direction: row;
    justify-content: space-between;
    align-items: center;
  }

  .filters {
    grid-template-columns: minmax(200px, 1.2fr) repeat(4, minmax(140px, 1fr));
    align-items: end;
  }

  .filters-panel {
    gap: var(--space-14);
  }

  .filters-header {
    justify-content: space-between;
    gap: var(--space-12);
  }

  .flags {
    flex-wrap: wrap;
    gap: var(--space-8);
    align-items: center;
  }

  .flags span {
    padding: 2px 6px;
    border: 1px solid var(--border);
    border-radius: 4px;
    font-size: var(--font-size-caption);
  }

  .flag-empty {
    color: var(--text-muted);
  }

  .summary-chips {
    flex-wrap: wrap;
  }

  .summary-chips span {
    display: inline-flex;
    align-items: center;
    min-height: 1.7rem;
    padding: 0 var(--space-8);
    border: 1px solid var(--border);
    border-radius: 4px;
    background: var(--bg);
    font-size: var(--font-size-caption);
  }

  .summary-chips--compact {
    gap: var(--space-8);
  }

  .action-link {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-height: 2.25rem;
    padding: var(--space-4) var(--space-20);
    border: 1px solid var(--btn-primary-border);
    border-radius: 4px;
    background: var(--btn-primary-bg);
    color: var(--btn-primary-fg);
    text-decoration: none;
  }

  .panel h2,
  .hero h1 {
    margin: 0;
  }

  .hero-actions {
    align-items: center;
    gap: var(--space-12);
  }

  .muted {
    margin: 0;
    color: var(--text-muted);
  }

  .eyebrow {
    margin: 0;
    color: var(--text-muted);
    font-size: var(--font-size-caption);
  }

  .transactions-layout {
    display: grid;
    grid-template-columns: minmax(0, 1.55fr) minmax(360px, 1fr);
    align-items: start;
  }

  .filters {
    display: grid;
  }

  .ledger-table-scroll {
    overflow-x: auto;
  }

  .ledger-table {
    width: 100%;
    border-collapse: collapse;
    min-width: 820px;
  }

  .ledger-table th,
  .ledger-table td {
    padding: 0.625rem var(--space-8);
    border-bottom: 1px solid var(--border);
    text-align: left;
    vertical-align: top;
  }

  .ledger-table th {
    color: var(--text-muted);
    font-size: var(--font-size-caption);
    font-weight: 500;
  }

  .ledger-table tbody tr.selected {
    background: color-mix(in srgb, var(--bg) 45%, var(--surface-raised));
  }

  .row-button {
    width: 100%;
    padding: 0;
    border: none;
    background: transparent;
    color: inherit;
    text-align: left;
    font: inherit;
    cursor: pointer;
  }

  .row-title,
  .inspector-amount {
    display: block;
    color: var(--text-h);
  }

  .row-subtitle {
    display: block;
    color: var(--text-muted);
    font-size: var(--font-size-caption);
  }

  .amount-cell {
    white-space: nowrap;
    color: var(--text-h);
  }

  .inspector-panel {
    position: sticky;
    top: var(--space-16);
  }

  .inspector-amount {
    margin: 0;
    font-size: clamp(1.5rem, 2vw, 2.25rem);
    line-height: 1.1;
  }

  .inspector-meta {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--space-12) var(--space-16);
    margin: 0;
  }

  .inspector-meta div {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    padding: var(--space-12);
    border: 1px solid var(--border);
    border-radius: 4px;
    background: var(--bg);
  }

  .inspector-meta dt {
    color: var(--text-muted);
    font-size: var(--font-size-caption);
  }

  .inspector-meta dd {
    margin: 0;
    color: var(--text-h);
  }

  .inspector-actions {
    flex-wrap: wrap;
  }

  .filters label {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }

  .error {
    margin: 0;
    padding: var(--space-12) var(--space-16);
    border: 1px solid var(--danger-border);
    border-radius: 4px;
    background: var(--danger-bg);
    color: var(--color-danger);
  }

  @media (max-width: 1100px) {
    .filters {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .transactions-layout {
      grid-template-columns: 1fr;
    }

    .inspector-panel {
      position: static;
    }
  }

  @media (max-width: 720px) {
    .hero,
    .filters-header,
    .ledger-header,
    .inspector-header {
      flex-direction: column;
    }

    .hero-actions {
      width: 100%;
      justify-content: flex-start;
    }

    .filters {
      grid-template-columns: 1fr;
    }

    .ledger-table,
    .ledger-table tbody,
    .ledger-table tr,
    .ledger-table td {
      display: block;
      min-width: 0;
    }

    .ledger-table {
      min-width: 0;
    }

    .ledger-table thead {
      position: absolute;
      width: 1px;
      height: 1px;
      overflow: hidden;
      clip: rect(0, 0, 0, 0);
      white-space: nowrap;
    }

    .ledger-table tbody {
      display: grid;
      gap: var(--space-12);
    }

    .ledger-table tbody tr {
      padding: var(--space-12);
      border: 1px solid var(--border);
      border-radius: 4px;
      background: var(--bg);
    }

    .ledger-table td {
      padding: 0;
      border: none;
    }

    .ledger-table td + td {
      margin-top: var(--space-8);
    }

    .ledger-table td::before {
      content: attr(data-label);
      display: block;
      margin-bottom: 0.125rem;
      color: var(--text-muted);
      font-size: var(--font-size-caption);
    }

    .ledger-table td:first-child::before {
      content: none;
    }

    .inspector-meta {
      grid-template-columns: 1fr;
    }
  }
</style>
