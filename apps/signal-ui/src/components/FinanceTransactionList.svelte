<script lang="ts">
  import { tick } from 'svelte'
  import { link } from 'svelte-spa-router'
  import Check from '@lucide/svelte/icons/check'
  import FilePenLine from '@lucide/svelte/icons/file-pen-line'
  import Pencil from '@lucide/svelte/icons/pencil'
  import X from '@lucide/svelte/icons/x'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalFinanceApiForAuth,
    type FinanceCategory,
    type FinanceTag,
    type FinanceTransaction,
  } from '../lib/finance/api'
  import { formatFinanceDateTime, formatFinanceMoney } from '../lib/finance/format'

  type EditableField = 'description' | 'category' | 'tags'
  type CatalogLoadState = 'loading' | 'ready' | 'error'

  let {
    tenantId,
    transactions,
    accountNameById = new Map<string, string>(),
    ariaLabel = 'Transactions',
    onTransactionUpdated,
  }: {
    tenantId: string
    transactions: FinanceTransaction[]
    accountNameById?: Map<string, string>
    ariaLabel?: string
    onTransactionUpdated: (transaction: FinanceTransaction) => void
  } = $props()

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  let categories = $state<FinanceCategory[]>([])
  let tags = $state<FinanceTag[]>([])
  let categoryCatalogState = $state<CatalogLoadState>('loading')
  let tagCatalogState = $state<CatalogLoadState>('loading')
  let editing = $state<{ transactionId: string; field: EditableField } | null>(null)
  let descriptionDraft = $state('')
  let categoryDraft = $state('')
  let tagDraft = $state<string[]>([])
  let saving = $state(false)
  let error = $state<string | null>(null)
  let descriptionInput = $state<HTMLInputElement | null>(null)

  const categoryNameById = $derived(new Map(categories.map((category) => [category.id, category.name])))
  const tagNameById = $derived(new Map(tags.map((tag) => [tag.id, tag.name])))

  $effect(() => {
    void tenantId
    loadCatalog()
  })

  function loadCatalog() {
    void loadCategories()
    void loadTags()
  }

  async function loadCategories() {
    if (!tenantId) return
    categoryCatalogState = 'loading'
    try {
      categories = await financeApi.listCategories({ tenantId })
      categoryCatalogState = 'ready'
    } catch {
      categoryCatalogState = 'error'
    }
  }

  async function loadTags() {
    if (!tenantId) return
    tagCatalogState = 'loading'
    try {
      tags = await financeApi.listTags({ tenantId })
      tagCatalogState = 'ready'
    } catch {
      tagCatalogState = 'error'
    }
  }

  function categoryName(categoryId: string | null | undefined): string {
    if (!categoryId) return 'No category'
    return categoryNameById.get(categoryId) ?? 'Unknown category'
  }

  function tagNames(tagIds: string[]): string[] {
    return tagIds.map((tagId) => tagNameById.get(tagId) ?? 'Unknown tag')
  }

  function transferBadgeLabel(item: FinanceTransaction): string | null {
    if (item.transferGroupId) return 'internal transfer'
    if (item.kind === 'transfer') return 'transfer'
    return null
  }

  function statusClass(item: FinanceTransaction): string {
    return item.status === 'pending' ? 'text-bg-warning' : 'text-bg-success'
  }

  async function startEdit(item: FinanceTransaction, field: EditableField) {
    if (field === 'category' && categoryCatalogState !== 'ready') return
    if (field === 'tags' && tagCatalogState !== 'ready') return
    error = null
    editing = { transactionId: item.id, field }
    descriptionDraft = item.description
    categoryDraft = item.categoryId ?? ''
    tagDraft = [...(item.tagIds ?? [])]
    if (field === 'description') {
      await tick()
      descriptionInput?.focus()
    }
  }

  function cancelEdit() {
    editing = null
    error = null
  }

  function handleDescriptionKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      event.preventDefault()
      cancelEdit()
      return
    }

    // Let the IME commit its candidate before a form submission is possible.
    if (event.key === 'Enter' && event.isComposing) event.preventDefault()
  }

  function toggleTag(tagId: string, checked: boolean) {
    tagDraft = checked ? [...tagDraft, tagId] : tagDraft.filter((id) => id !== tagId)
  }

  async function saveEdit(item: FinanceTransaction) {
    if (!editing || saving) return
    if (editing.field === 'category' && categoryCatalogState !== 'ready') {
      error = 'Categories are unavailable. Retry loading the catalog before saving.'
      return
    }
    if (editing.field === 'tags' && tagCatalogState !== 'ready') {
      error = 'Tags are unavailable. Retry loading the catalog before saving.'
      return
    }
    saving = true
    error = null
    try {
      const updated = await financeApi.updateTransaction({
        tenantId,
        transactionId: item.id,
        description: editing.field === 'description' ? descriptionDraft : item.description,
        amountMinor: item.amountMinor,
        effectiveAt: item.effectiveAt,
        categoryId: editing.field === 'category' ? (categoryDraft || null) : (item.categoryId ?? null),
        tagIds: editing.field === 'tags' ? tagDraft : (item.tagIds ?? []),
      })
      onTransactionUpdated(updated)
      editing = null
    } catch (saveError) {
      error = saveError instanceof Error ? saveError.message : 'Could not save this transaction change.'
    } finally {
      saving = false
    }
  }
