<script lang="ts">
  import { onMount } from 'svelte'
  import { documentTitle } from '../lib/document-title'
  import DocumentTitle from '../components/DocumentTitle.svelte'
  import { SvelteSet } from 'svelte/reactivity'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalFinanceApiForAuth,
    type FinanceTenantInvite,
    type FinanceTenantMember,
    type FinanceTenantSummary,
  } from '../lib/finance/api'
  import { supportedFinanceTenantDisplayCurrencies } from '../lib/finance/tenant-display-currencies'
  import { formatFinanceDateTime } from '../lib/finance/format'
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const financeShell = useFinanceShellState()

  let loading = $state(true)
  let error = $state<string | null>(null)
  let members = $state<FinanceTenantMember[]>([])
  let invites = $state<FinanceTenantInvite[]>([])
  let createName = $state('')
  let createCurrency = $state('USD')
  let createSeedDefaults = $state(true)
  let editName = $state('')
  let editCurrency = $state('USD')
  let inviteRecipient = $state('')
  let inviteCode = $state('')
  let activePanel = $state<TenantPanel | null>(null)
  const revealedInviteCodes = new SvelteSet<string>()
  let reactiveReady = $state(false)
  let archiveConfirmationTenantId = $state<string | null>(null)
  let archivingTenantId = $state<string | null>(null)
  let skipNextReactiveLoad = false

  type TenantPanel = 'create' | 'edit' | 'join' | 'invite'

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
        await loadTenantDetails()
      }
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load tenants'
    } finally {
      skipNextReactiveLoad = true
      reactiveReady = true
      loading = false
    }
  }

  async function loadTenantDetails() {
    if (!financeShell.selectedTenantId) {
      members = []
      invites = []
      return
    }

    ;[members, invites] = await Promise.all([
      financeApi.listTenantMembers({ tenantId: financeShell.selectedTenantId }),
      financeApi.listTenantInvites({ tenantId: financeShell.selectedTenantId }),
    ])
  }

  async function onTenantChange() {
    error = null

    try {
      await loadTenantDetails()
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load tenants'
    }
  }

  async function createTenant(event: SubmitEvent) {
    event.preventDefault()
    error = null

    try {
      await financeApi.createTenant({
        name: createName,
        displayCurrency: createCurrency,
        seedDefaults: createSeedDefaults,
      })
      createName = ''
      createSeedDefaults = true
      activePanel = null
      await financeShell.refreshTenants()
      await loadTenantDetails()
    } catch (createError) {
      error = createError instanceof Error ? createError.message : 'Failed to create tenant'
    }
  }

  async function archiveTenant(tenant: FinanceTenantSummary) {
    if (archivingTenantId) return
    error = null
    archivingTenantId = tenant.id

    try {
      await financeApi.archiveTenant({ tenantId: tenant.id })
      archiveConfirmationTenantId = null
      await financeShell.refreshTenants()
      await loadTenantDetails()
      activePanel = null
    } catch (archiveError) {
      error = archiveError instanceof Error ? archiveError.message : 'Failed to archive tenant'
    } finally {
      archivingTenantId = null
    }
  }

  async function updateTenant(event: SubmitEvent) {
    event.preventDefault()
    if (!financeShell.selectedTenantId) return
    error = null

    try {
      await financeApi.updateTenant({
        tenantId: financeShell.selectedTenantId,
        name: editName,
        displayCurrency: editCurrency,
      })
      await financeShell.refreshTenants()
      await loadTenantDetails()
      activePanel = null
    } catch (updateError) {
      error = updateError instanceof Error ? updateError.message : 'Failed to update tenant'
    }
  }

  async function createInvite(event: SubmitEvent) {
    event.preventDefault()
    if (!financeShell.selectedTenantId) return
    error = null

    try {
      await financeApi.createTenantInvite({
        tenantId: financeShell.selectedTenantId,
        recipient: inviteRecipient,
      })
      inviteRecipient = ''
      activePanel = null
      await loadTenantDetails()
    } catch (createError) {
      error = createError instanceof Error ? createError.message : 'Failed to create invite'
    }
  }

  async function acceptInvite(event: SubmitEvent) {
    event.preventDefault()
    error = null

    try {
      await financeApi.acceptTenantInvite({ code: inviteCode })
      inviteCode = ''
      activePanel = null
      await financeShell.refreshTenants()
      await loadTenantDetails()
    } catch (acceptError) {
      error = acceptError instanceof Error ? acceptError.message : 'Failed to accept invite'
    }
  }

  function syncEditFormFromTenant(tenant: FinanceTenantSummary | null) {
    if (!tenant) {
      editName = ''
      editCurrency = supportedFinanceTenantDisplayCurrencies[0]
      return
    }

    editName = tenant.name
    editCurrency = supportedFinanceTenantDisplayCurrencies.includes(tenant.displayCurrency as (typeof supportedFinanceTenantDisplayCurrencies)[number])
      ? tenant.displayCurrency
      : supportedFinanceTenantDisplayCurrencies[0]
  }

  function openPanel(panel: TenantPanel) {
    error = null
    if (panel === 'edit') {
      syncEditFormFromTenant(financeShell.selectedTenant)
    }
    activePanel = panel
  }

  function closePanel() {
    error = null
    activePanel = null
  }

  function selectTenant(tenantId: string) {
    closePanel()
    financeShell.selectTenant(tenantId)
  }

  function toggleInviteCode(inviteId: string) {
    if (revealedInviteCodes.has(inviteId)) {
      revealedInviteCodes.delete(inviteId)
    } else {
      revealedInviteCodes.add(inviteId)
    }
  }

  async function copyInviteCode(invite: FinanceTenantInvite) {
    error = null
    if (!window.navigator.clipboard?.writeText) {
      error = 'Copying invite codes is unavailable in this browser. Reveal the code and copy it manually.'
      return
    }
    try {
      await window.navigator.clipboard.writeText(invite.code)
    } catch {
      error = 'Could not copy the invite code. Reveal the code and copy it manually.'
    }
  }

  $effect(() => {
    if (financeShell.loading || !reactiveReady) return
    void financeShell.selectedTenantId
    if (skipNextReactiveLoad) {
      skipNextReactiveLoad = false
      return
    }
    void onTenantChange()
  })

