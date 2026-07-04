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
    syntheticLinkState = await financeApi.getSyntheticLinkState({ tenantId: financeShell.selectedTenantId, state: setupStateKey })
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

<section class="page" aria-labelledby="finance-synthetic-setup-heading">
  <header class="hero">
    <div>
      <h1 id="finance-synthetic-setup-heading">Synthetic setup</h1>
      <p class="muted">Configure one or more synthetic accounts, save the pending setup, and finish the local redirect link without leaving finance routes.</p>
    </div>
    <a href="/finance/connections" use:link>Back to connections</a>
  </header>
  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}

  {#if saveMessage}
    <p class="success" role="status">{saveMessage}</p>
  {/if}

  {#if loading}
    <p class="muted" role="status">Loading synthetic setup…</p>
  {:else if financeShell.needsTenantSelection}
    <section class="panel">
      {#if !financeShell.embedded}
        <label><span>Tenant</span><select value={financeShell.selectedTenantId} onchange={(event) => financeShell.selectTenant((event.currentTarget as HTMLSelectElement).value)} aria-label="Tenant"><option value="">Select tenant</option>{#each financeShell.tenants as tenant (tenant.id)}<option value={tenant.id}>{tenant.name}</option>{/each}</select></label>
      {/if}
      <p>Select an active tenant to continue on this finance route.</p>
    </section>
  {:else if !financeShell.selectedTenantId}
    <section class="panel">
      <p>Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a> before linking a synthetic provider.</p>
    </section>
  {:else}
    <section class="panel">
      {#if !financeShell.embedded}
        <label><span>Tenant</span><select value={financeShell.selectedTenantId} onchange={(event) => financeShell.selectTenant((event.currentTarget as HTMLSelectElement).value)} aria-label="Tenant"><option value="">Select tenant</option>{#each financeShell.tenants as tenant (tenant.id)}<option value={tenant.id}>{tenant.name}</option>{/each}</select></label>
      {/if}
      {#if setupStateKey}
        <p class="muted">Pending setup state: {setupStateKey}</p>
      {/if}
    </section>

    {#if !setupStateKey}
      <section class="panel">
        <p>Start synthetic setup from <a href="/finance/connections" use:link>Finance connections</a> to get a valid pending state.</p>
      </section>
    {:else}
      <form class="panel stack" onsubmit={saveConfiguration}>
        <div class="row">
          <div>
            <h2>Configured accounts</h2>
            <p class="muted">Duplicate names and currencies stay as separate rows after save and reload.</p>
          </div>
          <button class="secondary" type="button" onclick={() => void loadSyntheticLinkState()} disabled={saving || finishing}>
            Reload pending setup
          </button>
        </div>

        {#each configuredAccounts as account, index (account.rowId)}
          <div class="account-row">
            <label>
              <span>Account name {index + 1}</span>
              <input
                value={account.name}
                aria-label={`Account name ${index + 1}`}
                oninput={(event) => onConfiguredAccountNameInput(account.rowId, event)}
              />
            </label>
            <label>
              <span>Account currency {index + 1}</span>
              <input
                value={account.currency}
                aria-label={`Account currency ${index + 1}`}
                oninput={(event) => onConfiguredAccountCurrencyInput(account.rowId, event)}
              />
            </label>
            <button class="secondary" type="button" onclick={() => removeConfiguredAccount(account.rowId)}>
              Remove configured account {index + 1}
            </button>
          </div>
        {/each}

        <div class="action-row">
          <button class="secondary" type="button" onclick={addConfiguredAccount}>Add account</button>
          <button class="primary" type="submit" disabled={saving || finishing || !financeShell.selectedTenantId}>
            {#if saving}Saving…{:else}Save configuration{/if}
          </button>
          <button class="primary" type="button" onclick={() => void finishLink()} disabled={saving || finishing || !financeShell.selectedTenantId}>
            {#if finishing}Finishing…{:else}Finish link{/if}
          </button>
        </div>

        <p class="muted">
          {#if syntheticLinkState?.canFinish}
            Pending setup can be finished.
          {:else}
            Save at least one configured account before finishing.
          {/if}
        </p>
      </form>
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
  .action-row,
  .account-row {
    display: flex;
    gap: var(--space-12);
  }

  .hero,
  .row {
    justify-content: space-between;
    align-items: flex-start;
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

  .account-row {
    flex-wrap: wrap;
    align-items: end;
  }

  .account-row label {
    flex: 1 1 220px;
  }

  .action-row {
    flex-wrap: wrap;
    align-items: center;
  }

  .hero h1,
  .panel h2 {
    margin: 0;
  }

  .muted {
    margin: 0;
    color: var(--text-muted);
  }

  label {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }

  .error {
    color: var(--color-danger-red);
  }

  .success {
    color: var(--color-success-green);
  }

  @media (max-width: 640px) {
    .hero,
    .row,
    .account-row,
    .action-row {
      flex-direction: column;
      align-items: stretch;
    }
  }
</style>
