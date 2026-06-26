<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import FinanceSubnav from '../components/FinanceSubnav.svelte'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalFinanceApiForAuth,
    type FinanceBankConnection,
    type FinanceTenantSummary,
  } from '../lib/finance/api'
  import { formatFinanceDateTime } from '../lib/finance/format'
  import { chooseFinanceTenantId, setPreferredFinanceTenantId } from '../lib/finance/tenant-selection'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const monobankProvider = 'monobank'
  const pkoProvider = 'pko'

  let loading = $state(true)
  let error = $state<string | null>(null)
  let tenants = $state<FinanceTenantSummary[]>([])
  let selectedTenantId = $state('')
  let connections = $state<FinanceBankConnection[]>([])
  let token = $state('')
  let lastJobId = $state('')
  let finishingRedirect = false
  let deletingConnectionId = $state('')
  let confirmDeleteConnectionId = $state('')

  const selectedTenant = $derived(tenants.find((item) => item.id === selectedTenantId) ?? null)

  onMount(() => {
    void loadPage()
  })

  async function loadPage() {
    loading = true
    error = null
    try {
      tenants = await financeApi.listTenants()
      selectedTenantId = chooseFinanceTenantId(tenants)
      if (selectedTenantId) {
        setPreferredFinanceTenantId(selectedTenantId)
        await loadConnections()
        await finishRedirectIfReturned()
      }
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load connections'
    } finally {
      loading = false
    }
  }

  async function loadConnections() {
    if (!selectedTenantId) {
      return
    }
    confirmDeleteConnectionId = ''
    connections = await financeApi.listConnections({ tenantId: selectedTenantId })
  }

  async function linkMonobankToken(event: SubmitEvent) {
    event.preventDefault()
    error = null
    try {
      await financeApi.linkTokenConnection({ tenantId: selectedTenantId, provider: monobankProvider, token })
      token = ''
      await loadConnections()
    } catch (linkError) {
      error = linkError instanceof Error ? linkError.message : 'Failed to link monobank connection'
    }
  }

  async function startPkoRedirect() {
    if (!selectedTenantId) {
      return
    }
    error = null
    try {
      const started = await financeApi.startRedirectConnection({
        tenantId: selectedTenantId,
        provider: pkoProvider,
        callbackUrl: `${window.location.origin}/#/finance/connections`,
      })
      window.location.assign(started.authorizationUrl)
    } catch (startError) {
      error = startError instanceof Error ? startError.message : 'Failed to start PKO connection'
    }
  }

  async function finishRedirectIfReturned() {
    if (finishingRedirect || !selectedTenantId || window.location.hash !== '#/finance/connections') {
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
      await financeApi.finishRedirectConnection({ tenantId: selectedTenantId, provider: pkoProvider, code, state })
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

  async function triggerSync(connectionId: string) {
    const job = await financeApi.triggerConnectionSync({ tenantId: selectedTenantId, connectionId, reason: 'operator_ui' })
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
      await financeApi.deleteConnection({ tenantId: selectedTenantId, connectionId: connection.id })
      connections = connections.filter((item) => item.id !== connection.id)
      confirmDeleteConnectionId = ''
    } catch (deleteError) {
      error = deleteError instanceof Error ? deleteError.message : 'Failed to delete connection'
    } finally {
      deletingConnectionId = ''
    }
  }
</script>

<section class="page" aria-labelledby="finance-connections-heading">
  <header>
      <h1 id="finance-connections-heading">Finance connections</h1>
      <p class="muted">Use provider-specific monobank token linking or start PKO bank-login linking here, while keeping connection schedules and sync history visible. Deleting a link removes only local connection metadata and scheduled sync state.</p>
  </header>

  <FinanceSubnav current="/finance/connections" tenantName={selectedTenant?.name ?? ''} />

  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}

  {#if loading}
    <p class="muted" role="status">Loading connections…</p>
  {:else}
    <section class="panel">
      <label>
        <span>Tenant</span>
        <select
          bind:value={selectedTenantId}
          onchange={() => {
            setPreferredFinanceTenantId(selectedTenantId)
            void loadConnections()
          }}
          aria-label="Tenant"
        >
          <option value="">Select tenant</option>
          {#each tenants as tenant (tenant.id)}
            <option value={tenant.id}>{tenant.name}</option>
          {/each}
        </select>
      </label>
    </section>

    <div class="grid">
      <form class="panel" onsubmit={linkMonobankToken}>
        <h2>Link monobank</h2>
        <label>
          <span>Token</span>
          <input bind:value={token} aria-label="Monobank token" required />
        </label>
        <button class="primary" type="submit" disabled={!selectedTenantId}>Link monobank</button>
        <p class="muted">This flow always submits a monobank token for the selected tenant.</p>
      </form>

      <section class="panel">
        <h2>Connect PKO</h2>
        <p class="muted">Start the PKO bank-login flow, complete consent in Enable Banking, and return here to finish linking in the browser.</p>
        <button class="primary" type="button" disabled={!selectedTenantId} onclick={() => void startPkoRedirect()}>
          Connect PKO with bank login
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

  .error {
    color: var(--color-danger-red);
  }
</style>
