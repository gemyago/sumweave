<script lang="ts">
  import { onMount } from 'svelte'
  import FinanceSubnav from '../components/FinanceSubnav.svelte'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { createSignalFinanceApiForAuth, type FinanceTenantInvite, type FinanceTenantMember, type FinanceTenantSummary } from '../lib/finance/api'
  import { chooseFinanceTenantId, setPreferredFinanceTenantId } from '../lib/finance/tenant-selection'
  import { formatFinanceDateTime } from '../lib/finance/format'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))

  let loading = $state(true)
  let error = $state<string | null>(null)
  let tenants = $state<FinanceTenantSummary[]>([])
  let selectedTenantId = $state('')
  let members = $state<FinanceTenantMember[]>([])
  let invites = $state<FinanceTenantInvite[]>([])
  let createName = $state('')
  let createCurrency = $state('USD')
  let inviteRecipient = $state('')
  let inviteCode = $state('')

  const selectedTenant = $derived(tenants.find((item) => item.id === selectedTenantId) ?? null)

  onMount(() => { void loadPage() })

  async function loadPage() {
    loading = true
    error = null
    try {
      tenants = await financeApi.listTenants()
      selectedTenantId = chooseFinanceTenantId(tenants)
      if (selectedTenantId) {
        setPreferredFinanceTenantId(selectedTenantId)
        await loadTenantDetails()
      }
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load tenants'
    } finally {
      loading = false
    }
  }

  async function loadTenantDetails() {
    if (!selectedTenantId) {
      members = []
      invites = []
      return
    }
    ;[members, invites] = await Promise.all([
      financeApi.listTenantMembers({ tenantId: selectedTenantId }),
      financeApi.listTenantInvites({ tenantId: selectedTenantId }),
    ])
  }

  async function onTenantChange() {
    setPreferredFinanceTenantId(selectedTenantId)
    await loadTenantDetails()
  }

  async function createTenant(event: SubmitEvent) {
    event.preventDefault()
    await financeApi.createTenant({ name: createName, displayCurrency: createCurrency })
    createName = ''
    await loadPage()
  }

  async function createInvite(event: SubmitEvent) {
    event.preventDefault()
    if (!selectedTenantId) return
    await financeApi.createTenantInvite({ tenantId: selectedTenantId, recipient: inviteRecipient })
    inviteRecipient = ''
    await loadTenantDetails()
  }

  async function acceptInvite(event: SubmitEvent) {
    event.preventDefault()
    await financeApi.acceptTenantInvite({ code: inviteCode })
    inviteCode = ''
    await loadPage()
  }
</script>

<section class="page" aria-labelledby="finance-tenants-heading">
  <header><h1 id="finance-tenants-heading">Finance tenants</h1><p class="muted">Select the active tenant, create a new one, issue invites, or join with an invite code.</p></header>
  <FinanceSubnav current="/finance/tenants" tenantName={selectedTenant?.name ?? ''} />
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if loading}<p class="muted" role="status">Loading finance tenants…</p>{:else}
    <section class="panel">
      <label><span>Selected tenant</span><select bind:value={selectedTenantId} onchange={() => void onTenantChange()} aria-label="Selected tenant"><option value="">Select tenant</option>{#each tenants as tenant (tenant.id)}<option value={tenant.id}>{tenant.name} · {tenant.displayCurrency}</option>{/each}</select></label>
    </section>
    <div class="grid">
      <form class="panel" onsubmit={createTenant}>
        <h2>Create tenant</h2>
        <label><span>Name</span><input bind:value={createName} aria-label="Tenant name" required /></label>
        <label><span>Display currency</span><input bind:value={createCurrency} aria-label="Display currency" required /></label>
        <button class="primary" type="submit">Create tenant</button>
      </form>
      <form class="panel" onsubmit={acceptInvite}>
        <h2>Join with invite code</h2>
        <label><span>Invite code</span><input bind:value={inviteCode} aria-label="Invite code" required /></label>
        <button class="primary" type="submit">Accept invite</button>
      </form>
    </div>
    {#if selectedTenantId}
      <div class="grid">
        <section class="panel">
          <h2>Members</h2>
          {#if members.length === 0}<p class="muted">No members found.</p>{:else}<div class="stack">{#each members as member (member.userId)}<article class="card"><strong>{member.userId}</strong><span>{formatFinanceDateTime(member.joinedAt)}</span></article>{/each}</div>{/if}
        </section>
        <section class="panel">
          <h2>Invites</h2>
          <form class="stack" onsubmit={createInvite}>
            <label><span>Recipient</span><input bind:value={inviteRecipient} aria-label="Invite recipient" required /></label>
            <button class="primary" type="submit">Create invite</button>
          </form>
          {#if invites.length === 0}<p class="muted">No invites yet.</p>{:else}<div class="stack">{#each invites as invite (invite.id)}<article class="card"><strong>{invite.recipient}</strong><span>{invite.code}</span><span>{formatFinanceDateTime(invite.createdAt)}</span></article>{/each}</div>{/if}
        </section>
      </div>
    {/if}
  {/if}
</section>

<style>
  .page,.stack{display:flex;flex-direction:column;gap:var(--space-16)}
  .grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(280px,1fr));gap:var(--space-16)}
  .panel{display:flex;flex-direction:column;gap:var(--space-12);padding:var(--space-16);border:1px solid var(--border);border-radius:4px;background:var(--bg-elevated,var(--bg))}
  .panel h2,header h1{margin:0}
  label{display:flex;flex-direction:column;gap:var(--space-8)}
  .card{display:flex;justify-content:space-between;gap:var(--space-12);padding:var(--space-12);border:1px solid var(--border);border-radius:4px}
  .muted{margin:0;color:var(--text-muted)}
  .error{color:var(--color-danger-red)}
</style>
