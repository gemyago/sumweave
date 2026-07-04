<script lang="ts">
  import { onMount } from 'svelte'
  import { link } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalFinanceApiForAuth,
    type FinanceBankConnection,
    type FinanceDashboard,
    type FinanceTransaction,
  } from '../lib/finance/api'
  import { formatFinanceDate, formatFinanceDateTime, formatFinanceMoney } from '../lib/finance/format'
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'

  type ChartTone = 'accent' | 'danger' | 'success' | 'warning'

  interface ColumnChartItem {
    key: string
    label: string
    value: number
    formattedValue: string
    tone: ChartTone
  }

  interface RowChartItem {
    key: string
    label: string
    meta: string
    value: number
    formattedValue: string
    tone: ChartTone
  }

  interface AttentionItem {
    key: string
    title: string
    detail: string
    value: string
    tone: ChartTone
    href?: string
    hrefLabel?: string
  }

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const ACCOUNT_SECTION_LIMIT = 4
  const CATEGORY_SECTION_LIMIT = 4
  const TRANSACTION_SECTION_LIMIT = 5
  const CONNECTION_SECTION_LIMIT = 4

  let loading = $state(true)
  let error = $state<string | null>(null)
  let dashboard = $state<FinanceDashboard | null>(null)
  let recentTransactions = $state<FinanceTransaction[]>([])
  let recentConnections = $state<FinanceBankConnection[]>([])
  let loadingDashboard = $state(false)
  let dashboardPreset = $state('current_month')
  let customStartDate = $state('')
  let customEndDate = $state('')
  let reactiveReady = $state(false)
  let skipNextReactiveLoad = false
  const financeShell = useFinanceShellState()

  function maxMagnitude(values: number[]): number {
    return values.reduce((maximum, value) => Math.max(maximum, Math.abs(value)), 0)
  }

  function scaledPercent(value: number, maximum: number, minimum = 12): string {
    if (maximum <= 0) {
      return '0%'
    }

    return `${Math.max(minimum, Math.round((Math.abs(value) / maximum) * 100))}%`
  }

  function moneyTone(value: number): ChartTone {
    if (value < 0) {
      return 'danger'
    }

    if (value > 0) {
      return 'success'
    }

    return 'accent'
  }

  function severityTone(severity: string): ChartTone {
    if (severity === 'error') {
      return 'danger'
    }

    if (severity === 'warning') {
      return 'warning'
    }

    if (severity === 'success') {
      return 'success'
    }

    return 'accent'
  }

  function formatAttentionLabel(code: string): string {
    const label = code
      .split('_')
      .filter(Boolean)
      .map((segment) => segment.toLowerCase())
      .join(' ')

    return `${label.slice(0, 1).toUpperCase()}${label.slice(1)}`
  }

  const cashFlowChartItems = $derived.by<ColumnChartItem[]>(() => {
    if (!dashboard) {
      return []
    }

    const currency = dashboard.settled.displayCurrency

    return [
      {
        key: 'settled-income',
        label: 'Settled income',
        value: dashboard.settled.incomeMinor,
        formattedValue: formatFinanceMoney(dashboard.settled.incomeMinor, currency),
        tone: 'success',
      },
      {
        key: 'settled-expense',
        label: 'Settled expense',
        value: dashboard.settled.expenseMinor,
        formattedValue: formatFinanceMoney(dashboard.settled.expenseMinor, currency),
        tone: 'danger',
      },
      {
        key: 'pending-income',
        label: 'Pending income',
        value: dashboard.pending.incomeMinor,
        formattedValue: formatFinanceMoney(dashboard.pending.incomeMinor, currency),
        tone: 'accent',
      },
      {
        key: 'pending-expense',
        label: 'Pending expense',
        value: dashboard.pending.expenseMinor,
        formattedValue: formatFinanceMoney(dashboard.pending.expenseMinor, currency),
        tone: 'warning',
      },
    ]
  })

  const cashFlowChartMaximum = $derived.by(() =>
    maxMagnitude(cashFlowChartItems.map((item) => item.value)),
  )

  const cashFlowNetItems = $derived.by<RowChartItem[]>(() => {
    if (!dashboard) {
      return []
    }

    const currency = dashboard.settled.displayCurrency

    return [
      {
        key: 'settled-net',
        label: 'Settled net',
        meta: `${dashboard.settled.transactionCount} booked tx`,
        value: dashboard.settled.netMinor,
        formattedValue: formatFinanceMoney(dashboard.settled.netMinor, currency),
        tone: moneyTone(dashboard.settled.netMinor),
      },
      {
        key: 'pending-net',
        label: 'Pending net',
        meta: `${dashboard.pending.transactionCount} pending tx`,
        value: dashboard.pending.netMinor,
        formattedValue: formatFinanceMoney(dashboard.pending.netMinor, currency),
        tone: moneyTone(dashboard.pending.netMinor),
      },
    ]
  })

  const accountChartItems = $derived.by<RowChartItem[]>(() => {
    if (!dashboard) {
      return []
    }

    const displayCurrency = dashboard.settled.displayCurrency

    return dashboard.accountBalances
      .map((account) => {
        const bookedMinor = account.displayBookedMinor ?? account.nativeBookedMinor
        const tone: ChartTone = account.missingFx ? 'warning' : moneyTone(bookedMinor)

        return {
          key: account.accountId,
          label: account.accountName,
          meta: `${account.currency}${account.missingFx ? ' · Missing FX' : ''}`,
          value: bookedMinor,
          formattedValue: formatFinanceMoney(bookedMinor, displayCurrency),
          tone,
        }
      })
      .sort((left, right) => Math.abs(right.value) - Math.abs(left.value))
      .slice(0, ACCOUNT_SECTION_LIMIT)
  })

  const accountChartMaximum = $derived.by(() =>
    maxMagnitude(accountChartItems.map((item) => item.value)),
  )

  const categoryChartItems = $derived.by<RowChartItem[]>(() => {
    if (!dashboard) {
      return []
    }

    const displayCurrency = dashboard.settled.displayCurrency

    return dashboard.categoryBreakdowns
      .map((category) => {
        const value = category.kind === 'income' ? category.incomeMinor : category.expenseMinor
        const tone: ChartTone = category.kind === 'income' ? 'success' : 'warning'

        return {
          key: category.categoryId,
          label: category.categoryName,
          meta: `${category.kind} · ${category.transactionCount} tx`,
          value,
          formattedValue: formatFinanceMoney(value, displayCurrency),
          tone,
        }
      })
      .sort((left, right) => right.value - left.value)
      .slice(0, CATEGORY_SECTION_LIMIT)
  })

  const categoryChartMaximum = $derived.by(() =>
    maxMagnitude(categoryChartItems.map((item) => item.value)),
  )

  const balanceSummary = $derived.by(() => {
    if (!dashboard) {
      return null
    }

    const bookedMinor = dashboard.accountBalances.reduce(
      (total, account) => total + (account.displayBookedMinor ?? account.nativeBookedMinor),
      0,
    )
    const pendingMinor = dashboard.accountBalances.reduce(
      (total, account) => total + (account.displayPendingMinor ?? account.nativePendingMinor),
      0,
    )

    return {
      currency: dashboard.settled.displayCurrency,
      bookedMinor,
      pendingMinor,
      accountCount: dashboard.accountBalances.length,
    }
  })

  const visibleAccountBalances = $derived.by(() => {
    if (!dashboard) {
      return []
    }

    return [...dashboard.accountBalances]
      .sort(
        (left, right) =>
          Math.abs((right.displayBookedMinor ?? right.nativeBookedMinor))
          - Math.abs((left.displayBookedMinor ?? left.nativeBookedMinor)),
      )
      .slice(0, ACCOUNT_SECTION_LIMIT)
  })

  const visibleCategoryBreakdowns = $derived.by(() => {
    if (!dashboard) {
      return []
    }

    return [...dashboard.categoryBreakdowns]
      .sort((left, right) => {
        const leftValue = Math.max(left.expenseMinor, left.incomeMinor)
        const rightValue = Math.max(right.expenseMinor, right.incomeMinor)
        return rightValue - leftValue
      })
      .slice(0, CATEGORY_SECTION_LIMIT)
  })

  const visibleRecentTransactions = $derived.by(() =>
    recentTransactions.slice(0, TRANSACTION_SECTION_LIMIT),
  )

  const visibleRecentConnections = $derived.by(() =>
    recentConnections.slice(0, CONNECTION_SECTION_LIMIT),
  )

  const failedSyncConnections = $derived.by(() =>
    recentConnections.filter((connection) => connection.lastSyncError.trim().length > 0),
  )

  const attentionItems = $derived.by<AttentionItem[]>(() => {
    if (!dashboard) {
      return []
    }

    const activeDashboard = dashboard
    const items: AttentionItem[] = []

    if (activeDashboard.pending.transactionCount > 0) {
      items.push({
        key: 'pending-transactions',
        title: 'Pending transactions',
        detail: 'Unsettled activity still affects the active balance story.',
        value: `${activeDashboard.pending.transactionCount} pending`,
        tone: moneyTone(activeDashboard.pending.netMinor),
        href: '/finance/transactions',
        hrefLabel: 'Review transactions',
      })
    }

    if (activeDashboard.missingFx.length > 0) {
      items.push({
        key: 'missing-fx',
        title: 'Missing FX coverage',
        detail: `${activeDashboard.missingFx.length} transaction${activeDashboard.missingFx.length === 1 ? '' : 's'} need admin FX review.`,
        value: `${activeDashboard.missingFx.length} rate gap${activeDashboard.missingFx.length === 1 ? '' : 's'}`,
        tone: 'warning',
        href: '/admin/finance/fx',
        hrefLabel: 'Review in admin FX diagnostics',
      })
    }

    if (failedSyncConnections.length > 0) {
      items.push({
        key: 'failed-sync',
        title: 'Failed sync',
        detail:
          failedSyncConnections.length === 1
            ? `${failedSyncConnections[0].displayName} needs a retry.`
            : `${failedSyncConnections.length} connections need a retry.`,
        value: `${failedSyncConnections.length} sync issue${failedSyncConnections.length === 1 ? '' : 's'}`,
        tone: 'danger',
        href: '/finance/connections',
        hrefLabel: 'Review connections',
      })
    }

    activeDashboard.alerts.forEach((alert) => {
      const normalizedCode = alert.code.toLowerCase()

      if (failedSyncConnections.length > 0 && (normalizedCode.includes('connection') || normalizedCode.includes('sync'))) {
        return
      }

      if (activeDashboard.missingFx.length > 0 && normalizedCode.includes('fx')) {
        return
      }

      const importAlert = normalizedCode.includes('import')
      const connectionAlert = normalizedCode.includes('connection') || normalizedCode.includes('sync')
      const detail = importAlert
        ? 'Import follow-up still needs operator review.'
        : connectionAlert
          ? 'Connection health changed during this reporting window.'
          : 'Dashboard signal worth reviewing.'
      const href = importAlert
        ? '/finance/imports'
        : connectionAlert
          ? '/finance/connections'
          : undefined
      const hrefLabel = importAlert
        ? 'Review imports'
        : connectionAlert
          ? 'Review connections'
          : undefined

      items.push({
        key: `alert-${alert.code}-${alert.severity}`,
        title: formatAttentionLabel(alert.code),
        detail,
        value: `${alert.count} signal${alert.count === 1 ? '' : 's'}`,
        tone: severityTone(alert.severity),
        href,
        hrefLabel,
      })
    })

    return items
  })

  const hasAttentionItems = $derived.by(() => attentionItems.length > 0)

  onMount(() => {
    void loadPage()
  })

  async function loadPage() {
    loading = true
    reactiveReady = false
    error = null
    try {
      await financeShell.initialize()
      if (financeShell.selectedTenantId) {
        await loadDashboard()
      } else {
        dashboard = null
        recentTransactions = []
        recentConnections = []
      }
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load finance workspace'
    } finally {
      skipNextReactiveLoad = true
      reactiveReady = true
      loading = false
    }
  }

  async function loadDashboard(overrides: { preset?: string; startDate?: string; endDate?: string } = {}) {
    if (!financeShell.selectedTenantId) {
      dashboard = null
      recentTransactions = []
      recentConnections = []
      return
    }
    loadingDashboard = true
    error = null
    try {
      const [loaded, loadedTransactions, loadedConnections] = await Promise.all([
        financeApi.getDashboard({
          tenantId: financeShell.selectedTenantId,
          preset: overrides.preset ?? dashboardPreset,
          startDate: overrides.startDate ?? customStartDate,
          endDate: overrides.endDate ?? customEndDate,
        }),
        financeApi.listTransactions({ tenantId: financeShell.selectedTenantId, includeHidden: true }),
        financeApi.listConnections({ tenantId: financeShell.selectedTenantId }),
      ])
      dashboard = loaded
      recentTransactions = [...loadedTransactions]
        .sort((left, right) => right.effectiveAt.getTime() - left.effectiveAt.getTime())
        .slice(0, 5)
      recentConnections = [...loadedConnections].sort((left, right) => {
        const leftTime = left.lastSyncStartedAt?.getTime() ?? left.updatedAt.getTime()
        const rightTime = right.lastSyncStartedAt?.getTime() ?? right.updatedAt.getTime()
        return rightTime - leftTime
      })
      dashboardPreset = loaded.period.preset
      customStartDate = loaded.period.startDate.toISOString().slice(0, 10)
      customEndDate = loaded.period.endDate.toISOString().slice(0, 10)
    } catch (loadError) {
      recentTransactions = []
      recentConnections = []
      error = loadError instanceof Error ? loadError.message : 'Failed to load dashboard'
    } finally {
      loadingDashboard = false
    }
  }

  async function onTenantChange() {
    await loadDashboard()
  }

  $effect(() => {
    if (financeShell.loading || !reactiveReady) return
    void financeShell.selectedTenantId
    if (skipNextReactiveLoad) {
      skipNextReactiveLoad = false
      return
    }
    void onTenantChange()
  })

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

  async function openCurrentMonth() {
    dashboardPreset = 'current_month'
    await loadDashboard({ preset: 'current_month' })
  }