</script>

<DocumentTitle title={documentTitle('Tenants', 'Finance')} />

<section class="container-fluid px-0" aria-labelledby="finance-tenants-heading">
  <div class="d-grid gap-4">
    <header class="card border-0 shadow-sm">
      <div class="card-body p-4 p-xl-5">
        <div class="d-flex flex-column flex-lg-row justify-content-between gap-3 align-items-lg-center">
          <div>
            <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Workspace setup</p>
            <h1 id="finance-tenants-heading" class="h3 mb-2">Finance tenants</h1>
            <p class="text-body-secondary mb-0">
              Select an active workspace, then create, edit, invite, or join when needed.
            </p>
          </div>

          {#if financeShell.selectedTenant}
            <span class="badge text-bg-secondary align-self-start align-self-lg-center">
              Active tenant · {financeShell.selectedTenant.name}
            </span>
          {/if}
        </div>
      </div>
    </header>

    {#if error}
      <div class="alert alert-danger mb-0" role="alert">{error}</div>
    {/if}

    {#if loading}
      <div class="alert alert-secondary mb-0" role="status">Loading finance tenants…</div>
    {:else}
      <section class="card shadow-sm" aria-labelledby="finance-active-tenants-heading">
        <div class="card-body p-4 d-grid gap-3">
          <div class="d-flex flex-column flex-xxl-row justify-content-between gap-2 align-items-xxl-center">
            <div>
              <h2 id="finance-active-tenants-heading" class="h5 mb-1">Active tenants</h2>
              <p class="text-body-secondary mb-0">Choose the active workspace or archive a tenant that is no longer in use.</p>
            </div>
             <div class="d-flex flex-wrap gap-2 align-self-start align-self-xxl-center">
               <button class="btn btn-primary btn-sm" type="button" onclick={() => openPanel('create')}>Create tenant</button>
               <button class="btn btn-outline-primary btn-sm" type="button" onclick={() => openPanel('join')}>Join by code</button>
               {#if financeShell.selectedTenant}
                 <button class="btn btn-outline-primary btn-sm" type="button" onclick={() => openPanel('edit')}>Edit selected</button>
                 <button class="btn btn-outline-primary btn-sm" type="button" onclick={() => openPanel('invite')}>Invite member</button>
               {/if}
               <span class="badge text-bg-secondary align-self-center">{financeShell.tenants.length} active</span>
             </div>
          </div>

          {#if financeShell.tenants.length === 0}
             <div class="alert alert-light border mb-0" role="status">No joined active tenants. Use Create tenant or Join by code to get started.</div>
          {:else}
            <div class="list-group" aria-label="Active finance tenants" aria-busy={Boolean(archivingTenantId)}>
              {#each financeShell.tenants as tenant (tenant.id)}
                <article class="list-group-item p-2 p-sm-3" aria-label={`Tenant ${tenant.name}`}>
                  <div class="d-flex flex-column flex-sm-row justify-content-between align-items-sm-center gap-2">
                    <div class="min-w-0">
                      <div class="d-flex flex-wrap align-items-baseline gap-2">
                        <strong>{tenant.name}</strong>
                        <span class="small text-body-secondary">{tenant.displayCurrency}</span>
                        {#if tenant.id === financeShell.selectedTenantId}<span class="badge text-bg-secondary">active</span>{/if}
                      </div>
                      <span class="small text-body-secondary">Joined {formatFinanceDateTime(tenant.joinedAt)}</span>
                    </div>

                    {#if archiveConfirmationTenantId === tenant.id}
                      <div class="d-flex flex-wrap align-items-center gap-2" role="group" aria-label={`Confirm archive ${tenant.name}`}>
                        <span class="small text-danger">Archive this tenant?</span>
                        <button class="btn btn-outline-danger btn-sm" type="button" onclick={() => void archiveTenant(tenant)} disabled={Boolean(archivingTenantId)}>
                          {archivingTenantId === tenant.id ? 'Archiving…' : 'Confirm archive'}
                        </button>
                        <button class="btn btn-outline-secondary btn-sm" type="button" onclick={() => archiveConfirmationTenantId = null} disabled={Boolean(archivingTenantId)}>Cancel</button>
                      </div>
                    {:else}
                      <div class="d-flex flex-wrap gap-2">
                         <button class="btn btn-outline-primary btn-sm" type="button" onclick={() => selectTenant(tenant.id)} disabled={tenant.id === financeShell.selectedTenantId || Boolean(archivingTenantId)}>
                          {tenant.id === financeShell.selectedTenantId ? 'Selected' : 'Select'}
                        </button>
                        <button class="btn btn-outline-danger btn-sm" type="button" onclick={() => archiveConfirmationTenantId = tenant.id} disabled={Boolean(archivingTenantId)}>Archive</button>
                      </div>
                    {/if}
                  </div>
                </article>
              {/each}
            </div>
          {/if}
        </div>
      </section>

      {#if activePanel === 'create'}
        <form class="card shadow-sm" onsubmit={createTenant} aria-labelledby="finance-create-tenant-heading">
          <div class="card-body p-4 d-grid gap-3">
            <div class="d-flex flex-column flex-sm-row justify-content-between gap-2 align-items-sm-start">
              <div>
                <h2 id="finance-create-tenant-heading" class="h5 mb-1">Create tenant</h2>
                <p class="text-body-secondary mb-0">Start a finance workspace with a display currency.</p>
              </div>
              <button class="btn btn-outline-secondary btn-sm align-self-start" type="button" onclick={closePanel}>Cancel</button>
            </div>

            <div class="row g-3 align-items-end">
              <div class="col-12 col-xl-5">
                <label class="form-label" for="finance-tenant-name">Name</label>
                <input id="finance-tenant-name" class="form-control" bind:value={createName} aria-label="Tenant name" required />
              </div>
              <div class="col-12 col-xl-4">
                <label class="form-label" for="finance-tenant-currency">Display currency</label>
                <select id="finance-tenant-currency" class="form-select" bind:value={createCurrency} aria-label="Display currency" required>
                  {#each supportedFinanceTenantDisplayCurrencies as currencyCode (currencyCode)}
                    <option value={currencyCode}>{currencyCode}</option>
                  {/each}
                </select>
              </div>
              <div class="col-12 col-xl-3 d-grid">
                <button class="btn btn-primary" type="submit">Create tenant</button>
              </div>
            </div>

            <div class="form-check">
              <input id="finance-tenant-seed-defaults" class="form-check-input" type="checkbox" bind:checked={createSeedDefaults} />
              <label class="form-check-label" for="finance-tenant-seed-defaults">Add starter categories and tags</label>
              <div class="form-text">Recommended for a new workspace. You can create your own catalog instead.</div>
            </div>
          </div>
        </form>
      {:else if activePanel === 'join'}
        <form class="card shadow-sm" onsubmit={acceptInvite} aria-labelledby="finance-join-tenant-heading">
          <div class="card-body p-4 d-grid gap-3">
            <div class="d-flex flex-column flex-sm-row justify-content-between gap-2 align-items-sm-start">
              <div>
                <h2 id="finance-join-tenant-heading" class="h5 mb-1">Join by invite code</h2>
                <p class="text-body-secondary mb-0">Accept a shared invite without leaving Finance.</p>
              </div>
              <button class="btn btn-outline-secondary btn-sm align-self-start" type="button" onclick={closePanel}>Cancel</button>
            </div>
            <div class="row g-3 align-items-end">
              <div class="col-12 col-xl-9">
                <label class="form-label" for="finance-tenant-invite-code">Invite code</label>
                <input id="finance-tenant-invite-code" class="form-control" bind:value={inviteCode} aria-label="Invite code" required />
              </div>
              <div class="col-12 col-xl-3 d-grid">
                <button class="btn btn-primary" type="submit">Accept invite</button>
              </div>
            </div>
          </div>
        </form>
      {:else if activePanel === 'invite' && financeShell.selectedTenantId}
        <form class="card shadow-sm" onsubmit={createInvite} aria-labelledby="finance-invite-member-heading">
          <div class="card-body p-4 d-grid gap-3">
            <div class="d-flex flex-column flex-sm-row justify-content-between gap-2 align-items-sm-start">
              <div>
                <h2 id="finance-invite-member-heading" class="h5 mb-1">Invite member</h2>
                <p class="text-body-secondary mb-0">Create an invitation for the selected tenant.</p>
              </div>
              <button class="btn btn-outline-secondary btn-sm align-self-start" type="button" onclick={closePanel}>Cancel</button>
            </div>
            <div class="row g-3 align-items-end">
              <div class="col-12 col-xl-9">
                <label class="form-label" for="finance-tenant-invite-recipient">Recipient</label>
                <input id="finance-tenant-invite-recipient" class="form-control" bind:value={inviteRecipient} aria-label="Invite recipient" required />
              </div>
              <div class="col-12 col-xl-3 d-grid">
                <button class="btn btn-primary" type="submit">Create invite</button>
              </div>
            </div>
          </div>
        </form>
      {:else if activePanel === 'edit' && financeShell.selectedTenant}
        <form class="card shadow-sm" onsubmit={updateTenant}>
          <div class="card-body p-4 d-grid gap-3">
            <div class="d-flex flex-column flex-sm-row justify-content-between gap-2 align-items-sm-start">
              <div>
              <h2 class="h5 mb-1">Edit selected tenant</h2>
              <p class="text-body-secondary mb-0">
                Update the selected tenant name and display currency without leaving the finance workspace.
              </p>
              </div>
              <button class="btn btn-outline-secondary btn-sm align-self-start" type="button" onclick={closePanel}>Cancel</button>
            </div>

            <div class="row g-3 align-items-end">
              <div class="col-12 col-xl-5">
                <label class="form-label" for="finance-selected-tenant-name">Tenant name</label>
                <input
                  id="finance-selected-tenant-name"
                  class="form-control"
                  bind:value={editName}
                  aria-label="Tenant name"
                  required
                />
              </div>

              <div class="col-12 col-xl-4">
                <label class="form-label" for="finance-selected-tenant-currency">Display currency</label>
                <select
                  id="finance-selected-tenant-currency"
                  class="form-select"
                  bind:value={editCurrency}
                  aria-label="Display currency"
                  required
                >
                  {#each supportedFinanceTenantDisplayCurrencies as currencyCode (currencyCode)}
                    <option value={currencyCode}>{currencyCode}</option>
                  {/each}
                </select>
              </div>

              <div class="col-12 col-xl-3 d-grid">
                <button class="btn btn-primary" type="submit">Save tenant changes</button>
              </div>
            </div>
          </div>
        </form>
      {/if}

      {#if financeShell.selectedTenantId}
        <div class="row g-4">
          <div class="col-12 col-xl-6">
            <section class="card shadow-sm h-100">
              <div class="card-body p-4 d-grid gap-3">
                <div>
                  <h2 class="h5 mb-1">Members</h2>
                  <p class="text-body-secondary mb-0">Current people with access to this tenant.</p>
                </div>

                {#if members.length === 0}
                  <div class="alert alert-light border mb-0" role="status">No members found.</div>
                {:else}
                  <div class="list-group">
                    {#each members as member (member.userId)}
                      <article class="list-group-item d-flex flex-column flex-md-row justify-content-between gap-2 align-items-md-center">
                         <div>
                           {#if member.username}
                             <strong>{member.username}</strong>
                             <div class="small text-body-secondary">Technical ID · <code>{member.userId}</code></div>
                           {:else}
                             <strong>Username unavailable</strong>
                             <div class="small text-body-secondary">Technical ID · <code>{member.userId}</code></div>
                           {/if}
                         </div>
                         <span class="small text-body-secondary">Joined {formatFinanceDateTime(member.joinedAt)}</span>
                      </article>
                    {/each}
                  </div>
                {/if}
              </div>
            </section>
          </div>

           <div class="col-12 col-xl-6">
            <section class="card shadow-sm h-100">
              <div class="card-body p-4 d-grid gap-4">
                <div class="d-grid gap-3">
                  <div class="d-flex flex-column flex-sm-row justify-content-between gap-2 align-items-sm-start">
                    <div>
                    <h2 class="h5 mb-1">Invites</h2>
                    <p class="text-body-secondary mb-0">Review pending and accepted invitations for this tenant.</p>
                    </div>
                    <button class="btn btn-outline-primary btn-sm align-self-start" type="button" onclick={() => openPanel('invite')}>Invite member</button>
                  </div>
                </div>

                {#if invites.length === 0}
                  <div class="alert alert-light border mb-0" role="status">No invites yet.</div>
                {:else}
                  <div class="d-grid gap-3">
                    <section aria-labelledby="finance-pending-invites-heading">
                      <h3 id="finance-pending-invites-heading" class="h6 mb-2">Pending</h3>
                      {#if invites.some((invite) => !invite.acceptedAt)}
                        <div class="list-group">
                          {#each invites.filter((invite) => !invite.acceptedAt) as invite (invite.id)}
                            <article class="list-group-item d-grid gap-2">
                              <div class="d-flex flex-column flex-md-row justify-content-between gap-2 align-items-md-center">
                                <strong>{invite.recipient}</strong>
                                <span class="badge text-bg-warning align-self-start align-self-md-center">Pending</span>
                              </div>
                              <span class="small text-body-secondary">Created {formatFinanceDateTime(invite.createdAt)}</span>
                              {#if revealedInviteCodes.has(invite.id)}
                                <div class="d-flex flex-wrap align-items-center gap-2">
                                  <code>{invite.code}</code>
                                  <button class="btn btn-outline-secondary btn-sm" type="button" onclick={() => toggleInviteCode(invite.id)}>Hide code</button>
                                  <button class="btn btn-outline-primary btn-sm" type="button" onclick={() => void copyInviteCode(invite)}>Copy code</button>
                                </div>
                              {:else}
                                <div><button class="btn btn-outline-secondary btn-sm" type="button" onclick={() => toggleInviteCode(invite.id)}>Reveal code</button></div>
                              {/if}
                            </article>
                          {/each}
                        </div>
                      {:else}
                        <p class="small text-body-secondary mb-0">No pending invites.</p>
                      {/if}
                    </section>

                    <section aria-labelledby="finance-accepted-invites-heading">
                      <h3 id="finance-accepted-invites-heading" class="h6 mb-2">Accepted</h3>
                      {#if invites.some((invite) => invite.acceptedAt)}
                        <div class="list-group">
                          {#each invites.filter((invite) => invite.acceptedAt) as invite (invite.id)}
                            <article class="list-group-item d-grid gap-1">
                              <div class="d-flex flex-column flex-md-row justify-content-between gap-2 align-items-md-center">
                                <strong>{invite.recipient}</strong>
                                <span class="badge text-bg-success align-self-start align-self-md-center">Accepted</span>
                              </div>
                              <span class="small text-body-secondary">Accepted {formatFinanceDateTime(invite.acceptedAt ?? invite.createdAt)} · Created {formatFinanceDateTime(invite.createdAt)}</span>
                            </article>
                          {/each}
                        </div>
                      {:else}
                        <p class="small text-body-secondary mb-0">No accepted invites.</p>
                      {/if}
                    </section>
                  </div>
                {/if}
              </div>
            </section>
          </div>
        </div>
      {/if}

    {/if}
  </div>
</section>
