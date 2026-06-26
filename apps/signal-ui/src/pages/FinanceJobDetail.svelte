<script lang="ts">
  import { onMount } from 'svelte'
  import FinanceSubnav from '../components/FinanceSubnav.svelte'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { createSignalFinanceApiForAuth, type FinanceTenantSummary } from '../lib/finance/api'
  import { formatFinanceDateTime } from '../lib/finance/format'
  import { chooseFinanceTenantId, setPreferredFinanceTenantId } from '../lib/finance/tenant-selection'
  import JobDetail from './JobDetail.svelte'
  let { params = {} } = $props<{ params?: { jobId?: string } }>()

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))

  let loading = $state(true)
  let error = $state<string | null>(null)
  let tenants = $state<FinanceTenantSummary[]>([])
  let selectedTenantId = $state('')

  const selectedTenant = $derived(tenants.find((item) => item.id === selectedTenantId) ?? null)
  const needsTenantSelection = $derived(tenants.length > 1 && !selectedTenantId)

  onMount(() => {
    void loadWorkspace()
  })

  async function loadWorkspace() {
    loading = true
    error = null
    try {
      tenants = await financeApi.listTenants()
      selectedTenantId = chooseFinanceTenantId(tenants)
      if (selectedTenantId) {
        setPreferredFinanceTenantId(selectedTenantId)
      }
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load finance workspace'
    } finally {
      loading = false
    }
  }

  function onTenantChange() {
    setPreferredFinanceTenantId(selectedTenantId)
  }
</script>

<section class="page" aria-labelledby="finance-job-detail-workspace-heading">
  <header>
    <h1 id="finance-job-detail-workspace-heading">Finance job route</h1>
    <p class="muted">Resolve the active finance tenant once, then stay on the requested finance job detail route.</p>
  </header>

  <FinanceSubnav current="" tenantName={selectedTenant?.name ?? ''} />

  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}

  {#if loading}
    <p class="muted" role="status">Loading finance workspace…</p>
  {:else if needsTenantSelection}
    <section class="panel">
      <label>
        <span>Tenant</span>
        <select bind:value={selectedTenantId} onchange={onTenantChange} aria-label="Tenant">
          <option value="">Select tenant</option>
          {#each tenants as tenant (tenant.id)}
            <option value={tenant.id}>{tenant.name}</option>
          {/each}
        </select>
      </label>
    </section>
    <section class="panel">
      <p>Select an active tenant to continue on this finance route.</p>
    </section>
  {:else if !selectedTenantId}
    <section class="panel">
      <p>Create or join a tenant from <a href="/finance/tenants">Finance tenants</a> before opening this finance job detail route.</p>
    </section>
  {:else}
    <JobDetail {params} heading="Finance job detail" description="Inspect an import, sync, or FX job without losing finance context." primaryBackHref="/finance/connections" primaryBackLabel="Back to finance connections" secondaryBackHref="/finance/imports" secondaryBackLabel="Back to finance imports" formatDateValue={formatFinanceDateTime} />
  {/if}
</section>

<style>
  .page {
    display: flex;
    flex-direction: column;
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

  .panel label {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }

  .page h1 {
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