</script>

<section class="page" aria-labelledby="finance-heading">
  <header class="page-header panel">
    <div class="page-header-copy">
      <h1 id="finance-heading">Finance</h1>
      <p class="muted">Track balances, spending, cash flow, and recent activity for the active finance workspace.</p>
    </div>

    <div class="page-header-actions">
      <a href="/finance/transactions/new" use:link>Add transaction</a>
      <a href="/finance/accounts" use:link>Open accounts</a>
      <a href="/finance/transactions" use:link>Open transactions</a>
    </div>

    <div class="period-toolbar">
      <div class="period-toolbar__copy">
        <p class="eyebrow">Reporting window</p>
        {#if dashboard}
          <p class="period-summary">{formatFinanceDate(dashboard.period.startDate)} → {formatFinanceDate(dashboard.period.endDate)}</p>
          <p class="muted">Preset: {dashboard.period.preset || 'custom'}</p>
        {:else}
          <p class="period-summary">Choose a tenant and period.</p>
        {/if}
      </div>

      <div class="period-toolbar__controls">
        <div class="period-actions">
          <button class="secondary" type="button" onclick={() => void openPreviousPeriod()} disabled={!dashboard}>Previous period</button>
          <button class="secondary" type="button" onclick={() => void openCurrentMonth()} disabled={!financeShell.selectedTenantId}>Current month</button>
          <button class="secondary" type="button" onclick={() => void openNextPeriod()} disabled={!dashboard}>Next period</button>
        </div>

        <details class="compact-range-panel">
          <summary>Custom range</summary>
          <form class="custom-range" onsubmit={applyCustomRange}>
            <label>
              <span>Custom start date</span>
              <input type="date" bind:value={customStartDate} aria-label="Custom start date" />
            </label>
            <label>
              <span>Custom end date</span>
              <input type="date" bind:value={customEndDate} aria-label="Custom end date" />
            </label>
            <button class="primary" type="submit" disabled={!financeShell.selectedTenantId}>Apply custom range</button>
          </form>
        </details>
      </div>
    </div>
  </header>

  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}

  {#if loading}
    <p class="muted" role="status">Loading finance workspace…</p>
  {:else}
    {#if !financeShell.selectedTenantId}
      <section class="panel">
        {#if financeShell.needsTenantSelection}
          <p>Select an active tenant to continue on this finance route.</p>
        {:else}
          <p>Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a> before loading the dashboard.</p>
        {/if}
      </section>
    {:else if loadingDashboard}
          <p class="muted" role="status">Loading tenant dashboard…</p>
    {:else if dashboard}
      <section class="dashboard-grid dashboard-grid--primary">
        <article class="panel balance-panel">
          <div class="section-header section-header--stacked">
            <div>
              <p class="eyebrow">Balance</p>
              <h2>Booked balance</h2>
            </div>
            <a href="/finance/accounts" use:link>View all accounts</a>
          </div>

          {#if balanceSummary && balanceSummary.accountCount > 0}
            <div class="balance-hero">
              <p class="balance-hero__value">{formatFinanceMoney(balanceSummary.bookedMinor, balanceSummary.currency)}</p>
              <p class="muted">{balanceSummary.accountCount} accounts · Pending change {formatFinanceMoney(balanceSummary.pendingMinor, balanceSummary.currency)}</p>
            </div>
          {:else}
            <div class="balance-hero balance-hero--empty" role="status">
              <p class="balance-hero__title">No account balances yet.</p>
              <p class="muted">Connect or create accounts to turn the dashboard into a balance-first workspace.</p>
            </div>
          {/if}

          <div class="summary-chip-grid" aria-label="Balance summary">
            <article class="summary-chip summary-chip--success">
              <span>Income</span>
              <strong>{formatFinanceMoney(dashboard.settled.incomeMinor, dashboard.settled.displayCurrency)}</strong>
              <small>{dashboard.settled.transactionCount} settled transactions</small>
            </article>
            <article class="summary-chip summary-chip--danger">
              <span>Expense</span>
              <strong>{formatFinanceMoney(dashboard.settled.expenseMinor, dashboard.settled.displayCurrency)}</strong>
              <small>Booked outflow this period</small>
            </article>
            <article class="summary-chip summary-chip--accent">
              <span>Pending delta</span>
              <strong>{formatFinanceMoney(dashboard.pending.netMinor, dashboard.pending.displayCurrency)}</strong>
              <small>{dashboard.pending.transactionCount} pending transactions</small>
            </article>
          </div>

          {#if dashboard.nativeSettledTotals.length > 0}
            <div class="summary-list">
              {#each dashboard.nativeSettledTotals as total (total.currency)}
                <article class="summary-row">
                  <div>
                    <strong>{total.currency}</strong>
                    <p class="muted">Income {formatFinanceMoney(total.incomeMinor, total.currency)} · Expense {formatFinanceMoney(total.expenseMinor, total.currency)}</p>
                  </div>
                  <div class="summary-values">
                    <strong>{formatFinanceMoney(total.netMinor, total.currency)}</strong>
                    <span class="muted">Settled total</span>
                  </div>
                </article>
              {/each}
            </div>
          {/if}
        </article>

        <article class="panel section-panel">
          <div class="section-header">
            <div>
              <p class="eyebrow">Cash flow</p>
              <h2>Period flow</h2>
            </div>
          </div>

          <section class="section-chart-block" aria-labelledby="cash-flow-chart-heading">
            <div class="section-chart-header">
              <p class="chart-caption" id="cash-flow-chart-heading">Cash flow chart</p>
              <p class="muted">Booked and pending movement for the current reporting window.</p>
            </div>

            {#if cashFlowChartMaximum === 0}
              <div class="chart-empty-state" role="status">
                <div class="chart-empty-graphic chart-empty-graphic--columns" aria-hidden="true">
                  <span style="--chart-empty-size: 42%"></span>
                  <span style="--chart-empty-size: 68%"></span>
                  <span style="--chart-empty-size: 54%"></span>
                  <span style="--chart-empty-size: 30%"></span>
                </div>
                <div class="chart-empty-copy">
                  <p class="chart-empty-title">No settled or pending cash flow to chart for this period.</p>
                  <p class="muted">Adjust the reporting window or import activity to populate this chart section.</p>
                </div>
              </div>
            {:else}
              <div class="column-chart" aria-label="Cash flow chart">
                {#each cashFlowChartItems as item (item.key)}
                  <article class="column-chart__item">
                    <div class="column-chart__plot">
                      <span
                        class={`column-chart__bar column-chart__bar--${item.tone}`}
                        style={`height: ${scaledPercent(item.value, cashFlowChartMaximum, 18)}`}
                      ></span>
                    </div>
                    <strong class="column-chart__value">{item.formattedValue}</strong>
                    <span class="column-chart__label">{item.label}</span>
                  </article>
                {/each}
              </div>

              <div class="metric-pill-row">
                {#each cashFlowNetItems as item (item.key)}
                  <article class={`metric-pill metric-pill--${item.tone}`}>
                    <span>{item.label}</span>
                    <strong>{item.formattedValue}</strong>
                    <small>{item.meta}</small>
                  </article>
                {/each}
              </div>
            {/if}
          </section>
        </article>
      </section>

      <section class="dashboard-grid dashboard-grid--secondary">
        <article class="panel section-panel">
          <div class="section-header">
            <div>
              <p class="eyebrow">Spending</p>
              <h2>Top categories</h2>
            </div>
            <a href="/finance/categories" use:link>View all categories</a>
          </div>

          <section class="section-chart-block" aria-labelledby="category-breakdown-chart-heading">
            <div class="section-chart-header">
              <p class="chart-caption" id="category-breakdown-chart-heading">Category breakdown chart</p>
              <p class="muted">Largest income and expense categories for the selected period.</p>
            </div>

            {#if categoryChartItems.length === 0 || categoryChartMaximum === 0}
              <div class="chart-empty-state" role="status">
                <div class="chart-empty-graphic chart-empty-graphic--rows" aria-hidden="true">
                  <span style="--chart-empty-size: 78%"></span>
                  <span style="--chart-empty-size: 58%"></span>
                  <span style="--chart-empty-size: 36%"></span>
                </div>
                <div class="chart-empty-copy">
                  <p class="chart-empty-title">No category activity to chart for this period.</p>
                  <p class="muted">Categorized spending and income will appear here after transaction activity lands.</p>
                </div>
              </div>
            {:else}
              <div class="bar-chart" aria-label="Category breakdown chart">
                {#each categoryChartItems as item (item.key)}
                  <article class="bar-chart__row">
                    <div class="bar-chart__labels">
                      <strong>{item.label}</strong>
                      <span class="muted">{item.meta}</span>
                    </div>
                    <div class="bar-chart__track">
                      <span
                        class={`bar-chart__bar bar-chart__bar--${item.tone}`}
                        style={`width: ${scaledPercent(item.value, categoryChartMaximum)}`}
                      ></span>
                    </div>
                    <strong class="bar-chart__value">{item.formattedValue}</strong>
                  </article>
                {/each}
              </div>
            {/if}
          </section>

          {#if visibleCategoryBreakdowns.length === 0}
            <p class="muted">No category activity for this period.</p>
          {:else}
            <div class="summary-list">
              {#each visibleCategoryBreakdowns as category (category.categoryId)}
                <article class="summary-row summary-row--large">
                  <div>
                    <strong>{category.categoryName}</strong>
                    <p class="muted">{category.kind} · {category.transactionCount} tx</p>
                  </div>
                  <div class="summary-values">
                    <strong>{formatFinanceMoney(category.expenseMinor - category.incomeMinor, dashboard.settled.displayCurrency)}</strong>
                    <span class="muted">Income {formatFinanceMoney(category.incomeMinor, dashboard.settled.displayCurrency)} · Expense {formatFinanceMoney(category.expenseMinor, dashboard.settled.displayCurrency)}</span>
                  </div>
                </article>
              {/each}
            </div>
          {/if}
        </article>

        <article class="panel section-panel">
          <div class="section-header">
            <div>
              <p class="eyebrow">Accounts</p>
              <h2>Account snapshot</h2>
            </div>
            <a href="/finance/accounts" use:link>View all accounts</a>
          </div>

          <section class="section-chart-block" aria-labelledby="account-balances-chart-heading">
            <div class="section-chart-header">
              <p class="chart-caption" id="account-balances-chart-heading">Account balances chart</p>
              <p class="muted">Largest account balances before drilling into account detail.</p>
            </div>

            {#if accountChartItems.length === 0 || accountChartMaximum === 0}
              <div class="chart-empty-state" role="status">
                <div class="chart-empty-graphic chart-empty-graphic--rows" aria-hidden="true">
                  <span style="--chart-empty-size: 82%"></span>
                  <span style="--chart-empty-size: 63%"></span>
                  <span style="--chart-empty-size: 41%"></span>
                </div>
                <div class="chart-empty-copy">
                  <p class="chart-empty-title">No account balances to chart yet.</p>
                  <p class="muted">Linked accounts will appear here once the tenant has ledger history.</p>
                </div>
              </div>
            {:else}
              <div class="bar-chart" aria-label="Account balances chart">
                {#each accountChartItems as item (item.key)}
                  <article class="bar-chart__row">
                    <div class="bar-chart__labels">
                      <strong>{item.label}</strong>
                      <span class="muted">{item.meta}</span>
                    </div>
                    <div class="bar-chart__track">
                      <span
                        class={`bar-chart__bar bar-chart__bar--${item.tone}`}
                        style={`width: ${scaledPercent(item.value, accountChartMaximum)}`}
                      ></span>
                    </div>
                    <strong class="bar-chart__value">{item.formattedValue}</strong>
                  </article>
                {/each}
              </div>
            {/if}
          </section>

          {#if visibleAccountBalances.length === 0}
            <p class="muted">No accounts yet for this tenant.</p>
          {:else}
            <div class="summary-list">
              {#each visibleAccountBalances as account (account.accountId)}
                <article class="summary-row summary-row--large">
                  <div>
                    <strong>{account.accountName}</strong>
                    <p class="muted">{account.currency}{#if account.missingFx} · Missing FX{/if}</p>
                  </div>
                  <div class="summary-values">
                    <strong>{formatFinanceMoney(account.displayBookedMinor ?? account.nativeBookedMinor, dashboard.settled.displayCurrency)}</strong>
                    <span class="muted">Pending {formatFinanceMoney(account.displayPendingMinor ?? account.nativePendingMinor, dashboard.settled.displayCurrency)}</span>
                  </div>
                </article>
              {/each}
            </div>
          {/if}
        </article>
      </section>

      <section class="dashboard-grid dashboard-grid--secondary">
        <article class="panel section-panel">
          <div class="section-header">
            <div>
              <p class="eyebrow">Recent activity</p>
              <h2>Recent transactions</h2>
            </div>
            <a href="/finance/transactions" use:link>View all transactions</a>
          </div>

          {#if visibleRecentTransactions.length === 0}
            <p class="muted">No recent transactions for this tenant yet.</p>
          {:else}
            <div class="summary-list">
              {#each visibleRecentTransactions as transaction (transaction.id)}
                <article class="summary-row summary-row--large">
                  <div>
                    <strong>{transaction.description || transaction.kind}</strong>
                    <p class="muted">{transaction.kind} · {transaction.status} · {formatFinanceDateTime(transaction.effectiveAt)}</p>
                  </div>
                  <div class="summary-values">
                    <strong>{formatFinanceMoney(transaction.amountMinor, transaction.currency)}</strong>
                    <a href={`/finance/transactions/${transaction.id}`} use:link>Open record</a>
                  </div>
                </article>
              {/each}
            </div>
          {/if}
        </article>

        {#if hasAttentionItems}
          <article class="panel section-panel attention-panel">
            <div class="section-header">
              <div>
                <p class="eyebrow">Attention</p>
                <h2>Needs attention</h2>
              </div>
            </div>

            <div class="attention-strip">
              {#each attentionItems as item (item.key)}
                <article class={`attention-card attention-card--${item.tone}`}>
                  <div class="attention-card__copy">
                    <strong>{item.title}</strong>
                    <p class="muted">{item.detail}</p>
                  </div>
                  <div class="attention-card__meta">
                    <span class={`status-pill status-pill--${item.tone}`}>{item.value}</span>
                    {#if item.href && item.hrefLabel}
                      <a href={item.href} use:link>{item.hrefLabel}</a>
                    {/if}
                  </div>
                </article>
              {/each}
            </div>
          </article>
        {/if}

        <article class="panel section-panel">
          <div class="section-header">
            <div>
              <p class="eyebrow">Sync activity</p>
              <h2>Connections</h2>
            </div>
            <a href="/finance/connections" use:link>View all connections</a>
          </div>

          {#if visibleRecentConnections.length === 0}
            <p class="muted">No linked connections yet.</p>
          {:else}
            <div class="summary-list">
              {#each visibleRecentConnections as connection (connection.id)}
                <article class="summary-row summary-row--large">
                  <div>
                    <strong>{connection.displayName}</strong>
                    <p class="muted">{connection.provider} · {connection.state}</p>
                  </div>
                  <div class="summary-values">
                    <strong>
                      {#if connection.lastSyncError}
                        Needs review
                      {:else if connection.lastSuccessfulSyncAt}
                        Synced
                      {:else}
                        Waiting
                      {/if}
                    </strong>
                    <span class="muted">
                      {#if connection.lastSyncError}
                        {connection.lastSyncError}
                      {:else if connection.schedule?.nextRunAt}
                        Next run {formatFinanceDateTime(connection.schedule.nextRunAt)}
                      {:else if connection.lastSuccessfulSyncAt}
                        Last success {formatFinanceDateTime(connection.lastSuccessfulSyncAt)}
                      {:else}
                        No sync history yet
                      {/if}
                    </span>
                  </div>
                </article>
              {/each}
            </div>
          {/if}
        </article>
      </section>
    {/if}
  {/if}
</section>

<style>
  .page {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
  }

  .panel {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
    padding: var(--space-18);
    border: 1px solid var(--border);
    border-radius: 4px;
    background: var(--surface-raised);
  }

  .page-header {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: var(--space-16) var(--space-24);
    background: color-mix(in srgb, var(--surface-raised) 78%, var(--bg));
  }

  .page-header-copy {
    min-width: 0;
  }

  .page-header h1,
  .panel h2,
  .period-summary {
    margin: 0;
  }

  .page-header-actions,
  .period-actions,
  .section-header,
  .summary-values {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-8);
    align-items: center;
  }

  .page-header-actions {
    justify-content: flex-end;
    align-items: center;
  }

  .eyebrow {
    margin: 0;
    color: var(--text-muted);
    font-size: var(--font-size-caption);
  }

  .period-toolbar,
  .period-toolbar__copy,
  .period-toolbar__controls,
  .compact-range-panel,
  .custom-range,
  .dashboard-grid,
  .summary-list,
  .section-chart-block,
  .section-chart-header,
  .column-chart,
  .metric-pill-row,
  .bar-chart,
  .summary-chip-grid,
  .balance-panel,
  .balance-hero,
  .chart-empty-copy {
    display: grid;
    gap: var(--space-12);
  }

  .attention-strip {
    display: grid;
    gap: var(--space-10);
  }

  .attention-card {
    display: grid;
    gap: var(--space-8);
    padding: var(--space-12);
    border: 1px solid var(--border);
    border-radius: 4px;
    background: var(--bg);
  }

  .attention-card__copy,
  .attention-card__meta {
    display: grid;
    gap: var(--space-6);
  }

  .attention-card__meta {
    justify-items: start;
  }

  .period-toolbar {
    grid-column: 1 / -1;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: end;
    gap: var(--space-12) var(--space-16);
  }

  .period-toolbar__controls {
    justify-items: end;
  }

  .compact-range-panel {
    width: 100%;
    max-width: 34rem;
    padding: var(--space-10) var(--space-12);
    border: 1px solid var(--border);
    border-radius: 4px;
    background: var(--bg);
  }

  .compact-range-panel summary {
    cursor: pointer;
    color: var(--text-h);
    font-weight: 500;
  }

  .custom-range {
    grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
    align-items: end;
    margin-top: var(--space-12);
  }

  .custom-range label {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }

  .period-summary {
    font-size: 1.125rem;
    color: var(--text-h);
  }

  .dashboard-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .dashboard-grid--primary {
    align-items: stretch;
  }

  .section-panel {
    min-width: 0;
  }

  .section-header--stacked {
    align-items: start;
  }

  .balance-panel {
    align-content: start;
  }

  .balance-hero {
    padding: var(--space-12);
    border: 1px solid var(--border);
    border-radius: 4px;
    background: color-mix(in srgb, var(--surface-raised) 72%, var(--bg));
  }

  .balance-hero--empty {
    background: var(--bg);
  }

  .balance-hero__value,
  .balance-hero__title {
    margin: 0;
    color: var(--text-h);
  }

  .balance-hero__value {
    font-size: clamp(2rem, 3vw, 3rem);
    line-height: 1.05;
  }

  .balance-hero__title {
    font-weight: 700;
  }

  .summary-chip-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .summary-chip {
    display: grid;
    gap: var(--space-4);
    padding: var(--space-12);
    border: 1px solid var(--border);
    border-radius: 4px;
    background: var(--bg);
  }

  .summary-chip strong {
    color: var(--text-h);
  }

  .summary-chip small {
    color: var(--text-muted);
  }

  .section-chart-block {
    display: flex;
    flex-direction: column;
    min-height: 14.5rem;
    padding: var(--space-12);
    border: 1px solid var(--border);
    border-radius: 4px;
    background: color-mix(in srgb, var(--surface-raised) 72%, var(--bg));
  }

  .section-chart-header {
    gap: var(--space-4);
  }

  .chart-caption {
    margin: 0;
    color: var(--text-h);
    font-weight: 700;
  }

  .chart-empty {
    min-height: 3rem;
  }

  .chart-empty-state {
    display: grid;
    gap: var(--space-12);
  }

  .chart-empty-state {
    flex: 1;
    align-content: start;
  }

  .chart-empty-title {
    margin: 0;
    color: var(--text-h);
    font-weight: 700;
  }

  .chart-empty-graphic {
    display: grid;
    gap: var(--space-10);
    min-height: 7.5rem;
    padding: var(--space-12);
    border: 1px solid var(--border);
    border-radius: 4px;
    background: color-mix(in srgb, var(--bg) 92%, var(--surface-raised));
  }

  .chart-empty-graphic span {
    display: block;
    border: 1px solid color-mix(in srgb, var(--accent) 30%, var(--border));
    background: color-mix(in srgb, var(--accent) 18%, var(--bg));
    opacity: 0.55;
  }

  .chart-empty-graphic--columns {
    grid-template-columns: repeat(4, minmax(0, 1fr));
    align-items: end;
  }

  .chart-empty-graphic--columns span {
    min-height: max(1rem, var(--chart-empty-size));
    height: var(--chart-empty-size);
    border-radius: 2px 2px 0 0;
  }

  .chart-empty-graphic--rows {
    align-content: center;
  }

  .chart-empty-graphic--rows span {
    width: var(--chart-empty-size);
    min-height: 1rem;
    border-radius: 999px;
  }

  .column-chart {
    grid-template-columns: repeat(4, minmax(0, 1fr));
    align-items: end;
  }

  .column-chart__item {
    display: grid;
    gap: var(--space-8);
    min-width: 0;
  }

  .column-chart__plot {
    display: flex;
    align-items: end;
    min-height: 8.5rem;
    padding: var(--space-8);
    border: 1px solid var(--border);
    border-radius: 4px;
    background: var(--bg);
  }

  .column-chart__bar {
    width: 100%;
    min-height: 0.5rem;
    border-radius: 2px 2px 0 0;
  }

  .column-chart__value,
  .column-chart__label,
  .metric-pill span,
  .metric-pill small,
  .bar-chart__labels {
    min-width: 0;
  }

  .column-chart__value,
  .bar-chart__value {
    color: var(--text-h);
  }

  .column-chart__label {
    color: var(--text-muted);
    font-size: var(--font-size-caption);
  }

  .metric-pill-row {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .metric-pill {
    display: grid;
    gap: var(--space-4);
    padding: var(--space-10);
    border: 1px solid var(--border);
    border-radius: 4px;
    background: var(--bg);
  }

  .metric-pill strong {
    color: var(--text-h);
  }

  .metric-pill small {
    color: var(--text-muted);
  }

  .bar-chart__row {
    display: grid;
    grid-template-columns: minmax(0, 170px) minmax(0, 1fr) auto;
    gap: var(--space-10);
    align-items: center;
  }

  .bar-chart__labels {
    display: grid;
    gap: 0.125rem;
  }

  .bar-chart__track {
    position: relative;
    min-height: 0.875rem;
    border: 1px solid var(--border);
    border-radius: 999px;
    background: color-mix(in srgb, var(--bg) 90%, var(--surface-raised));
    overflow: hidden;
  }

  .bar-chart__bar {
    position: absolute;
    inset: 0 auto 0 0;
    border-radius: 999px;
  }

  .column-chart__bar--accent,
  .bar-chart__bar--accent,
  .metric-pill--accent,
   .summary-chip--accent,
   .attention-card--accent {
    background: color-mix(in srgb, var(--accent) 38%, var(--bg));
    border-color: color-mix(in srgb, var(--accent) 55%, var(--border));
  }

  .column-chart__bar--success,
  .bar-chart__bar--success,
  .metric-pill--success,
   .summary-chip--success,
   .attention-card--success {
    background: color-mix(in srgb, var(--color-success) 38%, var(--bg));
    border-color: color-mix(in srgb, var(--color-success) 55%, var(--border));
  }

  .column-chart__bar--warning,
  .bar-chart__bar--warning,
  .metric-pill--warning,
   .summary-chip--warning,
   .attention-card--warning {
    background: color-mix(in srgb, var(--color-warning) 42%, var(--bg));
    border-color: color-mix(in srgb, var(--color-warning) 55%, var(--border));
  }

  .column-chart__bar--danger,
  .bar-chart__bar--danger,
  .metric-pill--danger,
   .summary-chip--danger,
   .attention-card--danger {
    background: color-mix(in srgb, var(--color-danger) 38%, var(--bg));
    border-color: color-mix(in srgb, var(--color-danger) 55%, var(--border));
  }

  .section-header {
    justify-content: space-between;
  }

  .summary-list {
    grid-template-columns: 1fr;
  }

  .summary-row {
    display: flex;
    justify-content: space-between;
    gap: var(--space-12);
    align-items: center;
    padding: var(--space-12);
    border: 1px solid var(--border);
    border-radius: 4px;
    background: var(--bg);
  }

  .summary-row--large {
    align-items: flex-start;
  }

  .summary-values {
    flex-direction: column;
    align-items: flex-end;
    text-align: right;
  }

  .status-pill {
    display: inline-flex;
    align-items: center;
    min-height: 1.75rem;
    padding: 0 var(--space-8);
    border: 1px solid var(--border);
    border-radius: 4px;
    font-size: var(--font-size-caption);
    text-transform: capitalize;
  }

  .status-pill--warning {
    color: var(--color-warning);
    border-color: color-mix(in srgb, var(--color-warning) 55%, var(--border));
  }

  .status-pill--accent {
    color: var(--accent);
    border-color: color-mix(in srgb, var(--accent) 55%, var(--border));
  }

  .status-pill--danger {
    color: var(--color-danger);
    border-color: color-mix(in srgb, var(--color-danger) 55%, var(--border));
  }

  .status-pill--error {
    color: var(--color-danger);
    border-color: color-mix(in srgb, var(--color-danger) 55%, var(--border));
  }

  .status-pill--info {
    color: var(--accent);
    border-color: color-mix(in srgb, var(--accent) 55%, var(--border));
  }

  .status-pill--success {
    color: var(--color-success);
    border-color: color-mix(in srgb, var(--color-success) 55%, var(--border));
  }

  .muted {
    margin: 0;
    color: var(--text-muted);
  }

  .error {
    margin: 0;
    padding: var(--space-12) var(--space-16);
    border: 1px solid var(--danger-border);
    border-radius: 4px;
    background: var(--danger-bg);
    color: var(--color-danger);
  }

  @media (max-width: 1100px) {
    .dashboard-grid,
    .period-toolbar,
    .summary-chip-grid {
      grid-template-columns: 1fr;
    }

    .column-chart {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .period-toolbar__controls {
      justify-items: stretch;
    }

    .page-header-actions {
      justify-content: flex-start;
    }

    .bar-chart__row {
      grid-template-columns: 1fr;
    }
  }

  @media (max-width: 720px) {
    .page-header,
    .summary-row {
      display: flex;
      flex-direction: column;
      align-items: flex-start;
    }

    .page-header-actions,
    .summary-values {
      justify-content: flex-start;
      align-items: flex-start;
      text-align: left;
    }

    .page-header-actions {
      width: 100%;
    }

    .period-actions {
      width: 100%;
    }

    .period-actions :global(button) {
      width: 100%;
    }

    .column-chart,
    .metric-pill-row,
    .summary-chip-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
