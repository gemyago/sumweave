<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import FinanceSubnav from '../components/FinanceSubnav.svelte'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { createSignalFinanceApiForAuth, type FinanceDashboard, type FinanceTenantSummary } from '../lib/finance/api'
  import { chooseFinanceTenantId, setPreferredFinanceTenantId } from '../lib/finance/tenant-selection'
  import { formatFinanceDate, formatFinanceMoney } from '../lib/finance/format'

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))

  let loading = $state(true)
  let error = $state<string | null>(null)
  let tenants = $state<FinanceTenantSummary[]>([])
  let selectedTenantId = $state('')
  let dashboard = $state<FinanceDashboard | null>(null)
  let loadingDashboard = $state(false)
  let dashboardPreset = $state('current_month')
  let customStartDate = $state('')
  let customEndDate = $state('')
  const needsTenantSelection = $derived(tenants.length > 1 && !selectedTenantId)

  const selectedTenant = $derived(tenants.find((item) => item.id === selectedTenantId) ?? null)

  onMount(() => {
    void loadPage()
  })

  async function loadPage() {
    loading = true
    error = null
    try {
      tenants = await financeApi.listTenants()
      selectedTenantId = chooseFinanceTenantId(tenants)
      if (selectedTenantId) {
        setPreferredFinanceTenantId(selectedTenantId)
        await loadDashboard()
      } else {
        dashboard = null
      }
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load finance workspace'
    } finally {
      loading = false
    }
  }

  async function loadDashboard(overrides: { preset?: string; startDate?: string; endDate?: string } = {}) {
    if (!selectedTenantId) {
      dashboard = null
      return
    }
    loadingDashboard = true
    error = null
    try {
      const loaded = await financeApi.getDashboard({
        tenantId: selectedTenantId,
        preset: overrides.preset ?? dashboardPreset,
        startDate: overrides.startDate ?? customStartDate,
        endDate: overrides.endDate ?? customEndDate,
      })
      dashboard = loaded
      dashboardPreset = loaded.period.preset
      customStartDate = loaded.period.startDate.toISOString().slice(0, 10)
      customEndDate = loaded.period.endDate.toISOString().slice(0, 10)
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load dashboard'
    } finally {
      loadingDashboard = false
    }
  }

  async function onTenantChange() {
    setPreferredFinanceTenantId(selectedTenantId)
    await loadDashboard()
  }

  async function openPreviousPeriod() {
    if (!dashboard) return
    dashboardPreset = ''
    customStartDate = dashboard.period.previous.startDate.toISOString().slice(0, 10)
    customEndDate = dashboard.period.previous.endDate.toISOString().slice(0, 10)
    await loadDashboard({ startDate: customStartDate, endDate: customEndDate, preset: '' })
  }

  async function openNextPeriod() {
    if (!dashboard) return
    dashboardPreset = ''
    customStartDate = dashboard.period.next.startDate.toISOString().slice(0, 10)
    customEndDate = dashboard.period.next.endDate.toISOString().slice(0, 10)
    await loadDashboard({ startDate: customStartDate, endDate: customEndDate, preset: '' })
  }

  async function applyCustomRange(event: SubmitEvent) {
    event.preventDefault()
    dashboardPreset = ''
    await loadDashboard({ preset: '', startDate: customStartDate, endDate: customEndDate })
  }
</script>

