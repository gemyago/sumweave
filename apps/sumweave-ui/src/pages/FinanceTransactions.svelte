<script lang="ts">
  import { onMount } from 'svelte'
  import { documentTitle } from '../lib/document-title'
  import DocumentTitle from '../components/DocumentTitle.svelte'
  import { link } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalFinanceApiForAuth,
    type FinanceAccount,
    type FinanceTransaction,
  } from '../lib/finance/api'
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'
  import {
    acknowledgeFinanceLedgerRefresh,
    financeLedgerRefreshRevision,
    subscribeToFinanceLedgerRefresh,
  } from '../lib/finance/ledger-refresh'
  import FinancePager from '../components/FinancePager.svelte'
  import FinanceTransactionList from '../components/FinanceTransactionList.svelte'
  import JobStatus from '../components/JobStatus.svelte'
  import { defaultMatchingDateRange, matchingRangeFromDateInputs } from '../lib/finance/matching-range'
  import { requestFinanceLedgerRefresh } from '../lib/finance/ledger-refresh'
  import type { JobDetail } from '../lib/jobs/api'
  import { dateQueryValue, financeRouteQuery, readDateQuery, replaceFinanceRouteQuery } from '../lib/finance/url-filters'
  import { dateInputValue } from '../lib/date-range'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const transactionPageSize = 20
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const financeShell = useFinanceShellState()

  let loading = $state(true)
  let error = $state<string | null>(null)
  let accounts = $state<FinanceAccount[]>([])
  let transactions = $state<FinanceTransaction[]>([])
  let accountFilter = $state('')
  let kindFilter = $state('')
  let startDate = $state<Date | undefined>(undefined)
  let endDate = $state<Date | undefined>(undefined)
  let sortOrder = $state('desc')
  let transactionOffset = $state(0)
  let loadingList = $state(false)
  let reactiveReady = $state(false)
  let skipNextReactiveLoad = false
  const defaultMatchingRange = defaultMatchingDateRange()
  let matchingStartDate = $state(defaultMatchingRange.startDate)
  let matchingEndDate = $state(defaultMatchingRange.endDate)
  let matchingBusy = $state(false)
  let matchingError = $state<string | null>(null)
  let matchingOutcome = $state<string | null>(null)
  let matchingJobId = $state('')
  let matchingTenantId = $state('')
  let matchingTerminal = $state(false)

  const visibleTransactions = $derived(transactions)
  const accountNameById = $derived.by(() => new Map(accounts.map((account) => [account.id, account.name])))
  const hiddenAccountIds = $derived.by(() => new Set(accounts.filter((account) => account.hiddenAt).map((account) => account.id)))
  const visiblePendingCount = $derived(visibleTransactions.filter((item) => item.status === 'pending').length)
  const visibleHiddenCount = $derived(visibleTransactions.filter((item) => item.hiddenAt !== null).length)
  const activeFilterCount = $derived([accountFilter, kindFilter, startDate, endDate].filter(Boolean).length)
  const pageNumber = $derived(Math.floor(transactionOffset / transactionPageSize) + 1)
  const hasPreviousPage = $derived(transactionOffset > 0)
  const hasNextPage = $derived(transactions.length === transactionPageSize)

  onMount(() => {
    restoreFiltersFromUrl()
    void loadPage()
    return subscribeToFinanceLedgerRefresh((tenantId) => {
      if (financeShell.selectedTenantId !== tenantId) return
      transactionOffset = 0
      void loadTenantData(0)
    })
  })

  function restoreFiltersFromUrl() {
    const query = financeRouteQuery()
    accountFilter = query.get('accountId') ?? ''
    kindFilter = query.get('type') ?? ''
    sortOrder = query.get('sort') === 'asc' ? 'asc' : 'desc'
    startDate = readDateQuery(query, 'startDate')
    endDate = readDateQuery(query, 'endDate')
  }

  function persistFilters() {
    replaceFinanceRouteQuery({
      accountId: accountFilter || undefined,
      type: kindFilter || undefined,
      sort: sortOrder === 'asc' ? 'asc' : undefined,
      startDate: dateQueryValue(startDate),
      endDate: dateQueryValue(endDate),
    })
  }

  function exclusiveDateRangeEnd(value: Date | undefined): Date | undefined {
    if (!value) return undefined
    const valueAsDateInput = dateInputValue(value)
    const date = readDateQuery(new URLSearchParams(`date=${valueAsDateInput}`), 'date')
    if (!date) return undefined
    date.setDate(date.getDate() + 1)
    return date
  }

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

  async function loadTenantData(offset = transactionOffset): Promise<boolean> {
    const tenantId = financeShell.selectedTenantId
    if (!tenantId) {
      accounts = []
      transactions = []
      return false
    }
    const refreshRevision = financeLedgerRefreshRevision(tenantId)

    loadingList = true
    error = null

    try {
      const [loadedAccounts, loadedTransactions] = await Promise.all([
        financeApi.listAccounts({ tenantId, includeHidden: true }),
        financeApi.listTransactions({
          tenantId,
          accountId: accountFilter,
          kind: kindFilter,
          startDate,
          endDate: exclusiveDateRangeEnd(endDate),
          sort: sortOrder === 'asc' ? 'asc' : undefined,
          limit: transactionPageSize,
          offset,
        }),
      ])

      if (financeShell.selectedTenantId !== tenantId) return false
      accounts = loadedAccounts
      transactions = loadedTransactions
      acknowledgeFinanceLedgerRefresh(tenantId, refreshRevision)
      return true
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load transactions'
      return false
    } finally {
      loadingList = false
    }
  }

  function applyTransactionUpdate(updated: FinanceTransaction) {
    transactions = transactions.map((item) => item.id === updated.id ? updated : item)
  }

  function reloadFirstPage() {
    transactionOffset = 0
    persistFilters()
    void loadTenantData()
  }

  async function loadPreviousPage(): Promise<boolean> {
    const nextOffset = Math.max(0, transactionOffset - transactionPageSize)
    if (!await loadTenantData(nextOffset)) return false
    transactionOffset = nextOffset
    return true
  }

  async function loadNextPage(): Promise<boolean> {
    if (!hasNextPage) return false
    const nextOffset = transactionOffset + transactionPageSize
    if (!await loadTenantData(nextOffset)) return false
    transactionOffset = nextOffset
    return true
  }

  function selectTenant(tenantId: string) {
    transactionOffset = 0
    financeShell.selectTenant(tenantId)
  }

  async function submitTransferMatching(event: SubmitEvent) {
    event.preventDefault()
    if (!financeShell.selectedTenantId || matchingBusy || (matchingJobId && !matchingTerminal)) return

    const tenantId = financeShell.selectedTenantId
    matchingError = null
    matchingOutcome = null
    try {
      const range = matchingRangeFromDateInputs(matchingStartDate, matchingEndDate)
      matchingBusy = true
      matchingTerminal = false
      matchingJobId = ''
      matchingTenantId = tenantId
      const result = await financeApi.submitTransferMatching({
        tenantId,
        rangeStart: range.rangeStart,
        rangeEndExclusive: range.rangeEndExclusive,
      })
      matchingJobId = result.jobId
    } catch (submitError) {
      matchingError = submitError instanceof Error ? submitError.message : 'Could not start transfer matching.'
    } finally {
      matchingBusy = false
    }
  }

  function handleTransferMatchingTerminal(job: JobDetail) {
    if (job.status === 'succeeded') {
      matchingOutcome = 'Transfer matching completed.'
    } else if (job.status === 'failed') {
      matchingOutcome = 'Transfer matching failed. Some pairs may already have been matched. Running it again preserves existing pairs.'
    } else {
      return
    }

    matchingTerminal = true
    requestFinanceLedgerRefresh(matchingTenantId)
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

<DocumentTitle title={documentTitle('Transactions', 'Finance')} />

<section class="container-fluid px-0" aria-labelledby="finance-transactions-heading">
  <div class="d-grid gap-4">
    <header class="card border-0 shadow-sm">
      <div class="card-body p-4 p-xl-5 d-grid gap-4">
        <div class="d-flex flex-column flex-lg-row justify-content-between gap-3 align-items-lg-center">
          <div>
            <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Transactions workspace</p>
            <h1 id="finance-transactions-heading" class="h3 mb-2">Finance transactions</h1>
            <p class="text-body-secondary mb-0">
              Scan and correct common transaction fields inline, or open the full editor for advanced changes.
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
       <section id="finance-transactions-ledger" class="card shadow-sm" aria-busy={loadingList}>
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
              <p class="text-body-secondary mb-0">Adjust the tenant-local ledger scope. Filters are saved in this page URL.</p>
            </div>

            {#if loadingList}
              <span class="badge text-bg-secondary" role="status">Refreshing transactions…</span>
            {/if}
          </div>

          <div class="d-grid gap-3">
            {#if !financeShell.embedded}
              <div class="row g-3 align-items-end">
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
              </div>
            {/if}

            <div class="row g-3">
              <div class="col-12 col-md-6 col-xl-5">
                <label class="form-label" for="finance-transactions-account-filter">Account</label>
                <select id="finance-transactions-account-filter" class="form-select" bind:value={accountFilter} onchange={reloadFirstPage} aria-label="Account filter">
                  <option value="">Any account</option>
                  {#each accounts as account (account.id)}
                    <option value={account.id}>{account.name}{account.hiddenAt ? ' (Hidden)' : ''}</option>
                  {/each}
                </select>
              </div>

              <div class="col-12 col-md-6 col-xl-3">
                <label class="form-label" for="finance-transactions-kind-filter">Type</label>
                <select id="finance-transactions-kind-filter" class="form-select" bind:value={kindFilter} onchange={reloadFirstPage} aria-label="Transaction type filter">
                  <option value="">Any type</option>
                  <option value="income">income</option>
                  <option value="expense">expense</option>
                  <option value="refund">refund</option>
                  <option value="transfer">transfer</option>
                  <option value="regular">regular</option>
                  <option value="reconciliation">reconciliation</option>
                  <option value="opening_balance">opening balance</option>
                </select>
              </div>

              <div class="col-12 col-md-6 col-xl-4">
                <label class="form-label" for="finance-transactions-sort-order">Sort</label>
                <select id="finance-transactions-sort-order" class="form-select" bind:value={sortOrder} onchange={reloadFirstPage} aria-label="Sort order">
                  <option value="desc">Newest first</option>
                  <option value="asc">Oldest first</option>
                </select>
              </div>
            </div>

            <div class="row g-3">
              <div class="col-12 col-md-6">
                <label class="form-label" for="finance-transactions-start-date">From date</label>
                <input id="finance-transactions-start-date" class="form-control" type="date" value={dateInputValue(startDate)} onchange={(event) => { startDate = readDateQuery(new URLSearchParams(`date=${event.currentTarget.value}`), 'date'); reloadFirstPage() }} aria-label="Transaction start date" />
              </div>

              <div class="col-12 col-md-6">
                <label class="form-label" for="finance-transactions-end-date">To date</label>
                <input id="finance-transactions-end-date" class="form-control" type="date" value={dateInputValue(endDate)} onchange={(event) => { endDate = readDateQuery(new URLSearchParams(`date=${event.currentTarget.value}`), 'date'); reloadFirstPage() }} aria-label="Transaction end date" />
              </div>
            </div>
          </div>
        </div>
      </section>

      <form class="card shadow-sm" onsubmit={submitTransferMatching} aria-labelledby="finance-transfer-matching-heading">
        <div class="card-body p-4 d-grid gap-3">
          <div>
            <h2 id="finance-transfer-matching-heading" class="h5 mb-1">Match transfers</h2>
            <p class="text-body-secondary mb-0">Searches all accounts in this tenant. A matching partner may be up to 72 hours outside the selected dates.</p>
          </div>
          <div class="row g-3">
            <div class="col-12 col-md-6"><label class="form-label" for="finance-transfer-matching-start-date">Transfer matching start date</label><input id="finance-transfer-matching-start-date" class="form-control" type="date" bind:value={matchingStartDate} disabled={matchingBusy || Boolean(matchingJobId && !matchingTerminal)} required /></div>
            <div class="col-12 col-md-6"><label class="form-label" for="finance-transfer-matching-end-date">Transfer matching end date</label><input id="finance-transfer-matching-end-date" class="form-control" type="date" bind:value={matchingEndDate} disabled={matchingBusy || Boolean(matchingJobId && !matchingTerminal)} required /></div>
          </div>
          <div class="d-flex flex-wrap gap-2"><button class="btn btn-primary" type="submit" disabled={matchingBusy || Boolean(matchingJobId && !matchingTerminal)}>{matchingBusy ? 'Starting matching…' : matchingTerminal ? 'Run matching again' : 'Match transfers'}</button></div>
          {#if matchingError}<div class="alert alert-danger mb-0" role="alert">{matchingError}</div>{/if}
          {#if matchingOutcome}<div class={`alert ${matchingOutcome === 'Transfer matching completed.' ? 'alert-success' : 'alert-warning'} mb-0`} role="status">{matchingOutcome}</div>{/if}
          {#if matchingJobId}<JobStatus jobId={matchingJobId} openHref={`/finance/jobs/${encodeURIComponent(matchingJobId)}`} label="Transfer matching" linkLabel="Open finance job" observedDispatch onTerminal={handleTransferMatchingTerminal} />{/if}
        </div>
      </form>

      <section id="finance-transactions-ledger" class="card shadow-sm" aria-busy={loadingList}>
        <div class="card-body p-4 d-grid gap-3">
           <div>
             <h2 class="h5 mb-1">Ledger</h2>
              <p class="text-body-secondary mb-0">Edit description, category, and tags in place. Open the record icon for all fields. Showing up to {transactionPageSize} items per page.</p>
           </div>

          {#if visibleTransactions.length === 0}
            <div class="alert alert-light border mb-0" role="status">No transactions matched the current filters.</div>
          {:else}
            <FinanceTransactionList
              tenantId={financeShell.selectedTenantId}
              transactions={visibleTransactions}
              accountNameById={accountNameById}
              {hiddenAccountIds}
              ariaLabel="Transactions ledger"
              onTransactionUpdated={applyTransactionUpdate}
            />

             <FinancePager
               label="Transaction pages"
                status={loadingList ? 'Loading transaction page…' : `Page ${pageNumber}`}
               controls="finance-transactions-ledger"
               busy={loadingList}
                hasPrevious={sortOrder === 'desc' ? hasPreviousPage : hasNextPage}
                hasNext={sortOrder === 'desc' ? hasNextPage : hasPreviousPage}
                onPrevious={sortOrder === 'desc' ? loadPreviousPage : loadNextPage}
                onNext={sortOrder === 'desc' ? loadNextPage : loadPreviousPage}
             />
          {/if}
        </div>
      </section>
    {/if}
  </div>
</section>
