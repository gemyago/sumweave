<script lang="ts">
  import { onMount } from 'svelte'
  import { formatFinanceDateTime } from '../lib/finance/format'
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'
  import JobDetail from './JobDetail.svelte'
  let { params = {} } = $props<{ params?: { jobId?: string } }>()

  const financeShell = useFinanceShellState()

  let loading = $state(true)
  let error = $state<string | null>(null)

  onMount(() => {
    void loadWorkspace()
  })

  async function loadWorkspace() {
    loading = true
    error = null
    try {
      await financeShell.initialize()
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load finance workspace'
    } finally {
      loading = false
    }
  }
</script>

<section class="page" aria-labelledby="finance-job-detail-workspace-heading">
  <header>
    <h1 id="finance-job-detail-workspace-heading">Finance job route</h1>
    <p class="muted">Resolve the active finance tenant once, then stay on the requested finance job detail route.</p>
  </header>

  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}

  {#if loading}
    <p class="muted" role="status">Loading finance workspace…</p>
  {:else if financeShell.needsTenantSelection}
    <section class="panel">
      {#if !financeShell.embedded}
        <label>
          <span>Tenant</span>
          <select value={financeShell.selectedTenantId} onchange={(event) => financeShell.selectTenant((event.currentTarget as HTMLSelectElement).value)} aria-label="Tenant">
            <option value="">Select tenant</option>
            {#each financeShell.tenants as tenant (tenant.id)}
              <option value={tenant.id}>{tenant.name}</option>
            {/each}
          </select>
        </label>
      {/if}
      <p>Select an active tenant to continue on this finance route.</p>
    </section>
  {:else if !financeShell.selectedTenantId}
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

  label {
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
