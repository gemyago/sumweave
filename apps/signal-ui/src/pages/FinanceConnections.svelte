<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalFinanceApiForAuth,
    type FinanceBankConnection,
  } from '../lib/finance/api'
  import { formatFinanceDateTime } from '../lib/finance/format'
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const financeShell = useFinanceShellState()
  const monobankProvider = 'monobank'
  const pkoProvider = 'pko'
  const syntheticProvider = 'synthetic'

  let loading = $state(true)
  let error = $state<string | null>(null)
  let connections = $state<FinanceBankConnection[]>([])
  let token = $state('')
  let lastJobId = $state('')
  let finishingRedirect = false
  let deletingConnectionId = $state('')
  let confirmDeleteConnectionId = $state('')
  let reactiveReady = $state(false)
  let skipNextReactiveLoad = false
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
        await loadConnections()
        await finishRedirectIfReturned()
      }
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load connections'
    } finally {
      skipNextReactiveLoad = true
      reactiveReady = true
      loading = false
    }
  }

  async function loadConnections() {
    if (!financeShell.selectedTenantId) {
      connections = []
      return
    }
    confirmDeleteConnectionId = ''
    connections = await financeApi.listConnections({ tenantId: financeShell.selectedTenantId })
  }

  async function linkMonobankToken(event: SubmitEvent) {
    event.preventDefault()
    error = null
    try {
      await financeApi.linkTokenConnection({ tenantId: financeShell.selectedTenantId, provider: monobankProvider, token })
      token = ''
      await loadConnections()
    } catch (linkError) {
      error = linkError instanceof Error ? linkError.message : 'Failed to link monobank connection'
    }
  }

  async function startPkoRedirect() {
    if (!financeShell.selectedTenantId) {
      return
    }
    error = null
    try {
      const started = await financeApi.startRedirectConnection({
        tenantId: financeShell.selectedTenantId,
        provider: pkoProvider,
        callbackUrl: `${window.location.origin}/#/finance/connections`,
      })
      navigateToAuthorizationUrl(started.authorizationUrl)
    } catch (startError) {
      error = startError instanceof Error ? startError.message : 'Failed to start PKO connection'
    }
  }

  async function startSyntheticSetup() {
    if (!financeShell.selectedTenantId) {
      return
    }
    error = null
    try {
      const started = await financeApi.startRedirectConnection({
        tenantId: financeShell.selectedTenantId,
        provider: syntheticProvider,
        callbackUrl: `${window.location.origin}/#/finance/connections`,
      })
      navigateToAuthorizationUrl(started.authorizationUrl)
    } catch (startError) {
      error = startError instanceof Error ? startError.message : 'Failed to start synthetic setup'
    }
  }

  async function finishRedirectIfReturned() {
    if (finishingRedirect || !financeShell.selectedTenantId || window.location.hash !== '#/finance/connections') {
      return
    }
    const params = new URLSearchParams(window.location.search)
    const code = params.get('code')
    const state = params.get('state')
    if (!code || !state) {
      return
    }

    finishingRedirect = true
    try {
      await financeApi.finishRedirectConnection({ tenantId: financeShell.selectedTenantId, provider: pkoProvider, code, state })
      clearConsumedRedirectQuery()
    } catch (finishError) {
      error = finishError instanceof Error ? finishError.message : 'Failed to finish PKO connection'
      return
    } finally {
      finishingRedirect = false
    }

    await loadConnections()
  }

  function clearConsumedRedirectQuery() {
    window.history.replaceState({}, '', `${window.location.pathname}${window.location.hash}`)
  }

  function navigateToAuthorizationUrl(authorizationUrl: string) {
    const target = new URL(authorizationUrl, window.location.href)
    if (target.origin === window.location.origin && target.hash.startsWith('#/finance/')) {
      window.history.pushState({}, '', `${target.pathname}${target.search}${target.hash}`)
      window.dispatchEvent(new HashChangeEvent('hashchange'))
      return
    }
    window.location.assign(target.toString())
  }

  async function triggerSync(connectionId: string) {
    const job = await financeApi.triggerConnectionSync({ tenantId: financeShell.selectedTenantId, connectionId, reason: 'operator_ui' })
    lastJobId = job.jobId
    await loadConnections()
  }

  function getConnectionSecondaryIdentifier(connection: FinanceBankConnection) {
    if (connection.providerReference) {
      return `Provider ref: ${connection.providerReference}`
    }
    if (connection.externalId) {
      return `External id: ${connection.externalId}`
    }
    if (connection.createdAt) {
      return `Created: ${formatFinanceDateTime(connection.createdAt)}`
    }
    return `Connection id: ${connection.id}`
  }

  function requestDeleteConnection(connectionId: string) {
    confirmDeleteConnectionId = connectionId
  }

  function cancelDeleteConnection() {
    confirmDeleteConnectionId = ''
  }

  async function deleteConnection(connection: FinanceBankConnection) {
    error = null
    deletingConnectionId = connection.id
    try {
      await financeApi.deleteConnection({ tenantId: financeShell.selectedTenantId, connectionId: connection.id })
      connections = connections.filter((item) => item.id !== connection.id)
      confirmDeleteConnectionId = ''
    } catch (deleteError) {
      error = deleteError instanceof Error ? deleteError.message : 'Failed to delete connection'
    } finally {
      deletingConnectionId = ''
    }
  }

  $effect(() => {
    if (financeShell.loading || !reactiveReady) return
    void financeShell.selectedTenantId
    if (skipNextReactiveLoad) {
      skipNextReactiveLoad = false
      return
    }
    void loadConnections()
    void finishRedirectIfReturned()
  })
