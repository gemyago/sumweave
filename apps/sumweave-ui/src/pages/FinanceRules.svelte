<script lang="ts">
  import { onMount } from 'svelte'
  import { link, router } from 'svelte-spa-router'
  import DocumentTitle from '../components/DocumentTitle.svelte'
  import JobStatus from '../components/JobStatus.svelte'
  import { documentTitle } from '../lib/document-title'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalFinanceApiForAuth,
    type FinanceCategory,
    type FinanceClassificationMatchType,
    type FinanceClassificationRule,
  } from '../lib/finance/api'
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'
  import { classificationRangeFromDateInputs, defaultClassificationDateRange } from '../lib/finance/classification-range'
  import { requestFinanceLedgerRefresh } from '../lib/finance/ledger-refresh'
  import type { JobDetail } from '../lib/jobs/api'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const financeShell = useFinanceShellState()
  let loading = $state(true)
  let error = $state<string | null>(null)
  let rules = $state<FinanceClassificationRule[]>([])
  let categories = $state<FinanceCategory[]>([])
  let creating = $state(false)
  let editingId = $state<string | null>(null)
  let mutationBusy = $state(false)
  let matchType = $state<FinanceClassificationMatchType>('contains')
  let condition = $state('')
  let categoryId = $state('')
  let reactiveReady = $state(false)
  let skipNextReactiveLoad = false
  const defaultClassificationRange = defaultClassificationDateRange()
  let classificationStartDate = $state(defaultClassificationRange.startDate)
  let classificationEndDate = $state(defaultClassificationRange.endDate)
  let classificationBusy = $state(false)
  let classificationError = $state<string | null>(null)
  let classificationOutcome = $state<string | null>(null)
  let classificationJobId = $state('')

  const categoryFilter = $derived.by(() => {
    void router.location
    const query = window.location.hash.split('?')[1] ?? ''
    return new URLSearchParams(query).get('categoryId') ?? ''
  })
  const categoryNameById = $derived(new Map(categories.map((category) => [category.id, category.name])))

  onMount(() => { void loadPage() })

  async function loadPage() {
    loading = true
    reactiveReady = false
    error = null
    try {
      await financeShell.initialize()
      await loadRulesAndCategories()
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load classification rules.'
    } finally {
      skipNextReactiveLoad = true
      reactiveReady = true
      loading = false
    }
  }

  async function loadRulesAndCategories() {
    if (!financeShell.selectedTenantId) {
      rules = []
      categories = []
      return
    }
    ;[rules, categories] = await Promise.all([
      financeApi.listClassificationRules({ tenantId: financeShell.selectedTenantId, ...(categoryFilter ? { categoryId: categoryFilter } : {}) }),
      financeApi.listCategories({ tenantId: financeShell.selectedTenantId }),
    ])
  }

  function resetForm() {
    creating = false
    editingId = null
    matchType = 'contains'
    condition = ''
    categoryId = ''
  }

  function startCreate() {
    error = null
    editingId = null
    creating = true
    matchType = 'contains'
    condition = ''
    categoryId = categories[0]?.id ?? ''
  }

  function startEdit(rule: FinanceClassificationRule) {
    error = null
    creating = false
    editingId = rule.id
    matchType = rule.matchType
    condition = rule.condition
    categoryId = rule.categoryId
  }

  async function saveRule(event: SubmitEvent) {
    event.preventDefault()
    if (!financeShell.selectedTenantId || mutationBusy) return
    mutationBusy = true
    error = null
    try {
      if (editingId) {
        await financeApi.updateClassificationRule({ tenantId: financeShell.selectedTenantId, ruleId: editingId, matchType, condition, categoryId })
      } else {
        await financeApi.createClassificationRule({ tenantId: financeShell.selectedTenantId, matchType, condition, categoryId })
      }
      resetForm()
      await loadRulesAndCategories()
    } catch (saveError) {
      error = saveError instanceof Error ? saveError.message : 'Could not save the classification rule.'
    } finally {
      mutationBusy = false
    }
  }

  async function deleteRule(ruleId: string) {
    if (!financeShell.selectedTenantId || mutationBusy) return
    mutationBusy = true
    error = null
    try {
      await financeApi.deleteClassificationRule({ tenantId: financeShell.selectedTenantId, ruleId })
      if (editingId === ruleId) resetForm()
      await loadRulesAndCategories()
    } catch (deleteError) {
      error = deleteError instanceof Error ? deleteError.message : 'Could not delete the classification rule.'
    } finally {
      mutationBusy = false
    }
  }

  async function moveRule(rule: FinanceClassificationRule, direction: 'up' | 'down') {
    if (!financeShell.selectedTenantId || mutationBusy) return
    mutationBusy = true
    error = null
    try {
      await financeApi.moveClassificationRule({ tenantId: financeShell.selectedTenantId, ruleId: rule.id, direction })
      await loadRulesAndCategories()
    } catch (moveError) {
      error = moveError instanceof Error ? moveError.message : 'Could not move the classification rule.'
    } finally {
      mutationBusy = false
    }
  }

  async function submitClassification(event: SubmitEvent) {
    event.preventDefault()
    if (!financeShell.selectedTenantId || classificationBusy) return

    classificationError = null
    classificationOutcome = null
    try {
      const range = classificationRangeFromDateInputs(classificationStartDate, classificationEndDate)
      classificationBusy = true
      const result = await financeApi.submitTransactionClassification({
        tenantId: financeShell.selectedTenantId,
        rangeStart: range.rangeStart,
        rangeEndExclusive: range.rangeEndExclusive,
      })
      classificationJobId = result.jobId
    } catch (submitError) {
      classificationError = submitError instanceof Error ? submitError.message : 'Could not start classification.'
    } finally {
      classificationBusy = false
    }
  }

  function handleClassificationTerminal(job: JobDetail) {
    if (job.status === 'succeeded') {
      classificationOutcome = 'Classification completed. Ledger data has been refreshed.'
      requestFinanceLedgerRefresh(financeShell.selectedTenantId)
      return
    }
    if (job.status === 'failed') {
      classificationOutcome = 'Classification failed. Some transactions may already have been classified; running it again is safe.'
    }
  }

  $effect(() => {
    if (financeShell.loading || !reactiveReady) return
    void financeShell.selectedTenantId
    void categoryFilter
    if (skipNextReactiveLoad) {
      skipNextReactiveLoad = false
      return
    }
    void loadRulesAndCategories()
  })
