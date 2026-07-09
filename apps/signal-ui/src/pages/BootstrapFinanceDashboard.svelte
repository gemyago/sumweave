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
  import {
    formatFinanceDate,
    formatFinanceDateTime,
    formatFinanceMoney,
  } from '../lib/finance/format'
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'

  type BootstrapTone = 'primary' | 'success' | 'warning' | 'danger' | 'secondary'

  interface VisualMetric {
    key: string
    label: string
    detail: string
    formattedValue: string
    tone: BootstrapTone
    widthClass: string
  }

  interface AttentionItem {
    key: string
    title: string
    detail: string
    value: string
    tone: BootstrapTone
    href?: string
    hrefLabel?: string
  }

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  const ACCOUNT_SECTION_LIMIT = 4
  const CATEGORY_SECTION_LIMIT = 4
  const TRANSACTION_SECTION_LIMIT = 5

  let loading = $state(true)
  let loadingDashboard = $state(false)
  let error = $state<string | null>(null)
  let dashboard = $state<FinanceDashboard | null>(null)
  let recentTransactions = $state<FinanceTransaction[]>([])
  let recentConnections = $state<FinanceBankConnection[]>([])
  let dashboardPreset = $state('current_month')
  let customStartDate = $state('')
  let customEndDate = $state('')
  let reactiveReady = $state(false)
  let skipNextReactiveLoad = false

  const financeShell = useFinanceShellState()

  function maxMagnitude(values: number[]): number {
    return values.reduce((maximum, value) => Math.max(maximum, Math.abs(value)), 0)
  }

  function toneFromMoney(value: number): BootstrapTone {
    if (value < 0) return 'danger'
    if (value > 0) return 'success'
    return 'primary'
  }

  function toneFromSeverity(severity: string): BootstrapTone {
    if (severity === 'error') return 'danger'
    if (severity === 'warning') return 'warning'
    if (severity === 'success') return 'success'
    return 'primary'
  }

  function badgeClass(tone: BootstrapTone): string {
    return `text-bg-${tone}`
  }

  function progressClass(tone: BootstrapTone): string {
    return `bg-${tone}`
  }

  function widthClass(value: number, maximum: number): string {
    if (maximum <= 0) return 'w-25'
    const ratio = Math.abs(value) / maximum
    if (ratio <= 0.25) return 'w-25'
    if (ratio <= 0.5) return 'w-50'
    if (ratio <= 0.75) return 'w-75'
    return 'w-100'
  }

  function formatAttentionLabel(code: string): string {
    const label = code
      .split('_')
      .filter(Boolean)
      .map((segment) => segment.toLowerCase())
      .join(' ')

    return `${label.slice(0, 1).toUpperCase()}${label.slice(1)}`
  }

  function transactionBadgeClass(status: string): string {
    if (status === 'pending') return 'text-bg-warning'
    if (status === 'booked') return 'text-bg-success'
    return 'text-bg-secondary'
  }

  const cashFlowMetrics = $derived.by<VisualMetric[]>(() => {
    if (!dashboard) return []

    const currency = dashboard.settled.displayCurrency
    const items = [
      {
        key: 'settled-income',
        label: 'Settled income',
        detail: `${dashboard.settled.transactionCount} booked transactions`,
        value: dashboard.settled.incomeMinor,
        formattedValue: formatFinanceMoney(dashboard.settled.incomeMinor, currency),
        tone: 'success' as const,
      },
      {
        key: 'settled-expense',
        label: 'Settled expense',
        detail: 'Booked outflow in the active window',
        value: dashboard.settled.expenseMinor,
        formattedValue: formatFinanceMoney(dashboard.settled.expenseMinor, currency),
        tone: 'danger' as const,
      },
      {
        key: 'pending-income',
        label: 'Pending income',
        detail: `${dashboard.pending.transactionCount} pending transactions`,
        value: dashboard.pending.incomeMinor,
        formattedValue: formatFinanceMoney(dashboard.pending.incomeMinor, currency),
        tone: 'primary' as const,
      },
      {
        key: 'pending-expense',
        label: 'Pending expense',
        detail: 'Unsettled outflow still in motion',
        value: dashboard.pending.expenseMinor,
        formattedValue: formatFinanceMoney(dashboard.pending.expenseMinor, currency),
        tone: 'warning' as const,
      },
    ]

    const maximum = maxMagnitude(items.map((item) => item.value))

    return items.map((item) => ({
      key: item.key,
      label: item.label,
      detail: item.detail,
      formattedValue: item.formattedValue,
      tone: item.tone,
      widthClass: widthClass(item.value, maximum),
    }))
  })

  const balanceSummary = $derived.by(() => {
    if (!dashboard) return null

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
    if (!dashboard) return []

    return [...dashboard.accountBalances]
      .sort(
        (left, right) =>
          Math.abs(right.displayBookedMinor ?? right.nativeBookedMinor) -
          Math.abs(left.displayBookedMinor ?? left.nativeBookedMinor),
      )
      .slice(0, ACCOUNT_SECTION_LIMIT)
  })

  const visibleCategoryBreakdowns = $derived.by(() => {
    if (!dashboard) return []

    return [...dashboard.categoryBreakdowns]
      .sort((left, right) => {
        const leftValue = Math.max(left.expenseMinor, left.incomeMinor)
        const rightValue = Math.max(right.expenseMinor, right.incomeMinor)
        return rightValue - leftValue
      })
      .slice(0, CATEGORY_SECTION_LIMIT)
  })

  const categoryMetrics = $derived.by<VisualMetric[]>(() => {
    if (!dashboard) return []

    const currency = dashboard.settled.displayCurrency
    const baseItems = visibleCategoryBreakdowns.map((category) => {
      const value = category.kind === 'income' ? category.incomeMinor : category.expenseMinor
      const tone: BootstrapTone = category.kind === 'income' ? 'success' : 'warning'
      return {
        key: category.categoryId,
        label: category.categoryName,
        detail: `${category.kind} · ${category.transactionCount} tx`,
        value,
        formattedValue: formatFinanceMoney(value, currency),
        tone,
      }
    })

    const maximum = maxMagnitude(baseItems.map((item) => item.value))

    return baseItems.map((item) => ({
      key: item.key,
      label: item.label,
      detail: item.detail,
      formattedValue: item.formattedValue,
      tone: item.tone,
      widthClass: widthClass(item.value, maximum),
    }))
  })

  const accountMetrics = $derived.by<VisualMetric[]>(() => {
    if (!dashboard) return []

    const currency = dashboard.settled.displayCurrency
    const baseItems = visibleAccountBalances.map((account) => {
      const value = account.displayBookedMinor ?? account.nativeBookedMinor
      return {
        key: account.accountId,
        label: account.accountName,
        detail: `${account.currency}${account.missingFx ? ' · Missing FX' : ''}`,
        value,
        formattedValue: formatFinanceMoney(value, currency),
        tone: account.missingFx ? ('warning' as const) : toneFromMoney(value),
      }
    })

    const maximum = maxMagnitude(baseItems.map((item) => item.value))

    return baseItems.map((item) => ({
      key: item.key,
      label: item.label,
      detail: item.detail,
      formattedValue: item.formattedValue,
      tone: item.tone,
      widthClass: widthClass(item.value, maximum),
    }))
  })

  const visibleRecentTransactions = $derived.by(() => recentTransactions.slice(0, TRANSACTION_SECTION_LIMIT))

  const cashFlowHasActivity = $derived.by(() =>
    dashboard
      ? [
          dashboard.settled.incomeMinor,
          dashboard.settled.expenseMinor,
          dashboard.pending.incomeMinor,
          dashboard.pending.expenseMinor,
        ].some((value) => value !== 0)
      : false,
  )

  const failedSyncConnections = $derived.by(() =>
    recentConnections.filter((connection) => connection.lastSyncError.trim().length > 0),
  )

  const attentionItems = $derived.by<AttentionItem[]>(() => {
    if (!dashboard) return []

    const activeDashboard = dashboard
    const items: AttentionItem[] = []

    if (activeDashboard.pending.transactionCount > 0) {
      items.push({
        key: 'pending-transactions',
        title: 'Pending transactions',
        detail: 'Unsettled activity still affects the booked balance story.',
        value: `${activeDashboard.pending.transactionCount} pending`,
        tone: toneFromMoney(activeDashboard.pending.netMinor),
        href: '/finance/transactions',
        hrefLabel: 'Review transactions',
      })
    }

    if (activeDashboard.missingFx.length > 0) {
      items.push({
        key: 'missing-fx',
        title: 'Missing FX coverage',
        detail: `${activeDashboard.missingFx.length} transaction${activeDashboard.missingFx.length === 1 ? '' : 's'} still need FX follow-up.`,
        value: `${activeDashboard.missingFx.length} gap${activeDashboard.missingFx.length === 1 ? '' : 's'}`,
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
            : `${failedSyncConnections.length} connections need retries.`,
        value: `${failedSyncConnections.length} sync issue${failedSyncConnections.length === 1 ? '' : 's'}`,
        tone: 'danger',
        href: '/finance/connections',
        hrefLabel: 'Review connections',
      })
    }

    activeDashboard.alerts.forEach((alert) => {
      const normalizedCode = alert.code.toLowerCase()

      if (activeDashboard.missingFx.length > 0 && normalizedCode.includes('fx')) return
      if (failedSyncConnections.length > 0 && (normalizedCode.includes('connection') || normalizedCode.includes('sync'))) {
        return
      }

      const importAlert = normalizedCode.includes('import')
      const connectionAlert = normalizedCode.includes('connection') || normalizedCode.includes('sync')

      items.push({
        key: `${alert.code}-${alert.severity}`,
        title: formatAttentionLabel(alert.code),
        detail: importAlert
          ? 'Import follow-up still needs operator review.'
          : connectionAlert
            ? 'Connection health shifted during this reporting window.'
            : 'Worth a closer look before trusting the period story.',
        value: `${alert.count} signal${alert.count === 1 ? '' : 's'}`,
        tone: toneFromSeverity(alert.severity),
        href: importAlert ? '/finance/imports' : connectionAlert ? '/finance/connections' : undefined,
        hrefLabel: importAlert
          ? 'Review imports'
          : connectionAlert
            ? 'Review connections'
            : undefined,
      })
    })

    return items
  })

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
      const [loadedDashboard, loadedTransactions, loadedConnections] = await Promise.all([
        financeApi.getDashboard({
          tenantId: financeShell.selectedTenantId,
          preset: overrides.preset ?? dashboardPreset,
          startDate: overrides.startDate ?? customStartDate,
          endDate: overrides.endDate ?? customEndDate,
        }),
        financeApi.listTransactions({ tenantId: financeShell.selectedTenantId, includeHidden: true, limit: TRANSACTION_SECTION_LIMIT }),
        financeApi.listConnections({ tenantId: financeShell.selectedTenantId }),
      ])

      dashboard = loadedDashboard
      recentTransactions = [...loadedTransactions]
        .sort((left, right) => right.effectiveAt.getTime() - left.effectiveAt.getTime())
        .slice(0, TRANSACTION_SECTION_LIMIT)
      recentConnections = [...loadedConnections].sort((left, right) => {
        const leftTime = left.lastSyncStartedAt?.getTime() ?? left.updatedAt.getTime()
        const rightTime = right.lastSyncStartedAt?.getTime() ?? right.updatedAt.getTime()
        return rightTime - leftTime
      })
      dashboardPreset = loadedDashboard.period.preset
      customStartDate = loadedDashboard.period.startDate.toISOString().slice(0, 10)
      customEndDate = loadedDashboard.period.endDate.toISOString().slice(0, 10)
    } catch (loadError) {
      recentTransactions = []
      recentConnections = []
      error = loadError instanceof Error ? loadError.message : 'Failed to load dashboard'
    } finally {
      loadingDashboard = false
    }
  }

  $effect(() => {
    if (financeShell.loading || !reactiveReady) return
    void financeShell.selectedTenantId
    if (skipNextReactiveLoad) {
      skipNextReactiveLoad = false
      return
    }
    void loadDashboard()
  })

  async function openPreviousPeriod() {
    if (!dashboard) return
    dashboardPreset = ''
    customStartDate = dashboard.period.previous.startDate.toISOString().slice(0, 10)
    customEndDate = dashboard.period.previous.endDate.toISOString().slice(0, 10)
    await loadDashboard({ preset: '', startDate: customStartDate, endDate: customEndDate })
  }

  async function openCurrentMonth() {
    dashboardPreset = 'current_month'
    await loadDashboard({ preset: 'current_month' })
  }

  async function openNextPeriod() {
    if (!dashboard) return
    dashboardPreset = ''
    customStartDate = dashboard.period.next.startDate.toISOString().slice(0, 10)
    customEndDate = dashboard.period.next.endDate.toISOString().slice(0, 10)
    await loadDashboard({ preset: '', startDate: customStartDate, endDate: customEndDate })
  }

  async function applyCustomRange(event: SubmitEvent) {
    event.preventDefault()
    dashboardPreset = ''
    await loadDashboard({ preset: '', startDate: customStartDate, endDate: customEndDate })
  }
