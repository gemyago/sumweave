<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { createSignalFinanceApiForAuth, type FinanceAccount } from '../lib/finance/api'
  import { formatFinanceDateTime, formatFinanceMoney } from '../lib/finance/format'
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

  async function createAccount(event: SubmitEvent) {
    event.preventDefault()
    if (!financeShell.selectedTenantId) return
    error = null

    try {
      await financeApi.createAccount({
        tenantId: financeShell.selectedTenantId,
        name: accountName,
        currency: accountCurrency,
        kind: accountKind,
      })
      accountName = ''
      await loadAccounts()
    } catch (createError) {
      error = createError instanceof Error ? createError.message : 'Failed to create account'
    }
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

<section class="container-fluid px-0" aria-labelledby="finance-accounts-heading">
  <div class="d-grid gap-4">
    <header class="card border-0 shadow-sm">
      <div class="card-body p-4 p-xl-5 d-flex flex-column flex-lg-row justify-content-between gap-3 align-items-lg-center">
        <div>
          <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Accounts</p>
          <h1 id="finance-accounts-heading" class="h3 mb-2">Finance accounts</h1>
          <p class="text-body-secondary mb-0">
            Create accounts, review balance sources, and open dedicated account detail routes.
          </p>
        </div>

        <a class="btn btn-outline-secondary align-self-start align-self-lg-center" href="/finance/tenants" use:link>
          Manage tenants
        </a>
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
          <div class="row g-3 align-items-end">
            {#if !financeShell.embedded}
              <div class="col-12 col-lg-4">
                <label class="form-label" for="finance-accounts-tenant">Tenant</label>
                <select
                  id="finance-accounts-tenant"
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

            <div class="col-12 col-lg-4">
              <div class="form-check form-switch pt-lg-4">
                <input id="finance-accounts-hidden" class="form-check-input" type="checkbox" bind:checked={includeHidden} />
                <label class="form-check-label" for="finance-accounts-hidden">Include hidden</label>
              </div>
            </div>

            <div class="col-12 col-lg-4">
              <p class="small text-body-secondary mb-0">
                Toggle hidden accounts without leaving the account-management route.
              </p>
            </div>
          </div>
        </div>
      </section>

      {#if financeShell.needsTenantSelection}
        <div class="alert alert-warning mb-0" role="status">Select an active tenant to continue on this finance route.</div>
      {:else if !financeShell.selectedTenantId}
        <div class="alert alert-light border mb-0" role="status">
          Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a> before managing accounts.
        </div>
      {/if}

      <div class="row g-4">
        <div class="col-12 col-xl-4">
          <form class="card shadow-sm h-100" onsubmit={createAccount}>
            <div class="card-body p-4 d-grid gap-3">
              <div>
                <h2 class="h5 mb-1">Create account</h2>
                <p class="text-body-secondary mb-0">Set the basic reporting fields before transactions or sync activity begin.</p>
              </div>

              <div>
                <label class="form-label" for="finance-account-name">Name</label>
                <input id="finance-account-name" class="form-control" bind:value={accountName} aria-label="Account name" required />
              </div>

              <div>
                <label class="form-label" for="finance-account-currency">Currency</label>
                <input id="finance-account-currency" class="form-control" bind:value={accountCurrency} aria-label="Account currency" required />
              </div>

              <div>
                <label class="form-label" for="finance-account-kind">Kind</label>
                <select id="finance-account-kind" class="form-select" bind:value={accountKind} aria-label="Account kind">
                  <option value="manual">manual</option>
                  <option value="linked">linked</option>
                  <option value="imported">imported</option>
                  <option value="reconciliation">reconciliation</option>
                </select>
              </div>

              <div>
                <button class="btn btn-primary" type="submit" disabled={!financeShell.selectedTenantId}>
                  Create account
                </button>
              </div>
            </div>
          </form>
        </div>

        <div class="col-12 col-xl-8">
          <section class="card shadow-sm h-100">
            <div class="card-body p-4 d-grid gap-3">
              <div class="d-flex flex-column flex-md-row justify-content-between gap-2 align-items-md-center">
                <div>
                  <h2 class="h5 mb-1">Account list</h2>
                  <p class="text-body-secondary mb-0">Review current account sources and jump into account detail routes.</p>
                </div>

                <span class="badge text-bg-secondary align-self-start align-self-md-center">
                  {accounts.length} account{accounts.length === 1 ? '' : 's'}
                </span>
              </div>

              {#if !financeShell.selectedTenantId}
                <div class="alert alert-light border mb-0" role="status">No accounts yet.</div>
              {:else if accounts.length === 0}
                <div class="alert alert-light border mb-0" role="status">No accounts yet.</div>
              {:else}
                <div class="list-group">
                  {#each accounts as account (account.id)}
                    <article class="list-group-item d-grid gap-2">
                      <div class="d-flex flex-column flex-lg-row justify-content-between gap-3 align-items-lg-start">
                        <div class="d-grid gap-2">
                          <div>
                            <h3 class="h6 mb-1">{account.name}</h3>
                            <div class="d-flex flex-wrap gap-2">
                              <span class="badge text-bg-secondary">{account.kind}</span>
                              <span class="badge text-bg-light border text-body">{account.currency}</span>
                              {#if account.hiddenAt}
                                <span class="badge text-bg-warning">Hidden</span>
                              {/if}
                            </div>
                          </div>

                          <p class="small text-body-secondary mb-0">
                            Provider {account.provider || 'manual'} · Updated {formatFinanceDateTime(account.updatedAt)}
                          </p>
                          <div class="d-flex flex-wrap gap-2 small">
                            <span>Booked balance {formatFinanceMoney(account.bookedBalanceMinor, account.currency)}</span>
                            <span>Pending balance {formatFinanceMoney(account.pendingBalanceMinor, account.currency)}</span>
                          </div>
                        </div>

                        <a class="btn btn-outline-secondary btn-sm align-self-start" href={`/finance/accounts/${encodeURIComponent(account.id)}`} use:link>
                          Open account detail
                        </a>
                      </div>
                    </article>
                  {/each}
                </div>
              {/if}
            </div>
          </section>
        </div>
      </div>
    {/if}
  </div>
</section>
