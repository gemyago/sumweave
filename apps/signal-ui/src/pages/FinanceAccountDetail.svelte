<script lang="ts">
  import { onMount } from 'svelte'
  import { documentTitle } from '../lib/document-title'
  import DocumentTitle from '../components/DocumentTitle.svelte'
  import { link } from 'svelte-spa-router'
  import Pencil from '@lucide/svelte/icons/pencil'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalFinanceApiForAuth,
    type FinanceAccount,
    type FinanceTransaction,
  } from '../lib/finance/api'
  import {
    formatFinanceDateTime,
    formatFinanceMoney,
  } from '../lib/finance/format'
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'
  import FinanceProviderEvidence from '../components/FinanceProviderEvidence.svelte'
  import FinancePager from '../components/FinancePager.svelte'
  import FinanceTransactionList from '../components/FinanceTransactionList.svelte'

  let { params = {} } = $props<{ params?: { accountId?: string } }>()

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const recentTransactionPageSize = 10
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const financeShell = useFinanceShellState()

  let loading = $state(true)
  let error = $state<string | null>(null)
  let account = $state<FinanceAccount | null>(null)
  let transactions = $state<FinanceTransaction[]>([])
  let renaming = $state(false)
  let nameDraft = $state('')
  let mutationBusy = $state(false)
  let hideConfirmationOpen = $state(false)
  let transactionOffset = $state(0)
  let loadingTransactions = $state(false)
  let reactiveReady = $state(false)
  let skipNextReactiveLoad = false

  onMount(() => {
    void loadPage()
  })

  async function loadPage() {
    loading = true
    error = null
    account = null
    transactions = []
    reactiveReady = false

    try {
      await financeShell.initialize()
      if (!financeShell.selectedTenantId || !params.accountId) {
        return
      }

      await loadAccountDetail()
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load account detail'
    } finally {
      skipNextReactiveLoad = true
      reactiveReady = true
      loading = false
    }
  }

  const recentPageNumber = $derived(Math.floor(transactionOffset / recentTransactionPageSize) + 1)
  const hasPreviousPage = $derived(transactionOffset > 0)
  const hasNextPage = $derived(transactions.length === recentTransactionPageSize)

  async function loadAccountDetail(offset = transactionOffset): Promise<boolean> {
    if (!financeShell.selectedTenantId || !params.accountId) {
      return false
    }

    loadingTransactions = true
    try {
      const [loadedAccount, tx] = await Promise.all([
        financeApi.getAccount({ tenantId: financeShell.selectedTenantId, accountId: params.accountId }),
        financeApi.listTransactions({
          tenantId: financeShell.selectedTenantId,
          accountId: params.accountId,
          limit: recentTransactionPageSize,
          offset,
        }),
      ])

      account = loadedAccount
      nameDraft = loadedAccount.name
      transactions = tx
      return true
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load account detail'
      return false
    } finally {
      loadingTransactions = false
    }
  }

  async function loadPreviousPage(): Promise<boolean> {
    if (!hasPreviousPage) return false
    const nextOffset = Math.max(0, transactionOffset - recentTransactionPageSize)
    if (!await loadAccountDetail(nextOffset)) return false
    transactionOffset = nextOffset
    return true
  }

  async function loadNextPage(): Promise<boolean> {
    if (!hasNextPage) return false
    const nextOffset = transactionOffset + recentTransactionPageSize
    if (!await loadAccountDetail(nextOffset)) return false
    transactionOffset = nextOffset
    return true
  }

  function applyTransactionUpdate(updated: FinanceTransaction) {
    transactions = transactions.map((item) => item.id === updated.id ? updated : item)
  }

  async function renameAccount(event: SubmitEvent) {
    event.preventDefault()
    if (!account || !financeShell.selectedTenantId) return
    mutationBusy = true
    error = null
    try {
      await financeApi.renameAccount({ tenantId: financeShell.selectedTenantId, accountId: account.id, name: nameDraft })
      await loadAccountDetail()
      renaming = false
    } catch (renameError) {
      error = renameError instanceof Error ? renameError.message : 'Failed to rename account'
    } finally {
      mutationBusy = false
    }
  }

  async function hideAccount() {
    if (!account || !financeShell.selectedTenantId) return
    mutationBusy = true
    error = null
    try {
      await financeApi.hideAccount({ tenantId: financeShell.selectedTenantId, accountId: account.id })
      await loadAccountDetail()
      hideConfirmationOpen = false
    } catch (hideError) {
      error = hideError instanceof Error ? hideError.message : 'Failed to hide account'
    } finally {
      mutationBusy = false
    }
  }

  async function restoreAccount() {
    if (!account || !financeShell.selectedTenantId) return
    mutationBusy = true
    error = null
    try {
      await financeApi.restoreAccount({ tenantId: financeShell.selectedTenantId, accountId: account.id })
      await loadAccountDetail()
    } catch (restoreError) {
      error = restoreError instanceof Error ? restoreError.message : 'Failed to restore account'
    } finally {
      mutationBusy = false
    }
  }

  $effect(() => {
    if (financeShell.loading || !reactiveReady) return
    const tenantId = financeShell.selectedTenantId
    const accountId = params.accountId
    if (skipNextReactiveLoad) {
      skipNextReactiveLoad = false
      return
    }
    transactionOffset = 0
    void tenantId
    void accountId
    void loadAccountDetail()
  })