<section class="page" aria-labelledby="finance-heading">
  <header class="page-header">
    <div>
      <h1 id="finance-heading">Finance</h1>
      <p class="muted">Tenant-aware finance workspace with period KPIs, alerts, account balances, and import or sync follow-through.</p>
    </div>
  </header>

  <FinanceSubnav current="/finance" tenantName={selectedTenant?.name ?? ''} />

  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}

  {#if loading}
    <p class="muted" role="status">Loading finance workspace…</p>
  {:else}
    <section class="panel controls-panel">
      <label>
        <span>Tenant</span>
        <select bind:value={selectedTenantId} onchange={() => void onTenantChange()} aria-label="Tenant">
          <option value="">Select tenant</option>
          {#if tenants.length === 0}
            <option value="">No tenants yet</option>
          {/if}
          {#each tenants as tenant (tenant.id)}
            <option value={tenant.id}>{tenant.name} · {tenant.displayCurrency}</option>
          {/each}
        </select>
      </label>

      {#if dashboard}
        <div class="period-actions">
          <button class="secondary" type="button" onclick={() => void openPreviousPeriod()}>Previous period</button>
          <button class="secondary" type="button" onclick={() => { dashboardPreset = 'current_month'; void loadDashboard({ preset: 'current_month' }) }}>Current month</button>
          <button class="secondary" type="button" onclick={() => void openNextPeriod()}>Next period</button>
        </div>
      {/if}

      <form class="custom-range" onsubmit={applyCustomRange}>
        <label>
          <span>Custom start date</span>
          <input type="date" bind:value={customStartDate} aria-label="Custom start date" />
        </label>
        <label>
          <span>Custom end date</span>
          <input type="date" bind:value={customEndDate} aria-label="Custom end date" />
        </label>
        <button class="primary" type="submit" disabled={!selectedTenantId}>Apply custom range</button>
      </form>
    </section>

    {#if !selectedTenantId}
      <section class="panel">
        {#if needsTenantSelection}
          <p>Select an active tenant to continue on this finance route.</p>
        {:else}
          <p>Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a> before loading the dashboard.</p>
        {/if}
      </section>
    {:else if loadingDashboard}
      <p class="muted" role="status">Loading tenant dashboard…</p>
    {:else if dashboard}
      <section class="panel">
        <h2>Reporting period</h2>
        <p class="muted">{formatFinanceDate(dashboard.period.startDate)} → {formatFinanceDate(dashboard.period.endDate)} · preset {dashboard.period.preset || 'custom'}</p>
      </section>

      <section class="kpi-grid" aria-label="Finance KPIs">
        <article class="panel">
          <h2>Settled net</h2>
          <p>{formatFinanceMoney(dashboard.settled.netMinor, dashboard.settled.displayCurrency)}</p>
          <p class="muted">Income {formatFinanceMoney(dashboard.settled.incomeMinor, dashboard.settled.displayCurrency)} · Expense {formatFinanceMoney(dashboard.settled.expenseMinor, dashboard.settled.displayCurrency)}</p>
        </article>
        <article class="panel">
          <h2>Pending net</h2>
          <p>{formatFinanceMoney(dashboard.pending.netMinor, dashboard.pending.displayCurrency)}</p>
          <p class="muted">{dashboard.pending.transactionCount} pending transactions</p>
        </article>
        <article class="panel">
          <h2>Alerts</h2>
          <p>{dashboard.alerts.length}</p>
          <p class="muted">Sync, import, and missing-FX cues stay visible here.</p>
        </article>
      </section>

      <section class="panel">
        <h2>Alerts and missing FX</h2>
        {#if dashboard.alerts.length === 0 && dashboard.missingFx.length === 0}
          <p class="muted">No active dashboard alerts.</p>
        {:else}
          <div class="stack">
            {#each dashboard.alerts as alert (`${alert.code}-${alert.severity}`)}
              <article class="card-row">
                <strong>{alert.code}</strong>
                <span>{alert.severity}</span>
                <span>{alert.count}</span>
              </article>
            {/each}
            {#each dashboard.missingFx as missing (`${missing.transactionId}-${missing.baseCurrency}-${missing.rateDate.toISOString()}`)}
              <article class="card-row">
                <strong>Missing FX</strong>
                <span>{missing.baseCurrency}/{missing.quoteCurrency}</span>
                <span>{formatFinanceDate(missing.rateDate)}</span>
              </article>
            {/each}
          </div>
          {#if dashboard.missingFx.length > 0}
            <p><a href="/admin/finance/fx" use:link>Open admin FX diagnostics</a></p>
          {/if}
        {/if}
      </section>

      <section class="panel">
        <div class="section-header">
          <h2>Account balances</h2>
          <a href="/finance/accounts" use:link>Open accounts</a>
        </div>
        {#if dashboard.accountBalances.length === 0}
          <p class="muted">No accounts yet for this tenant.</p>
        {:else}
          <div class="stack">
            {#each dashboard.accountBalances as account (account.accountId)}
              <article class="card-row">
                <div>
                  <strong>{account.accountName}</strong>
                  <p class="muted">{account.currency}</p>
                </div>
                <div>
                  <p>{formatFinanceMoney(account.displayBookedMinor ?? account.nativeBookedMinor, dashboard.settled.displayCurrency)}</p>
                  <p class="muted">Pending {formatFinanceMoney(account.displayPendingMinor ?? account.nativePendingMinor, dashboard.settled.displayCurrency)}</p>
                </div>
              </article>
            {/each}
          </div>
        {/if}
      </section>

      <section class="panel">
        <div class="section-header">
          <h2>Category breakdowns</h2>
          <a href="/finance/transactions" use:link>Open transactions</a>
        </div>
        {#if dashboard.categoryBreakdowns.length === 0}
          <p class="muted">No category activity for this period.</p>
        {:else}
          <div class="stack">
            {#each dashboard.categoryBreakdowns as category (category.categoryId)}
              <article class="card-row">
                <strong>{category.categoryName}</strong>
                <span>{category.kind}</span>
                <span>{category.transactionCount} tx</span>
              </article>
            {/each}
          </div>
        {/if}
      </section>
    {/if}
  {/if}
</section>

<style>
  .page, .stack { display: flex; flex-direction: column; gap: var(--space-16); }
  .page-header h1, .panel h2 { margin: 0; }
  .panel { display: flex; flex-direction: column; gap: var(--space-12); padding: var(--space-16); border: 1px solid var(--border); border-radius: 4px; background: var(--bg-elevated, var(--bg)); }
  .controls-panel, .custom-range { display: grid; gap: var(--space-16); }
  .custom-range { grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); align-items: end; }
  .controls-panel label, .custom-range label { display: flex; flex-direction: column; gap: var(--space-8); }
  .period-actions, .section-header { display: flex; flex-wrap: wrap; gap: var(--space-12); align-items: center; justify-content: space-between; }
  .kpi-grid { display: grid; gap: var(--space-16); grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); }
  .card-row { display: flex; justify-content: space-between; gap: var(--space-16); align-items: flex-start; padding: var(--space-12); border: 1px solid var(--border); border-radius: 4px; }
  .muted { margin: 0; color: var(--text-muted); }
  .error { color: var(--color-danger-red); }
</style>
