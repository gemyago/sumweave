<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
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

  let { params = {} } = $props<{ params?: { accountId?: string } }>()

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const financeShell = useFinanceShellState()

  let loading = $state(true)
  let error = $state<string | null>(null)
  let account = $state<FinanceAccount | null>(null)
  let transactions = $state<FinanceTransaction[]>([])
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

  async function loadAccountDetail() {
    account = null
    transactions = []

    if (!financeShell.selectedTenantId || !params.accountId) {
      return
    }

    const [accounts, tx] = await Promise.all([
      financeApi.listAccounts({ tenantId: financeShell.selectedTenantId }),
      financeApi.listTransactions({ tenantId: financeShell.selectedTenantId, accountId: params.accountId, limit: 10 }),
    ])

    account = accounts.find((item) => item.id === params.accountId) ?? null
    transactions = tx
  }

  $effect(() => {
    if (financeShell.loading || !reactiveReady) return
    void financeShell.selectedTenantId
    void params.accountId
    if (skipNextReactiveLoad) {
      skipNextReactiveLoad = false
      return
    }
    void loadAccountDetail()
  })
</script>

<section class="container-fluid px-0" aria-labelledby="finance-account-detail-heading">
  <div class="d-grid gap-4">
    <header class="card border-0 shadow-sm">
      <div class="card-body p-4 p-xl-5 d-flex flex-column flex-lg-row justify-content-between gap-3 align-items-lg-center">
        <div>
          <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Account detail</p>
          <h1 id="finance-account-detail-heading" class="h3 mb-2">Finance account detail</h1>
          <p class="text-body-secondary mb-0">
            Review one account, its reporting context, and recent transactions without using a split-pane workspace.
          </p>
        </div>

        <div class="d-flex flex-wrap gap-2">
          <a class="btn btn-outline-secondary btn-sm" href="/finance/accounts" use:link>Back to accounts</a>
          <a class="btn btn-outline-secondary btn-sm" href="/finance/transactions" use:link>Open transactions</a>
        </div>
      </div>
    </header>

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
      <div class="row g-4">
        <div class="col-12 col-xl-5">
          <section class="card shadow-sm h-100">
            <div class="card-body p-4 d-grid gap-3">
              <div class="d-flex flex-column gap-2">
                <div>
                  <h2 class="h4 mb-1">{account.name}</h2>
                  <div class="d-flex flex-wrap gap-2">
                    <span class="badge text-bg-secondary">{account.kind}</span>
                    <span class="badge text-bg-light border text-body">{account.currency}</span>
                    <span class="badge text-bg-light border text-body">Provider {account.provider || 'manual'}</span>
                  </div>
                </div>

                <p class="small text-body-secondary mb-0">
                  Created {formatFinanceDateTime(account.createdAt)} · Updated {formatFinanceDateTime(account.updatedAt)}
                </p>
                <div class="d-flex flex-wrap gap-2">
                  <span class="badge text-bg-light border text-body">Booked balance {formatFinanceMoney(account.bookedBalanceMinor, account.currency)}</span>
                  <span class="badge text-bg-light border text-body">Pending balance {formatFinanceMoney(account.pendingBalanceMinor, account.currency)}</span>
                </div>
              </div>
            </div>
          </section>
        </div>

        <div class="col-12 col-xl-7">
          <section class="card shadow-sm h-100">
            <div class="card-body p-4 d-grid gap-3">
              <div class="d-flex flex-column flex-md-row justify-content-between gap-2 align-items-md-center">
                <div>
                  <h2 class="h5 mb-1">Recent transactions</h2>
                  <p class="text-body-secondary mb-0">Latest activity already scoped to this account.</p>
                </div>

                <span class="badge text-bg-secondary align-self-start align-self-md-center">
                  {transactions.length} transaction{transactions.length === 1 ? '' : 's'}
                </span>
              </div>

              {#if transactions.length === 0}
                <div class="alert alert-light border mb-0" role="status">No transactions yet.</div>
              {:else}
                <div class="table-responsive">
                  <table class="table align-middle mb-0">
                    <thead>
                      <tr>
                        <th scope="col">Description</th>
                        <th scope="col">When</th>
                        <th scope="col">Status</th>
                        <th scope="col">Amount</th>
                        <th scope="col">Open</th>
                      </tr>
                    </thead>
                    <tbody>
                      {#each transactions as item (item.id)}
                        <tr>
                          <td>
                            <div class="d-grid gap-1">
                              <strong>{item.description || item.kind}</strong>
                              <span class="small text-body-secondary">{item.kind}</span>
                            </div>
                          </td>
                          <td>{formatFinanceDateTime(item.effectiveAt)}</td>
                          <td>
                            <span class={`badge ${item.status === 'pending' ? 'text-bg-warning' : 'text-bg-success'}`}>
                              {item.status}
                            </span>
                          </td>
                          <td>{formatFinanceMoney(item.amountMinor, item.currency)}</td>
                          <td>
                            <a href={`/finance/transactions/${encodeURIComponent(item.id)}`} use:link>Open transaction</a>
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
      </div>
    {:else}
      <div class="alert alert-light border mb-0" role="status">Account not found for the selected tenant.</div>
    {/if}
  </div>
</section>
