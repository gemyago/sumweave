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
  const transactionPageSize = 20
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
  let transactionOffset = $state(0)
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
  const accountNameById = $derived.by(() => new Map(accounts.map((account) => [account.id, account.name])))
  const visiblePendingCount = $derived(visibleTransactions.filter((item) => item.status === 'pending').length)
  const visibleHiddenCount = $derived(visibleTransactions.filter((item) => item.hiddenAt !== null).length)
  const activeFilterCount = $derived([accountFilter, statusFilter, sourceFilter].filter(Boolean).length)
  const pageNumber = $derived(Math.floor(transactionOffset / transactionPageSize) + 1)
  const hasPreviousPage = $derived(transactionOffset > 0)
  const hasNextPage = $derived(transactions.length === transactionPageSize)

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
          limit: transactionPageSize,
          offset: transactionOffset,
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

  function badgeClass(flag: string): string {
    if (flag === 'pending') return 'text-bg-warning'
    if (flag === 'hidden') return 'text-bg-secondary'
    if (flag === 'refund') return 'text-bg-success'
    if (flag === 'reconciliation') return 'text-bg-primary'
    return 'text-bg-light border text-body'
  }

  function reloadFirstPage() {
    transactionOffset = 0
    void loadTenantData()
  }

  function loadPreviousPage() {
    transactionOffset = Math.max(0, transactionOffset - transactionPageSize)
    void loadTenantData()
  }

  function loadNextPage() {
    if (!hasNextPage) return
    transactionOffset += transactionPageSize
    void loadTenantData()
  }

  function selectTenant(tenantId: string) {
    transactionOffset = 0
    financeShell.selectTenant(tenantId)
  }

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

