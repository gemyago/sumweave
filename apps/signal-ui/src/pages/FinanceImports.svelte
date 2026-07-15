<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte'
  import { link } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { createSignalFinanceApiForAuth, FinanceApiError, type FinanceCSVImportAudit, type FinanceCSVImportPreview, type FinanceCSVRejectedRow } from '../lib/finance/api'
  import { formatFinanceDateTime } from '../lib/finance/format'
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'

  const requiredHeaders = ['Date', 'Account', 'Category', 'Tags', 'Expense amount', 'Income amount', 'Currency']
  const supportedHeaders = [...requiredHeaders, 'Description']
  const maxCSVImportBytes = 64 * 1024 * 1024
  const sampleCSV = `Date,Account,Category,Tags,Expense amount,Income amount,Currency,Description
29.05.26,Daily account,Groceries,"home, food","8\u00a0300,00",,PLN,"Monthly groceries"
30.05.26,Daily account,Salary,"work, income",,"12 500,50",PLN,"May salary"`
  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const financeShell = useFinanceShellState()

  let loading = $state(true)
  let previewing = $state(false)
  let updatingPreview = $state(false)
  let selectionDirty = $state(false)
  let confirming = $state(false)
  let error = $state<string | null>(null)
  let notice = $state<string | null>(null)
  let fileName = $state('finance-transactions.csv')
  let csv = $state(sampleCSV)
  let preview = $state<FinanceCSVImportPreview | null>(null)
  let audit = $state<FinanceCSVImportAudit | null>(null)
  let recentImports = $state<FinanceCSVImportAudit[]>([])
  let recentImportsTenantId = $state<string | null>(null)
  let openingAuditId = $state<string | null>(null)
  let pollTimer: ReturnType<typeof setTimeout> | undefined
  let previewUpdateTimer: ReturnType<typeof setTimeout> | undefined
  let previewRequestSequence = 0
  let activeWorkspace = $state<HTMLElement | undefined>(undefined)
  const creationSummaries = $derived.by((): Array<{ label: string; values: string[] }> => preview ? [
    { label: 'Would create accounts', values: preview.wouldCreateAccounts },
    { label: 'Would create categories', values: preview.wouldCreateCategories },
    { label: 'Would create tags', values: preview.wouldCreateTags },
  ] : [])
  const matchedHeaders = $derived.by(() => {
    const sourceHeaders = preview?.headers ?? []
    return supportedHeaders.filter((header) => sourceHeaders.includes(header))
  })
  const ignoredHeaders = $derived.by(() => {
    const sourceHeaders = preview?.headers ?? []
    return sourceHeaders.filter((header) => !supportedHeaders.includes(header))
  })

  onMount(() => { void loadPage() })
  onDestroy(() => {
    if (pollTimer) clearTimeout(pollTimer)
    if (previewUpdateTimer) clearTimeout(previewUpdateTimer)
  })

  $effect(() => {
    if (financeShell.loading || financeShell.selectedTenantId === recentImportsTenantId) return
    void loadRecentImports()
  })

  async function loadPage() {
    loading = true
    error = null
    try {
      await financeShell.initialize()
      await loadRecentImports()
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load imports'
    } finally { loading = false }
  }

  function resetPreview() {
    previewRequestSequence++
    preview = null
    audit = null
    updatingPreview = false
    selectionDirty = false
    if (pollTimer) clearTimeout(pollTimer)
    if (previewUpdateTimer) clearTimeout(previewUpdateTimer)
  }

  async function focusActiveWorkspace() {
    await tick()
    activeWorkspace?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    activeWorkspace?.focus({ preventScroll: true })
  }

  function isTerminalAudit(value: FinanceCSVImportAudit): boolean {
    return value.status === 'completed' || value.status === 'failed'
  }

  function mergeRecentImport(items: FinanceCSVImportAudit[], updated: FinanceCSVImportAudit): FinanceCSVImportAudit[] {
    const withoutUpdated = items.filter((item) => item.importId !== updated.importId)
    return [updated, ...withoutUpdated]
  }

  async function loadRecentImports(activeAudit?: FinanceCSVImportAudit) {
    if (!financeShell.selectedTenantId) {
      recentImports = []
      recentImportsTenantId = null
      return
    }
    const tenantId = financeShell.selectedTenantId
    const items = await financeApi.listRecentCSVImportAudits({ tenantId })
    if (financeShell.selectedTenantId !== tenantId) return
    recentImports = activeAudit ? mergeRecentImport(items, activeAudit) : items
    recentImportsTenantId = tenantId
  }

  async function selectFile(event: Event) {
    const file = (event.currentTarget as HTMLInputElement).files?.[0]
    if (!file) return
    if (file.size > maxCSVImportBytes) {
      error = 'CSV files must be 64 MiB or smaller.'
      notice = null
      return
    }
    try {
      csv = await file.text()
      fileName = file.name
      notice = `Loaded ${file.name}. Review or edit its contents before previewing.`
      error = null
      resetPreview()
    } catch {
      error = 'Could not read the selected CSV file.'
    }
  }

  async function copySample() {
    try {
      await navigator.clipboard.writeText(sampleCSV)
      notice = 'Sample CSV copied to the clipboard.'
    } catch { notice = 'Copy is unavailable in this browser. Select the sample text to copy it.' }
  }

  function downloadSample() {
    const url = URL.createObjectURL(new Blob([sampleCSV], { type: 'text/csv;charset=utf-8' }))
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = 'finance-transactions-sample.csv'
    anchor.click()
    URL.revokeObjectURL(url)
  }

  async function previewImport(event: SubmitEvent) {
    event.preventDefault()
    if (!financeShell.selectedTenantId) return
    previewing = true
    error = null
    notice = null
    resetPreview()
    const requestSequence = ++previewRequestSequence
    await focusActiveWorkspace()
    try {
      const nextPreview = await financeApi.previewCSVImport({ tenantId: financeShell.selectedTenantId, fileName, csv })
      if (requestSequence === previewRequestSequence) preview = nextPreview
    } catch (previewError) {
      if (requestSequence === previewRequestSequence) {
        error = previewError instanceof FinanceApiError && previewError.status === 400
          ? 'We could not validate this CSV preview. Check the file and try again.'
          : previewError instanceof Error ? previewError.message : 'Failed to preview import'
      }
    } finally {
      if (requestSequence === previewRequestSequence) previewing = false
    }
  }

  function updateAccountSelection(accountName: string, selected: boolean) {
    if (!preview || !financeShell.selectedTenantId) return
    preview = {
      ...preview,
      accountOptions: preview.accountOptions.map((option) => option.name === accountName ? { ...option, selected } : option),
    }
    selectionDirty = true
    updatingPreview = true
    error = null
    if (previewUpdateTimer) clearTimeout(previewUpdateTimer)
    const requestSequence = ++previewRequestSequence
    const selectedAccountNames = preview.accountOptions.filter((option) => option.selected).map((option) => option.name)
    previewUpdateTimer = setTimeout(() => {
      void regeneratePreview(requestSequence, selectedAccountNames)
    }, 300)
  }

  async function regeneratePreview(requestSequence: number, selectedAccountNames: string[]) {
    if (!financeShell.selectedTenantId) return
    try {
      const nextPreview = await financeApi.previewCSVImport({
        tenantId: financeShell.selectedTenantId,
        fileName,
        csv,
        selectedAccountNames,
      })
      if (requestSequence === previewRequestSequence) {
        preview = nextPreview
        selectionDirty = false
      }
    } catch (previewError) {
      if (requestSequence === previewRequestSequence) error = previewError instanceof Error ? previewError.message : 'Failed to update preview'
    } finally {
      if (requestSequence === previewRequestSequence) updatingPreview = false
    }
  }

  async function refreshAudit(importId: string, scheduleRefresh = false) {
    if (!financeShell.selectedTenantId) return
    try {
      const refreshedAudit = await financeApi.getCSVImportAudit({ tenantId: financeShell.selectedTenantId, importId })
      audit = refreshedAudit
      if (isTerminalAudit(refreshedAudit)) {
        recentImports = mergeRecentImport(recentImports, refreshedAudit)
        void loadRecentImports(refreshedAudit)
      }
      if (scheduleRefresh && !isTerminalAudit(refreshedAudit)) {
        pollTimer = setTimeout(() => void refreshAudit(importId, true), 1500)
      }
    } catch (auditError) {
      error = auditError instanceof Error ? auditError.message : 'Failed to refresh import progress'
    }
  }

  async function confirmImport() {
    if (!preview || !financeShell.selectedTenantId) return
    confirming = true
    error = null
    try {
      const confirmation = await financeApi.confirmCSVImport({ tenantId: financeShell.selectedTenantId, importId: preview.importId })
      await refreshAudit(confirmation.importId, true)
      await loadRecentImports()
    } catch (confirmError) {
      if (confirmError instanceof FinanceApiError && confirmError.status === 409) {
        notice = 'This import was already confirmed. Its durable audit was reopened.'
        await refreshAudit(preview.importId, true)
        await loadRecentImports()
        return
      }
      error = confirmError instanceof Error ? confirmError.message : 'Failed to confirm import'
    } finally { confirming = false }
  }

  async function reopenAudit(importId: string) {
    preview = null
    audit = null
    error = null
    notice = null
    openingAuditId = importId
    await focusActiveWorkspace()
    try {
      await refreshAudit(importId, true)
    } finally {
      openingAuditId = null
    }
  }

  function issueLabel(issue: FinanceCSVRejectedRow) {
    return `Row ${issue.rowNumber}${issue.field ? ` · ${issue.field}` : ''}: ${issue.reason}`
  }