</script>

<section
  class="container-fluid px-0"
  aria-labelledby="finance-dashboard-heading"
  data-bootstrap-finance-dashboard="true"
>
  <div class="d-grid gap-4">
    <header class="card border-0 shadow-sm">
      <div class="card-body p-4 p-xl-5">
        <div class="d-flex flex-column flex-lg-row justify-content-between gap-3">
          <div>
            <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Finance overview</p>
            <h1 id="finance-dashboard-heading" class="h3 mb-2">Finance dashboard</h1>
            <p class="text-body-secondary mb-0">
              Balances, cash flow, and recent activity for the selected reporting window.
            </p>
          </div>

          <div class="d-flex flex-wrap gap-2 align-content-start">
            <a class="btn btn-primary btn-sm" href="/finance/transactions/new" use:link>Add transaction</a>
            <a class="btn btn-outline-secondary btn-sm" href="/finance/accounts" use:link>Open accounts</a>
            <a class="btn btn-outline-secondary btn-sm" href="/finance/transactions" use:link>
              Open transactions
            </a>
          </div>
        </div>

        <hr class="my-4" />

        <div class="row g-4 align-items-start">
          <div class="col-12 col-xl-5">
            <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Reporting period</p>
            {#if dashboard}
              <h2 class="h5 mb-1">
                {formatFinanceDate(dashboard.period.startDate)} → {formatFinanceDate(dashboard.period.endDate)}
              </h2>
              <p class="text-body-secondary mb-2">Preset: {dashboard.period.preset || 'custom'}</p>
            {:else}
              <h2 class="h5 mb-1">Choose a tenant and period</h2>
              <p class="text-body-secondary mb-2">Use the header tenant selector to load this dashboard.</p>
            {/if}
          </div>

          <div class="col-12 col-xl-7">
            <div class="d-flex flex-wrap gap-2 mb-3">
              <button type="button" class="btn btn-outline-secondary btn-sm" onclick={() => void openPreviousPeriod()} disabled={!dashboard}>
                Previous period
              </button>
              <button type="button" class="btn btn-outline-secondary btn-sm" onclick={() => void openCurrentMonth()} disabled={!financeShell.selectedTenantId}>
                Current month
              </button>
              <button type="button" class="btn btn-outline-secondary btn-sm" onclick={() => void openNextPeriod()} disabled={!dashboard}>
                Next period
              </button>
            </div>

            <details class="border rounded-3 p-3">
              <summary class="fw-semibold">Custom range</summary>
              <form class="row g-3 mt-1" onsubmit={applyCustomRange}>
                <div class="col-12 col-md-4">
                  <label class="form-label" for="finance-start-date">Custom start date</label>
                  <input id="finance-start-date" class="form-control" type="date" bind:value={customStartDate} aria-label="Custom start date" />
                </div>
                <div class="col-12 col-md-4">
                  <label class="form-label" for="finance-end-date">Custom end date</label>
                  <input id="finance-end-date" class="form-control" type="date" bind:value={customEndDate} aria-label="Custom end date" />
                </div>
                <div class="col-12 col-md-4 d-grid align-content-end">
                  <button class="btn btn-primary" type="submit" disabled={!financeShell.selectedTenantId}>
                    Apply custom range
                  </button>
                </div>
              </form>
            </details>
          </div>
        </div>
      </div>
    </header>

    {#if error}
      <div class="alert alert-danger mb-0" role="alert">{error}</div>
    {/if}

    {#if loading}
      <div class="alert alert-secondary mb-0" role="status">Loading finance workspace…</div>
    {:else if !financeShell.selectedTenantId}
      <div class="card shadow-sm">
        <div class="card-body p-4">
          <h2 class="h5 mb-2">Finance workspace required</h2>
          {#if financeShell.needsTenantSelection}
            <p class="mb-0">Select an active tenant to continue on this finance route.</p>
          {:else}
            <p class="mb-0">
              Create or join a tenant from <a href="/finance/tenants" use:link>Finance tenants</a>
              before loading the dashboard.
            </p>
          {/if}
        </div>
      </div>
    {:else if loadingDashboard}
      <div class="alert alert-secondary mb-0" role="status">Loading tenant dashboard…</div>
    {:else if dashboard}
      <div class="row g-4">
        <div class="col-12 col-xxl-7">
          <div class="card shadow-sm h-100">
            <div class="card-body p-4 d-grid gap-4">
              <div class="d-flex flex-column flex-md-row justify-content-between gap-3 align-items-md-start">
                <div>
                  <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Balance-first summary</p>
                  <h2 class="h5 mb-2">Booked balance story</h2>
                  {#if balanceSummary && balanceSummary.accountCount > 0}
                    <p class="display-6 mb-1">{formatFinanceMoney(balanceSummary.bookedMinor, balanceSummary.currency)}</p>
                    <p class="text-body-secondary mb-0">
                      {balanceSummary.accountCount} accounts · pending movement {formatFinanceMoney(balanceSummary.pendingMinor, balanceSummary.currency)}
                    </p>
                  {:else}
                    <p class="h4 mb-1">No booked balances yet</p>
                    <p class="text-body-secondary mb-0">
                      Connect or create accounts to start tracking balances here.
                    </p>
                  {/if}
                </div>

                {#if balanceSummary}
                  <span class={`badge ${badgeClass(toneFromMoney(balanceSummary.bookedMinor))}`}>
                    {balanceSummary.bookedMinor < 0 ? 'Net outflow' : balanceSummary.bookedMinor > 0 ? 'Net inflow' : 'Even period'}
                  </span>
                {/if}
              </div>

              <div class="row g-3" aria-label="Balance summary">
                <div class="col-12 col-md-4">
                  <div class="border rounded-3 p-3 h-100 bg-body-tertiary">
                    <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Income</p>
                    <p class="fs-5 fw-semibold mb-1">
                      {formatFinanceMoney(dashboard.settled.incomeMinor, dashboard.settled.displayCurrency)}
                    </p>
                    <p class="small text-body-secondary mb-0">{dashboard.settled.transactionCount} settled transactions</p>
                  </div>
                </div>
                <div class="col-12 col-md-4">
                  <div class="border rounded-3 p-3 h-100 bg-body-tertiary">
                    <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Expense</p>
                    <p class="fs-5 fw-semibold mb-1">
                      {formatFinanceMoney(dashboard.settled.expenseMinor, dashboard.settled.displayCurrency)}
                    </p>
                    <p class="small text-body-secondary mb-0">Booked outflow this period</p>
                  </div>
                </div>
                <div class="col-12 col-md-4">
                  <div class="border rounded-3 p-3 h-100 bg-body-tertiary">
                    <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Pending delta</p>
                    <p class="fs-5 fw-semibold mb-1">
                      {formatFinanceMoney(dashboard.pending.netMinor, dashboard.pending.displayCurrency)}
                    </p>
                    <p class="small text-body-secondary mb-0">{dashboard.pending.transactionCount} unsettled transactions</p>
                  </div>
                </div>
              </div>

              {#if dashboard.nativeSettledTotals.length > 0}
                <div>
                  <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Native totals</p>
                  <div class="list-group">
                    {#each dashboard.nativeSettledTotals as total (total.currency)}
                      <div class="list-group-item d-flex flex-column flex-md-row justify-content-between gap-2 align-items-md-center">
                        <div>
                          <strong>{total.currency}</strong>
                          <p class="small text-body-secondary mb-0">
                            Income {formatFinanceMoney(total.incomeMinor, total.currency)} · Expense {formatFinanceMoney(total.expenseMinor, total.currency)}
                          </p>
                        </div>
                        <strong>{formatFinanceMoney(total.netMinor, total.currency)}</strong>
                      </div>
                    {/each}
                  </div>
                </div>
              {/if}
            </div>
          </div>
        </div>

        <div class="col-12 col-xxl-5">
          <div class="card shadow-sm h-100">
            <div class="card-body p-4 d-grid gap-4">
              <div>
                <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Cash-flow visual</p>
                <h2 class="h5 mb-1">Period flow</h2>
                <p class="text-body-secondary mb-0">Booked and pending movement for the current reporting window.</p>
              </div>

              {#if !cashFlowHasActivity}
                <div class="alert alert-light border mb-0" role="status">
                  No settled or pending cash flow to chart for this period.
                </div>
              {:else}
                <div class="d-grid gap-3" aria-label="Cash flow chart">
                  {#each cashFlowMetrics as item (item.key)}
                    <div>
                      <div class="d-flex justify-content-between gap-3 mb-1">
                        <div>
                          <strong>{item.label}</strong>
                          <p class="small text-body-secondary mb-0">{item.detail}</p>
                        </div>
                        <strong class="text-nowrap">{item.formattedValue}</strong>
                      </div>
                      <div class="progress" aria-hidden="true">
                        <div class={`progress-bar ${progressClass(item.tone)} ${item.widthClass}`}></div>
                      </div>
                    </div>
                  {/each}
                </div>
              {/if}

              <div class="row g-3">
                <div class="col-12 col-sm-6">
                  <div class="border rounded-3 p-3 h-100">
                    <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Settled net</p>
                    <p class="fs-5 fw-semibold mb-1">
                      {formatFinanceMoney(dashboard.settled.netMinor, dashboard.settled.displayCurrency)}
                    </p>
                    <p class="small text-body-secondary mb-0">{dashboard.settled.transactionCount} booked transactions</p>
                  </div>
                </div>
                <div class="col-12 col-sm-6">
                  <div class="border rounded-3 p-3 h-100">
                    <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Pending net</p>
                    <p class="fs-5 fw-semibold mb-1">
                      {formatFinanceMoney(dashboard.pending.netMinor, dashboard.pending.displayCurrency)}
                    </p>
                    <p class="small text-body-secondary mb-0">{dashboard.pending.transactionCount} pending transactions</p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="col-12 col-xl-6">
          <div class="card shadow-sm h-100">
            <div class="card-body p-4 d-grid gap-4">
              <div class="d-flex flex-column flex-md-row justify-content-between gap-2 align-items-md-center">
                <div>
                  <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Spending focus</p>
                  <h2 class="h5 mb-1">Top categories</h2>
                  <p class="text-body-secondary mb-0">Largest income and expense categories for the selected period.</p>
                </div>
                <a class="btn btn-outline-secondary btn-sm" href="/finance/categories" use:link>View all categories</a>
              </div>

              {#if categoryMetrics.length === 0}
                <div class="alert alert-light border mb-0" role="status">No category activity to chart for this period.</div>
              {:else}
                <div class="d-grid gap-3" aria-label="Category breakdown chart">
                  {#each categoryMetrics as item (item.key)}
                    <div>
                      <div class="d-flex justify-content-between gap-3 mb-1">
                        <div>
                          <strong>{item.label}</strong>
                          <p class="small text-body-secondary mb-0">{item.detail}</p>
                        </div>
                        <strong class="text-nowrap">{item.formattedValue}</strong>
                      </div>
                      <div class="progress" aria-hidden="true">
                        <div class={`progress-bar ${progressClass(item.tone)} ${item.widthClass}`}></div>
                      </div>
                    </div>
                  {/each}
                </div>
              {/if}
            </div>
          </div>
        </div>

        <div class="col-12 col-xl-6">
          <div class="card shadow-sm h-100">
            <div class="card-body p-4 d-grid gap-4">
              <div class="d-flex flex-column flex-md-row justify-content-between gap-2 align-items-md-center">
                <div>
                  <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Account snapshot</p>
                  <h2 class="h5 mb-1">Largest balances</h2>
                  <p class="text-body-secondary mb-0">Largest booked and pending balances across connected accounts.</p>
                </div>
                <a class="btn btn-outline-secondary btn-sm" href="/finance/accounts" use:link>View all accounts</a>
              </div>

              {#if visibleAccountBalances.length === 0}
                <div class="alert alert-light border mb-0" role="status">No account balances to chart yet.</div>
              {:else}
                <div class="d-grid gap-4">
                  <div class="d-grid gap-3" aria-label="Account balances chart">
                    {#each accountMetrics as item (item.key)}
                      <div>
                        <div class="d-flex justify-content-between gap-3 mb-1">
                          <div>
                            <strong>{item.label}</strong>
                            <p class="small text-body-secondary mb-0">{item.detail}</p>
                          </div>
                          <strong class="text-nowrap">{item.formattedValue}</strong>
                        </div>
                        <div class="progress" aria-hidden="true">
                          <div class={`progress-bar ${progressClass(item.tone)} ${item.widthClass}`}></div>
                        </div>
                      </div>
                    {/each}
                  </div>

                  <div class="table-responsive">
                    <table class="table table-sm align-middle mb-0">
                      <thead>
                        <tr>
                          <th scope="col">Account</th>
                          <th scope="col">Booked</th>
                          <th scope="col">Pending</th>
                        </tr>
                      </thead>
                      <tbody>
                        {#each visibleAccountBalances as account (account.accountId)}
                          <tr>
                            <td>
                              <div class="d-grid gap-1">
                                <a href={`/finance/accounts/${account.accountId}`} use:link>{account.accountName}</a>
                                <div class="d-flex flex-wrap gap-2">
                                  <span class="badge text-bg-secondary">{account.currency}</span>
                                  {#if account.missingFx}
                                    <span class="badge text-bg-warning">Missing FX</span>
                                  {/if}
                                </div>
                              </div>
                            </td>
                            <td>{formatFinanceMoney(account.displayBookedMinor ?? account.nativeBookedMinor, dashboard.settled.displayCurrency)}</td>
                            <td>{formatFinanceMoney(account.displayPendingMinor ?? account.nativePendingMinor, dashboard.settled.displayCurrency)}</td>
                          </tr>
                        {/each}
                      </tbody>
                    </table>
                  </div>
                </div>
              {/if}
            </div>
          </div>
        </div>

        <div class="col-12 col-xl-7">
          <div class="card shadow-sm h-100">
            <div class="card-body p-4 d-grid gap-4">
              <div class="d-flex flex-column flex-md-row justify-content-between gap-2 align-items-md-center">
                <div>
                  <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Recent activity</p>
                  <h2 class="h5 mb-1">Recent transactions</h2>
                  <p class="text-body-secondary mb-0">Latest booked and pending activity for the selected tenant.</p>
                </div>
                <a class="btn btn-outline-secondary btn-sm" href="/finance/transactions" use:link>View all transactions</a>
              </div>

              {#if visibleRecentTransactions.length === 0}
                <div class="alert alert-light border mb-0" role="status">No recent transactions for this tenant yet.</div>
              {:else}
                <div class="table-responsive">
                  <table class="table align-middle mb-0">
                    <thead>
                      <tr>
                        <th scope="col">Description</th>
                        <th scope="col">When</th>
                        <th scope="col">Status</th>
                        <th scope="col">Amount</th>
                        <th scope="col">Open</th>
                      </tr>
                    </thead>
                    <tbody>
                      {#each visibleRecentTransactions as transaction (transaction.id)}
                        <tr>
                          <td>
                            <div class="d-grid gap-1">
                              <strong>{transaction.description || transaction.kind}</strong>
                              <span class="small text-body-secondary">{transaction.kind}</span>
                            </div>
                          </td>
                          <td>{formatFinanceDateTime(transaction.effectiveAt)}</td>
                          <td>
                            <span class={`badge ${transactionBadgeClass(transaction.status)}`}>{transaction.status}</span>
                          </td>
                          <td>{formatFinanceMoney(transaction.amountMinor, transaction.currency)}</td>
                          <td>
                            <a href={`/finance/transactions/${transaction.id}`} use:link>Open record</a>
                          </td>
                        </tr>
                      {/each}
                    </tbody>
                  </table>
                </div>
              {/if}
            </div>
          </div>
        </div>

        <div class="col-12 col-xl-5">
          <div class="card shadow-sm h-100">
            <div class="card-body p-4 d-grid gap-4">
              <div>
                <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Attention states</p>
                <h2 class="h5 mb-1">Needs attention</h2>
                <p class="text-body-secondary mb-0">Alerts, FX gaps, and sync issues stay visible near the first activity viewport.</p>
              </div>

              {#if attentionItems.length === 0}
                <div class="alert alert-success mb-0" role="status">No active attention signals right now.</div>
              {:else}
                <div class="list-group">
                  {#each attentionItems as item (item.key)}
                    <div class="list-group-item d-grid gap-2">
                      <div class="d-flex flex-wrap justify-content-between gap-2 align-items-start">
                        <div>
                          <strong>{item.title}</strong>
                          <p class="small text-body-secondary mb-0">{item.detail}</p>
                        </div>
                        <span class={`badge ${badgeClass(item.tone)}`}>{item.value}</span>
                      </div>
                      {#if item.href && item.hrefLabel}
                        <div>
                          <a href={item.href} use:link>{item.hrefLabel}</a>
                        </div>
                      {/if}
                    </div>
                  {/each}
                </div>
              {/if}
            </div>
          </div>
        </div>
      </div>
    {/if}
  </div>
</section>