<section class="container-fluid px-0" aria-labelledby="finance-transactions-heading">
  <div class="d-grid gap-4">
    <header class="card border-0 shadow-sm">
      <div class="card-body p-4 p-xl-5 d-grid gap-4">
        <div class="d-flex flex-column flex-lg-row justify-content-between gap-3 align-items-lg-center">
          <div>
            <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Transactions workspace</p>
            <h1 id="finance-transactions-heading" class="h3 mb-2">Finance transactions</h1>
            <p class="text-body-secondary mb-0">
              Browse the ledger table and use row actions to open dedicated create or edit routes when needed.
            </p>
          </div>

          <div class="d-flex flex-wrap gap-2">
            <a class="btn btn-outline-secondary" href="/finance/imports" use:link>Import CSV</a>
            <a class="btn btn-primary" href="/finance/transactions/new" use:link>Create transaction</a>
          </div>
        </div>

        <div class="d-flex flex-wrap gap-2" aria-label="Transaction summaries">
          <span class="badge text-bg-secondary">{visibleTransactions.length} visible</span>
          <span class="badge text-bg-warning">{visiblePendingCount} pending</span>
          <span class="badge text-bg-dark">{visibleHiddenCount} hidden</span>
          <span class="badge text-bg-light border text-body">{activeFilterCount} filters</span>
        </div>
      </div>
    </header>

    {#if error}
      <div class="alert alert-danger mb-0" role="alert">{error}</div>
    {/if}

    {#if loading}
      <div class="alert alert-secondary mb-0" role="status">Loading transactions…</div>
    {:else if financeShell.needsTenantSelection}
      <section class="card shadow-sm">
        <div class="card-body p-4 d-grid gap-3">
          {#if !financeShell.embedded}
            <div class="col-12 col-lg-5 px-0">
              <label class="form-label" for="finance-transactions-tenant">Tenant</label>
              <select
                id="finance-transactions-tenant"
                class="form-select"
                value={financeShell.selectedTenantId}
                onchange={(event) => selectTenant((event.currentTarget as HTMLSelectElement).value)}
                aria-label="Tenant"
              >
                <option value="">Select tenant</option>
                {#each financeShell.tenants as tenant (tenant.id)}
                  <option value={tenant.id}>{tenant.name}</option>
                {/each}
              </select>
            </div>
          {/if}

          <div class="alert alert-warning mb-0" role="status">Select an active tenant to continue on this finance route.</div>
        </div>
      </section>
    {:else if !financeShell.selectedTenantId}
      <div class="alert alert-light border mb-0" role="status">Select a finance tenant to load transaction history and editor links.</div>
    {:else}
      <section class="card shadow-sm">
        <div class="card-body p-4 d-grid gap-3">
          <div class="d-flex flex-column flex-md-row justify-content-between gap-2 align-items-md-center">
            <div>
              <h2 class="h5 mb-1">Browse filters</h2>
              <p class="text-body-secondary mb-0">Adjust the tenant-local ledger scope without leaving the transactions route.</p>
            </div>

            {#if loadingList}
              <span class="badge text-bg-secondary" role="status">Refreshing transactions…</span>
            {/if}
          </div>

          <div class="row g-3 align-items-end">
            {#if !financeShell.embedded}
              <div class="col-12 col-md-6 col-xl-3">
                <label class="form-label" for="finance-transactions-tenant-filter">Tenant</label>
                <select
                  id="finance-transactions-tenant-filter"
                  class="form-select"
                  value={financeShell.selectedTenantId}
                  onchange={(event) => selectTenant((event.currentTarget as HTMLSelectElement).value)}
                  aria-label="Tenant"
                >
                  <option value="">Select tenant</option>
                  {#each financeShell.tenants as tenant (tenant.id)}
                    <option value={tenant.id}>{tenant.name}</option>
                  {/each}
                </select>
              </div>
            {/if}

            <div class="col-12 col-md-6 col-xl-3">
              <label class="form-label" for="finance-transactions-account-filter">Account</label>
              <select id="finance-transactions-account-filter" class="form-select" bind:value={accountFilter} onchange={reloadFirstPage} aria-label="Account filter">
                <option value="">Any account</option>
                {#each accounts as account (account.id)}
                  <option value={account.id}>{account.name}</option>
                {/each}
              </select>
            </div>

            <div class="col-12 col-md-6 col-xl-2">
              <label class="form-label" for="finance-transactions-status-filter">Status</label>
              <select id="finance-transactions-status-filter" class="form-select" bind:value={statusFilter} onchange={reloadFirstPage} aria-label="Transaction status filter">
                <option value="">Any status</option>
                <option value="pending">pending</option>
                <option value="booked">booked</option>
              </select>
            </div>

            <div class="col-12 col-md-6 col-xl-2">
              <label class="form-label" for="finance-transactions-source-filter">Source</label>
              <select id="finance-transactions-source-filter" class="form-select" bind:value={sourceFilter} onchange={reloadFirstPage} aria-label="Transaction source filter">
                <option value="">Any source</option>
                <option value="manual">manual</option>
                <option value="provider">provider</option>
                <option value="csv">csv</option>
                <option value="system">system</option>
              </select>
            </div>

            <div class="col-12 col-md-6 col-xl-2">
              <label class="form-label" for="finance-transactions-sort-order">Sort</label>
              <select id="finance-transactions-sort-order" class="form-select" bind:value={sortOrder} aria-label="Sort order">
                <option value="desc">Newest first</option>
                <option value="asc">Oldest first</option>
              </select>
            </div>
          </div>
        </div>
      </section>

      <section class="card shadow-sm">
        <div class="card-body p-4 d-grid gap-3">
          <div>
            <h2 class="h5 mb-1">Ledger</h2>
            <p class="text-body-secondary mb-0">Use the table to scan transactions and edit a row on the dedicated transaction screen. Showing up to {transactionPageSize} items per page.</p>
          </div>

          {#if visibleTransactions.length === 0}
            <div class="alert alert-light border mb-0" role="status">No transactions matched the current filters.</div>
          {:else}
            <div class="table-responsive">
              <table class="table table-hover align-middle mb-0" aria-label="Transactions ledger">
                <thead>
                  <tr>
                    <th scope="col">Description</th>
                    <th scope="col">Effective</th>
                    <th scope="col">Account</th>
                    <th scope="col">Category</th>
                    <th scope="col">Source</th>
                    <th scope="col">Amount</th>
                    <th scope="col">State</th>
                    <th scope="col">Edit</th>
                  </tr>
                </thead>
                <tbody>
                  {#each visibleTransactions as item (item.id)}
                    <tr>
                      <td>
                        <span class="d-block fw-semibold text-body">{item.description || item.kind}</span>
                        <span class="small text-body-secondary">{item.kind} · {item.status}</span>
                      </td>
                      <td>{formatFinanceDateTime(item.effectiveAt)}</td>
                      <td>{accountName(item.accountId)}</td>
                      <td>{item.categoryId || '—'}</td>
                      <td>{item.source}</td>
                      <td>{formatFinanceMoney(item.amountMinor, item.currency)}</td>
                      <td>
                        <div class="d-flex flex-wrap gap-1">
                          {#each transactionFlags(item) as flag (flag)}
                            <span class={`badge ${badgeClass(flag)}`}>{flag}</span>
                          {/each}
                          {#if transactionFlags(item).length === 0}
                            <span class="badge text-bg-light border text-body">clear</span>
                          {/if}
                        </div>
                      </td>
                      <td>
                        <a class="btn btn-outline-primary btn-sm" href={`/finance/transactions/${item.id}`} use:link>Edit</a>
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>

            <nav class="d-flex flex-column flex-md-row justify-content-between gap-2 align-items-md-center" aria-label="Transaction pages">
              <span class="text-body-secondary small">Page {pageNumber}</span>
              <div class="btn-group" role="group" aria-label="Transaction pagination controls">
                <button class="btn btn-outline-secondary btn-sm" type="button" onclick={loadPreviousPage} disabled={loadingList || !hasPreviousPage}>Previous</button>
                <button class="btn btn-outline-secondary btn-sm" type="button" onclick={loadNextPage} disabled={loadingList || !hasNextPage}>Next</button>
              </div>
            </nav>
          {/if}
        </div>
      </section>
    {/if}
  </div>
</section>
