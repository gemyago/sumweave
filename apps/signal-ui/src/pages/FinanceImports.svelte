<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import FinanceSubnav from '../components/FinanceSubnav.svelte'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { createSignalFinanceApiForAuth, type FinanceCSVImportAudit, type FinanceCSVImportPreview, type FinanceTenantSummary } from '../lib/finance/api'
  import { chooseFinanceTenantId, setPreferredFinanceTenantId } from '../lib/finance/tenant-selection'
  import { formatFinanceDateTime } from '../lib/finance/format'
  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  let loading = $state(true); let error = $state<string | null>(null); let tenants = $state<FinanceTenantSummary[]>([]); let selectedTenantId = $state(''); let importType = $state('transactions'); let fileName = $state('sample.csv'); let csv = $state('account,amount\nChecking,100'); let preview = $state<FinanceCSVImportPreview | null>(null); let mapping = $state<Record<string, string>>({}); let audit = $state<FinanceCSVImportAudit | null>(null); const selectedTenant = $derived(tenants.find((item) => item.id === selectedTenantId) ?? null)
  onMount(() => { void loadPage() })
  async function loadPage() { loading = true; error = null; try { tenants = await financeApi.listTenants(); selectedTenantId = chooseFinanceTenantId(tenants); if (selectedTenantId) { setPreferredFinanceTenantId(selectedTenantId) } } catch (loadError) { error = loadError instanceof Error ? loadError.message : 'Failed to load imports' } finally { loading = false } }
  async function previewImport(event: SubmitEvent) { event.preventDefault(); preview = await financeApi.previewCSVImport({ tenantId: selectedTenantId, importType, fileName, csv }); mapping = { ...preview.mapping } }
  async function confirmImport() { if (!preview) return; const confirmation = await financeApi.confirmCSVImport({ tenantId: selectedTenantId, importId: preview.importId, mapping }); audit = await financeApi.getCSVImportAudit({ tenantId: selectedTenantId, importId: confirmation.importId }) }
</script>

<section class="page" aria-labelledby="finance-imports-heading">
  <header>
    <h1 id="finance-imports-heading">Finance imports</h1>
    <p class="muted">Preview CSV input, confirm mapping, and deep-link to the durable finance import job.</p>
  </header>
  <FinanceSubnav current="/finance/imports" tenantName={selectedTenant?.name ?? ''} />
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if loading}
    <p class="muted" role="status">Loading imports…</p>
  {:else}
    <section class="panel">
      <label>
        <span>Tenant</span>
        <select bind:value={selectedTenantId} onchange={() => setPreferredFinanceTenantId(selectedTenantId)} aria-label="Tenant">
          <option value="">Select tenant</option>
          {#each tenants as tenant (tenant.id)}
            <option value={tenant.id}>{tenant.name}</option>
          {/each}
        </select>
      </label>
    </section>
    <form class="panel" onsubmit={previewImport}>
      <h2>Preview import</h2>
      <label><span>Import type</span><select bind:value={importType} aria-label="Import type"><option value="transactions">transactions</option><option value="accounts">accounts</option></select></label>
      <label><span>File name</span><input bind:value={fileName} aria-label="Import file name" required /></label>
      <label><span>CSV</span><textarea bind:value={csv} aria-label="Import CSV" rows="8" required></textarea></label>
      <button class="primary" type="submit" disabled={!selectedTenantId}>Preview import</button>
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

<style>.page,.stack{display:flex;flex-direction:column;gap:var(--space-16)}.panel{display:flex;flex-direction:column;gap:var(--space-12);padding:var(--space-16);border:1px solid var(--border);border-radius:4px;background:var(--bg-elevated,var(--bg))}.panel h2,header h1{margin:0}.muted{margin:0;color:var(--text-muted)}.error{color:var(--color-danger-red)}textarea{width:100%;box-sizing:border-box}</style>
