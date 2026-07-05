<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalFinanceApiForAuth,
    type FinanceCSVImportAudit,
    type FinanceCSVImportPreview,
  } from '../lib/finance/api'
  import { formatFinanceDateTime } from '../lib/finance/format'
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const financeShell = useFinanceShellState()

  let loading = $state(true)
  let previewing = $state(false)
  let confirming = $state(false)
  let error = $state<string | null>(null)
  let importType = $state('transactions')
  let fileName = $state('sample.csv')
  let csv = $state('account,amount\nChecking,100')
  let preview = $state<FinanceCSVImportPreview | null>(null)
  let mapping = $state<Record<string, string>>({})
  let audit = $state<FinanceCSVImportAudit | null>(null)

  onMount(() => {
    void loadPage()
  })

  async function loadPage() {
    loading = true
    error = null

    try {
      await financeShell.initialize()
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load imports'
    } finally {
      loading = false
    }
  }

  async function previewImport(event: SubmitEvent) {
    event.preventDefault()
    if (!financeShell.selectedTenantId) return

    previewing = true
    error = null
    audit = null

    try {
      preview = await financeApi.previewCSVImport({
        tenantId: financeShell.selectedTenantId,
        importType,
        fileName,
        csv,
      })
      mapping = { ...preview.mapping }
    } catch (previewError) {
      error = previewError instanceof Error ? previewError.message : 'Failed to preview import'
      preview = null
      mapping = {}
    } finally {
      previewing = false
    }
  }

  async function confirmImport() {
    if (!preview || !financeShell.selectedTenantId) return

    confirming = true
    error = null

    try {
      const confirmation = await financeApi.confirmCSVImport({
        tenantId: financeShell.selectedTenantId,
        importId: preview.importId,
        mapping,
      })
      audit = await financeApi.getCSVImportAudit({
        tenantId: financeShell.selectedTenantId,
        importId: confirmation.importId,
      })
    } catch (confirmError) {
      error = confirmError instanceof Error ? confirmError.message : 'Failed to confirm import'
      audit = null
    } finally {
      confirming = false
    }
  }
</script>

