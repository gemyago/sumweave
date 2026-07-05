<script lang="ts">
  import { onMount } from 'svelte'
  import { link, replace } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalFinanceApiForAuth,
    type FinanceSyntheticLinkState,
    type FinanceSyntheticLinkStateConfiguredAccount,
  } from '../lib/finance/api'
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const syntheticProvider = 'synthetic'
  const financeShell = useFinanceShellState()

  type ConfiguredAccountRow = {
    rowId: string
    key?: string
    name: string
    currency: string
  }

  let loading = $state(true)
  let saving = $state(false)
  let finishing = $state(false)
  let error = $state<string | null>(null)
  let saveMessage = $state<string | null>(null)
  let setupStateKey = $state('')
  let syntheticLinkState = $state<FinanceSyntheticLinkState | null>(null)
  let configuredAccounts = $state<ConfiguredAccountRow[]>([])
  let nextRowId = 0
  let reactiveReady = $state(false)
  let skipNextReactiveLoad = false

  onMount(() => {
    void loadPage()
  })

  function readSetupStateFromHash(): string {
    if (typeof window === 'undefined') {
      return ''
    }

    const hash = window.location.hash.startsWith('#') ? window.location.hash.slice(1) : window.location.hash
    const queryIndex = hash.indexOf('?')
    if (queryIndex < 0) {
      return ''
    }
    if (hash.slice(0, queryIndex) !== '/finance/connections/synthetic') {
      return ''
    }
    return new URLSearchParams(hash.slice(queryIndex + 1)).get('state')?.trim() ?? ''
  }

  function createBlankAccountRow(): ConfiguredAccountRow {
    nextRowId += 1
    return { rowId: `local-${nextRowId}`, name: '', currency: '' }
  }

  function mapConfiguredAccounts(accounts: FinanceSyntheticLinkStateConfiguredAccount[]): ConfiguredAccountRow[] {
    if (accounts.length === 0) {
      return [createBlankAccountRow()]
    }

    return accounts.map((account) => {
      nextRowId += 1
      return {
        rowId: `saved-${nextRowId}`,
        key: account.key,
        name: account.name,
        currency: account.currency,
      }
    })
  }

  async function loadPage() {
    loading = true
    error = null
    saveMessage = null
    setupStateKey = readSetupStateFromHash()
    reactiveReady = false

    try {
      await financeShell.initialize()
      if (!financeShell.selectedTenantId || !setupStateKey) {
        if (!setupStateKey) {
          configuredAccounts = [createBlankAccountRow()]
        }
        return
      }
      await loadSyntheticLinkState()
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load synthetic setup'
    } finally {
      skipNextReactiveLoad = true
      reactiveReady = true
      loading = false
    }
  }

  async function loadSyntheticLinkState() {
    if (!financeShell.selectedTenantId || !setupStateKey) {
      return
    }

    error = null
    saveMessage = null
    syntheticLinkState = await financeApi.getSyntheticLinkState({
      tenantId: financeShell.selectedTenantId,
      state: setupStateKey,
    })
    configuredAccounts = mapConfiguredAccounts(syntheticLinkState.configuredAccounts)
  }

  function updateConfiguredAccount(rowId: string, field: 'name' | 'currency', value: string) {
    configuredAccounts = configuredAccounts.map((account) =>
      account.rowId === rowId ? { ...account, [field]: value } : account,
    )
  }

  function onConfiguredAccountNameInput(rowId: string, event: Event) {
    updateConfiguredAccount(rowId, 'name', (event.currentTarget as HTMLInputElement).value)
  }

  function onConfiguredAccountCurrencyInput(rowId: string, event: Event) {
    updateConfiguredAccount(rowId, 'currency', (event.currentTarget as HTMLInputElement).value)
  }

  function addConfiguredAccount() {
    configuredAccounts = [...configuredAccounts, createBlankAccountRow()]
  }

  function removeConfiguredAccount(rowId: string) {
    configuredAccounts = configuredAccounts.filter((account) => account.rowId !== rowId)
  }

  function normalizeConfiguredAccounts(): Array<{ key?: string; name: string; currency: string }> | null {
    const normalized: Array<{ key?: string; name: string; currency: string }> = []

    for (const [index, account] of configuredAccounts.entries()) {
      const name = account.name.trim()
      const currency = account.currency.trim()

      if (!name && !currency) {
        continue
      }
      if (!name || !currency) {
        error = `Configured account ${index + 1} requires both name and currency.`
        return null
      }

      normalized.push({ ...(account.key ? { key: account.key } : {}), name, currency })
    }

    if (normalized.length === 0) {
      error = 'Add at least one configured account before saving or finishing.'
      return null
    }

    return normalized
  }

  async function saveConfiguration(event?: SubmitEvent) {
    event?.preventDefault()
    if (!financeShell.selectedTenantId || !setupStateKey || saving) {
      return false
    }

    const normalized = normalizeConfiguredAccounts()
    if (!normalized) {
      return false
    }

    saving = true
    error = null
    saveMessage = null

    try {
      syntheticLinkState = await financeApi.saveSyntheticLinkState({
        tenantId: financeShell.selectedTenantId,
        state: setupStateKey,
        configuredAccounts: normalized,
      })
      configuredAccounts = mapConfiguredAccounts(syntheticLinkState.configuredAccounts)
      saveMessage = 'Configuration saved.'
      return true
    } catch (saveError) {
      error = saveError instanceof Error ? saveError.message : 'Failed to save synthetic configuration'
      return false
    } finally {
      saving = false
    }
  }

  async function finishLink() {
    if (!financeShell.selectedTenantId || !setupStateKey || finishing) {
      return
    }

    finishing = true
    error = null
    const saved = await saveConfiguration()
    if (!saved) {
      finishing = false
      return
    }

    try {
      await financeApi.finishRedirectConnection({
        tenantId: financeShell.selectedTenantId,
        provider: syntheticProvider,
        state: setupStateKey,
      })
      replace('/finance/connections')
    } catch (finishError) {
      error = finishError instanceof Error ? finishError.message : 'Failed to finish synthetic link'
    } finally {
      finishing = false
    }
  }

  $effect(() => {
    if (financeShell.loading || !reactiveReady) return
    if (!financeShell.selectedTenantId) {
      syntheticLinkState = null
      configuredAccounts = [createBlankAccountRow()]
      return
    }
    if (skipNextReactiveLoad) {
      skipNextReactiveLoad = false
      return
    }
    void loadSyntheticLinkState()
  })
