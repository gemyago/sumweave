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
  let addingCategory = $state(false)
  let addingTag = $state(false)
  let editingCategoryId = $state<string | null>(null)
  let categoryEditName = $state('')
  let categoryEditKind = $state('expense')
  let editingTagId = $state<string | null>(null)
  let tagEditName = $state('')
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
      loadCategories(),
      loadTags(),
    ])
  }

  async function loadCategories() {
    if (!financeShell.selectedTenantId) return []
    return financeApi.listCategories({ tenantId: financeShell.selectedTenantId })
  }

  async function loadTags() {
    if (!financeShell.selectedTenantId) return []
    return financeApi.listTags({ tenantId: financeShell.selectedTenantId })
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
      addingCategory = false
      categories = await loadCategories()
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
      addingTag = false
      tags = await loadTags()
    } catch (createError) {
      error = createError instanceof Error ? createError.message : 'Failed to create tag'
    }
  }

  function startCategoryEdit(category: FinanceCategory) {
    error = null
    editingCategoryId = category.id
    categoryEditName = category.name
    categoryEditKind = category.kind
  }

  function cancelCategoryEdit() {
    editingCategoryId = null
    categoryEditName = ''
    categoryEditKind = 'expense'
  }

  async function updateCategory(event: SubmitEvent, categoryId: string) {
    event.preventDefault()
    if (!financeShell.selectedTenantId) return
    error = null

    try {
      await financeApi.updateCategory({
        tenantId: financeShell.selectedTenantId,
        categoryId,
        name: categoryEditName,
        kind: categoryEditKind,
      })
      cancelCategoryEdit()
      categories = await loadCategories()
    } catch (updateError) {
      error = updateError instanceof Error ? updateError.message : 'Failed to update category'
    }
  }

  function startTagEdit(tag: FinanceTag) {
    error = null
    editingTagId = tag.id
    tagEditName = tag.name
  }

  function cancelTagEdit() {
    editingTagId = null
    tagEditName = ''
  }

  async function renameTag(event: SubmitEvent, tagId: string) {
    event.preventDefault()
    if (!financeShell.selectedTenantId) return
    error = null

    try {
      await financeApi.renameTag({
        tenantId: financeShell.selectedTenantId,
        tagId,
        name: tagEditName,
      })
      cancelTagEdit()
      tags = await loadTags()
    } catch (renameError) {
      error = renameError instanceof Error ? renameError.message : 'Failed to update tag'
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
          Manage tenant-local reporting categories and tags with on-demand add forms and inline editing.
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

      <section class="card shadow-sm" aria-labelledby="finance-categories-list-heading">
        <div class="card-body p-4 d-grid gap-3">
          <div class="d-flex flex-wrap justify-content-between align-items-start gap-3">
            <div>
              <h2 id="finance-categories-list-heading" class="h5 mb-1">Categories</h2>
              <p class="text-body-secondary mb-0">Keep income and expense reporting labels distinct.</p>
            </div>
            <button class="btn btn-primary text-nowrap" type="button" onclick={() => { addingCategory = true }} disabled={!financeShell.selectedTenantId}>
              Add category
            </button>
          </div>

          {#if addingCategory}
            <form class="border rounded p-3 d-grid gap-3" onsubmit={createCategory}>
              <div>
                <label class="form-label" for="finance-category-name">Category name</label>
                <input id="finance-category-name" class="form-control" bind:value={categoryName} required />
              </div>
              <div>
                <label class="form-label" for="finance-category-kind">Kind</label>
                <select id="finance-category-kind" class="form-select" bind:value={categoryKind}>
                  <option value="expense">expense</option>
                  <option value="income">income</option>
                </select>
              </div>
              <div class="d-flex flex-wrap gap-2">
                <button class="btn btn-primary" type="submit">Save category</button>
                <button class="btn btn-outline-secondary" type="button" onclick={() => { addingCategory = false; categoryName = '' }}>Cancel</button>
              </div>
            </form>
          {/if}

          {#if categories.length === 0}
            <div class="alert alert-light border mb-0" role="status">No categories yet.</div>
          {:else}
            <div class="list-group">
              {#each categories as category (category.id)}
                <article class="list-group-item">
                  {#if editingCategoryId === category.id}
                    <form class="d-grid gap-2" onsubmit={(event) => updateCategory(event, category.id)}>
                      <label class="form-label mb-0" for={`finance-category-edit-name-${category.id}`}>Category name</label>
                      <input id={`finance-category-edit-name-${category.id}`} class="form-control" bind:value={categoryEditName} required />
                      <div>
                        <label class="form-label mb-0" for={`finance-category-edit-kind-${category.id}`}>Type</label>
                        <select id={`finance-category-edit-kind-${category.id}`} class="form-select" bind:value={categoryEditKind}>
                          <option value="expense">expense</option>
                          <option value="income">income</option>
                        </select>
                      </div>
                      <div class="d-flex flex-wrap gap-2">
                        <button class="btn btn-primary btn-sm" type="submit">Save</button>
                        <button class="btn btn-outline-secondary btn-sm" type="button" onclick={cancelCategoryEdit}>Cancel</button>
                      </div>
                    </form>
                  {:else}
                    <div class="d-flex flex-column flex-sm-row justify-content-between align-items-sm-center gap-2">
                      <div class="d-flex flex-wrap align-items-center gap-2">
                        <strong>{category.name}</strong>
                        <span class="badge text-bg-secondary">{category.kind}</span>
                        {#if category.seededDefault}<span class="badge text-bg-light border">Starter default</span>{/if}
                      </div>
                      <button class="btn btn-outline-secondary btn-sm" type="button" onclick={() => startCategoryEdit(category)}>Edit {category.name}</button>
                    </div>
                  {/if}
                </article>
              {/each}
            </div>
          {/if}
        </div>
      </section>

      <section class="card shadow-sm" aria-labelledby="finance-tags-list-heading">
        <div class="card-body p-4 d-grid gap-3">
          <div class="d-flex flex-wrap justify-content-between align-items-start gap-3">
            <div>
              <h2 id="finance-tags-list-heading" class="h5 mb-1">Tags</h2>
              <p class="text-body-secondary mb-0">Create lightweight transaction tags without leaving this route.</p>
            </div>
            <button class="btn btn-primary text-nowrap" type="button" onclick={() => { addingTag = true }} disabled={!financeShell.selectedTenantId}>
              Add tag
            </button>
          </div>

          {#if addingTag}
            <form class="border rounded p-3 d-grid gap-3" onsubmit={createTag}>
              <div>
                <label class="form-label" for="finance-tag-name">Tag name</label>
                <input id="finance-tag-name" class="form-control" bind:value={tagName} required />
              </div>
              <div class="d-flex flex-wrap gap-2">
                <button class="btn btn-primary" type="submit">Save tag</button>
                <button class="btn btn-outline-secondary" type="button" onclick={() => { addingTag = false; tagName = '' }}>Cancel</button>
              </div>
            </form>
          {/if}

          {#if tags.length === 0}
            <div class="alert alert-light border mb-0" role="status">No tags yet.</div>
          {:else}
            <div class="list-group">
              {#each tags as tag (tag.id)}
                <article class="list-group-item">
                  {#if editingTagId === tag.id}
                    <form class="d-grid gap-2" onsubmit={(event) => renameTag(event, tag.id)}>
                      <label class="form-label mb-0" for={`finance-tag-edit-name-${tag.id}`}>Tag name</label>
                      <input id={`finance-tag-edit-name-${tag.id}`} class="form-control" bind:value={tagEditName} required />
                      <div class="d-flex flex-wrap gap-2">
                        <button class="btn btn-primary btn-sm" type="submit">Save</button>
                        <button class="btn btn-outline-secondary btn-sm" type="button" onclick={cancelTagEdit}>Cancel</button>
                      </div>
                    </form>
                  {:else}
                    <div class="d-flex flex-column flex-sm-row justify-content-between align-items-sm-center gap-2">
                      <strong>{tag.name}</strong>
                      <button class="btn btn-outline-secondary btn-sm" type="button" onclick={() => startTagEdit(tag)}>Edit {tag.name}</button>
                    </div>
                  {/if}
                </article>
              {/each}
            </div>
          {/if}
        </div>
      </section>
    {/if}
  </div>
</section>
