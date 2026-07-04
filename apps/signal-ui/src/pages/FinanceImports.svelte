<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { createSignalFinanceApiForAuth, type FinanceCSVImportAudit, type FinanceCSVImportPreview } from '../lib/finance/api'
  import { formatFinanceDateTime } from '../lib/finance/format'
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'
  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const financeShell = useFinanceShellState()
  let loading = $state(true); let error = $state<string | null>(null); let importType = $state('transactions'); let fileName = $state('sample.csv'); let csv = $state('account,amount\nChecking,100'); let preview = $state<FinanceCSVImportPreview | null>(null); let mapping = $state<Record<string, string>>({}); let audit = $state<FinanceCSVImportAudit | null>(null)
  onMount(() => { void loadPage() })
  async function loadPage() { loading = true; error = null; try { await financeShell.initialize() } catch (loadError) { error = loadError instanceof Error ? loadError.message : 'Failed to load imports' } finally { loading = false } }
  async function previewImport(event: SubmitEvent) { event.preventDefault(); if (!financeShell.selectedTenantId) return; preview = await financeApi.previewCSVImport({ tenantId: financeShell.selectedTenantId, importType, fileName, csv }); mapping = { ...preview.mapping } }
  async function confirmImport() { if (!preview || !financeShell.selectedTenantId) return; const confirmation = await financeApi.confirmCSVImport({ tenantId: financeShell.selectedTenantId, importId: preview.importId, mapping }); audit = await financeApi.getCSVImportAudit({ tenantId: financeShell.selectedTenantId, importId: confirmation.importId }) }
</script>

<section class="page" aria-labelledby="finance-imports-heading">
  <header>
    <h1 id="finance-imports-heading">Finance imports</h1>
    <p class="muted">Preview CSV input, confirm mapping, and deep-link to the durable finance import job.</p>
  </header>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if loading}
    <p class="muted" role="status">Loading imports…</p>
  {:else}
    {#if !financeShell.embedded || financeShell.needsTenantSelection || !financeShell.selectedTenantId}
      <section class="panel">
        {#if !financeShell.embedded}
          <label><span>Tenant</span><select value={financeShell.selectedTenantId} onchange={(event) => financeShell.selectTenant((event.currentTarget as HTMLSelectElement).value)} aria-label="Tenant"><option value="">Select tenant</option>{#each financeShell.tenants as tenant (tenant.id)}<option value={tenant.id}>{tenant.name}</option>{/each}</select></label>
        {/if}
        {#if financeShell.needsTenantSelection}
          <p>Select an active tenant to continue on this finance route.</p>
        {:else if !financeShell.selectedTenantId}
          <p>Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a> before importing CSV data.</p>
        {/if}
      </section>
    {/if}
    <form class="panel" onsubmit={previewImport}>
      <h2>Preview import</h2>
      <label><span>Import type</span><select bind:value={importType} aria-label="Import type"><option value="transactions">transactions</option><option value="accounts">accounts</option></select></label>
      <label><span>File name</span><input bind:value={fileName} aria-label="Import file name" required /></label>
      <label><span>CSV</span><textarea bind:value={csv} aria-label="Import CSV" rows="8" required></textarea></label>
      <button class="primary" type="submit" disabled={!financeShell.selectedTenantId}>Preview import</button>
    </form>
    {#if preview}
      <section class="panel">
        <h2>Preview result</h2>
        <p class="muted">Import {preview.importId} · headers {preview.headers.join(', ') || '—'}</p>
        <div class="stack">
          {#each Object.entries(mapping) as [header, field] (header)}
            <label><span>{header}</span><input value={field} oninput={(event) => { mapping = { ...mapping, [header]: (event.currentTarget as HTMLInputElement).value } }} aria-label={`Mapping ${header}`} /></label>
          {/each}
        </div>
        <p class="muted">Would create accounts: {preview.wouldCreateAccounts.join(', ') || '—'}</p>
        <p class="muted">Would create categories: {preview.wouldCreateCategories.join(', ') || '—'}</p>
        <p class="muted">Would create tags: {preview.wouldCreateTags.join(', ') || '—'}</p>
        <button class="primary" type="button" onclick={() => void confirmImport()}>Confirm import</button>
      </section>
    {/if}
    {#if audit}
      <section class="panel">
        <h2>Import audit</h2>
        <p class="muted">Status {audit.status} · imported {audit.importedCount} rows · created {formatFinanceDateTime(audit.createdAt)}</p>
        <p><a href={`/finance/jobs/${encodeURIComponent(audit.jobId)}`} use:link>Open finance job detail</a></p>
      </section>
    {/if}
  {/if}
</section>

<style>.page,.stack{display:flex;flex-direction:column;gap:var(--space-16)}.panel{display:flex;flex-direction:column;gap:var(--space-12);padding:var(--space-16);border:1px solid var(--border);border-radius:4px;background:var(--bg-elevated,var(--bg))}.panel h2,header h1{margin:0}.muted{margin:0;color:var(--text-muted)}label{display:flex;flex-direction:column;gap:var(--space-8)}.error{color:var(--color-danger-red)}textarea{width:100%;box-sizing:border-box}</style>