</script>

<DocumentTitle title={documentTitle('Rules', 'Finance')} />

<section class="container-fluid px-0" aria-labelledby="finance-rules-heading">
  <div class="d-grid gap-4">
    <header class="card border-0 shadow-sm">
      <div class="card-body p-4 p-xl-5 d-flex flex-column flex-lg-row justify-content-between gap-3 align-items-lg-center">
        <div>
          <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Classification</p>
          <h1 id="finance-rules-heading" class="h3 mb-2">Classification rules</h1>
          <p class="text-body-secondary mb-0">Rules are evaluated from top to bottom. The first matching rule wins.</p>
        </div>
        <button class="btn btn-primary align-self-start" type="button" onclick={startCreate} disabled={!financeShell.selectedTenantId || mutationBusy}>Add rule</button>
      </div>
    </header>

    {#if error}<div class="alert alert-danger mb-0" role="alert">{error}</div>{/if}

    {#if loading}
      <div class="alert alert-secondary mb-0" role="status">Loading classification rules…</div>
    {:else if financeShell.needsTenantSelection}
      <div class="alert alert-warning mb-0" role="status">Select an active tenant to continue on this finance route.</div>
    {:else if !financeShell.selectedTenantId}
      <div class="alert alert-light border mb-0" role="status">Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a> before managing rules.</div>
    {:else}
      {#if categoryFilter}
        <div class="alert alert-info mb-0 d-flex flex-wrap justify-content-between gap-2 align-items-center" role="status">
          <span>Showing rules that reference this category.</span>
          <a href="/finance/rules" use:link>Clear filter</a>
        </div>
      {/if}

      <form class="card shadow-sm" onsubmit={submitClassification} aria-labelledby="finance-classification-run-heading">
        <div class="card-body p-4 d-grid gap-3">
          <div>
            <h2 id="finance-classification-run-heading" class="h5 mb-1">Run classification</h2>
            <p class="text-body-secondary mb-0">Classify eligible uncategorized transactions from the selected inclusive local dates.</p>
          </div>
          <div class="row g-3">
            <div class="col-12 col-md-6"><label class="form-label" for="finance-classification-start-date">Classification start date</label><input id="finance-classification-start-date" class="form-control" type="date" bind:value={classificationStartDate} disabled={classificationBusy} required /></div>
            <div class="col-12 col-md-6"><label class="form-label" for="finance-classification-end-date">Classification end date</label><input id="finance-classification-end-date" class="form-control" type="date" bind:value={classificationEndDate} disabled={classificationBusy} required /></div>
          </div>
          <div class="d-flex flex-wrap gap-2"><button class="btn btn-primary" type="submit" disabled={classificationBusy}>{classificationBusy ? 'Starting classification…' : classificationJobId ? 'Run classification again' : 'Run classification'}</button></div>
          {#if classificationError}<div class="alert alert-danger mb-0" role="alert">{classificationError}</div>{/if}
          {#if classificationOutcome}<div class={`alert ${classificationOutcome.startsWith('Classification completed') ? 'alert-success' : 'alert-warning'} mb-0`} role="status">{classificationOutcome}</div>{/if}
          {#if classificationJobId}<JobStatus jobId={classificationJobId} openHref={`/finance/jobs/${encodeURIComponent(classificationJobId)}`} label="Classification" linkLabel="Open finance job" observedDispatch onTerminal={handleClassificationTerminal} />{/if}
        </div>
      </form>

      {#if creating || editingId}
        <form class="card shadow-sm" onsubmit={saveRule}>
          <div class="card-body p-4 d-grid gap-3">
            <h2 class="h5 mb-0">{editingId ? 'Edit rule' : 'Add rule'}</h2>
            <div class="row g-3">
              <div class="col-12 col-md-4"><label class="form-label" for="finance-rules-match-type">Match type</label><select id="finance-rules-match-type" class="form-select" bind:value={matchType} disabled={mutationBusy}><option value="contains">contains</option><option value="exact">exact</option></select></div>
              <div class="col-12 col-md-8"><label class="form-label" for="finance-rules-condition">Condition</label><input id="finance-rules-condition" class="form-control" bind:value={condition} disabled={mutationBusy} required /></div>
              <div class="col-12"><label class="form-label" for="finance-rules-category">Target category</label><select id="finance-rules-category" class="form-select" bind:value={categoryId} disabled={mutationBusy} required>{#each categories as category (category.id)}<option value={category.id}>{category.name}</option>{/each}</select></div>
            </div>
            <div class="d-flex flex-wrap gap-2"><button class="btn btn-primary" type="submit" disabled={mutationBusy}>{mutationBusy ? 'Saving…' : 'Save rule'}</button><button class="btn btn-outline-secondary" type="button" onclick={resetForm} disabled={mutationBusy}>Cancel</button></div>
          </div>
        </form>
      {/if}

      <section class="card shadow-sm" aria-labelledby="finance-rules-list-heading" aria-busy={mutationBusy}>
        <div class="card-body p-4 d-grid gap-3">
          <div><h2 id="finance-rules-list-heading" class="h5 mb-1">Evaluation order</h2><p class="text-body-secondary mb-0">New rules are appended. Use adjacent controls to change precedence.</p></div>
          {#if rules.length === 0}
            <div class="alert alert-light border mb-0" role="status">No classification rules yet.</div>
          {:else}
            <ol class="list-group list-group-numbered">
              {#each rules as rule, index (rule.id)}
                <li class="list-group-item d-flex flex-column flex-md-row justify-content-between gap-3">
                  <div class="me-md-auto"><div class="fw-semibold">{rule.matchType} “{rule.condition}”</div><div class="small text-body-secondary">Target: {categoryNameById.get(rule.categoryId) ?? 'Unknown category'} · Position {rule.position}</div></div>
                  <div class="d-flex flex-wrap gap-2 align-self-md-center"><button class="btn btn-outline-secondary btn-sm" type="button" onclick={() => void moveRule(rule, 'up')} disabled={mutationBusy || index === 0} aria-label={`Move ${rule.condition} up`}>Move up</button><button class="btn btn-outline-secondary btn-sm" type="button" onclick={() => void moveRule(rule, 'down')} disabled={mutationBusy || index === rules.length - 1} aria-label={`Move ${rule.condition} down`}>Move down</button><button class="btn btn-outline-primary btn-sm" type="button" onclick={() => startEdit(rule)} disabled={mutationBusy}>Edit</button><button class="btn btn-outline-danger btn-sm" type="button" onclick={() => void deleteRule(rule.id)} disabled={mutationBusy}>Delete</button></div>
                </li>
              {/each}
            </ol>
          {/if}
        </div>
      </section>
    {/if}
  </div>
</section>
