<script lang="ts">
  import { onMount } from 'svelte'
  import { documentTitle } from '../lib/document-title'
  import DocumentTitle from '../components/DocumentTitle.svelte'
  import { link } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { createSignalFinanceApiForAuth, type FinanceAccount } from '../lib/finance/api'
  import { formatFinanceMoney } from '../lib/finance/format'
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const financeShell = useFinanceShellState()

  let loading = $state(true)
  let error = $state<string | null>(null)
  let accounts = $state<FinanceAccount[]>([])
  let includeHidden = $state(false)
  let reactiveReady = $state(false)
  let skipNextReactiveLoad = false

  onMount(() => {
    void loadPage()
  })

  async function loadPage() {
    loading = true
    reactiveReady = false
    error = null

    try {
      await financeShell.initialize()
      await loadAccounts()
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load accounts'
    } finally {
      skipNextReactiveLoad = true
      reactiveReady = true
      loading = false
    }
  }

  async function loadAccounts() {
    if (!financeShell.selectedTenantId) {
      accounts = []
      return
    }

    accounts = await financeApi.listAccounts({ tenantId: financeShell.selectedTenantId, includeHidden })
  }

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

<DocumentTitle title={documentTitle('Accounts', 'Finance')} />

<section class="container-fluid px-0" aria-labelledby="finance-accounts-heading">
  <div class="d-grid gap-4">
    <header class="card border-0 shadow-sm">
      <div class="card-body p-4 p-xl-5 d-flex flex-column flex-lg-row justify-content-between gap-3 align-items-lg-center">
        <div>
          <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Accounts workspace</p>
          <h1 id="finance-accounts-heading" class="h3 mb-2">Finance accounts</h1>
          <p class="text-body-secondary mb-0">Browse account balances and open a focused account detail when you need to manage one.</p>
        </div>

        <div class="d-flex flex-wrap gap-2">
          <a class="btn btn-outline-secondary" href="/finance/tenants" use:link>Manage tenants</a>
          <a class="btn btn-primary" href="/finance/accounts/new" use:link>Create account</a>
        </div>
      </div>
    </header>

    {#if error}
      <div class="alert alert-danger mb-0" role="alert">{error}</div>
    {/if}

    {#if loading}
      <div class="alert alert-secondary mb-0" role="status">Loading accounts…</div>
    {:else}
      <section class="card shadow-sm">
        <div class="card-body p-4 d-grid gap-3">
          <div class="d-flex flex-column flex-md-row justify-content-between gap-3 align-items-md-center">
            <div>
              <h2 class="h5 mb-1">Browse accounts</h2>
              <p class="text-body-secondary mb-0">Hidden accounts stay available here for management and historical review.</p>
            </div>
            <div class="d-flex flex-wrap align-items-center gap-3">
              <div class="form-check form-switch">
                <input id="finance-accounts-hidden" class="form-check-input" type="checkbox" bind:checked={includeHidden} />
                <label class="form-check-label" for="finance-accounts-hidden">Include hidden</label>
              </div>
              <span class="badge text-bg-secondary">{accounts.length} account{accounts.length === 1 ? '' : 's'}</span>
            </div>
          </div>

          {#if financeShell.needsTenantSelection}
            <div class="alert alert-warning mb-0" role="status">Select an active tenant to continue on this finance route.</div>
          {:else if !financeShell.selectedTenantId}
            <div class="alert alert-light border mb-0" role="status">Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a> before managing accounts.</div>
          {:else if accounts.length === 0}
            <div class="alert alert-light border mb-0" role="status">No accounts yet. <a href="/finance/accounts/new" use:link>Create an account</a> to start recording transactions.</div>
          {:else}
            <div class="table-responsive">
              <table class="table table-hover align-middle mb-0" aria-label="Accounts">
                <thead>
                  <tr>
                    <th scope="col">Account</th>
                    <th scope="col" class="d-none d-md-table-cell">Type</th>
                    <th scope="col" class="d-none d-lg-table-cell">Booked</th>
                    <th scope="col" class="d-none d-lg-table-cell">Pending</th>
                    <th scope="col"><span class="visually-hidden">Actions</span></th>
                  </tr>
                </thead>
                <tbody>
                  {#each accounts as account (account.id)}
                    <tr>
                      <td>
                        <div class="fw-semibold">{account.name}</div>
                        <div class="d-flex flex-wrap gap-2 mt-1 small text-body-secondary">
                          <span>{account.currency}</span>
                          <span class="d-md-none">{account.kind}</span>
                          {#if account.hiddenAt}<span class="badge text-bg-warning">Hidden</span>{/if}
                        </div>
                        <div class="d-lg-none small text-body-secondary mt-1">
                          Booked {formatFinanceMoney(account.bookedBalanceMinor, account.currency)} · Pending {formatFinanceMoney(account.pendingBalanceMinor, account.currency)}
                        </div>
                      </td>
                      <td class="d-none d-md-table-cell"><span class="badge text-bg-secondary">{account.kind}</span></td>
                      <td class="d-none d-lg-table-cell">{formatFinanceMoney(account.bookedBalanceMinor, account.currency)}</td>
                      <td class="d-none d-lg-table-cell">{formatFinanceMoney(account.pendingBalanceMinor, account.currency)}</td>
                      <td class="text-end"><a class="btn btn-outline-secondary btn-sm" href={`/finance/accounts/${encodeURIComponent(account.id)}`} use:link>Open details</a></td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
            <p class="small text-body-secondary mb-0">Balances are current account balances. Updated account data is available on the detail route.</p>
          {/if}
        </div>
      </section>
    {/if}
  </div>
</section>