</script>

<DocumentTitle title={documentTitle(account?.name ?? 'Account detail', 'Accounts')} />

<section class="container-fluid px-0" aria-labelledby="finance-account-detail-heading">
  <div class="d-grid gap-4">
    {#if !account}
      <h1 id="finance-account-detail-heading" class="visually-hidden">Finance account detail</h1>
    {/if}

    {#if error}
      <div class="alert alert-danger mb-0" role="alert">{error}</div>
    {/if}

    {#if loading}
      <div class="alert alert-secondary mb-0" role="status">Loading account detail…</div>
    {:else if financeShell.needsTenantSelection}
      <section class="card shadow-sm">
        <div class="card-body p-4 d-grid gap-3">
          {#if !financeShell.embedded}
            <div class="col-12 col-lg-5 px-0">
              <label class="form-label" for="finance-account-detail-tenant">Tenant</label>
              <select
                id="finance-account-detail-tenant"
                class="form-select"
                value={financeShell.selectedTenantId}
                onchange={(event) =>
                  financeShell.selectTenant((event.currentTarget as HTMLSelectElement).value)}
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
      <div class="alert alert-light border mb-0" role="status">
        Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a> before opening this account detail route.
      </div>
    {:else if account}
      <div class="d-grid gap-4">
        <section class="card shadow-sm">
          <div class="card-body p-4 d-grid gap-3">
            <div class="d-grid gap-3">
                <div class="d-flex flex-column flex-md-row justify-content-between align-items-md-start gap-3">
                  <div>
                   {#if renaming}
                    <form class="d-grid gap-2" onsubmit={renameAccount}>
                      <label class="form-label mb-0" for="finance-account-rename">Account name</label>
                      <div class="d-flex flex-wrap gap-2">
                        <input id="finance-account-rename" class="form-control form-control-sm" bind:value={nameDraft} disabled={mutationBusy} required />
                        <button class="btn btn-outline-success btn-sm" type="submit" disabled={mutationBusy}>{mutationBusy ? 'Saving…' : 'Save'}</button>
                        <button class="btn btn-outline-secondary btn-sm" type="button" onclick={() => { renaming = false; nameDraft = account?.name ?? '' }} disabled={mutationBusy}>Cancel</button>
                      </div>
                    </form>
                  {:else}
                    <div class="d-flex flex-wrap align-items-center gap-2">
                      <h1 id="finance-account-detail-heading" class="h3 mb-0">{account.name}</h1>
                       <button class="btn btn-outline-secondary btn-sm d-inline-flex align-items-center justify-content-center py-3 px-3 py-md-2 px-md-2" type="button" onclick={() => renaming = true} disabled={mutationBusy} aria-label="Edit account name" title="Edit account name"><Pencil size={16} /></button>
                    </div>
                  {/if}
                  </div>
                  <div class="d-flex flex-wrap gap-2">
                    <span class="badge text-bg-secondary">{account.kind}</span>
                    <span class="badge text-bg-light border text-body">{account.currency}</span>
                    <span class="badge text-bg-light border text-body">Provider {account.provider || 'manual'}</span>
                    {#if account.hiddenAt}<span class="badge text-bg-warning">Hidden historical source</span>{/if}
                  </div>
                </div>

                <p class="small text-body-secondary mb-0">
                  Created {formatFinanceDateTime(account.createdAt)} · Updated {formatFinanceDateTime(account.updatedAt)}
                </p>
                <div class="d-flex flex-wrap gap-2">
                  <span class="badge text-bg-light border text-body">Booked balance {formatFinanceMoney(account.bookedBalanceMinor, account.currency)}</span>
                  <span class="badge text-bg-light border text-body">Pending balance {formatFinanceMoney(account.pendingBalanceMinor, account.currency)}</span>
                </div>
               {#if account.hiddenAt}
                 <div class="alert alert-warning mb-0" role="status">
                   <strong>Hidden account.</strong> Removed from current dashboard reporting and new transactions. History and provider sync stay available.
                 </div>
               {/if}
                <div class="d-flex flex-wrap gap-2">
                  {#if account.hiddenAt}
                   <button class="btn btn-outline-success btn-sm" type="button" onclick={() => void restoreAccount()} disabled={mutationBusy}>{mutationBusy ? 'Restoring…' : 'Restore account'}</button>
                 {:else}
                   <button class="btn btn-outline-danger btn-sm" type="button" onclick={() => hideConfirmationOpen = true} disabled={mutationBusy}>Hide account</button>
                 {/if}
               </div>
               {#if hideConfirmationOpen}
                 <div class="alert alert-warning mb-0 d-grid gap-2" aria-label="Confirm hide account">
                   <strong>Hide {account.name}?</strong>
                   <span>It will be removed from current dashboard reporting and new transaction choices. Its history and provider sync will continue.</span>
                   <div class="d-flex flex-wrap gap-2">
                     <button class="btn btn-danger btn-sm" type="button" onclick={() => void hideAccount()} disabled={mutationBusy}>{mutationBusy ? 'Hiding…' : 'Confirm hide'}</button>
                     <button class="btn btn-outline-secondary btn-sm" type="button" onclick={() => hideConfirmationOpen = false} disabled={mutationBusy}>Cancel</button>
                   </div>
                 </div>
               {/if}
               <FinanceProviderEvidence tenantId={financeShell.selectedTenantId} entityId={account.id} entityLabel="account" scope="account" />
            </div>
          </div>
        </section>

        <section id="finance-account-recent-transactions" class="card shadow-sm" aria-busy={loadingTransactions}>
          <div class="card-body p-4 d-grid gap-3">
              <div class="d-flex flex-column flex-md-row justify-content-between gap-2 align-items-md-center">
                <div>
                  <h2 class="h5 mb-1">Recent transactions</h2>
                  <p class="text-body-secondary mb-0">Latest activity already scoped to this account.</p>
                </div>

                <span class="badge text-bg-secondary align-self-start align-self-md-center">
                  {transactions.length} transaction{transactions.length === 1 ? '' : 's'} on this page
                </span>
              </div>

              {#if transactions.length === 0}
                <div class="alert alert-light border mb-0" role="status">No transactions yet.</div>
              {:else}
                <FinanceTransactionList
                  tenantId={financeShell.selectedTenantId}
                  transactions={transactions}
                   accountNameById={new Map(account ? [[account.id, account.name]] : [])}
                   hiddenAccountIds={new Set(account?.hiddenAt ? [account.id] : [])}
                  ariaLabel="Recent transactions"
                  onTransactionUpdated={applyTransactionUpdate}
                />

                <FinancePager
                  label="Recent transaction pages"
                  status={loadingTransactions ? 'Loading recent transaction page…' : `Page ${recentPageNumber}`}
                  controls="finance-account-recent-transactions"
                  busy={loadingTransactions}
                  hasPrevious={hasPreviousPage}
                  hasNext={hasNextPage}
                  onPrevious={loadPreviousPage}
                  onNext={loadNextPage}
                />
              {/if}
            </div>
        </section>
      </div>
    {:else}
      <div class="alert alert-light border mb-0" role="status">Account not found for the selected tenant.</div>
    {/if}
  </div>
</section>