<section class="container-fluid px-0" aria-labelledby="finance-imports-heading">
  <div class="d-grid gap-4">
    <header class="card border-0 shadow-sm">
      <div class="card-body p-4 p-xl-5">
        <p class="text-uppercase text-body-secondary fw-semibold small mb-2">CSV imports</p>
        <h1 id="finance-imports-heading" class="h3 mb-2">Finance imports</h1>
        <p class="text-body-secondary mb-0">
          Preview CSV input, confirm mapping, and follow the durable finance import job from one route.
        </p>
      </div>
    </header>

    {#if error}
      <div class="alert alert-danger mb-0" role="alert">{error}</div>
    {/if}

    {#if loading}
      <div class="alert alert-secondary mb-0" role="status">Loading imports…</div>
    {:else}
      {#if !financeShell.embedded || financeShell.needsTenantSelection || !financeShell.selectedTenantId}
        <section class="card shadow-sm">
          <div class="card-body p-4 d-grid gap-3">
            {#if !financeShell.embedded}
              <div class="col-12 col-lg-5 px-0">
                <label class="form-label" for="finance-imports-tenant">Tenant</label>
                <select
                  id="finance-imports-tenant"
                  class="form-select"
                  value={financeShell.selectedTenantId}
                  onchange={(event) => financeShell.selectTenant((event.currentTarget as HTMLSelectElement).value)}
                  aria-label="Tenant"
                >
                  <option value="">Select tenant</option>
                  {#each financeShell.tenants as tenant (tenant.id)}
                    <option value={tenant.id}>{tenant.name}</option>
                  {/each}
                </select>
              </div>
            {/if}

            {#if financeShell.needsTenantSelection}
              <div class="alert alert-warning mb-0" role="status">Select an active tenant to continue on this finance route.</div>
            {:else if !financeShell.selectedTenantId}
              <div class="alert alert-light border mb-0" role="status">
                Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a> before importing CSV data.
              </div>
            {/if}
          </div>
        </section>
      {/if}

      <form class="card shadow-sm" onsubmit={previewImport}>
        <div class="card-body p-4 d-grid gap-4">
          <div>
            <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Step 1</p>
            <h2 class="h5 mb-1">Preview import</h2>
            <p class="text-body-secondary mb-0">Choose the import type, provide a file name, and preview the parsed CSV headers.</p>
          </div>

          <div class="row g-3">
            <div class="col-12 col-md-4">
              <label class="form-label" for="finance-import-type">Import type</label>
              <select id="finance-import-type" class="form-select" bind:value={importType} aria-label="Import type">
                <option value="transactions">transactions</option>
                <option value="accounts">accounts</option>
              </select>
            </div>

            <div class="col-12 col-md-8">
              <label class="form-label" for="finance-import-file-name">File name</label>
              <input id="finance-import-file-name" class="form-control" bind:value={fileName} aria-label="Import file name" required />
            </div>

            <div class="col-12">
              <label class="form-label" for="finance-import-csv">CSV</label>
              <textarea id="finance-import-csv" class="form-control" bind:value={csv} aria-label="Import CSV" rows="8" required></textarea>
            </div>
          </div>

          <div>
            <button class="btn btn-primary" type="submit" disabled={!financeShell.selectedTenantId || previewing}>
              {#if previewing}Previewing…{:else}Preview import{/if}
            </button>
          </div>
        </div>
      </form>

      {#if preview}
        <section class="card shadow-sm">
          <div class="card-body p-4 d-grid gap-4">
            <div>
              <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Step 2</p>
              <h2 class="h5 mb-1">Preview result</h2>
              <p class="text-body-secondary mb-0">Import {preview.importId} · headers {preview.headers.join(', ') || '—'}</p>
            </div>

            <div class="row g-4">
              <div class="col-12 col-xl-7">
                <div class="table-responsive">
                  <table class="table align-middle mb-0">
                    <thead>
                      <tr>
                        <th scope="col">Header</th>
                        <th scope="col">Mapped field</th>
                      </tr>
                    </thead>
                    <tbody>
                      {#each Object.entries(mapping) as [header, field] (header)}
                        <tr>
                          <td><strong>{header}</strong></td>
                          <td>
                            <input
                              class="form-control"
                              value={field}
                              oninput={(event) => {
                                mapping = {
                                  ...mapping,
                                  [header]: (event.currentTarget as HTMLInputElement).value,
                                }
                              }}
                              aria-label={`Mapping ${header}`}
                            />
                          </td>
                        </tr>
                      {/each}
                    </tbody>
                  </table>
                </div>
              </div>

              <div class="col-12 col-xl-5 d-grid gap-3">
                <div class="border rounded-3 p-3 bg-body-tertiary">
                  <h3 class="h6 mb-2">Would create accounts</h3>
                  <p class="mb-0 text-body-secondary">{preview.wouldCreateAccounts.join(', ') || '—'}</p>
                </div>
                <div class="border rounded-3 p-3 bg-body-tertiary">
                  <h3 class="h6 mb-2">Would create categories</h3>
                  <p class="mb-0 text-body-secondary">{preview.wouldCreateCategories.join(', ') || '—'}</p>
                </div>
                <div class="border rounded-3 p-3 bg-body-tertiary">
                  <h3 class="h6 mb-2">Would create tags</h3>
                  <p class="mb-0 text-body-secondary">{preview.wouldCreateTags.join(', ') || '—'}</p>
                </div>
              </div>
            </div>

            <div>
              <button class="btn btn-primary" type="button" onclick={() => void confirmImport()} disabled={confirming}>
                {#if confirming}Confirming…{:else}Confirm import{/if}
              </button>
            </div>
          </div>
        </section>
      {/if}

      {#if audit}
        <section class="card shadow-sm">
          <div class="card-body p-4 d-grid gap-3">
            <div>
              <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Step 3</p>
              <h2 class="h5 mb-1">Import audit</h2>
              <p class="text-body-secondary mb-0">
                Status {audit.status} · imported {audit.importedCount} rows · created {formatFinanceDateTime(audit.createdAt)}
              </p>
            </div>

            <div>
              <a class="btn btn-outline-secondary" href={`/finance/jobs/${encodeURIComponent(audit.jobId)}`} use:link>
                Open finance job detail
              </a>
            </div>
          </div>
        </section>
      {/if}
    {/if}
  </div>
</section>
