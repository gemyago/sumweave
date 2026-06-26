<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import FinanceSubnav from '../components/FinanceSubnav.svelte'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalFinanceApiForAuth,
    type FinanceAccount,
    type FinanceCategory,
    type FinanceTenantSummary,
    type FinanceTransaction,
  } from '../lib/finance/api'
  import { formatFinanceDateTime } from '../lib/finance/format'
  import { chooseFinanceTenantId, setPreferredFinanceTenantId } from '../lib/finance/tenant-selection'

  let { params = {} } = $props<{ params?: { transactionId?: string } }>()

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const isCreateMode = $derived(!params.transactionId)

  let loading = $state(true)
  let saving = $state(false)
  let error = $state<string | null>(null)
  let saveMessage = $state<string | null>(null)
  let tenants = $state<FinanceTenantSummary[]>([])
  let selectedTenantId = $state('')
  let accounts = $state<FinanceAccount[]>([])
  let categories = $state<FinanceCategory[]>([])
  let transaction = $state<FinanceTransaction | null>(null)
  let form = $state(makeBlankForm())

  const selectedTenant = $derived(tenants.find((item) => item.id === selectedTenantId) ?? null)
  const needsTenantSelection = $derived(tenants.length > 1 && !selectedTenantId)
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
      effectiveAt: '',
      categoryId: '',
      transferGroupId: '',
    }
  }

  function toDateTimeLocalValue(value: Date | null): string {
    if (!value) {
      return ''
    }
    const offset = value.getTimezoneOffset() * 60_000
    return new Date(value.getTime() - offset).toISOString().slice(0, 16)
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

  async function loadPage() {
    loading = true
    error = null
    saveMessage = null
    try {
      tenants = await financeApi.listTenants()
      selectedTenantId = chooseFinanceTenantId(tenants)
      if (!selectedTenantId) {
        accounts = []
        categories = []
        transaction = null
        form = makeBlankForm()
        return
      }
      setPreferredFinanceTenantId(selectedTenantId)
      await loadEditorData()
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load transaction editor'
    } finally {
      loading = false
    }
  }

  async function loadEditorData() {
    if (!selectedTenantId) {
      return
    }
    saveMessage = null
    ;[accounts, categories] = await Promise.all([
      financeApi.listAccounts({ tenantId: selectedTenantId }),
      financeApi.listCategories({ tenantId: selectedTenantId }),
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
      tenantId: selectedTenantId,
      transactionId: params.transactionId ?? '',
    })
    fillFormFromTransaction(transaction)
  }

  async function onTenantChange() {
    setPreferredFinanceTenantId(selectedTenantId)
    await loadEditorData()
  }

  function syncCurrencyWithAccount() {
    const account = accounts.find((item) => item.id === form.accountId)
    if (account && isCreateMode) {
      form.currency = account.currency
    }
  }

  async function saveTransaction(event: SubmitEvent) {
    event.preventDefault()
    if (!selectedTenantId) {
      return
    }
    saving = true
    error = null
    saveMessage = null
    try {
      if (isCreateMode) {
        const created = await financeApi.createTransaction({
          tenantId: selectedTenantId,
          accountId: form.accountId,
          source: form.source,
          status: form.status,
          kind: form.kind,
          amountMinor: Number(form.amountMinor),
          currency: form.currency,
          description: form.description,
          effectiveAt: new Date(form.effectiveAt),
          categoryId: form.categoryId || undefined,
          transferGroupId: form.transferGroupId || undefined,
        })
        transaction = created
        saveMessage = 'Transaction recorded.'
        fillFormFromTransaction(created)
      } else {
        transaction = await financeApi.updateTransaction({
          tenantId: selectedTenantId,
          transactionId: params.transactionId ?? '',
          description: form.description,
          amountMinor: Number(form.amountMinor),
          effectiveAt: new Date(form.effectiveAt),
          categoryId: form.categoryId || null,
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
</script>

<section class="page" aria-labelledby="finance-transaction-editor-heading">
  <header class="hero">
    <div>
      <h1 id="finance-transaction-editor-heading">
        {#if isCreateMode}Record transaction{:else}Edit transaction{/if}
      </h1>
      <p class="muted">
        {#if isCreateMode}
          Dedicated single-record entry screen for finance transactions.
        {:else}
          Focused edit route with provider-original context preserved alongside user-controlled reporting fields.
        {/if}
      </p>
    </div>
    <a href="/finance/transactions" use:link>Back to transactions</a>
  </header>

  <FinanceSubnav current="/finance/transactions" tenantName={selectedTenant?.name ?? ''} />

  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}
  {#if saveMessage}
    <p class="success" role="status">{saveMessage}</p>
  {/if}

  {#if loading}
    <p class="muted" role="status">Loading transaction editor…</p>
  {:else}
    <section class="panel">
      <label>
        <span>Tenant</span>
        <select bind:value={selectedTenantId} onchange={() => void onTenantChange()} aria-label="Tenant">
          <option value="">Select tenant</option>
          {#each tenants as tenant (tenant.id)}
            <option value={tenant.id}>{tenant.name}</option>
          {/each}
        </select>
      </label>
    </section>

    {#if needsTenantSelection}
      <section class="panel">
        <p>Select an active tenant to continue on this finance route.</p>
      </section>
    {:else if !selectedTenantId}
      <section class="panel">
        <p>Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a> before editing transactions.</p>
      </section>
    {:else}
      <section class="panel context-panel">
        <div>
          <h2>Transaction context</h2>
          <p class="muted">
            {#if transaction}
              {transaction.source} · {transaction.status} · {transaction.kind} · {transaction.currency}
            {:else}
              {form.source} · {form.status} · {form.kind} · {form.currency}
            {/if}
          </p>
        </div>
        <div class="flags" aria-label="Transaction state flags">
          {#each stateFlags as flag (flag)}
            <span>{flag}</span>
          {/each}
        </div>
      </section>

      {#if transaction?.providerOriginal}
        <section class="panel provider-panel">
          <h2>Provider original</h2>
          <p class="muted">Description {transaction.providerOriginal.description}</p>
          <p class="muted">
            Amount {transaction.providerOriginal.currency} {transaction.providerOriginal.amountMinor}
          </p>
          <p class="muted">
            Effective
            {transaction.providerOriginal.effectiveAt
              ? formatFinanceDateTime(transaction.providerOriginal.effectiveAt)
              : '—'}
          </p>
        </section>
      {/if}

      <form class="panel editor-form" onsubmit={saveTransaction}>
        <h2>{#if isCreateMode}Create details{:else}Editable reporting fields{/if}</h2>

        <label>
          <span>Account</span>
          <select
            bind:value={form.accountId}
            onchange={syncCurrencyWithAccount}
            aria-label="Transaction account"
            disabled={!isCreateMode}
            required
          >
            <option value="">Select account</option>
            {#each accounts as account (account.id)}
              <option value={account.id}>{account.name}</option>
            {/each}
          </select>
        </label>

        <label>
          <span>Category</span>
          <select bind:value={form.categoryId} aria-label="Transaction category">
            <option value="">No category</option>
            {#each categories as category (category.id)}
              <option value={category.id}>{category.name}</option>
            {/each}
          </select>
        </label>

        <label>
          <span>Kind</span>
          <select bind:value={form.kind} aria-label="Transaction kind" disabled={!isCreateMode}>
            <option value="expense">expense</option>
            <option value="income">income</option>
            <option value="refund">refund</option>
            <option value="transfer">transfer</option>
            <option value="reconciliation">reconciliation</option>
          </select>
        </label>

        <label>
          <span>Status</span>
          <select bind:value={form.status} aria-label="Transaction status" disabled={!isCreateMode}>
            <option value="booked">booked</option>
            <option value="pending">pending</option>
          </select>
        </label>

        <label>
          <span>Source</span>
          <select bind:value={form.source} aria-label="Transaction source" disabled={!isCreateMode}>
            <option value="manual">manual</option>
            <option value="provider">provider</option>
            <option value="csv">csv</option>
            <option value="system">system</option>
          </select>
        </label>

        <label>
          <span>Currency</span>
          <input bind:value={form.currency} aria-label="Transaction currency" disabled={!isCreateMode} required />
        </label>

        <label>
          <span>Amount minor</span>
          <input bind:value={form.amountMinor} aria-label="Amount minor" type="number" required />
        </label>

        <label>
          <span>Description</span>
          <input bind:value={form.description} aria-label="Transaction description" />
        </label>

        <label>
          <span>Effective at</span>
          <input bind:value={form.effectiveAt} aria-label="Transaction effective at" type="datetime-local" required />
        </label>

        <label>
          <span>Transfer group</span>
          <input bind:value={form.transferGroupId} aria-label="Transfer group" disabled={!isCreateMode} />
        </label>

        <div class="action-row">
          <button class="primary" type="submit" disabled={saving || !selectedTenantId}>
            {#if saving}Saving…{:else}Save transaction{/if}
          </button>
          <a href="/finance/transactions" use:link>Cancel</a>
        </div>
      </form>
    {/if}
  {/if}
</section>

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
  }

  .hero,
  .context-panel,
  .action-row {
    display: flex;
    gap: var(--space-16);
  }

  .hero,
  .context-panel {
    justify-content: space-between;
    align-items: flex-start;
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

  .editor-form {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: var(--space-16);
  }

  .flags {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-8);
  }

  .flags span {
    padding: 2px 6px;
    border: 1px solid var(--border);
    border-radius: 4px;
  }

  .action-row {
    align-items: center;
    flex-wrap: wrap;
  }

  .hero h1,
  .panel h2 {
    margin: 0;
  }

  .muted {
    margin: 0;
    color: var(--text-muted);
  }

  .error {
    color: var(--color-danger-red);
  }

  .success {
    color: var(--color-success-green);
  }

  @media (max-width: 640px) {
    .hero,
    .context-panel,
    .action-row {
      flex-direction: column;
      align-items: stretch;
    }
  }
</style>
