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
    await financeApi.createCategory({
      tenantId: financeShell.selectedTenantId,
      name: categoryName,
      kind: categoryKind,
    })
    categoryName = ''
    await loadCatalogs()
  }

  async function createTag(event: SubmitEvent) {
    event.preventDefault()
    if (!financeShell.selectedTenantId) return
    await financeApi.createTag({ tenantId: financeShell.selectedTenantId, name: tagName })
    tagName = ''
    await loadCatalogs()
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

<section class="page" aria-labelledby="finance-categories-heading">
  <header>
    <h1 id="finance-categories-heading">Finance categories and tags</h1>
    <p class="muted">Manage tenant-local reporting categories and transaction tags.</p>
  </header>

  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}

  {#if loading}
    <p class="muted" role="status">Loading categories…</p>
  {:else}
    {#if !financeShell.embedded || financeShell.needsTenantSelection || !financeShell.selectedTenantId}
      <section class="panel stack">
        {#if !financeShell.embedded}
          <label>
            <span>Tenant</span>
            <select
              value={financeShell.selectedTenantId}
              onchange={(event) => financeShell.selectTenant((event.currentTarget as HTMLSelectElement).value)}
              aria-label="Tenant"
            >
              <option value="">Select tenant</option>
              {#each financeShell.tenants as tenant (tenant.id)}
                <option value={tenant.id}>{tenant.name}</option>
              {/each}
            </select>
          </label>
        {/if}

        {#if financeShell.needsTenantSelection}
          <p>Select an active tenant to continue on this finance route.</p>
        {:else if !financeShell.selectedTenantId}
          <p>Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a> before managing categories.</p>
        {/if}
      </section>
    {/if}

    <div class="grid">
      <form class="panel" onsubmit={createCategory}>
        <h2>Create category</h2>
        <label>
          <span>Name</span>
          <input bind:value={categoryName} aria-label="Category name" required />
        </label>
        <label>
          <span>Kind</span>
          <select bind:value={categoryKind} aria-label="Category kind">
            <option value="expense">expense</option>
            <option value="income">income</option>
          </select>
        </label>
        <button class="primary" type="submit" disabled={!financeShell.selectedTenantId}>
          Create category
        </button>
        <div class="stack">
          {#each categories as category (category.id)}
            <article class="card"><strong>{category.name}</strong><span>{category.kind}</span></article>
          {:else}
            <p class="muted">No categories yet.</p>
          {/each}
        </div>
      </form>

      <form class="panel" onsubmit={createTag}>
        <h2>Create tag</h2>
        <label>
          <span>Name</span>
          <input bind:value={tagName} aria-label="Tag name" required />
        </label>
        <button class="primary" type="submit" disabled={!financeShell.selectedTenantId}>
          Create tag
        </button>
        <div class="stack">
          {#each tags as tag (tag.id)}
            <article class="card"><strong>{tag.name}</strong></article>
          {:else}
            <p class="muted">No tags yet.</p>
          {/each}
        </div>
      </form>
    </div>
  {/if}
</section>

<style>
  .page,
  .stack {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
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

  .card {
    display: flex;
    justify-content: space-between;
    gap: var(--space-12);
    padding: var(--space-12);
    border: 1px solid var(--border);
    border-radius: 4px;
  }

  .panel h2,
  header h1 {
    margin: 0;
  }

  .muted {
    margin: 0;
    color: var(--text-muted);
  }

  label {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }

  .error {
    color: var(--color-danger-red);
  }
</style>
