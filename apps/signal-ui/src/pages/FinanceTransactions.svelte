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
  const accountNameById = $derived.by(() => new Map(accounts.map((account) => [account.id, account.name])))
  const selectedTransaction = $derived.by(
    () => visibleTransactions.find((item) => item.id === selectedTransactionId) ?? visibleTransactions[0] ?? null,
  )
  const visiblePendingCount = $derived(visibleTransactions.filter((item) => item.status === 'pending').length)
  const visibleHiddenCount = $derived(visibleTransactions.filter((item) => item.hiddenAt !== null).length)
  const activeFilterCount = $derived([accountFilter, statusFilter, sourceFilter].filter(Boolean).length)

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

  function badgeClass(flag: string): string {
    if (flag === 'pending') return 'text-bg-warning'
    if (flag === 'hidden') return 'text-bg-secondary'
    if (flag === 'refund') return 'text-bg-success'
    if (flag === 'reconciliation') return 'text-bg-primary'
    return 'text-bg-light border text-body'
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

<section class="container-fluid px-0" aria-labelledby="finance-transactions-heading">
  <div class="d-grid gap-4">
    <header class="card border-0 shadow-sm">
      <div class="card-body p-4 p-xl-5 d-grid gap-4">
        <div class="d-flex flex-column flex-lg-row justify-content-between gap-3 align-items-lg-center">
          <div>
            <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Transactions workspace</p>
            <h1 id="finance-transactions-heading" class="h3 mb-2">Finance transactions</h1>
            <p class="text-body-secondary mb-0">
              Browse a ledger table, keep filter context visible, and open dedicated create or edit routes when needed.
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
                onchange={(event) => financeShell.selectTenant((event.currentTarget as HTMLSelectElement).value)}
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
                  onchange={(event) => financeShell.selectTenant((event.currentTarget as HTMLSelectElement).value)}
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
              <select id="finance-transactions-account-filter" class="form-select" bind:value={accountFilter} onchange={() => void loadTenantData()} aria-label="Account filter">
                <option value="">Any account</option>
                {#each accounts as account (account.id)}
                  <option value={account.id}>{account.name}</option>
                {/each}
              </select>
            </div>

            <div class="col-12 col-md-6 col-xl-2">
              <label class="form-label" for="finance-transactions-status-filter">Status</label>
              <select id="finance-transactions-status-filter" class="form-select" bind:value={statusFilter} onchange={() => void loadTenantData()} aria-label="Transaction status filter">
                <option value="">Any status</option>
                <option value="pending">pending</option>
                <option value="booked">booked</option>
              </select>
            </div>

            <div class="col-12 col-md-6 col-xl-2">
              <label class="form-label" for="finance-transactions-source-filter">Source</label>
              <select id="finance-transactions-source-filter" class="form-select" bind:value={sourceFilter} onchange={() => void loadTenantData()} aria-label="Transaction source filter">
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

      <div class="row g-4 align-items-start">
        <div class="col-12 col-xl-7">
          <section class="card shadow-sm h-100">
            <div class="card-body p-4 d-grid gap-3">
              <div>
                <h2 class="h5 mb-1">Ledger</h2>
                <p class="text-body-secondary mb-0">Select a row to keep transaction context visible beside the browse results.</p>
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
                        <th scope="col">Open</th>
                      </tr>
                    </thead>
                    <tbody>
                      {#each visibleTransactions as item (item.id)}
                        <tr class:table-active={selectedTransaction?.id === item.id}>
                          <td>
                            <button
                              type="button"
                              class="btn btn-link p-0 text-start text-decoration-none"
                              aria-pressed={selectedTransaction?.id === item.id}
                              onclick={() => selectTransaction(item.id)}
                            >
                              <span class="d-block fw-semibold text-body">{item.description || item.kind}</span>
                              <span class="small text-body-secondary">{item.kind} · {item.status}</span>
                            </button>
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
                            <a href={`/finance/transactions/${item.id}`} use:link>Open record</a>
                          </td>
                        </tr>
                      {/each}
                    </tbody>
                  </table>
                </div>
              {/if}
            </div>
          </section>
        </div>

        <div class="col-12 col-xl-5">
          <aside class="card shadow-sm h-100" aria-label="Selected transaction details">
            <div class="card-body p-4 d-grid gap-3">
              {#if selectedTransaction}
                <div class="d-flex flex-column flex-md-row justify-content-between gap-2 align-items-md-center">
                  <div>
                    <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Selected transaction</p>
                    <h2 class="h5 mb-1">{selectedTransaction.description || selectedTransaction.kind}</h2>
                  </div>

                  <a class="btn btn-outline-secondary btn-sm align-self-start align-self-md-center" href={`/finance/transactions/${selectedTransaction.id}`} use:link>
                    Open transaction
                  </a>
                </div>

                <p class="display-6 mb-0">{formatFinanceMoney(selectedTransaction.amountMinor, selectedTransaction.currency)}</p>

                <div class="d-flex flex-wrap gap-2">
                  <span class="badge text-bg-secondary">{selectedTransaction.status}</span>
                  <span class="badge text-bg-light border text-body">{selectedTransaction.source}</span>
                  <span class="badge text-bg-light border text-body">{selectedTransaction.kind}</span>
                </div>

                <div class="row g-3">
                  <div class="col-12 col-sm-6"><strong>Effective</strong><div>{formatFinanceDateTime(selectedTransaction.effectiveAt)}</div></div>
                  <div class="col-12 col-sm-6"><strong>Account</strong><div>{accountName(selectedTransaction.accountId)}</div></div>
                  <div class="col-12 col-sm-6"><strong>Category</strong><div>{selectedTransaction.categoryId || '—'}</div></div>
                  <div class="col-12 col-sm-6"><strong>Transfer group</strong><div>{selectedTransaction.transferGroupId || '—'}</div></div>
                  <div class="col-12 col-sm-6"><strong>Hidden</strong><div>{selectedTransaction.hiddenAt ? 'Yes' : 'No'}</div></div>
                  <div class="col-12 col-sm-6"><strong>Updated</strong><div>{formatFinanceDateTime(selectedTransaction.updatedAt)}</div></div>
                </div>

                {#if selectedTransaction.providerOriginal}
                  <section class="border rounded-3 p-3 bg-body-tertiary">
                    <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Provider original</p>
                    <div class="row g-3">
                      <div class="col-12"><strong>Description</strong><div>{selectedTransaction.providerOriginal.description || '—'}</div></div>
                      <div class="col-12 col-sm-6"><strong>Amount</strong><div>{formatFinanceMoney(selectedTransaction.providerOriginal.amountMinor, selectedTransaction.providerOriginal.currency)}</div></div>
                      <div class="col-12 col-sm-6"><strong>Effective</strong><div>{formatFinanceDateTime(selectedTransaction.providerOriginal.effectiveAt)}</div></div>
                    </div>
                  </section>
                {/if}

                <div class="d-flex flex-wrap gap-2">
                  <a class="btn btn-outline-secondary btn-sm" href={`/finance/accounts/${selectedTransaction.accountId}`} use:link>Open account</a>
                  <a class="btn btn-primary btn-sm" href={`/finance/transactions/${selectedTransaction.id}`} use:link>Edit transaction</a>
                </div>
              {:else}
                <div class="alert alert-light border mb-0" role="status">Select a transaction row to inspect it here.</div>
              {/if}
            </div>
          </aside>
        </div>
      </div>
    {/if}
  </div>
</section>
