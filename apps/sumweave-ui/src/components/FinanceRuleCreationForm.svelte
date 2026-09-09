<script lang="ts">
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalFinanceApiForAuth,
    type FinanceCategory,
    type FinanceClassificationMatchType,
    type FinanceTag,
  } from '../lib/finance/api'

  let {
    tenantId,
    offerId,
    description,
    categoryId,
    categories,
    tags,
    initialTagIds,
    tagCatalogState,
    onRetryTags,
    onCancel,
    onSaved,
  }: {
    tenantId: string
    offerId: number
    description: string
    categoryId: string
    categories: FinanceCategory[]
    tags: FinanceTag[]
    initialTagIds: string[]
    tagCatalogState: 'loading' | 'ready' | 'error'
    onRetryTags?: () => void
    onCancel: () => void
    onSaved: () => void
  } = $props()

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  let matchType = $state<FinanceClassificationMatchType>('contains')
  let condition = $state('')
  let targetCategoryId = $state('')
  let selectedTagIds = $state<string[]>([])
  let saving = $state(false)
  let error = $state<string | null>(null)
  let initializedOfferId = $state<number | null>(null)

  $effect(() => {
    if (offerId === initializedOfferId) return
    initializedOfferId = offerId
    matchType = 'contains'
    condition = description
    targetCategoryId = categoryId
    selectedTagIds = [...initialTagIds]
  })

  const tagById = $derived(new Map(tags.map((tag) => [tag.id, tag])))
  const availableTags = $derived(tags.filter((tag) => !tag.hiddenAt))
  const unavailableTagIds = $derived(selectedTagIds.filter((tagId) => Boolean(tagById.get(tagId)?.hiddenAt) || !tagById.has(tagId)))
  const canSave = $derived(tagCatalogState === 'ready' && unavailableTagIds.length === 0)

  function toggleTag(tagId: string, checked: boolean) {
    selectedTagIds = checked ? [...selectedTagIds, tagId] : selectedTagIds.filter((id) => id !== tagId)
  }

  function removeUnavailableTag(tagId: string) {
    selectedTagIds = selectedTagIds.filter((id) => id !== tagId)
  }

  async function saveRule(event: SubmitEvent) {
    event.preventDefault()
    if (saving || !canSave) return
    saving = true
    error = null
    try {
      await financeApi.createClassificationRule({
        tenantId,
        matchType,
        condition,
        categoryId: targetCategoryId,
        tagIds: selectedTagIds,
      })
      onSaved()
    } catch (saveError) {
      error = saveError instanceof Error ? saveError.message : 'Could not create the rule.'
    } finally {
      saving = false
    }
  }
</script>

<form class="border rounded p-3 d-grid gap-3" onsubmit={saveRule} aria-label="Create classification rule">
  <div>
    <h3 class="h6 mb-1">Create a classification rule?</h3>
    <p class="small text-body-secondary mb-0">The saved category assignment stays independent of this optional rule.</p>
  </div>
  <div class="row g-3">
    <div class="col-12 col-md-4">
      <label class="form-label" for="finance-rule-match-type">Match type</label>
      <select id="finance-rule-match-type" class="form-select" bind:value={matchType} disabled={saving}>
        <option value="contains">contains</option>
        <option value="exact">exact</option>
      </select>
    </div>
    <div class="col-12 col-md-8">
      <label class="form-label" for="finance-rule-condition">Rule condition</label>
      <input id="finance-rule-condition" class="form-control" bind:value={condition} disabled={saving} required />
    </div>
    <div class="col-12">
      <label class="form-label" for="finance-rule-category">Rule category</label>
      <select id="finance-rule-category" class="form-select" bind:value={targetCategoryId} disabled={saving} required>
        {#each categories as category (category.id)}
          <option value={category.id}>{category.name}</option>
        {/each}
      </select>
    </div>
    <fieldset class="col-12" disabled={saving || tagCatalogState !== 'ready'}>
      <legend class="form-label mb-2">Rule tags</legend>
      {#if tagCatalogState === 'loading'}
        <p class="form-text mb-0">Loading tag catalog…</p>
      {:else if availableTags.length === 0}
        <p class="form-text mb-0">No tenant tags are available. This rule will assign only its category.</p>
      {:else}
        <div class="d-flex flex-wrap gap-3" aria-label="Rule tags">
          {#each availableTags as tag (tag.id)}
            <div class="form-check">
              <input id={`finance-rule-tag-${tag.id}`} class="form-check-input" type="checkbox" checked={selectedTagIds.includes(tag.id)} onchange={(event) => toggleTag(tag.id, event.currentTarget.checked)} />
              <label class="form-check-label" for={`finance-rule-tag-${tag.id}`}>{tag.name}</label>
            </div>
          {/each}
        </div>
      {/if}
    </fieldset>
  </div>
  {#if tagCatalogState === 'error'}
    <div class="alert alert-warning mb-0" role="alert">Tag catalog is unavailable. Refresh it before saving this rule. {#if onRetryTags}<button class="btn btn-link btn-sm p-0 align-baseline" type="button" onclick={onRetryTags}>Retry tag catalog</button>{/if}</div>
  {/if}
  {#if unavailableTagIds.length}
    <div class="alert alert-warning mb-0" role="alert">
      <div class="mb-2">Selected tags are unavailable. Refresh the catalog or remove them before saving.</div>
      <div class="d-flex flex-wrap gap-2">{#each unavailableTagIds as tagId (tagId)}<button class="btn btn-outline-warning btn-sm" type="button" onclick={() => removeUnavailableTag(tagId)} aria-label={`Remove unavailable tag ${tagId}`}>Remove unavailable tag {tagId}</button>{/each}</div>
    </div>
  {/if}
  {#if error}<div class="alert alert-danger mb-0" role="alert">{error}</div>{/if}
  <div class="d-flex flex-wrap gap-2">
    <button class="btn btn-primary" type="submit" disabled={saving || !canSave}>{saving ? 'Saving rule…' : 'Save rule'}</button>
    <button class="btn btn-outline-secondary" type="button" onclick={onCancel} disabled={saving}>Cancel rule</button>
  </div>
</form>
