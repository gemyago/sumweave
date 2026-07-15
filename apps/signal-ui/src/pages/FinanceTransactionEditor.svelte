<script lang="ts">
  import { onMount } from 'svelte'
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
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'
  import { supportedFinanceTenantDisplayCurrencies } from '../lib/finance/tenant-display-currencies'

  let { params = {} } = $props<{ params?: { transactionId?: string } }>()

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const isCreateMode = $derived(!params.transactionId)
  const financeShell = useFinanceShellState()

  let loading = $state(true)
  let saving = $state(false)
  let error = $state<string | null>(null)
  let saveMessage = $state<string | null>(null)
  let accounts = $state<FinanceAccount[]>([])
  let categories = $state<FinanceCategory[]>([])
  let tags = $state<FinanceTag[]>([])
  let transaction = $state<FinanceTransaction | null>(null)
  let form = $state(makeBlankForm())
  let reactiveReady = $state(false)
  let skipNextReactiveLoad = false

  const stateFlags = $derived(makeTransactionFlags())

  onMount(() => {
    void loadPage()
  })

  function makeBlankForm() {
    return {
      accountId: '',
      source: 'manual',
      status: 'booked',
      kind: 'expense',
      amountMinor: '0',
      currency: 'USD',
      description: '',
      effectiveAt: localTodayDateTimeValue(),
      categoryId: '',
      tagIds: [] as string[],
      transferGroupId: '',
    }
  }

  function fillFormFromTransaction(item: FinanceTransaction) {
    form = {
      accountId: item.accountId,
      source: item.source,
      status: item.status,
      kind: item.kind,
      amountMinor: String(item.amountMinor),
      currency: item.currency,
      description: item.description,
      effectiveAt: toDateTimeLocalValue(item.effectiveAt),
      categoryId: item.categoryId ?? '',
      tagIds: [...item.tagIds],
      transferGroupId: item.transferGroupId ?? '',
    }
  }

  function makeTransactionFlags(): string[] {
    if (transaction) {
      return [
        transaction.status === 'pending' ? 'pending' : '',
        transaction.hiddenAt ? 'hidden' : '',
        transaction.transferGroupId ? 'transfer' : '',
        transaction.kind === 'refund' ? 'refund' : '',
        transaction.kind === 'reconciliation' ? 'reconciliation' : '',
      ].filter(Boolean)
    }

    return [
      form.status === 'pending' ? 'pending' : '',
      form.transferGroupId ? 'transfer' : '',
      form.kind === 'refund' ? 'refund' : '',
      form.kind === 'reconciliation' ? 'reconciliation' : '',
    ].filter(Boolean)
  }

  function flagBadgeClass(flag: string): string {
    if (flag === 'pending') return 'text-bg-warning'
    if (flag === 'hidden') return 'text-bg-secondary'
    if (flag === 'refund') return 'text-bg-success'
    if (flag === 'reconciliation') return 'text-bg-primary'
    return 'text-bg-light border text-body'
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
    ;[accounts, categories, tags] = await Promise.all([
      financeApi.listAccounts({ tenantId: financeShell.selectedTenantId }),
      financeApi.listCategories({ tenantId: financeShell.selectedTenantId }),
      financeApi.listTags({ tenantId: financeShell.selectedTenantId, includeHidden: true }),
    ])

    if (isCreateMode) {
      transaction = null
      form = makeBlankForm()
      if (accounts.length > 0) {
        form.accountId = accounts[0].id
        form.currency = accounts[0].currency
      }
      return
    }

    transaction = await financeApi.getTransaction({
      tenantId: financeShell.selectedTenantId,
      transactionId: params.transactionId ?? '',
    })
    fillFormFromTransaction(transaction)
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
    error = null
    saveMessage = null

    try {
      if (isCreateMode) {
        const created = await financeApi.createTransaction({
          tenantId: financeShell.selectedTenantId,
          accountId: form.accountId,
          source: form.source,
          status: form.status,
          kind: form.kind,
          amountMinor: Number(form.amountMinor),
          currency: form.currency,
          description: form.description,
          effectiveAt: fromDateTimeLocalValue(form.effectiveAt),
          categoryId: form.categoryId || undefined,
          tagIds: form.tagIds,
          transferGroupId: form.transferGroupId || undefined,
        })
        transaction = created
        saveMessage = 'Transaction recorded.'
        fillFormFromTransaction(created)
      } else {
        transaction = await financeApi.updateTransaction({
          tenantId: financeShell.selectedTenantId,
          transactionId: params.transactionId ?? '',
          description: form.description,
          amountMinor: Number(form.amountMinor),
          effectiveAt: fromDateTimeLocalValue(form.effectiveAt),
          categoryId: form.categoryId || null,
          tagIds: form.tagIds,
        })
        saveMessage = 'Transaction updated.'
        if (transaction) {
          fillFormFromTransaction(transaction)
        }
      }
    } catch (saveError) {
      error = saveError instanceof Error ? saveError.message : 'Failed to save transaction'
    } finally {
      saving = false
    }
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
          <h1 id="finance-transaction-editor-heading" class="h3 mb-2">
            {#if isCreateMode}Record transaction{:else}Edit transaction{/if}
          </h1>
          <p class="text-body-secondary mb-0">
            {#if isCreateMode}
              Dedicated single-record entry screen for finance transactions.
            {:else}
              Focused edit route with provider-original context preserved beside operator-controlled reporting fields.
            {/if}
          </p>
        </div>

        <a class="btn btn-outline-secondary align-self-start align-self-lg-center" href="/finance/transactions" use:link>
          Back to transactions
        </a>
      </div>
    </header>

    {#if error}
      <div class="alert alert-danger mb-0" role="alert">{error}</div>
    {/if}

    {#if saveMessage}
      <div class="alert alert-success mb-0" role="status">{saveMessage}</div>
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

      <section class="card shadow-sm">
        <div class="card-body p-4 d-flex flex-column flex-lg-row justify-content-between gap-3 align-items-lg-center">
          <div>
            <h2 class="h5 mb-1">Transaction context</h2>
            <p class="text-body-secondary mb-0">
              {#if transaction}
                {transaction.source} · {transaction.status} · {transaction.kind} · {transaction.currency}
              {:else}
                {form.source} · {form.status} · {form.kind} · {form.currency}
              {/if}
            </p>
            {#if transaction?.tagIds.length}
              <p class="small text-body-secondary mb-0 mt-2">
                Tags: {transaction.tagIds.map((tagId) => tags.find((tag) => tag.id === tagId)?.name ?? 'Unknown tag').join(', ')}
              </p>
            {/if}
          </div>

          <div class="d-flex flex-wrap gap-2" aria-label="Transaction state flags">
            {#each stateFlags as flag (flag)}
              <span class={`badge ${flagBadgeClass(flag)}`}>{flag}</span>
            {/each}
          </div>
        </div>
      </section>

      {#if transaction?.providerOriginal}
        <section class="card shadow-sm">
          <div class="card-body p-4 d-grid gap-3">
            <div>
              <h2 class="h5 mb-1">Provider original</h2>
              <p class="text-body-secondary mb-0">Original synced values remain visible next to editable reporting fields.</p>
            </div>

            <div class="row g-3">
              <div class="col-12"><strong>Description</strong><div>{transaction.providerOriginal.description || '—'}</div></div>
              <div class="col-12 col-md-6"><strong>Amount</strong><div>{formatFinanceMoney(transaction.providerOriginal.amountMinor, transaction.providerOriginal.currency)}</div></div>
              <div class="col-12 col-md-6"><strong>Effective</strong><div>{transaction.providerOriginal.effectiveAt ? formatFinanceDateTime(transaction.providerOriginal.effectiveAt) : '—'}</div></div>
            </div>
          </div>
        </section>
      {/if}

      <form class="card shadow-sm" onsubmit={saveTransaction}>
        <div class="card-body p-4 d-grid gap-4">
          <div>
            <h2 class="h5 mb-1">{#if isCreateMode}Create details{:else}Editable reporting fields{/if}</h2>
            <p class="text-body-secondary mb-0">
              Shared single-record editor for both dedicated create and dedicated edit routes.
            </p>
          </div>

          <div class="row g-3">
            <div class="col-12 col-md-6">
              <label class="form-label" for="finance-transaction-account">Account</label>
              <select id="finance-transaction-account" class="form-select" bind:value={form.accountId} onchange={syncCurrencyWithAccount} aria-label="Transaction account" disabled={!isCreateMode} required>
                <option value="">Select account</option>
                {#each accounts as account (account.id)}
                  <option value={account.id}>{account.name}</option>
                {/each}
              </select>
            </div>

            <div class="col-12 col-md-6">
              <label class="form-label" for="finance-transaction-category">Category</label>
              <select id="finance-transaction-category" class="form-select" bind:value={form.categoryId} aria-label="Transaction category">
                <option value="">No category</option>
                {#each categories as category (category.id)}
                  <option value={category.id}>{category.name}</option>
                {/each}
              </select>
            </div>

            <fieldset class="col-12">
              <legend class="form-label mb-2">Tags</legend>
              {#if tags.length === 0}
                <p class="form-text mb-0">No tenant tags are available. Manage tags from Categories.</p>
              {:else}
                <div class="d-flex flex-wrap gap-3" aria-label="Transaction tags">
                  {#each tags as tag (tag.id)}
                    <div class="form-check">
                      <input id={`finance-transaction-tag-${tag.id}`} class="form-check-input" type="checkbox" value={tag.id} bind:group={form.tagIds} disabled={Boolean(tag.hiddenAt) && !form.tagIds.includes(tag.id)} />
                      <label class="form-check-label" for={`finance-transaction-tag-${tag.id}`}>{tag.name}{tag.hiddenAt ? ' (hidden)' : ''}</label>
                    </div>
                  {/each}
                </div>
              {/if}
              <p class="form-text mb-0">Choose existing tenant tags. Create tags from Categories.</p>
            </fieldset>

            <div class="col-12 col-md-4">
              <label class="form-label" for="finance-transaction-kind">Kind</label>
              <select id="finance-transaction-kind" class="form-select" bind:value={form.kind} aria-label="Transaction kind" disabled={!isCreateMode}>
                <option value="expense">expense</option>
                <option value="income">income</option>
                <option value="refund">refund</option>
                <option value="transfer">transfer</option>
                <option value="reconciliation">reconciliation</option>
              </select>
            </div>

            <div class="col-12 col-md-4">
              <label class="form-label" for="finance-transaction-amount">Amount minor</label>
              <input id="finance-transaction-amount" class="form-control" bind:value={form.amountMinor} aria-label="Amount minor" type="number" required />
            </div>

            <div class="col-12 col-md-4">
              <label class="form-label" for="finance-transaction-currency">Currency</label>
              <select id="finance-transaction-currency" class="form-select" bind:value={form.currency} aria-label="Transaction currency" disabled={!isCreateMode} required>
                {#each supportedFinanceTenantDisplayCurrencies as currencyCode (currencyCode)}
                  <option value={currencyCode}>{currencyCode}</option>
                {/each}
              </select>
            </div>

            <div class="col-12">
              <label class="form-label" for="finance-transaction-description">Description</label>
              <input id="finance-transaction-description" class="form-control" bind:value={form.description} aria-label="Transaction description" />
            </div>

            <div class="col-12 col-lg-6">
              <label class="form-label" for="finance-transaction-effective-at">Effective at</label>
              <input id="finance-transaction-effective-at" class="form-control" bind:value={form.effectiveAt} aria-label="Transaction effective at" type="datetime-local" required />
            </div>

            <div class="col-12 col-md-4">
              <label class="form-label" for="finance-transaction-status">Status</label>
              <select id="finance-transaction-status" class="form-select" bind:value={form.status} aria-label="Transaction status" disabled={!isCreateMode}>
                <option value="booked">booked</option>
                <option value="pending">pending</option>
              </select>
            </div>

            <div class="col-12 col-md-4">
              <label class="form-label" for="finance-transaction-source">Source</label>
              <select id="finance-transaction-source" class="form-select" bind:value={form.source} aria-label="Transaction source" disabled={!isCreateMode}>
                <option value="manual">manual</option>
                <option value="provider">provider</option>
                <option value="csv">csv</option>
                <option value="system">system</option>
              </select>
            </div>

            <div class="col-12 col-md-4">
              <label class="form-label" for="finance-transaction-transfer-group">Transfer group</label>
              <input id="finance-transaction-transfer-group" class="form-control" bind:value={form.transferGroupId} aria-label="Transfer group" disabled={!isCreateMode} />
            </div>
          </div>

          <div class="d-flex flex-wrap gap-2">
            <button class="btn btn-primary" type="submit" disabled={saving || !financeShell.selectedTenantId}>
              {#if saving}Saving…{:else}Save transaction{/if}
            </button>
            <a class="btn btn-outline-secondary" href="/finance/transactions" use:link>Cancel</a>
          </div>
        </div>
      </form>
    {/if}
  </div>
</section>
