<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalFinanceApiForAuth,
    type FinanceCategory,
    type FinanceTag,
  } from '../lib/finance/api'
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() =>
    createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }),
  )
  const financeShell = useFinanceShellState()

  let loading = $state(true)
  let error = $state<string | null>(null)
  let categories = $state<FinanceCategory[]>([])
  let tags = $state<FinanceTag[]>([])
  let categoryName = $state('')
  let categoryKind = $state('expense')
  let tagName = $state('')
  let reactiveReady = $state(false)
  let skipNextReactiveLoad = false

  onMount(() => {
    void loadPage()
  })

  async function loadPage() {
    loading = true
    reactiveReady = false
    error = null

    try {
      await financeShell.initialize()
      await loadCatalogs()
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load categories'
    } finally {
      skipNextReactiveLoad = true
      reactiveReady = true
      loading = false
    }
  }

  async function loadCatalogs() {
    if (!financeShell.selectedTenantId) {
      categories = []
      tags = []
      return
    }

    ;[categories, tags] = await Promise.all([
      financeApi.listCategories({ tenantId: financeShell.selectedTenantId }),
      financeApi.listTags({ tenantId: financeShell.selectedTenantId }),
    ])
  }

  async function createCategory(event: SubmitEvent) {
    event.preventDefault()
    if (!financeShell.selectedTenantId) return
    error = null

    try {
      await financeApi.createCategory({
        tenantId: financeShell.selectedTenantId,
        name: categoryName,
        kind: categoryKind,
      })
      categoryName = ''
      await loadCatalogs()
    } catch (createError) {
      error = createError instanceof Error ? createError.message : 'Failed to create category'
    }
  }

  async function createTag(event: SubmitEvent) {
    event.preventDefault()
    if (!financeShell.selectedTenantId) return
    error = null

    try {
      await financeApi.createTag({ tenantId: financeShell.selectedTenantId, name: tagName })
      tagName = ''
      await loadCatalogs()
    } catch (createError) {
      error = createError instanceof Error ? createError.message : 'Failed to create tag'
    }
  }

  $effect(() => {
    if (financeShell.loading || !reactiveReady) return
    void financeShell.selectedTenantId
    if (skipNextReactiveLoad) {
      skipNextReactiveLoad = false
      return
    }
    void loadCatalogs()
  })
</script>

<section class="container-fluid px-0" aria-labelledby="finance-categories-heading">
  <div class="d-grid gap-4">
    <header class="card border-0 shadow-sm">
      <div class="card-body p-4 p-xl-5">
        <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Categories and tags</p>
        <h1 id="finance-categories-heading" class="h3 mb-2">Finance categories</h1>
        <p class="text-body-secondary mb-0">
          Manage tenant-local reporting categories and tags with dedicated create forms and list views.
        </p>
      </div>
    </header>

    {#if error}
      <div class="alert alert-danger mb-0" role="alert">{error}</div>
    {/if}

    {#if loading}
      <div class="alert alert-secondary mb-0" role="status">Loading categories…</div>
    {:else}
      {#if !financeShell.embedded || financeShell.needsTenantSelection || !financeShell.selectedTenantId}
        <section class="card shadow-sm">
          <div class="card-body p-4 d-grid gap-3">
            {#if !financeShell.embedded}
              <div class="col-12 col-lg-5 px-0">
                <label class="form-label" for="finance-categories-tenant">Tenant</label>
                <select
                  id="finance-categories-tenant"
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

            {#if financeShell.needsTenantSelection}
              <div class="alert alert-warning mb-0" role="status">Select an active tenant to continue on this finance route.</div>
            {:else if !financeShell.selectedTenantId}
              <div class="alert alert-light border mb-0" role="status">
                Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a> before managing categories.
              </div>
            {/if}
          </div>
        </section>
      {/if}

      <div class="row g-4">
        <div class="col-12 col-xl-6">
          <form class="card shadow-sm h-100" onsubmit={createCategory}>
            <div class="card-body p-4 d-grid gap-3">
              <div>
                <h2 class="h5 mb-1">Categories</h2>
                <p class="text-body-secondary mb-0">Create reporting categories and keep income and expense labels distinct.</p>
              </div>

              <div>
                <label class="form-label" for="finance-category-name">Name</label>
                <input id="finance-category-name" class="form-control" bind:value={categoryName} aria-label="Category name" required />
              </div>

              <div>
                <label class="form-label" for="finance-category-kind">Kind</label>
                <select id="finance-category-kind" class="form-select" bind:value={categoryKind} aria-label="Category kind">
                  <option value="expense">expense</option>
                  <option value="income">income</option>
                </select>
              </div>

              <div>
                <button class="btn btn-primary" type="submit" disabled={!financeShell.selectedTenantId}>
                  Create category
                </button>
              </div>

              {#if categories.length === 0}
                <div class="alert alert-light border mb-0" role="status">No categories yet.</div>
              {:else}
                <div class="list-group">
                  {#each categories as category (category.id)}
                    <article class="list-group-item d-flex justify-content-between gap-3 align-items-center">
                      <strong>{category.name}</strong>
                      <span class="badge text-bg-secondary">{category.kind}</span>
                    </article>
                  {/each}
                </div>
              {/if}
            </div>
          </form>
        </div>

        <div class="col-12 col-xl-6">
          <form class="card shadow-sm h-100" onsubmit={createTag}>
            <div class="card-body p-4 d-grid gap-3">
              <div>
                <h2 class="h5 mb-1">Tags</h2>
                <p class="text-body-secondary mb-0">Create lightweight transaction tags without leaving the category route.</p>
              </div>

              <div>
                <label class="form-label" for="finance-tag-name">Name</label>
                <input id="finance-tag-name" class="form-control" bind:value={tagName} aria-label="Tag name" required />
              </div>

              <div>
                <button class="btn btn-primary" type="submit" disabled={!financeShell.selectedTenantId}>
                  Create tag
                </button>
              </div>

              {#if tags.length === 0}
                <div class="alert alert-light border mb-0" role="status">No tags yet.</div>
              {:else}
                <div class="list-group">
                  {#each tags as tag (tag.id)}
                    <article class="list-group-item">
                      <strong>{tag.name}</strong>
                    </article>
                  {/each}
                </div>
              {/if}
            </div>
          </form>
        </div>
      </div>
    {/if}
  </div>
</section>
