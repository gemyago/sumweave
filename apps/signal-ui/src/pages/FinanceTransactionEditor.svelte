<script lang="ts">
  import { onMount, tick } from 'svelte'
  import { link } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalFinanceApiForAuth,
    type FinanceAccount,
    type FinanceCategory,
    type FinanceTag,
    type FinanceTransaction,
  } from '../lib/finance/api'
  import { formatFinanceDateTime, formatFinanceMoney } from '../lib/finance/format'
  import { formatMinorAmountForInput, parseMajorAmountToMinor } from '../lib/finance/money'
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'
  import { supportedFinanceTenantDisplayCurrencies } from '../lib/finance/tenant-display-currencies'
  import { isMatchedTransfer, transferLinkEligibilityIssue } from '../lib/finance/transfer-eligibility'
  import {
    defaultTransferCandidateRange,
    transferCandidateRangeFromDateInputs,
  } from '../lib/finance/transfer-range'
  import FinanceProviderEvidence from '../components/FinanceProviderEvidence.svelte'
  import FinancePager from '../components/FinancePager.svelte'

  let { params = {} } = $props<{ params?: { transactionId?: string } }>()

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const isCreateMode = $derived(!params.transactionId)
  const financeShell = useFinanceShellState()

  let loading = $state(true)
  let saving = $state(false)
  let error = $state<string | null>(null)
  let saveError = $state<string | null>(null)
  let saveMessage = $state<string | null>(null)
  let saveStatus = $state<HTMLElement | undefined>(undefined)
  let accounts = $state<FinanceAccount[]>([])
  let categories = $state<FinanceCategory[]>([])
  let tags = $state<FinanceTag[]>([])
  let transaction = $state<FinanceTransaction | null>(null)
  let form = $state(makeBlankForm())
  let transferPartner = $state<FinanceTransaction | null>(null)
  let partnerLoading = $state(false)
  let partnerError = $state<string | null>(null)
  let candidateOpen = $state(false)
  let candidates = $state<FinanceTransaction[]>([])
  let candidateOffset = $state(0)
  let candidateFromDate = $state('')
  let candidateBeforeDate = $state('')
  let selectedCandidateId = $state('')
  let candidatesLoading = $state(false)
  let candidateError = $state<string | null>(null)
  let confirmingTransfer = $state(false)
  let unlinkConfirmationOpen = $state(false)
  let unlinkingTransfer = $state(false)
  let transferMessage = $state<string | null>(null)
  let reactiveReady = $state(false)
  let skipNextReactiveLoad = false

  const candidatePageSize = 20
  const hasMatchedTransfer = $derived(transaction ? isMatchedTransfer(transaction) : false)
  const hasPreviousCandidatePage = $derived(candidateOffset > 0)
  const hasNextCandidatePage = $derived(candidates.length === candidatePageSize)
  const selectableAccounts = $derived(accounts.filter((account) => !account.hiddenAt))
  const candidatePageNumber = $derived(Math.floor(candidateOffset / candidatePageSize) + 1)
  const selectedCandidate = $derived(candidates.find((item) => item.id === selectedCandidateId) ?? null)
  const selectedCandidateIssue = $derived(transaction && selectedCandidate
    ? transferLinkEligibilityIssue(transaction, selectedCandidate)
    : null)

  onMount(() => {
    void loadPage()
  })

  function makeBlankForm() {
    return {
      accountId: '',
      source: 'manual',
      status: 'booked',
      kind: 'expense',
      amount: '0.00',
      currency: 'USD',
      description: '',
      effectiveAt: localTodayDateTimeValue(),
      categoryId: '',
      tagIds: [] as string[],
    }
  }

  function fillFormFromTransaction(item: FinanceTransaction) {
    form = {
      accountId: item.accountId,
      source: item.source,
      status: item.status,
      kind: item.kind,
      amount: formatMinorAmountForInput(item.amountMinor),
      currency: item.currency,
      description: item.description,
      effectiveAt: toDateTimeLocalValue(item.effectiveAt),
      categoryId: item.categoryId ?? '',
      tagIds: [...item.tagIds],
    }
  }

  async function loadPage() {
    loading = true
    error = null
    saveMessage = null
    reactiveReady = false

    try {
      await financeShell.initialize()
      if (!financeShell.selectedTenantId) {
        accounts = []
        categories = []
        tags = []
        transaction = null
        form = makeBlankForm()
        return
      }
      await loadEditorData()
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load transaction editor'
    } finally {
      skipNextReactiveLoad = true
      reactiveReady = true
      loading = false
    }
  }

  async function loadEditorData() {
    if (!financeShell.selectedTenantId) {
      return
    }

    saveMessage = null
    transferPartner = null
    partnerError = null
    closeCandidateWorkflow()
    unlinkConfirmationOpen = false
    ;[accounts, categories, tags] = await Promise.all([
      financeApi.listAccounts({ tenantId: financeShell.selectedTenantId, includeHidden: true }),
      financeApi.listCategories({ tenantId: financeShell.selectedTenantId }),
      financeApi.listTags({ tenantId: financeShell.selectedTenantId, includeHidden: true }),
    ])

    if (isCreateMode) {
      transaction = null
      form = makeBlankForm()
        if (selectableAccounts.length > 0) {
          form.accountId = selectableAccounts[0].id
          form.currency = selectableAccounts[0].currency
      }
      return
    }

    transaction = await financeApi.getTransaction({
      tenantId: financeShell.selectedTenantId,
      transactionId: params.transactionId ?? '',
    })
    fillFormFromTransaction(transaction)
    if (isMatchedTransfer(transaction)) {
      void loadTransferPartner(transaction)
    }
  }

  function accountName(accountId: string): string {
    return accounts.find((account) => account.id === accountId)?.name ?? 'Unknown account'
  }

  function candidateIssue(candidate: FinanceTransaction): string | null {
    return transaction ? transferLinkEligibilityIssue(transaction, candidate) : 'Transaction details are unavailable.'
  }

  function openCandidateWorkflow() {
    if (!transaction) return
    const range = defaultTransferCandidateRange(transaction.effectiveAt)
    candidateFromDate = range.effectiveFromDate
    candidateBeforeDate = range.effectiveBeforeDate
    candidateOffset = 0
    selectedCandidateId = ''
    candidateError = null
    candidateOpen = true
    void loadCandidates()
  }

  function closeCandidateWorkflow() {
    candidateOpen = false
    candidates = []
    candidateOffset = 0
    selectedCandidateId = ''
    candidateError = null
  }

  async function loadCandidates(offset = candidateOffset): Promise<boolean> {
    if (!transaction || !financeShell.selectedTenantId) return false
    candidatesLoading = true
    candidateError = null
    try {
      const range = transferCandidateRangeFromDateInputs(candidateFromDate, candidateBeforeDate)
      const page = await financeApi.listTransferCandidates({
        tenantId: financeShell.selectedTenantId,
        transactionId: transaction.id,
        effectiveFrom: range.effectiveFrom,
        effectiveBefore: range.effectiveBefore,
        limit: candidatePageSize,
        offset,
      })
      candidates = page.items
      if (!candidates.some((candidate) => candidate.id === selectedCandidateId)) {
        selectedCandidateId = ''
      }
      return true
    } catch (loadError) {
      candidateError = loadError instanceof Error ? loadError.message : 'Failed to load transfer candidates.'
      return false
    } finally {
      candidatesLoading = false
    }
  }

  function applyCandidateRange() {
    candidateOffset = 0
    selectedCandidateId = ''
    void loadCandidates()
  }

  async function loadPreviousCandidatePage(): Promise<boolean> {
    if (!hasPreviousCandidatePage) return false
    const nextOffset = Math.max(0, candidateOffset - candidatePageSize)
    if (!await loadCandidates(nextOffset)) return false
    candidateOffset = nextOffset
    selectedCandidateId = ''
    return true
  }

  async function loadNextCandidatePage(): Promise<boolean> {
    if (!hasNextCandidatePage) return false
    const nextOffset = candidateOffset + candidatePageSize
    if (!await loadCandidates(nextOffset)) return false
    candidateOffset = nextOffset
    selectedCandidateId = ''
    return true
  }

  async function loadTransferPartner(source: FinanceTransaction) {
    if (!financeShell.selectedTenantId) return
    partnerLoading = true
    partnerError = null
    try {
      transferPartner = await financeApi.getTransferPartner({
        tenantId: financeShell.selectedTenantId,
        transactionId: source.id,
      })
    } catch (loadError) {
      partnerError = loadError instanceof Error ? loadError.message : 'Failed to load the linked transfer partner.'
    } finally {
      partnerLoading = false
    }
  }

  async function confirmTransferLink() {
    if (!transaction || !selectedCandidate || selectedCandidateIssue || !financeShell.selectedTenantId) return
    confirmingTransfer = true
    candidateError = null
    try {
      await financeApi.linkTransferPair({
        tenantId: financeShell.selectedTenantId,
        firstTransactionId: transaction.id,
        secondTransactionId: selectedCandidate.id,
      })
      transferMessage = 'Internal transfer linked. The matched pair is excluded from income and expense reporting.'
      await loadEditorData()
    } catch (linkError) {
      candidateError = linkError instanceof Error
        ? `Transfer link could not be completed. The record may have changed; refresh candidates and try again. ${linkError.message}`
        : 'Transfer link could not be completed. The record may have changed; refresh candidates and try again.'
    } finally {
      confirmingTransfer = false
    }
  }

  async function confirmTransferUnlink() {
    if (!transaction || !transferPartner || !financeShell.selectedTenantId) return
    unlinkingTransfer = true
    partnerError = null
    try {
      await financeApi.unlinkTransferPair({
        tenantId: financeShell.selectedTenantId,
        firstTransactionId: transaction.id,
        secondTransactionId: transferPartner.id,
      })
      transferMessage = 'Internal transfer unlinked. Both records return to income and expense reporting.'
      await loadEditorData()
    } catch (unlinkError) {
      partnerError = unlinkError instanceof Error
        ? `Transfer unlink could not be completed. Refresh the linked record and try again. ${unlinkError.message}`
        : 'Transfer unlink could not be completed. Refresh the linked record and try again.'
    } finally {
      unlinkingTransfer = false
    }
  }

  function syncCurrencyWithAccount() {
    const account = accounts.find((item) => item.id === form.accountId)
    if (account && isCreateMode) {
      form.currency = account.currency
    }
  }

  function toDateTimeLocalValue(value: Date): string {
    if (Number.isNaN(value.getTime())) {
      throw new TypeError('Cannot render an invalid transaction timestamp')
    }
    const pad = (part: number) => String(part).padStart(2, '0')
    return `${value.getFullYear()}-${pad(value.getMonth() + 1)}-${pad(value.getDate())}T${pad(value.getHours())}:${pad(value.getMinutes())}`
  }

  function localTodayDateTimeValue(): string {
    const today = new Date()
    const pad = (part: number) => String(part).padStart(2, '0')
    return `${today.getFullYear()}-${pad(today.getMonth() + 1)}-${pad(today.getDate())}T00:00`
  }

  function fromDateTimeLocalValue(value: string): Date {
    const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(value)
    if (!match) {
      throw new TypeError('Enter a valid local date and time')
    }
    const [year, month, day, hour, minute] = match.slice(1).map(Number)
    const parsed = new Date(year, month - 1, day, hour, minute)
    if (
      parsed.getFullYear() !== year || parsed.getMonth() !== month - 1 || parsed.getDate() !== day ||
      parsed.getHours() !== hour || parsed.getMinutes() !== minute
    ) {
      throw new TypeError('Enter a local date and time that exists')
    }
    return parsed
  }

  async function saveTransaction(event: SubmitEvent) {
    event.preventDefault()
    if (!financeShell.selectedTenantId) {
      return
    }

    saving = true
    saveError = null
    saveMessage = null

    try {
      const amountMinor = parseMajorAmountToMinor(form.amount)
      if (isCreateMode) {
        const created = await financeApi.createTransaction({
          tenantId: financeShell.selectedTenantId,
          accountId: form.accountId,
          source: form.source,
          status: form.status,
          kind: form.kind,
          amountMinor,
          currency: form.currency,
          description: form.description,
          effectiveAt: fromDateTimeLocalValue(form.effectiveAt),
          categoryId: form.categoryId || undefined,
          tagIds: form.tagIds,
        })
        transaction = created
        saveMessage = 'Transaction recorded.'
        fillFormFromTransaction(created)
      } else {
        transaction = await financeApi.updateTransaction({
          tenantId: financeShell.selectedTenantId,
          transactionId: params.transactionId ?? '',
          description: form.description,
          amountMinor,
          effectiveAt: fromDateTimeLocalValue(form.effectiveAt),
          categoryId: form.categoryId || null,
          tagIds: form.tagIds,
        })
        saveMessage = 'Transaction updated.'
        if (transaction) {
          fillFormFromTransaction(transaction)
        }
      }
      await tick()
      saveStatus?.focus({ preventScroll: true })
    } catch (saveFailure) {
      saveError = saveFailure instanceof Error ? saveFailure.message : 'Failed to save transaction'
    } finally {
      saving = false
    }
  }

  function clearSaveFeedback() {
    saveError = null
    saveMessage = null
  }

  $effect(() => {
    if (financeShell.loading || !reactiveReady) return
    void financeShell.selectedTenantId
    void params.transactionId
    if (skipNextReactiveLoad) {
      skipNextReactiveLoad = false
      return
    }
    void loadEditorData()
  })
