<script lang="ts">
  import { onMount } from 'svelte'
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
  let editName = $state('')
  let editCurrency = $state('USD')
  let inviteRecipient = $state('')
  let inviteCode = $state('')
  let reactiveReady = $state(false)
  let syncedEditTenantKey = $state('')
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
      await financeApi.createTenant({ name: createName, displayCurrency: createCurrency })
      createName = ''
      await financeShell.refreshTenants()
      await loadTenantDetails()
    } catch (createError) {
      error = createError instanceof Error ? createError.message : 'Failed to create tenant'
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
      syncedEditTenantKey = ''
      return
    }

    editName = tenant.name
    editCurrency = supportedFinanceTenantDisplayCurrencies.includes(tenant.displayCurrency as (typeof supportedFinanceTenantDisplayCurrencies)[number])
      ? tenant.displayCurrency
      : supportedFinanceTenantDisplayCurrencies[0]
    syncedEditTenantKey = `${tenant.id}:${tenant.updatedAt.toISOString()}`
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

  $effect(() => {
    const tenant = financeShell.selectedTenant
    const tenantKey = tenant ? `${tenant.id}:${tenant.updatedAt.toISOString()}` : ''
    if (tenantKey === syncedEditTenantKey) {
      return
    }
    syncEditFormFromTenant(tenant)
  })
</script>

<section class="container-fluid px-0" aria-labelledby="finance-tenants-heading">
  <div class="d-grid gap-4">
    <header class="card border-0 shadow-sm">
      <div class="card-body p-4 p-xl-5">
        <div class="d-flex flex-column flex-lg-row justify-content-between gap-3 align-items-lg-center">
          <div>
            <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Workspace setup</p>
            <h1 id="finance-tenants-heading" class="h3 mb-2">Finance tenants</h1>
            <p class="text-body-secondary mb-0">
              Select the active workspace, create a tenant, invite teammates, or join with an invite code.
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
      <section class="card shadow-sm">
        <div class="card-body p-4 d-grid gap-3">
          <div>
            <h2 class="h5 mb-1">Selected tenant</h2>
            <p class="text-body-secondary mb-0">
              Use one active tenant across the finance workspace and keep tenant management here.
            </p>
          </div>

          <div class="row g-3 align-items-end">
            <div class="col-12 col-lg-6 col-xl-5">
              <label class="form-label" for="finance-tenants-selected">Selected tenant</label>
              <select
                id="finance-tenants-selected"
                class="form-select"
                value={financeShell.selectedTenantId}
                onchange={(event) =>
                  financeShell.selectTenant((event.currentTarget as HTMLSelectElement).value)}
                aria-label="Selected tenant"
              >
                <option value="">Select tenant</option>
                {#each financeShell.tenants as tenant (tenant.id)}
                  <option value={tenant.id}>{tenant.name} · {tenant.displayCurrency}</option>
                {/each}
              </select>
            </div>

            <div class="col-12 col-lg-6">
              <p class="small text-body-secondary mb-0">
                {#if financeShell.tenants.length === 0}
                  No joined tenants yet. Create one below or join with an invite code.
                {:else}
                  Member and invite details load for the currently selected tenant.
                {/if}
              </p>
            </div>
          </div>
        </div>
      </section>

      {#if financeShell.selectedTenant}
        <form class="card shadow-sm" onsubmit={updateTenant}>
          <div class="card-body p-4 d-grid gap-3">
            <div>
              <h2 class="h5 mb-1">Edit selected tenant</h2>
              <p class="text-body-secondary mb-0">
                Update the selected tenant name and display currency without leaving the finance workspace.
              </p>
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

      <div class="row g-4">
        <div class="col-12 col-xl-6">
          <form class="card shadow-sm h-100" onsubmit={createTenant}>
            <div class="card-body p-4 d-grid gap-3">
              <div>
                <h2 class="h5 mb-1">Create tenant</h2>
                <p class="text-body-secondary mb-0">Start a new finance workspace with a display currency.</p>
              </div>

              <div>
                <label class="form-label" for="finance-tenant-name">Name</label>
                <input id="finance-tenant-name" class="form-control" bind:value={createName} aria-label="Tenant name" required />
              </div>

              <div>
                <label class="form-label" for="finance-tenant-currency">Display currency</label>
                <select id="finance-tenant-currency" class="form-select" bind:value={createCurrency} aria-label="Display currency" required>
                  {#each supportedFinanceTenantDisplayCurrencies as currencyCode (currencyCode)}
                    <option value={currencyCode}>{currencyCode}</option>
                  {/each}
                </select>
              </div>

              <div>
                <button class="btn btn-primary" type="submit">Create tenant</button>
              </div>
            </div>
          </form>
        </div>

        <div class="col-12 col-xl-6">
          <form class="card shadow-sm h-100" onsubmit={acceptInvite}>
            <div class="card-body p-4 d-grid gap-3">
              <div>
                <h2 class="h5 mb-1">Join with invite code</h2>
                <p class="text-body-secondary mb-0">Accept a shared invite without leaving the finance workspace.</p>
              </div>

              <div>
                <label class="form-label" for="finance-tenant-invite-code">Invite code</label>
                <input id="finance-tenant-invite-code" class="form-control" bind:value={inviteCode} aria-label="Invite code" required />
              </div>

              <div>
                <button class="btn btn-primary" type="submit">Accept invite</button>
              </div>
            </div>
          </form>
        </div>
      </div>

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
                        <strong>{member.userId}</strong>
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
                  <div>
                    <h2 class="h5 mb-1">Invites</h2>
                    <p class="text-body-secondary mb-0">Create and review outstanding invite codes for this tenant.</p>
                  </div>

                  <form class="row g-3 align-items-end" onsubmit={createInvite}>
                    <div class="col-12 col-lg-8">
                      <label class="form-label" for="finance-tenant-invite-recipient">Recipient</label>
                      <input id="finance-tenant-invite-recipient" class="form-control" bind:value={inviteRecipient} aria-label="Invite recipient" required />
                    </div>
                    <div class="col-12 col-lg-4 d-grid">
                      <button class="btn btn-primary" type="submit">Create invite</button>
                    </div>
                  </form>
                </div>

                {#if invites.length === 0}
                  <div class="alert alert-light border mb-0" role="status">No invites yet.</div>
                {:else}
                  <div class="list-group">
                    {#each invites as invite (invite.id)}
                      <article class="list-group-item d-grid gap-2">
                        <div class="d-flex flex-column flex-md-row justify-content-between gap-2 align-items-md-center">
                          <strong>{invite.recipient}</strong>
                          <span class="small text-body-secondary">Created {formatFinanceDateTime(invite.createdAt)}</span>
                        </div>
                        <div>
                          <span class="small text-body-secondary">Invite code</span>
                          <div><code>{invite.code}</code></div>
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
    {/if}
  </div>
</section>