</script>

<section class="container-fluid px-0" aria-labelledby="finance-synthetic-setup-heading">
  <div class="d-grid gap-4">
    <header class="card border-0 shadow-sm">
      <div class="card-body p-4 p-xl-5 d-flex flex-column flex-lg-row justify-content-between gap-3 align-items-lg-center">
        <div>
          <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Synthetic linking</p>
          <h1 id="finance-synthetic-setup-heading" class="h3 mb-2">Synthetic setup</h1>
          <p class="text-body-secondary mb-0">
            Configure one or more synthetic accounts, save pending state, and finish the local redirect flow without leaving Finance routes.
          </p>
        </div>

        <a class="btn btn-outline-secondary align-self-start align-self-lg-center" href="/finance/connections" use:link>
          Back to connections
        </a>
      </div>
    </header>

    {#if error}
      <div class="alert alert-danger mb-0" role="alert">{error}</div>
    {/if}

    {#if saveMessage}
      <div class="alert alert-success mb-0" role="status">{saveMessage}</div>
    {/if}

    {#if loading}
      <div class="alert alert-secondary mb-0" role="status">Loading synthetic setup…</div>
    {:else if financeShell.needsTenantSelection}
      <section class="card shadow-sm">
        <div class="card-body p-4 d-grid gap-3">
          {#if !financeShell.embedded}
            <div class="col-12 col-lg-5 px-0">
              <label class="form-label" for="finance-synthetic-tenant">Tenant</label>
              <select
                id="finance-synthetic-tenant"
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
        Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a> before linking a synthetic provider.
      </div>
    {:else}
      <section class="card shadow-sm">
        <div class="card-body p-4 d-flex flex-column flex-lg-row justify-content-between gap-3 align-items-lg-center">
          <div>
            <h2 class="h5 mb-1">Pending setup state</h2>
            <p class="text-body-secondary mb-0">
              {#if setupStateKey}
                State <code>{setupStateKey}</code>
              {:else}
                No pending synthetic setup state is present in this route yet.
              {/if}
            </p>
          </div>

          {#if setupStateKey}
            <button class="btn btn-outline-secondary btn-sm align-self-start align-self-lg-center" type="button" onclick={() => void loadSyntheticLinkState()} disabled={saving || finishing}>
              Reload pending setup
            </button>
          {/if}
        </div>
      </section>

      {#if !setupStateKey}
        <div class="alert alert-light border mb-0" role="status">
          Start synthetic setup from <a href="/finance/connections" use:link>Finance connections</a> to get a valid pending state.
        </div>
      {:else}
        <form class="card shadow-sm" onsubmit={saveConfiguration}>
          <div class="card-body p-4 d-grid gap-4">
            <div>
              <h2 class="h5 mb-1">Configured accounts</h2>
              <p class="text-body-secondary mb-0">
                Duplicate names and currencies stay as separate rows after save and reload.
              </p>
            </div>

            <div class="d-grid gap-3">
              {#each configuredAccounts as account, index (account.rowId)}
                <div class="border rounded-3 p-3">
                  <div class="row g-3 align-items-end">
                    <div class="col-12 col-lg-5">
                      <label class="form-label" for={`synthetic-account-name-${index}`}>Account name {index + 1}</label>
                      <input
                        id={`synthetic-account-name-${index}`}
                        class="form-control"
                        value={account.name}
                        aria-label={`Account name ${index + 1}`}
                        oninput={(event) => onConfiguredAccountNameInput(account.rowId, event)}
                      />
                    </div>

                    <div class="col-12 col-lg-4">
                      <label class="form-label" for={`synthetic-account-currency-${index}`}>Account currency {index + 1}</label>
                      <input
                        id={`synthetic-account-currency-${index}`}
                        class="form-control"
                        value={account.currency}
                        aria-label={`Account currency ${index + 1}`}
                        oninput={(event) => onConfiguredAccountCurrencyInput(account.rowId, event)}
                      />
                    </div>

                    <div class="col-12 col-lg-3 d-grid">
                      <button class="btn btn-outline-secondary" type="button" onclick={() => removeConfiguredAccount(account.rowId)}>
                        Remove configured account {index + 1}
                      </button>
                    </div>
                  </div>
                </div>
              {/each}
            </div>

            <div class="d-flex flex-wrap gap-2">
              <button class="btn btn-outline-secondary" type="button" onclick={addConfiguredAccount}>Add account</button>
              <button class="btn btn-primary" type="submit" disabled={saving || finishing || !financeShell.selectedTenantId}>
                {#if saving}Saving…{:else}Save configuration{/if}
              </button>
              <button class="btn btn-success" type="button" onclick={() => void finishLink()} disabled={saving || finishing || !financeShell.selectedTenantId}>
                {#if finishing}Finishing…{:else}Finish link{/if}
              </button>
            </div>

            <div class={`alert ${syntheticLinkState?.canFinish ? 'alert-success' : 'alert-light border'} mb-0`} role="status">
              {#if syntheticLinkState?.canFinish}
                Pending setup can be finished.
              {:else}
                Save at least one configured account before finishing.
              {/if}
            </div>
          </div>
        </form>
      {/if}
    {/if}
  </div>
</section>