</script>

<section class="page" aria-labelledby="finance-connections-heading">
  <header>
      <h1 id="finance-connections-heading">Finance connections</h1>
      <p class="muted">Use provider-specific monobank token linking, PKO bank-login linking, or synthetic local setup here while keeping connection schedules and sync history visible. Deleting a link removes only local connection metadata and scheduled sync state.</p>
  </header>
  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}

  {#if loading}
    <p class="muted" role="status">Loading connections…</p>
  {:else if financeShell.needsTenantSelection}
    <section class="panel">
      {#if !financeShell.embedded}
        <label><span>Tenant</span><select value={financeShell.selectedTenantId} onchange={(event) => financeShell.selectTenant((event.currentTarget as HTMLSelectElement).value)} aria-label="Tenant"><option value="">Select tenant</option>{#each financeShell.tenants as tenant (tenant.id)}<option value={tenant.id}>{tenant.name}</option>{/each}</select></label>
      {/if}
      <p>Select an active tenant to continue on this finance route.</p>
    </section>
  {:else if !financeShell.selectedTenantId}
    <section class="panel"><p>Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a> before linking finance providers.</p></section>
  {:else}
    {#if !financeShell.embedded}
      <section class="panel">
        <label><span>Tenant</span><select value={financeShell.selectedTenantId} onchange={(event) => financeShell.selectTenant((event.currentTarget as HTMLSelectElement).value)} aria-label="Tenant"><option value="">Select tenant</option>{#each financeShell.tenants as tenant (tenant.id)}<option value={tenant.id}>{tenant.name}</option>{/each}</select></label>
      </section>
    {/if}

    <div class="grid">
      <form class="panel" onsubmit={linkMonobankToken}>
        <h2>Link monobank</h2>
        <label>
          <span>Token</span>
          <input bind:value={token} aria-label="Monobank token" required />
        </label>
        <button class="primary" type="submit" disabled={!financeShell.selectedTenantId}>Link monobank</button>
        <p class="muted">This flow always submits a monobank token for the selected tenant.</p>
      </form>

      <section class="panel">
        <h2>Connect PKO</h2>
        <p class="muted">Start the PKO bank-login flow, complete consent in Enable Banking, and return here to finish linking in the browser.</p>
        <button class="primary" type="button" disabled={!financeShell.selectedTenantId} onclick={() => void startPkoRedirect()}>
          Connect PKO with bank login
        </button>
      </section>

      <section class="panel">
        <h2>Configure synthetic provider</h2>
        <p class="muted">Start the local synthetic setup flow, configure one or more synthetic accounts, then finish the link back in finance connections.</p>
        <button class="primary" type="button" disabled={!financeShell.selectedTenantId} onclick={() => void startSyntheticSetup()}>
          Start synthetic setup
        </button>
      </section>

      <section class="panel">
        <h2>Operator notes</h2>
        <p class="muted">Use per-connection schedule state, last job id, and next run visibility here. Admin diagnostics stay cross-cutting and sanitized.</p>
        {#if lastJobId}
          <p><a href={`/finance/jobs/${encodeURIComponent(lastJobId)}`} use:link>Open latest finance job</a></p>
        {/if}
      </section>
    </div>

    <div class="stack">
      {#each connections as connection (connection.id)}
        <article class="panel">
          <div class="row">
            <div>
              <h2>{connection.displayName}</h2>
              <p class="muted">{connection.provider} · {connection.state}</p>
              <p class="muted">{getConnectionSecondaryIdentifier(connection)}</p>
            </div>
            <div class="actions">
              <button class="primary" type="button" onclick={() => void triggerSync(connection.id)} disabled={deletingConnectionId === connection.id}>Sync now</button>
              <button class="secondary" type="button" onclick={() => requestDeleteConnection(connection.id)} disabled={deletingConnectionId === connection.id || confirmDeleteConnectionId === connection.id}>
                Delete link
              </button>
            </div>
          </div>

          <div class="schedule muted">
            <span>Last started: {formatFinanceDateTime(connection.lastSyncStartedAt)}</span>
            <span>Last success: {formatFinanceDateTime(connection.lastSuccessfulSyncAt)}</span>
            <span>Next run: {formatFinanceDateTime(connection.schedule?.nextRunAt ?? null)}</span>
          </div>

          {#if connection.schedule}
            <div class="schedule muted">
              <span>Schedule: {connection.schedule.enabled ? 'enabled' : 'disabled'}</span>
              <span>Interval: {connection.schedule.intervalSeconds}s</span>
            </div>
          {/if}

          {#if connection.lastSyncJobId}
            <p><a href={`/finance/jobs/${encodeURIComponent(connection.lastSyncJobId)}`} use:link>Open last sync job</a></p>
          {/if}

          {#if confirmDeleteConnectionId === connection.id}
            <div class="delete-confirmation" aria-live="polite">
              <p>Delete {connection.displayName} ({getConnectionSecondaryIdentifier(connection)})? This removes only the local link metadata and schedule. Imported ledger history stays.</p>
              <div class="actions">
                <button class="danger" type="button" onclick={() => void deleteConnection(connection)} disabled={deletingConnectionId === connection.id}>
                  {deletingConnectionId === connection.id ? 'Deleting…' : 'Confirm delete'}
                </button>
                <button class="secondary" type="button" onclick={cancelDeleteConnection} disabled={deletingConnectionId === connection.id}>
                  Cancel
                </button>
              </div>
            </div>
          {/if}
        </article>
      {:else}
        <p class="muted">No connections yet.</p>
      {/each}
    </div>
  {/if}
</section>

<style>
  .page,
  .stack {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: var(--space-16);
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

  .row {
    display: flex;
    justify-content: space-between;
    gap: var(--space-12);
    align-items: flex-start;
  }

  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-8);
    justify-content: flex-end;
  }

  .schedule {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-12);
  }

  .delete-confirmation {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    padding: var(--space-12);
    border: 1px solid var(--danger-border);
    border-radius: 4px;
    background: var(--danger-bg);
    color: var(--text-h);
  }

  .panel h2,
  header h1 {
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
</style>
