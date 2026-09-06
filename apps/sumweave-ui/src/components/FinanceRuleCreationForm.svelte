<script lang="ts">
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalFinanceApiForAuth,
    type FinanceCategory,
    type FinanceClassificationMatchType,
  } from '../lib/finance/api'

  let {
    tenantId,
    offerId,
    description,
    categoryId,
    categories,
    onCancel,
    onSaved,
  }: {
    tenantId: string
    offerId: number
    description: string
    categoryId: string
    categories: FinanceCategory[]
    onCancel: () => void
    onSaved: () => void
  } = $props()

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  let matchType = $state<FinanceClassificationMatchType>('contains')
  let condition = $state('')
  let targetCategoryId = $state('')
  let saving = $state(false)
  let error = $state<string | null>(null)
  let initializedOfferId = $state<number | null>(null)

  $effect(() => {
    if (offerId === initializedOfferId) return
    initializedOfferId = offerId
    matchType = 'contains'
    condition = description
    targetCategoryId = categoryId
  })

  async function saveRule(event: SubmitEvent) {
    event.preventDefault()
    if (saving) return
    saving = true
    error = null
    try {
      await financeApi.createClassificationRule({
        tenantId,
        matchType,
        condition,
        categoryId: targetCategoryId,
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
  </div>
  {#if error}<div class="alert alert-danger mb-0" role="alert">{error}</div>{/if}
  <div class="d-flex flex-wrap gap-2">
    <button class="btn btn-primary" type="submit" disabled={saving}>{saving ? 'Saving rule…' : 'Save rule'}</button>
    <button class="btn btn-outline-secondary" type="button" onclick={onCancel} disabled={saving}>Cancel rule</button>
  </div>
</form>