</script>

<section class="container-fluid px-0" aria-labelledby="finance-transaction-editor-heading">
  <div class="d-grid gap-4">
    <header class="card border-0 shadow-sm">
      <div class="card-body p-4 p-xl-5 d-flex flex-column flex-lg-row justify-content-between gap-3 align-items-lg-center">
        <div>
          <h1 id="finance-transaction-editor-heading" class="h3 mb-0">{#if isCreateMode}Record transaction{:else}Transaction{/if}</h1>
        </div>

        <a class="btn btn-outline-secondary align-self-start align-self-lg-center" href="/finance/transactions" use:link>
          Back to transactions
        </a>
      </div>
    </header>

    {#if error}
      <div class="alert alert-danger mb-0" role="alert">{error}</div>
    {/if}

    {#if transferMessage}
      <div class="alert alert-success mb-0" role="status">{transferMessage}</div>
    {/if}

    {#if loading}
      <div class="alert alert-secondary mb-0" role="status">Loading transaction editor…</div>
    {:else if financeShell.needsTenantSelection}
      <section class="card shadow-sm">
        <div class="card-body p-4 d-grid gap-3">
          {#if !financeShell.embedded}
            <div class="col-12 col-lg-5 px-0">
              <label class="form-label" for="finance-transaction-editor-tenant">Tenant</label>
              <select
                id="finance-transaction-editor-tenant"
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

          <div class="alert alert-warning mb-0" role="status">Select an active tenant to continue on this finance route.</div>
        </div>
      </section>
    {:else if !financeShell.selectedTenantId}
      <div class="alert alert-light border mb-0" role="status">
        Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a> before editing transactions.
      </div>
    {:else}
      {#if !financeShell.embedded}
        <section class="card shadow-sm">
          <div class="card-body p-4">
            <div class="col-12 col-lg-5 px-0">
              <label class="form-label" for="finance-transaction-editor-selected-tenant">Tenant</label>
              <select
                id="finance-transaction-editor-selected-tenant"
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
          </div>
        </section>
      {/if}

      <form class="card shadow-sm" onsubmit={saveTransaction} oninput={clearSaveFeedback} onchange={clearSaveFeedback}>
        <div class="card-body p-4 d-grid gap-4">
          <h2 class="h5 mb-0">Details</h2>
          <div class="row g-3">
            <div class="col-12 col-md-6"><label class="form-label" for="finance-transaction-account">Account</label><select id="finance-transaction-account" class="form-select" bind:value={form.accountId} onchange={syncCurrencyWithAccount} aria-label="Transaction account" disabled={!isCreateMode} required><option value="">Select account</option>{#each isCreateMode ? selectableAccounts : accounts as account (account.id)}<option value={account.id}>{account.name}{account.hiddenAt ? ' (Hidden)' : ''}</option>{/each}</select>{#if isCreateMode && selectableAccounts.length === 0}<div class="form-text">No active accounts are available. Restore an account or create one before recording a transaction.</div>{/if}</div>
            <div class="col-12 col-md-6"><label class="form-label" for="finance-transaction-category">Category</label><select id="finance-transaction-category" class="form-select" bind:value={form.categoryId} aria-label="Transaction category"><option value="">No category</option>{#each categories as category (category.id)}<option value={category.id}>{category.name}</option>{/each}</select></div>
            <fieldset class="col-12"><legend class="form-label mb-2">Tags</legend>{#if tags.length === 0}<p class="form-text mb-0">No tenant tags are available.</p>{:else}<div class="d-flex flex-wrap gap-3" aria-label="Transaction tags">{#each tags as tag (tag.id)}<div class="form-check"><input id={`finance-transaction-tag-${tag.id}`} class="form-check-input" type="checkbox" value={tag.id} bind:group={form.tagIds} disabled={Boolean(tag.hiddenAt) && !form.tagIds.includes(tag.id)} /><label class="form-check-label" for={`finance-transaction-tag-${tag.id}`}>{tag.name}{tag.hiddenAt ? ' (hidden)' : ''}</label></div>{/each}</div>{/if}</fieldset>
            <div class="col-12 col-md-4"><label class="form-label" for="finance-transaction-kind">Kind</label><select id="finance-transaction-kind" class="form-select" bind:value={form.kind} aria-label="Transaction kind" disabled={!isCreateMode}><option value="expense">expense</option><option value="income">income</option><option value="regular">regular</option><option value="refund">refund</option><option value="transfer">transfer</option><option value="reconciliation">reconciliation</option></select></div>
            <div class="col-12 col-md-4"><label class="form-label" for="finance-transaction-amount">Amount</label><input id="finance-transaction-amount" class="form-control" bind:value={form.amount} aria-label="Amount" inputmode="decimal" type="text" required /><div class="form-text">Major units, up to two decimal places.</div></div>
            <div class="col-12 col-md-4"><label class="form-label" for="finance-transaction-currency">Currency</label><select id="finance-transaction-currency" class="form-select" bind:value={form.currency} aria-label="Transaction currency" disabled={!isCreateMode} required>{#each supportedFinanceTenantDisplayCurrencies as currencyCode (currencyCode)}<option value={currencyCode}>{currencyCode}</option>{/each}</select></div>
            <div class="col-12"><label class="form-label" for="finance-transaction-description">Description</label><input id="finance-transaction-description" class="form-control" bind:value={form.description} aria-label="Transaction description" /></div>
            <div class="col-12 col-lg-6"><label class="form-label" for="finance-transaction-effective-at">Effective at</label><input id="finance-transaction-effective-at" class="form-control" bind:value={form.effectiveAt} aria-label="Transaction effective at" type="datetime-local" required /></div>
            <div class="col-12 col-md-4"><label class="form-label" for="finance-transaction-status">Status</label><select id="finance-transaction-status" class="form-select" bind:value={form.status} aria-label="Transaction status" disabled={!isCreateMode}><option value="booked">booked</option><option value="pending">pending</option></select></div>
            <div class="col-12 col-md-4"><label class="form-label" for="finance-transaction-source">Source</label><select id="finance-transaction-source" class="form-select" bind:value={form.source} aria-label="Transaction source" disabled={!isCreateMode}><option value="manual">manual</option><option value="provider">provider</option><option value="csv">csv</option><option value="system">system</option></select></div>
          </div>
          <div class="d-flex flex-wrap gap-2"><button class="btn btn-primary" type="submit" disabled={saving || !financeShell.selectedTenantId}>{#if saving}Saving…{:else}Save transaction{/if}</button><a class="btn btn-outline-secondary" href="/finance/transactions" use:link>Cancel</a></div>
          {#if saveError}<div class="alert alert-danger mb-0" role="alert">{saveError}</div>{/if}
          {#if saveMessage}<div class="alert alert-success mb-0" role="status" tabindex="-1" bind:this={saveStatus}>{saveMessage}</div>{/if}
        </div>
      </form>

      {#if transaction?.providerOriginal}
        <section class="card shadow-sm">
          <div class="card-body p-4 d-grid gap-3">
            <div>
              <h2 class="h5 mb-1">Original synced values</h2>
            </div>

            <div class="row g-3">
              <div class="col-12"><strong>Description</strong><div>{transaction.providerOriginal.description || '—'}</div></div>
              <div class="col-12 col-md-6"><strong>Amount</strong><div>{formatFinanceMoney(transaction.providerOriginal.amountMinor, transaction.providerOriginal.currency)}</div></div>
              <div class="col-12 col-md-6"><strong>Effective</strong><div>{transaction.providerOriginal.effectiveAt ? formatFinanceDateTime(transaction.providerOriginal.effectiveAt) : '—'}</div></div>
            </div>
          </div>
        </section>
      {/if}

      {#if transaction}
        <section class="card shadow-sm" aria-labelledby="finance-transfer-workflow-heading">
          <div class="card-body p-4 d-grid gap-3">
            <div class="d-flex flex-column flex-md-row justify-content-between gap-3 align-items-md-center">
              <div>
                <h2 id="finance-transfer-workflow-heading" class="h5 mb-1">Internal transfer</h2>
                <p class="text-body-secondary mb-0">
                  {#if hasMatchedTransfer}
                    This record is linked as an internal transfer. Account balances remain unchanged and the pair is excluded from income and expense reporting.
                  {:else}
                    Match this booked record to an opposite-direction record in another account.
                  {/if}
                </p>
              </div>

              {#if !hasMatchedTransfer}
                <button class="btn btn-primary align-self-start align-self-md-center" type="button" onclick={openCandidateWorkflow}>
                  Link transfer
                </button>
              {/if}
            </div>

            {#if hasMatchedTransfer}
              {#if partnerLoading}
                <div class="alert alert-secondary mb-0" role="status">Loading linked transfer…</div>
              {:else if partnerError}
                <div class="alert alert-danger mb-0 d-flex flex-column flex-md-row justify-content-between gap-2 align-items-md-center" role="alert">
                  <span>{partnerError}</span>
                  <button class="btn btn-outline-danger btn-sm align-self-start" type="button" onclick={() => transaction && void loadTransferPartner(transaction)}>Retry linked transfer</button>
                </div>
              {:else if transferPartner}
                <div class="border rounded p-3 d-grid gap-2">
                  <strong>Linked with {accountName(transferPartner.accountId)}</strong>
                  <span>{transferPartner.description || transferPartner.kind} · {transferPartner.kind} · {formatFinanceDateTime(transferPartner.effectiveAt)}</span>
                   <span>{formatFinanceMoney(transferPartner.amountMinor, transferPartner.currency)}</span>
                   <div class="d-flex flex-wrap gap-2">
                     <a class="btn btn-outline-primary btn-sm" href={`/finance/transactions/${encodeURIComponent(transferPartner.id)}`} use:link>Open linked transaction</a>
                     <button class="btn btn-outline-danger btn-sm" type="button" onclick={() => unlinkConfirmationOpen = true}>Unlink transfer</button>
                   </div>
                </div>

                {#if unlinkConfirmationOpen}
                  <div class="alert alert-warning mb-0 d-grid gap-2" aria-label="Confirm unlink transfer">
                    <strong>Unlink this internal transfer?</strong>
                    <span>{transaction.description || transaction.kind} ({accountName(transaction.accountId)}) will be unlinked from {transferPartner.description || transferPartner.kind} ({accountName(transferPartner.accountId)}).</span>
                    <span>Both records will return to income and expense reporting.</span>
                    <div class="d-flex flex-wrap gap-2">
                      <button class="btn btn-danger btn-sm" type="button" onclick={() => void confirmTransferUnlink()} disabled={unlinkingTransfer}>
                        {unlinkingTransfer ? 'Unlinking…' : 'Confirm unlink'}
                      </button>
                      <button class="btn btn-outline-secondary btn-sm" type="button" onclick={() => unlinkConfirmationOpen = false} disabled={unlinkingTransfer}>Cancel</button>
                    </div>
                  </div>
                {/if}
              {/if}
            {:else if candidateOpen}
              <div id="finance-transfer-candidates" class="border rounded p-3 d-grid gap-3" aria-label="Transfer candidates" aria-busy={candidatesLoading}>
                <div class="d-flex flex-column flex-md-row justify-content-between gap-2 align-items-md-center">
                  <div>
                    <h3 class="h6 mb-1">Transfer candidates</h3>
                    <p class="small text-body-secondary mb-0">Candidates are from other visible accounts. The effective-before boundary is exclusive.</p>
                  </div>
                  <button class="btn btn-outline-secondary btn-sm align-self-start" type="button" onclick={closeCandidateWorkflow}>Close candidates</button>
                </div>

                <div class="row g-3 align-items-end">
                  <div class="col-12 col-md-5">
                    <label class="form-label" for="finance-transfer-effective-from">Effective from</label>
                    <input id="finance-transfer-effective-from" class="form-control" type="date" bind:value={candidateFromDate} />
                  </div>
                  <div class="col-12 col-md-5">
                    <label class="form-label" for="finance-transfer-effective-before">Effective before (exclusive)</label>
                    <input id="finance-transfer-effective-before" class="form-control" type="date" bind:value={candidateBeforeDate} />
                  </div>
                  <div class="col-12 col-md-2">
                    <button class="btn btn-outline-primary w-100" type="button" onclick={applyCandidateRange} disabled={candidatesLoading}>Apply</button>
                  </div>
                </div>

                {#if candidateError}
                  <div class="alert alert-danger mb-0 d-flex flex-column flex-md-row justify-content-between gap-2 align-items-md-center" role="alert">
                    <span>{candidateError}</span>
                    <button class="btn btn-outline-danger btn-sm align-self-start" type="button" onclick={() => void loadCandidates()} disabled={candidatesLoading}>Refresh candidates</button>
                  </div>
                {/if}

                {#if candidates.length === 0}
                  {#if candidatesLoading}
                    <div class="alert alert-secondary mb-0" role="status">Loading transfer candidates…</div>
                  {:else}
                  <div class="alert alert-light border mb-0" role="status">No transfer candidates matched this date range.</div>
                  {/if}
                {:else}
                  <div class="table-responsive">
                    <table class="table align-middle mb-0" aria-label="Transfer candidates">
                      <thead>
                        <tr><th scope="col">Select</th><th scope="col" class="d-none d-md-table-cell">Account</th><th scope="col">Description</th><th scope="col" class="d-none d-md-table-cell">Kind</th><th scope="col" class="d-none d-md-table-cell">Effective</th><th scope="col" class="d-none d-md-table-cell">Amount</th></tr>
                      </thead>
                      <tbody>
                        {#each candidates as candidate (candidate.id)}
                            {@const issue = candidateIssue(candidate)}
                            <tr>
                            <td><input class="form-check-input" type="radio" name="transfer-candidate" value={candidate.id} checked={selectedCandidateId === candidate.id} onchange={() => selectedCandidateId = candidate.id} disabled={Boolean(issue)} aria-label={`Select ${candidate.description || candidate.kind}`} /></td>
                            <td class="d-none d-md-table-cell">{accountName(candidate.accountId)}</td>
                            <td><div>{candidate.description || candidate.kind}</div><small class="d-md-none text-body-secondary d-block">{candidate.kind} · {accountName(candidate.accountId)} · {formatFinanceDateTime(candidate.effectiveAt)} · {formatFinanceMoney(candidate.amountMinor, candidate.currency)}</small></td>
                            <td class="d-none d-md-table-cell">{candidate.kind}</td>
                            <td class="d-none d-md-table-cell">{formatFinanceDateTime(candidate.effectiveAt)}</td>
                            <td class="d-none d-md-table-cell">{formatFinanceMoney(candidate.amountMinor, candidate.currency)}</td>
                          </tr>
                        {/each}
                      </tbody>
                    </table>
                  </div>

                  <FinancePager
                    label="Transfer candidate pages"
                    status={candidatesLoading ? 'Loading transfer candidate page…' : `Page ${candidatePageNumber}`}
                    controls="finance-transfer-candidates"
                    busy={candidatesLoading}
                    hasPrevious={hasPreviousCandidatePage}
                    hasNext={hasNextCandidatePage}
                    onPrevious={loadPreviousCandidatePage}
                    onNext={loadNextCandidatePage}
                  />
                {/if}

                {#if selectedCandidate}
                  <div class="alert alert-info mb-0 d-grid gap-2" aria-label="Confirm transfer link">
                    <strong>Confirm internal transfer</strong>
                    <span>{transaction.description || transaction.kind} ({accountName(transaction.accountId)}, {formatFinanceDateTime(transaction.effectiveAt)}, {formatFinanceMoney(transaction.amountMinor, transaction.currency)}) will link with {selectedCandidate.description || selectedCandidate.kind} ({accountName(selectedCandidate.accountId)}, {formatFinanceDateTime(selectedCandidate.effectiveAt)}, {formatFinanceMoney(selectedCandidate.amountMinor, selectedCandidate.currency)}).</span>
                    <span>The matched pair will be excluded from income and expense reporting; account balances remain unchanged.</span>
                    <div><button class="btn btn-primary btn-sm" type="button" onclick={() => void confirmTransferLink()} disabled={confirmingTransfer || Boolean(selectedCandidateIssue)}>{confirmingTransfer ? 'Linking…' : 'Confirm link transfer'}</button></div>
                  </div>
                {/if}
              </div>
            {/if}
          </div>
        </section>
      {/if}

      {#if transaction}
        <section class="card shadow-sm">
          <div class="card-body p-4">
            <FinanceProviderEvidence tenantId={financeShell.selectedTenantId} entityId={transaction.id} entityLabel="transaction" scope="transaction" />
          </div>
        </section>
      {/if}

    {/if}
  </div>
</section>