</script>

<section class="container-fluid px-0" aria-labelledby="finance-imports-heading">
  <div class="d-grid gap-4">
    <header class="card border-0 shadow-sm"><div class="card-body p-4 p-xl-5">
      <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Transaction CSV import</p>
      <h1 id="finance-imports-heading" class="h3 mb-2">Finance imports</h1>
      <p class="text-body-secondary mb-0">Preview fixed-format transactions, then follow the durable import job.</p>
    </div></header>

    {#if notice}<div class="alert alert-info mb-0" role="status">{notice}</div>{/if}
    {#if loading}
      <div class="alert alert-secondary mb-0" role="status">Loading imports…</div>
    {:else}
      {#if !financeShell.embedded || financeShell.needsTenantSelection || !financeShell.selectedTenantId}
        <section class="card shadow-sm"><div class="card-body p-4 d-grid gap-3">
          {#if !financeShell.embedded}<div class="col-12 col-lg-5 px-0"><label class="form-label" for="finance-imports-tenant">Tenant</label><select id="finance-imports-tenant" class="form-select" value={financeShell.selectedTenantId} onchange={(event) => financeShell.selectTenant((event.currentTarget as HTMLSelectElement).value)}><option value="">Select tenant</option>{#each financeShell.tenants as tenant (tenant.id)}<option value={tenant.id}>{tenant.name}</option>{/each}</select></div>{/if}
          {#if financeShell.needsTenantSelection}<div class="alert alert-warning mb-0" role="status">Select an active tenant to continue on this finance route.</div>
          {:else if !financeShell.selectedTenantId}<div class="alert alert-light border mb-0" role="status">Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a> before importing transactions.</div>{/if}
        </div></section>
      {/if}

      <form class="card shadow-sm" onsubmit={previewImport}><div class="card-body p-4 d-grid gap-4">
        <div><p class="text-uppercase text-body-secondary fw-semibold small mb-2">Step 1</p><h2 class="h5 mb-1">Choose a transaction CSV</h2><p class="text-body-secondary mb-0">Select a file or paste/edit CSV below. Transaction imports always use this fixed contract.</p></div>
        <div class="alert alert-light border mb-0"><strong>Required headers:</strong> <code>{requiredHeaders.join(',')}</code><br /><strong>Optional header:</strong> <code>Description</code>. Headers are matched by name in any order. Missing or blank descriptions import as <code>n/a</code>. Unsupported extra columns are ignored wherever they occur. CSVs support up to 250,000 data rows (header excluded) and 64 MiB. Dates are strict <code>dd.MM.yy</code> (<code>00</code>–<code>99</code> means 2000–2099). Supported currencies: USD, EUR, PLN, UAH. Quote multi-tags such as <code>"home, food"</code> and localized amounts such as <code>"8&nbsp;300,00"</code>.</div>
        <div class="d-flex flex-wrap gap-2"><button class="btn btn-outline-secondary" type="button" onclick={() => void copySample()}>Copy sample</button><button class="btn btn-outline-secondary" type="button" onclick={downloadSample}>Download sample CSV</button></div>
        <div class="row gx-0 gy-3"><div class="col-12 col-md-5 pe-md-3"><label class="form-label" for="finance-import-file">CSV file</label><input id="finance-import-file" class="form-control" type="file" accept=".csv,text/csv" onchange={(event) => void selectFile(event)} /></div><div class="col-12 col-md-7"><label class="form-label" for="finance-import-file-name">File name</label><input id="finance-import-file-name" class="form-control" bind:value={fileName} oninput={resetPreview} required /></div><div class="col-12"><label class="form-label" for="finance-import-csv">CSV contents</label><textarea id="finance-import-csv" class="form-control" bind:value={csv} oninput={resetPreview} rows="10" required></textarea></div></div>
        <div><button class="btn btn-primary" type="submit" disabled={!financeShell.selectedTenantId || previewing}>{#if previewing}Previewing…{:else}Preview transactions{/if}</button></div>
      </div></form>

      <section bind:this={activeWorkspace} class="card shadow-sm" tabindex="-1" aria-labelledby="finance-import-workspace-heading"><div class="card-body p-4 d-grid gap-4">
        <div><p class="text-uppercase text-body-secondary fw-semibold small mb-2">Step 2</p><h2 id="finance-import-workspace-heading" class="h5 mb-1">Active import workspace</h2><p class="text-body-secondary mb-0">Preview a transaction CSV or open a recent audit; the active outcome appears here.</p></div>
        {#if previewing}
          <div class="alert alert-info mb-0" role="status" aria-live="polite">Previewing transaction CSV…</div>
        {:else if updatingPreview}
          <div class="alert alert-info mb-0" role="status" aria-live="polite">Updating preview…</div>
        {:else if openingAuditId}
          <div class="alert alert-info mb-0" role="status" aria-live="polite">Loading selected import audit…</div>
        {:else if confirming}
          <div class="alert alert-info mb-0" role="status" aria-live="polite">Confirming valid rows…</div>
        {:else if error}
          <div class="alert alert-danger mb-0" role="alert">{error}</div>
        {:else if !preview && !audit}
          <p class="text-body-secondary mb-0" role="status">No active import yet. Start with Step 1 or reopen a durable audit below.</p>
        {/if}

        {#if preview}
          <div class="d-grid gap-4">
           <div><h3 class="h5 mb-1">Preview result</h3><p class="text-body-secondary mb-0">Import {preview.importId}. Matched supported headers: {matchedHeaders.join(', ') || '—'}.</p><p class="text-body-secondary mb-0">Transactions to import: {preview.importableCount}</p>{#if ignoredHeaders.length}<p class="text-body-secondary mb-0">Ignored source headers: {ignoredHeaders.join(', ')}.</p>{/if}</div>
           {#if preview.accountOptions?.length}
             <fieldset class="border rounded p-3 mb-0"><legend class="float-none w-auto px-2 h6 mb-2">Accounts to include</legend><p class="text-body-secondary small mb-3">Only checked account rows are included in this preview and import.</p><div class="d-grid gap-2">{#each preview.accountOptions as option (option.name)}<label class="form-check"><input class="form-check-input" type="checkbox" checked={option.selected} onchange={(event) => updateAccountSelection(option.name, (event.currentTarget as HTMLInputElement).checked)} /><span class="form-check-label">{option.name} <span class="text-body-secondary">({option.sourceRowCount} source {option.sourceRowCount === 1 ? 'row' : 'rows'})</span></span></label>{/each}</div></fieldset>
           {/if}
          <div class="row g-3">{#each creationSummaries as summary (summary.label)}<div class="col-12 col-lg-4"><div class="border rounded p-3 h-100"><h3 class="h6">{summary.label}</h3><p class="mb-0 text-body-secondary">{summary.values.join(', ') || 'None'}</p></div></div>{/each}</div>
          {#if preview.rejectedRows.length || preview.duplicateRows.length}<div class="alert alert-warning mb-0" role="status"><strong>Some rows will not import.</strong> Rejected and duplicate rows below are excluded; confirmation only queues the remaining valid rows.</div>{/if}
          {#if preview.rejectedRows.length}<div><h3 class="h6">Rejected rows</h3><ul class="list-group">{#each preview.rejectedRows as issue (`rejected-${issue.rowNumber}-${issue.reason}`)}<li class="list-group-item">{issueLabel(issue)}</li>{/each}</ul></div>{/if}
          {#if preview.duplicateRows.length}<div><h3 class="h6">Duplicate rows</h3><ul class="list-group">{#each preview.duplicateRows as issue (`duplicate-${issue.rowNumber}-${issue.reason}`)}<li class="list-group-item">{issueLabel(issue)}</li>{/each}</ul></div>{/if}
           {#if !audit || !isTerminalAudit(audit)}<div><button class="btn btn-primary" type="button" onclick={() => void confirmImport()} disabled={confirming || updatingPreview || selectionDirty || preview.importableCount === 0}>{#if confirming}Confirming…{:else}Confirm valid rows{/if}</button>{#if preview.importableCount === 0}<p class="text-body-secondary small mb-0 mt-2">Select at least one account with importable rows before confirming.</p>{/if}</div>{/if}
          </div>
        {/if}

        {#if audit}
          <div class="d-grid gap-3">
          <div><h3 class="h5 mb-1">Import audit</h3><p class="text-body-secondary mb-0">Status {audit.status} · imported {audit.importedCount} rows · created {formatFinanceDateTime(audit.createdAt)}</p></div>
          {#if audit.status !== 'completed' && audit.status !== 'failed'}<div class="alert alert-info mb-0" role="status">Import is {audit.status}. This page refreshes progress automatically.</div>{/if}
          {#if audit.rejectedRows.length}<div><h3 class="h6">Final rejected rows</h3><ul class="list-group">{#each audit.rejectedRows as issue (`audit-${issue.rowNumber}-${issue.reason}`)}<li class="list-group-item">{issueLabel(issue)}</li>{/each}</ul></div>{/if}
          {#if audit.rowOutcomes.length}<div class="table-responsive"><table class="table align-middle mb-0"><thead><tr><th scope="col">Row</th><th scope="col">Outcome</th><th scope="col">Reason</th></tr></thead><tbody>{#each audit.rowOutcomes as outcome (outcome.rowNumber)}<tr><td>{outcome.rowNumber}</td><td>{outcome.status}</td><td>{outcome.reason}</td></tr>{/each}</tbody></table></div>{/if}
          <div class="d-flex flex-wrap gap-2"><button class="btn btn-outline-secondary" type="button" onclick={() => void refreshAudit(audit!.importId, !isTerminalAudit(audit!))}>Refresh audit</button>{#if audit.jobId}<a class="btn btn-outline-secondary" href={`/finance/jobs/${encodeURIComponent(audit.jobId)}`} use:link>Open finance job detail</a>{/if}</div>
          </div>
        {/if}
      </div></section>

      {#if financeShell.selectedTenantId}
        <section class="card shadow-sm"><div class="card-body p-4 d-grid gap-3">
          <div><p class="text-uppercase text-body-secondary fw-semibold small mb-2">Recent imports</p><h2 class="h5 mb-1">Reopen a durable import audit</h2><p class="text-body-secondary mb-0">Recent confirmed transaction imports for this tenant remain available after leaving this page.</p></div>
          {#if recentImports.length}
            <div class="list-group">{#each recentImports as recent (recent.importId)}<div class="list-group-item d-flex flex-column flex-md-row justify-content-between gap-2 align-items-md-center"><div><strong>{recent.status}</strong><p class="small text-body-secondary mb-0">Imported {recent.importedCount} rows · {formatFinanceDateTime(recent.createdAt)}</p></div><button class="btn btn-outline-secondary btn-sm align-self-start align-self-md-center" type="button" onclick={() => void reopenAudit(recent.importId)} disabled={openingAuditId === recent.importId} aria-current={audit?.importId === recent.importId ? 'true' : undefined}>{#if openingAuditId === recent.importId}Loading audit…{:else if audit?.importId === recent.importId}Audit open{:else}Open audit{/if}</button></div>{/each}</div>
          {:else}<p class="text-body-secondary mb-0" role="status">No confirmed transaction imports for this tenant yet.</p>{/if}
        </div></section>
      {/if}
    {/if}
  </div>
</section>
