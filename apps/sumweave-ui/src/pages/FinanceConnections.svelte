<script lang="ts">
  import { onMount, tick } from 'svelte'
  import { documentTitle } from '../lib/document-title'
  import DocumentTitle from '../components/DocumentTitle.svelte'
  import { link } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalFinanceApiForAuth,
    type FinanceBankConnection,
    type FinanceConnectionSyncedAccount,
  } from '../lib/finance/api'
  import JobStatus from '../components/JobStatus.svelte'
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
  let jobIdsByConnection = $state<Record<string, string>>({})
  let finishingRedirect = false
  let deletingConnectionId = $state('')
  let confirmDeleteConnectionId = $state('')
  let renamingConnectionId = $state('')
  let connectionNameDraft = $state('')
  let savingConnectionId = $state('')
  let renameError = $state<string | null>(null)
  let renamedConnectionId = $state('')
  let renameSuccess = $state('')
  let reactiveReady = $state(false)
  let skipNextReactiveLoad = false
  let syncedAccountsByConnection = $state<Record<string, FinanceConnectionSyncedAccount[]>>({})
  let syncedAccountsLoadingId = $state('')
  let syncedAccountsErrors = $state<Record<string, string>>({})
  let syncedAccountsOpen = $state<Record<string, boolean>>({})

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
    syncedAccountsByConnection = {}
    syncedAccountsErrors = {}
    connections = await financeApi.listConnections({ tenantId: financeShell.selectedTenantId })
  }

  async function linkMonobankToken(event: SubmitEvent) {
    event.preventDefault()
    error = null

    try {
      await financeApi.linkTokenConnection({
        tenantId: financeShell.selectedTenantId,
        provider: monobankProvider,
        token,
      })
      token = ''
      await loadConnections()
    } catch (linkError) {
      error = linkError instanceof Error ? linkError.message : 'Failed to link monobank connection'
    }
  }

  async function startPkoRedirect() {
    if (!financeShell.selectedTenantId) return
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
    if (!financeShell.selectedTenantId) return
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
      await financeApi.finishRedirectConnection({
        tenantId: financeShell.selectedTenantId,
        provider: pkoProvider,
        code,
        state,
      })
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
    if (!financeShell.selectedTenantId) return
    error = null

    try {
      const job = await financeApi.triggerConnectionSync({
        tenantId: financeShell.selectedTenantId,
        connectionId,
        reason: 'operator_ui',
      })
      jobIdsByConnection = { ...jobIdsByConnection, [connectionId]: job.jobId }
      const remainingAccounts = { ...syncedAccountsByConnection }
      delete remainingAccounts[connectionId]
      syncedAccountsByConnection = remainingAccounts
      const remainingErrors = { ...syncedAccountsErrors }
      delete remainingErrors[connectionId]
      syncedAccountsErrors = remainingErrors
      await loadConnections()
      if (syncedAccountsOpen[connectionId]) {
        void loadSyncedAccounts(connectionId)
      }
    } catch (syncError) {
      error = syncError instanceof Error ? syncError.message : 'Failed to trigger sync'
    }
  }

  async function loadSyncedAccounts(connectionId: string, force = false) {
    if (!financeShell.selectedTenantId || syncedAccountsLoadingId === connectionId || (!force && connectionId in syncedAccountsByConnection)) {
      return
    }

    syncedAccountsLoadingId = connectionId
    const remainingErrors = { ...syncedAccountsErrors }
    delete remainingErrors[connectionId]
    syncedAccountsErrors = remainingErrors
    try {
      const accounts = await financeApi.listConnectionSyncedAccounts({
        tenantId: financeShell.selectedTenantId,
        connectionId,
      })
      syncedAccountsByConnection = { ...syncedAccountsByConnection, [connectionId]: accounts }
    } catch (loadError) {
      syncedAccountsErrors = {
        ...syncedAccountsErrors,
        [connectionId]: loadError instanceof Error ? loadError.message : 'Failed to load synced accounts',
      }
    } finally {
      syncedAccountsLoadingId = ''
    }
  }

  function handleSyncedAccountsToggle(connectionId: string, event: Event) {
    const open = (event.currentTarget as HTMLDetailsElement).open
    syncedAccountsOpen = { ...syncedAccountsOpen, [connectionId]: open }
    if (open) {
      void loadSyncedAccounts(connectionId)
    }
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

  function renameControlId(connectionId: string) {
    return `finance-connection-rename-trigger-${connectionId}`
  }

  function renameInputId(connectionId: string) {
    return `finance-connection-rename-${connectionId}`
  }

  function renameErrorId(connectionId: string) {
    return `finance-connection-rename-error-${connectionId}`
  }

  async function focusRenameInput(connectionId: string) {
    await tick()
    document.getElementById(renameInputId(connectionId))?.focus()
  }

  async function focusRenameControl(connectionId: string) {
    await tick()
    document.getElementById(renameControlId(connectionId))?.focus({ preventScroll: true })
  }

  function requestDeleteConnection(connectionId: string) {
    confirmDeleteConnectionId = connectionId
  }

  function cancelDeleteConnection() {
    confirmDeleteConnectionId = ''
  }

  function startRenameConnection(connection: FinanceBankConnection) {
    error = null
    renameError = null
    renamedConnectionId = ''
    renameSuccess = ''
    renamingConnectionId = connection.id
    connectionNameDraft = connection.displayName
    void focusRenameInput(connection.id)
  }

  function cancelRenameConnection(connectionId: string) {
    renameError = null
    renamingConnectionId = ''
    connectionNameDraft = ''
    void focusRenameControl(connectionId)
  }

  async function renameConnection(event: SubmitEvent, connection: FinanceBankConnection) {
    event.preventDefault()
    if (!financeShell.selectedTenantId) return

    let focusTarget: 'input' | 'control'
    error = null
    renameError = null
    renamedConnectionId = ''
    renameSuccess = ''
    savingConnectionId = connection.id
    try {
      const displayName = connectionNameDraft.trim()
      await financeApi.renameConnection({
        tenantId: financeShell.selectedTenantId,
        connectionId: connection.id,
        name: displayName,
      })
      connections = connections.map((item) => item.id === connection.id ? { ...item, displayName } : item)
      renamingConnectionId = ''
      connectionNameDraft = ''
      renamedConnectionId = connection.id
      renameSuccess = `Renamed connection to ${displayName}.`
      focusTarget = 'control'
    } catch (renameFailure) {
      renameError = renameFailure instanceof Error ? renameFailure.message : 'Failed to rename connection'
      focusTarget = 'input'
    } finally {
      savingConnectionId = ''
    }

    if (focusTarget === 'control') {
      await focusRenameControl(connection.id)
    } else if (focusTarget === 'input') {
      await focusRenameInput(connection.id)
    }
  }

  async function deleteConnection(connection: FinanceBankConnection) {
    if (!financeShell.selectedTenantId) return
    error = null
    deletingConnectionId = connection.id

    try {
      await financeApi.deleteConnection({
        tenantId: financeShell.selectedTenantId,
        connectionId: connection.id,
      })
      connections = connections.filter((item) => item.id !== connection.id)
      confirmDeleteConnectionId = ''
    } catch (deleteError) {
      error = deleteError instanceof Error ? deleteError.message : 'Failed to delete connection'
    } finally {
      deletingConnectionId = ''
    }
  }

  function badgeClass(state: string) {
    if (state === 'active') return 'text-bg-success'
    if (state === 'failed') return 'text-bg-danger'
    return 'text-bg-secondary'
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

<DocumentTitle title={documentTitle('Connections & sync', 'Finance')} />

<section class="container-fluid px-0" aria-labelledby="finance-connections-heading">
  <div class="d-grid gap-4">
    <header class="card border-0 shadow-sm">
      <div class="card-body p-4 p-xl-5 d-grid gap-3">
        <div class="d-flex flex-column flex-lg-row justify-content-between gap-3 align-items-lg-center">
          <div>
            <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Connections and sync</p>
            <h1 id="finance-connections-heading" class="h3 mb-2">Finance connections</h1>
            <p class="text-body-secondary mb-0">
              Link monobank with a token, start PKO bank login through Enable Banking, or finish synthetic local setup without exposing raw provider payloads.
            </p>
          </div>

        </div>
      </div>
    </header>

    {#if error}
      <div class="alert alert-danger mb-0" role="alert">{error}</div>
    {/if}

    {#if loading}
      <div class="alert alert-secondary mb-0" role="status">Loading connections…</div>
    {:else if financeShell.needsTenantSelection}
      <section class="card shadow-sm">
        <div class="card-body p-4 d-grid gap-3">
          {#if !financeShell.embedded}
            <div class="col-12 col-lg-5 px-0">
              <label class="form-label" for="finance-connections-tenant">Tenant</label>
              <select
                id="finance-connections-tenant"
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
        Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a> before linking finance providers.
      </div>
    {:else}
      {#if !financeShell.embedded}
        <section class="card shadow-sm">
          <div class="card-body p-4">
            <div class="col-12 col-lg-5 px-0">
              <label class="form-label" for="finance-connections-selected-tenant">Tenant</label>
              <select
                id="finance-connections-selected-tenant"
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
          </div>
        </section>
      {/if}

      <div class="row g-4">
        <div class="col-12 col-xl-6 col-xxl-3">
          <form class="card shadow-sm h-100" onsubmit={linkMonobankToken}>
            <div class="card-body p-4 d-grid gap-3">
              <div>
                <h2 class="h5 mb-1">Link monobank</h2>
                <p class="text-body-secondary mb-0">Submit a monobank token for the active tenant.</p>
              </div>

              <div>
                <label class="form-label" for="finance-monobank-token">Token</label>
                <input id="finance-monobank-token" class="form-control" bind:value={token} aria-label="Monobank token" required />
              </div>

              <div>
                <button class="btn btn-primary" type="submit" disabled={!financeShell.selectedTenantId}>
                  Link monobank
                </button>
              </div>

              <p class="small text-body-secondary mb-0">This route never asks for a generic provider name field.</p>
            </div>
          </form>
        </div>

        <div class="col-12 col-xl-6 col-xxl-3">
          <section class="card shadow-sm h-100">
            <div class="card-body p-4 d-grid gap-3">
              <div>
                <h2 class="h5 mb-1">Connect PKO</h2>
                <p class="text-body-secondary mb-0">Start the Enable Banking redirect and return here to finish linking.</p>
              </div>

              <div>
                <button class="btn btn-primary" type="button" disabled={!financeShell.selectedTenantId} onclick={() => void startPkoRedirect()}>
                  Connect PKO with bank login
                </button>
              </div>
            </div>
          </section>
        </div>

        <div class="col-12 col-xl-6 col-xxl-3">
          <section class="card shadow-sm h-100">
            <div class="card-body p-4 d-grid gap-3">
              <div>
                <h2 class="h5 mb-1">Synthetic setup</h2>
                <p class="text-body-secondary mb-0">Stay in-app, configure accounts locally, and finish the link back on Finance connections.</p>
              </div>

              <div>
                <button class="btn btn-primary" type="button" disabled={!financeShell.selectedTenantId} onclick={() => void startSyntheticSetup()}>
                  Start synthetic setup
                </button>
              </div>
            </div>
          </section>
        </div>

        <div class="col-12 col-xl-6 col-xxl-3">
          <section class="card shadow-sm h-100">
            <div class="card-body p-4 d-grid gap-3">
              <div>
                <h2 class="h5 mb-1">Operator notes</h2>
                <p class="text-body-secondary mb-0">Connection cards below keep schedule state, last sync, and durable job links visible.</p>
              </div>

              <ul class="small text-body-secondary mb-0 ps-3 d-grid gap-2">
                <li>Imported ledger history stays after local link deletion.</li>
                <li>Schedule visibility remains tenant-local on this route.</li>
                <li>Admin diagnostics stay cross-cutting and sanitized.</li>
              </ul>
            </div>
          </section>
        </div>
      </div>

      <section class="card shadow-sm">
        <div class="card-body p-4 d-grid gap-3">
          <div class="d-flex flex-column flex-md-row justify-content-between gap-2 align-items-md-center">
            <div>
              <h2 class="h5 mb-1">Linked connections</h2>
              <p class="text-body-secondary mb-0">Review provider state, sync timing, and retry or delete supported links.</p>
            </div>

            <span class="badge text-bg-secondary align-self-start align-self-md-center">
              {connections.length} connection{connections.length === 1 ? '' : 's'}
            </span>
          </div>

          {#if connections.length === 0}
            <div class="alert alert-light border mb-0" role="status">No connections yet.</div>
          {:else}
            <div class="d-grid gap-3">
              {#each connections as connection (connection.id)}
                <article class="card border">
                  <div class="card-body d-grid gap-3">
                    <div class="d-flex flex-column flex-xl-row justify-content-between gap-3 align-items-xl-start">
                      <div class="d-grid gap-2">
                        {#if renamingConnectionId === connection.id}
                          <form class="d-grid gap-2" onsubmit={(event) => void renameConnection(event, connection)}>
                            <label class="form-label mb-0" for={renameInputId(connection.id)}>Connection name</label>
                            <div class="d-flex flex-wrap gap-2">
                              <input
                                id={renameInputId(connection.id)}
                                class="form-control form-control-sm"
                                bind:value={connectionNameDraft}
                                aria-describedby={renameError ? renameErrorId(connection.id) : undefined}
                                aria-invalid={renameError !== null}
                                disabled={savingConnectionId === connection.id}
                                required
                              />
                              <button class="btn btn-outline-success btn-sm" type="submit" disabled={savingConnectionId === connection.id}>
                                {savingConnectionId === connection.id ? 'Saving…' : 'Save'}
                              </button>
                              <button class="btn btn-outline-secondary btn-sm" type="button" onclick={() => cancelRenameConnection(connection.id)} disabled={savingConnectionId === connection.id}>Cancel</button>
                            </div>
                            {#if renameError}
                              <div id={renameErrorId(connection.id)} class="alert alert-danger mb-0" role="alert">{renameError}</div>
                            {/if}
                          </form>
                        {:else}
                          <div>
                            <div class="d-flex flex-wrap align-items-center gap-2">
                              <h3 class="h6 mb-1">{connection.displayName}</h3>
                              <button
                                id={renameControlId(connection.id)}
                                class="btn btn-outline-secondary btn-sm"
                                type="button"
                                aria-label={`Rename connection ${connection.displayName} (${getConnectionSecondaryIdentifier(connection)})`}
                                onclick={() => startRenameConnection(connection)}
                                disabled={savingConnectionId !== '' || deletingConnectionId === connection.id}
                              >
                                Rename
                              </button>
                            </div>
                            <div class="d-flex flex-wrap gap-2">
                              <span class="badge text-bg-secondary">{connection.provider}</span>
                              <span class={`badge ${badgeClass(connection.state)}`}>{connection.state}</span>
                            </div>
                            {#if renamedConnectionId === connection.id}
                              <div class="visually-hidden" role="status" aria-live="polite">{renameSuccess}</div>
                            {/if}
                          </div>
                        {/if}

                        <p class="small text-body-secondary mb-0">{getConnectionSecondaryIdentifier(connection)}</p>
                      </div>

                      <div class="d-flex flex-wrap gap-2">
                        <button class="btn btn-primary btn-sm" type="button" onclick={() => void triggerSync(connection.id)} disabled={deletingConnectionId === connection.id}>
                          Sync now
                        </button>
                        <button
                          class="btn btn-outline-danger btn-sm"
                          type="button"
                          onclick={() => requestDeleteConnection(connection.id)}
                          disabled={deletingConnectionId === connection.id || confirmDeleteConnectionId === connection.id}
                        >
                          Delete link
                        </button>
                      </div>
                    </div>

                    <div class="row g-3 small text-body-secondary">
                      <div class="col-12 col-md-4">Last started: {formatFinanceDateTime(connection.lastSyncStartedAt)}</div>
                      <div class="col-12 col-md-4">Last success: {formatFinanceDateTime(connection.lastSuccessfulSyncAt)}</div>
                      <div class="col-12 col-md-4">Next run: {formatFinanceDateTime(connection.schedule?.nextRunAt ?? null)}</div>
                    </div>

                    {#if connection.schedule}
                      <div class="d-flex flex-wrap gap-2">
                        <span class="badge text-bg-light border text-body">
                          Schedule {connection.schedule.enabled ? 'enabled' : 'disabled'}
                        </span>
                        <span class="badge text-bg-light border text-body">
                          Interval {connection.schedule.intervalSeconds}s
                        </span>
                      </div>
                    {/if}

                    {#if connection.lastSyncJobId}
                      <div>
                        <a href={`/finance/jobs/${encodeURIComponent(connection.lastSyncJobId)}`} use:link>Open last sync job</a>
                      </div>
                    {/if}

                    {#if jobIdsByConnection[connection.id]}
                      <JobStatus
                        jobId={jobIdsByConnection[connection.id]}
                        openHref={`/finance/jobs/${encodeURIComponent(jobIdsByConnection[connection.id])}`}
                        label="Sync"
                      />
                    {/if}

                    <details class="border rounded p-3" ontoggle={(event) => handleSyncedAccountsToggle(connection.id, event)}>
                      <summary class="fw-semibold">Synced accounts</summary>
                      <div class="mt-3 d-grid gap-2">
                        {#if syncedAccountsLoadingId === connection.id}
                          <div class="text-body-secondary" role="status">Loading synced accounts…</div>
                        {:else if syncedAccountsErrors[connection.id]}
                          <div class="alert alert-danger mb-0 d-flex flex-wrap justify-content-between gap-2 align-items-center" role="alert">
                            <span>{syncedAccountsErrors[connection.id]}</span>
                            <button class="btn btn-outline-danger btn-sm" type="button" onclick={() => void loadSyncedAccounts(connection.id, true)}>Retry synced accounts</button>
                          </div>
                        {:else if connection.id in syncedAccountsByConnection}
                          {#if syncedAccountsByConnection[connection.id].length === 0}
                            <div class="text-body-secondary" role="status">No synced accounts yet.</div>
                          {:else}
                            <div class="list-group list-group-flush">
                              {#each syncedAccountsByConnection[connection.id] as account (account.financeAccountId)}
                                <a class="list-group-item list-group-item-action d-flex flex-column flex-md-row justify-content-between gap-1" href={`/finance/accounts/${encodeURIComponent(account.financeAccountId)}`} use:link>
                                  <span>{account.name} · {account.currency}</span>
                                  <span class="small text-body-secondary">Last successful sync: {formatFinanceDateTime(account.lastSuccessfulSyncAt)}</span>
                                </a>
                              {/each}
                            </div>
                          {/if}
                        {:else}
                          <div class="text-body-secondary">Open to load account names, currencies, and last successful sync times.</div>
                        {/if}
                      </div>
                    </details>

                    {#if confirmDeleteConnectionId === connection.id}
                      <div class="alert alert-danger mb-0" aria-live="polite">
                        <p class="mb-2">
                          Delete {connection.displayName} ({getConnectionSecondaryIdentifier(connection)})? This removes only the local link metadata and schedule. Imported ledger history stays.
                        </p>
                        <div class="d-flex flex-wrap gap-2">
                          <button class="btn btn-danger btn-sm" type="button" onclick={() => void deleteConnection(connection)} disabled={deletingConnectionId === connection.id}>
                            {deletingConnectionId === connection.id ? 'Deleting…' : 'Confirm delete'}
                          </button>
                          <button class="btn btn-outline-secondary btn-sm" type="button" onclick={cancelDeleteConnection} disabled={deletingConnectionId === connection.id}>
                            Cancel
                          </button>
                        </div>
                      </div>
                    {/if}
                  </div>
                </article>
              {/each}
            </div>
          {/if}
        </div>
      </section>
    {/if}
  </div>
</section>