</script>

<div class="list-group" aria-label={ariaLabel} aria-busy={saving}>
  {#each transactions as item (item.id)}
    <article class="list-group-item p-2 p-sm-3">
      <div class="d-flex justify-content-between gap-3 align-items-start">
        <div class="flex-grow-1 min-w-0">
          <div class="d-flex flex-wrap gap-2 align-items-baseline">
            <strong>{formatFinanceMoney(item.amountMinor, item.currency)}</strong>
            <span class="small text-body-secondary">{formatFinanceDateTime(item.effectiveAt)}</span>
            <span class="small text-body-secondary">{accountNameById.get(item.accountId) ?? 'Unknown account'}</span>
            {#if item.status !== 'booked'}<span class={`badge ${statusClass(item)}`}>{item.status}</span>{/if}
            {#if item.kind !== 'regular' && item.kind !== 'transfer'}<span class="badge text-bg-secondary">{item.kind}</span>{/if}
            {#if item.hiddenAt}<span class="badge text-bg-secondary">hidden</span>{/if}
            {#if transferBadgeLabel(item)}<span class="badge text-bg-secondary">{transferBadgeLabel(item)}</span>{/if}
          </div>

          <div class="mt-1">
            {#if editing?.transactionId === item.id && editing.field === 'description'}
              <form class="input-group input-group-sm" onsubmit={(event) => { event.preventDefault(); void saveEdit(item) }}>
                <label class="visually-hidden" for={`transaction-description-${item.id}`}>Description</label>
                <input id={`transaction-description-${item.id}`} class="form-control" bind:this={descriptionInput} bind:value={descriptionDraft} disabled={saving} onkeydown={handleDescriptionKeydown} />
                <button class="btn btn-outline-success finance-transaction-list-action" type="submit" disabled={saving} aria-label="Save description" title="Save description"><Check size={16} /></button>
                <button class="btn btn-outline-secondary finance-transaction-list-action" type="button" onclick={cancelEdit} disabled={saving} aria-label="Cancel description edit" title="Cancel description edit"><X size={16} /></button>
              </form>
            {:else}
              <div class="d-flex gap-1 align-items-center">
                <span class="fw-semibold">{item.description || item.kind}</span>
                <button class="btn btn-outline-secondary btn-sm finance-transaction-list-action" type="button" onclick={() => startEdit(item, 'description')} aria-label="Edit description" title="Edit description"><Pencil size={14} /></button>
              </div>
            {/if}
          </div>
        </div>

        <a class="btn btn-outline-primary btn-sm finance-transaction-list-action" href={`/finance/transactions/${encodeURIComponent(item.id)}`} use:link aria-label="Open full transaction details" title="Open full transaction details"><FilePenLine size={16} /></a>
      </div>

      <div class="d-flex flex-column flex-md-row gap-2 mt-2">
        <div class="d-flex flex-grow-1 gap-2 align-items-start">
          <div class="flex-grow-1 min-w-0 d-grid gap-2">
            {#if editing?.transactionId === item.id && editing.field === 'category'}
              <div class="finance-transaction-list-editor-row d-flex flex-nowrap gap-2 align-items-center">
                <label class="visually-hidden" for={`transaction-category-${item.id}`}>Category</label>
                <select id={`transaction-category-${item.id}`} class="form-select form-select-sm finance-transaction-list-editor flex-grow-1" bind:value={categoryDraft} disabled={saving}>
                  <option value="">No category</option>
                  {#each categories as category (category.id)}<option value={category.id}>{category.name}</option>{/each}
                </select>
                <span class="btn-group btn-group-sm flex-shrink-0">
                  <button class="btn btn-outline-success finance-transaction-list-action" type="button" onclick={() => void saveEdit(item)} disabled={saving} aria-label="Save category" title="Save category"><Check size={14} /></button>
                  <button class="btn btn-outline-secondary finance-transaction-list-action" type="button" onclick={cancelEdit} disabled={saving} aria-label="Cancel category edit" title="Cancel category edit"><X size={14} /></button>
                </span>
              </div>
            {:else}
              <div class="d-flex flex-wrap gap-2 align-items-center">
                <span class="min-w-0">{categoryName(item.categoryId)}</span>
                {#if categoryCatalogState === 'ready'}
                  <button class="btn btn-outline-secondary btn-sm finance-transaction-list-action flex-shrink-0" type="button" onclick={() => startEdit(item, 'category')} aria-label="Edit category" title="Edit category"><Pencil size={14} /></button>
                {/if}
              </div>
            {/if}
            {#if categoryCatalogState === 'loading'}
              <div class="small text-body-secondary mt-2">Loading categories…</div>
            {:else if categoryCatalogState === 'error'}
              <div class="alert alert-warning py-2 px-3 mt-2 mb-0 small" role="alert">Categories could not load. <button class="btn btn-link btn-sm p-0 align-baseline" type="button" onclick={() => void loadCategories()}>Retry category catalog</button></div>
            {/if}
          </div>
        </div>

        <div class="d-flex flex-grow-1 gap-2 align-items-start">
          <div class="flex-grow-1 min-w-0 d-grid gap-2">
            {#if editing?.transactionId === item.id && editing.field === 'tags'}
              <div class="finance-transaction-list-editor-row d-flex flex-nowrap gap-2 align-items-start">
                <fieldset class="finance-transaction-list-editor flex-grow-1 mb-0" disabled={saving}>
                  <legend class="visually-hidden">Tags</legend>
                  <div class="finance-transaction-list-tag-choices d-flex flex-wrap gap-2">
                    {#each tags as tag (tag.id)}
                      <label class="form-check form-check-inline mb-0"><input class="form-check-input" type="checkbox" checked={tagDraft.includes(tag.id)} onchange={(event) => toggleTag(tag.id, event.currentTarget.checked)} /> <span class="form-check-label">{tag.name}</span></label>
                    {/each}
                  </div>
                </fieldset>
                <span class="btn-group btn-group-sm flex-shrink-0">
                  <button class="btn btn-outline-success finance-transaction-list-action" type="button" onclick={() => void saveEdit(item)} disabled={saving} aria-label="Save tags" title="Save tags"><Check size={14} /></button>
                  <button class="btn btn-outline-secondary finance-transaction-list-action" type="button" onclick={cancelEdit} disabled={saving} aria-label="Cancel tags edit" title="Cancel tags edit"><X size={14} /></button>
                </span>
              </div>
            {:else}
              <div class="d-flex flex-wrap gap-2 align-items-center">
                {#if (item.tagIds ?? []).length}
                  <div class="d-flex flex-wrap gap-1 min-w-0">{#each tagNames(item.tagIds ?? []) as name, index (`${item.id}-${index}`)}<span class="badge text-bg-secondary">{name}</span>{/each}</div>
                {:else}
                  <span class="text-body-secondary">No tags</span>
                {/if}
                {#if tagCatalogState === 'ready'}
                  <button class="btn btn-outline-secondary btn-sm finance-transaction-list-action flex-shrink-0" type="button" onclick={() => startEdit(item, 'tags')} aria-label="Edit tags" title="Edit tags"><Pencil size={14} /></button>
                {/if}
              </div>
            {/if}
            {#if tagCatalogState === 'loading'}
              <div class="small text-body-secondary mt-2">Loading tags…</div>
            {:else if tagCatalogState === 'error'}
              <div class="alert alert-warning py-2 px-3 mt-2 mb-0 small" role="alert">Tags could not load. <button class="btn btn-link btn-sm p-0 align-baseline" type="button" onclick={() => void loadTags()}>Retry tag catalog</button></div>
            {/if}
          </div>
        </div>
      </div>
      {#if error && editing?.transactionId === item.id}<div class="alert alert-danger py-2 px-3 mt-3 mb-0" role="alert">{error}</div>{/if}
    </article>
  {/each}
</div>
