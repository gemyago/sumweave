<script lang="ts">
  import { onMount, untrack } from 'svelte'
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
  import { dateInputValue, withDateInput } from '../lib/date-range'
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'
  import FinanceTransactionList from '../components/FinanceTransactionList.svelte'

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
  let customStartDate = $state<Date | undefined>(undefined)
  let customEndDate = $state<Date | undefined>(undefined)
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

    const completeBooked = dashboard.accountBalances.every((account) => account.displayBookedMinor !== null)
    const completePending = dashboard.accountBalances.every((account) => account.displayPendingMinor !== null)

    return {
      currency: dashboard.settled.displayCurrency,
      bookedMinor: completeBooked
        ? dashboard.accountBalances.reduce((total, account) => total + (account.displayBookedMinor ?? 0), 0)
        : null,
      pendingMinor: completePending
        ? dashboard.accountBalances.reduce((total, account) => total + (account.displayPendingMinor ?? 0), 0)
        : null,
      accountCount: dashboard.accountBalances.length,
    }
  })

  const visibleAccountBalances = $derived.by(() => {
    if (!dashboard) return []

    return [...dashboard.accountBalances]
      .sort(
        (left, right) =>
          Math.abs(right.displayBookedMinor ?? 0) - Math.abs(left.displayBookedMinor ?? 0),
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
      const value = account.displayBookedMinor ?? 0
      return {
        key: account.accountId,
        label: account.accountName,
        detail: `${account.currency}${account.missingFx ? ' · Missing FX' : ''}`,
        value,
        formattedValue: account.displayBookedMinor === null ? 'Unavailable' : formatFinanceMoney(value, currency),
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
  const accountNameById = $derived(new Map((dashboard?.accountBalances ?? []).map((account) => [account.accountId, account.accountName])))

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
    recentConnections.filter((connection) => (connection.lastSyncError?.trim().length ?? 0) > 0),
  )

  const currentFxRates = $derived.by(() => dashboard?.currentFxRates ?? [])
  const staleFxRates = $derived.by(() => currentFxRates.filter((rate) => rate.stale))
  const isHistoricalPeriod = $derived.by(() => dashboard?.period.preset !== 'current_month')

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
        detail: `${activeDashboard.missingFx.length} value${activeDashboard.missingFx.length === 1 ? '' : 's'} cannot be converted with a current FX rate.`,
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

  async function loadDashboard(overrides: { preset?: string; startDate?: Date; endDate?: Date } = {}) {
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
          startDate: overrides.preset && overrides.preset !== 'custom' ? undefined : overrides.startDate ?? customStartDate,
          endDate: overrides.preset && overrides.preset !== 'custom' ? undefined : overrides.endDate ?? customEndDate,
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
      customStartDate = loadedDashboard.period.startDate
      customEndDate = loadedDashboard.period.endDate
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
    void untrack(() => loadDashboard())
  })

  async function openPreviousPeriod() {
    dashboardPreset = 'previous_month'
    await loadDashboard({ preset: 'previous_month' })
  }

  async function openCurrentMonth() {
    dashboardPreset = 'current_month'
    await loadDashboard({ preset: 'current_month' })
  }

  async function openNextPeriod() {
    dashboardPreset = 'next_month'
    await loadDashboard({ preset: 'next_month' })
  }

  async function applyCustomRange(event: SubmitEvent) {
    event.preventDefault()
    dashboardPreset = 'custom'
    if (!customStartDate || !customEndDate ||
      Number.isNaN(customStartDate.getTime()) || Number.isNaN(customEndDate.getTime())) {
      error = 'Choose valid start and end dates.'
      return
    }
    await loadDashboard({ preset: 'custom', startDate: customStartDate, endDate: customEndDate })
  }

  function applyTransactionUpdate(updated: FinanceTransaction) {
    recentTransactions = recentTransactions.map((item) => item.id === updated.id ? updated : item)
  }

  function customRangeStartDate(value: string): Date | undefined {
    const date = withDateInput(undefined, value)
    if (!date) return undefined
    date.setHours(0, 0, 0, 0)
    return date
  }

  function customRangeEndDate(value: string): Date | undefined {
    const date = withDateInput(undefined, value)
    if (!date) return undefined
    date.setHours(23, 59, 59, 999)
    return date
  }
</script>

<section
  class="container-fluid px-0"
  aria-labelledby="finance-dashboard-heading"
  data-bootstrap-finance-dashboard="true"
>
  <div class="d-grid gap-4">
    <header class="card border-0 shadow-sm">
      <div class="card-body p-3 p-xl-5">
        <div class="d-flex flex-column flex-lg-row justify-content-between gap-3">
          <div>
            <p class="d-none d-sm-block text-uppercase text-body-secondary fw-semibold small mb-2">Finance overview</p>
            <h1 id="finance-dashboard-heading" class="h3 mb-2">Finance dashboard</h1>
            <p class="d-none d-sm-block text-body-secondary mb-0">
              Display-currency balances, flows, categories, and pending values use current FX valuation and can change after a rate refresh.
            </p>
          </div>

          <div class="d-flex flex-wrap gap-2 align-content-start">
            <a class="btn btn-primary btn-sm" href="/finance/transactions/new" use:link aria-label="Add transaction">
              <span class="d-sm-none">Add</span><span class="d-none d-sm-inline">Add transaction</span>
            </a>
            <a class="btn btn-outline-secondary btn-sm" href="/finance/accounts" use:link aria-label="Open accounts">
              <span class="d-sm-none">Accounts</span><span class="d-none d-sm-inline">Open accounts</span>
            </a>
            <a class="btn btn-outline-secondary btn-sm" href="/finance/transactions" use:link aria-label="Open transactions">
              <span class="d-sm-none">Transactions</span><span class="d-none d-sm-inline">Open transactions</span>
            </a>
          </div>
        </div>

        <hr class="my-3 my-xl-4" />

        <div class="row g-4 align-items-start">
          <div class="col-12 col-xl-5">
            <p class="d-none d-sm-block text-uppercase text-body-secondary fw-semibold small mb-2">Reporting period</p>
            {#if dashboard}
              <h2 class="h5 mb-1">
                {formatFinanceDate(dashboard.period.startDate)} → {formatFinanceDate(dashboard.period.endDate)}
              </h2>
              <p class="text-body-secondary mb-2">Preset: {dashboard.period.preset || 'custom'}</p>
              {#if isHistoricalPeriod}
                <p class="text-body-secondary small mb-0">Past activity is valued using today’s latest FX rates, not an end-of-period rate.</p>
              {/if}
            {:else}
              <h2 class="h5 mb-1">Choose a tenant and period</h2>
              <p class="text-body-secondary mb-2">Use the header tenant selector to load this dashboard.</p>
            {/if}
          </div>

          <div class="col-12 col-xl-7">
            <div class="d-flex flex-wrap gap-2 mb-3">
              <button type="button" class="btn btn-outline-secondary btn-sm" onclick={() => void openPreviousPeriod()} disabled={!dashboard}>
                Previous month
              </button>
              <button type="button" class="btn btn-outline-secondary btn-sm" onclick={() => void openCurrentMonth()} disabled={!financeShell.selectedTenantId}>
                Current month
              </button>
              <button type="button" class="btn btn-outline-secondary btn-sm" onclick={() => void openNextPeriod()} disabled={!dashboard}>
                Next month
              </button>
            </div>

            <details class="border rounded-3 p-2">
              <summary class="fw-semibold">Custom range</summary>
              <form class="row g-2 mt-1" onsubmit={applyCustomRange}>
                <div class="col-12 col-md-5">
                  <label class="form-label mb-1" for="finance-start-date">Custom start date</label>
                  <input
                    id="finance-start-date"
                    class="form-control"
                    type="date"
                    value={dateInputValue(customStartDate)}
                    oninput={(event) => customStartDate = customRangeStartDate(event.currentTarget.value)}
                    aria-label="Custom start date"
                  />
                </div>
                <div class="col-12 col-md-5">
                  <label class="form-label mb-1" for="finance-end-date">Custom end date</label>
                  <input
                    id="finance-end-date"
                    class="form-control"
                    type="date"
                    value={dateInputValue(customEndDate)}
                    oninput={(event) => customEndDate = customRangeEndDate(event.currentTarget.value)}
                    aria-label="Custom end date"
                  />
                </div>
                <div class="col-12 col-md-2 d-grid align-content-end">
                  <button class="btn btn-primary" type="submit" disabled={!financeShell.selectedTenantId}>
                    Apply
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
      {#if staleFxRates.length > 0}
        <div class="alert alert-warning mb-0" role="alert">
          <strong>Current FX valuation may be stale.</strong>
          {staleFxRates.length} rate{staleFxRates.length === 1 ? '' : 's'} exceeded the refresh threshold; displayed values use the last successful rate.
          <a class="alert-link" href="/admin/finance/fx" use:link>Refresh current rates</a>.
        </div>
      {/if}
      <div class="row g-4">
        <div class="col-12 col-xxl-7">
          <div class="card shadow-sm h-100">
            <div class="card-body p-4 d-grid gap-4">
              <div class="d-flex flex-column flex-md-row justify-content-between gap-3 align-items-md-start">
                <div>
                  <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Balance-first summary</p>
                  <h2 class="h5 mb-2">Booked balance story</h2>
                  {#if balanceSummary && balanceSummary.accountCount > 0}
                    {#if balanceSummary.bookedMinor === null}
                      <p class="display-6 mb-1">Booked total unavailable</p>
                    {:else}
                      <p class="display-6 mb-1">{formatFinanceMoney(balanceSummary.bookedMinor, balanceSummary.currency)}</p>
                    {/if}
                    <p class="text-body-secondary mb-0">
                      {balanceSummary.accountCount} accounts · pending movement {balanceSummary.pendingMinor === null ? 'unavailable' : formatFinanceMoney(balanceSummary.pendingMinor, balanceSummary.currency)}
                    </p>
                  {:else}
                    <p class="h4 mb-1">No booked balances yet</p>
                    <p class="text-body-secondary mb-0">
                      Connect or create accounts to start tracking balances here.
                    </p>
                  {/if}
                </div>

                {#if balanceSummary && balanceSummary.bookedMinor !== null}
                  <span class={`badge ${badgeClass(toneFromMoney(balanceSummary.bookedMinor))}`}>
                    {balanceSummary.bookedMinor < 0 ? 'Net outflow' : balanceSummary.bookedMinor > 0 ? 'Net inflow' : 'Even period'}
                  </span>
                {/if}
              </div>

              <div class="border rounded-3 p-3 bg-body-tertiary">
                <div class="d-flex flex-column flex-md-row justify-content-between gap-2">
                  <div>
                    <p class="text-uppercase text-body-secondary fw-semibold small mb-1">Current FX valuation</p>
                    <p class="small text-body-secondary mb-0">Latest successful rates revalue display-currency totals after refresh; native totals below remain separate corroboration.</p>
                  </div>
                  <span class="badge text-bg-secondary align-self-md-start">{currentFxRates.length} current rate{currentFxRates.length === 1 ? '' : 's'}</span>
                </div>
                {#if currentFxRates.length > 0}
                  <div class="small text-body-secondary mt-2 d-grid gap-1">
                    {#each currentFxRates as rate (`${rate.provider}-${rate.baseCurrency}-${rate.quoteCurrency}`)}
                      <div><strong>{rate.baseCurrency} → {rate.quoteCurrency}</strong> · {rate.provider} · effective {formatFinanceDateTime(rate.effectiveAt)} · refreshed {formatFinanceDateTime(rate.lastSuccessfulRefreshAt)}{rate.stale ? ' · stale' : ''}</div>
                    {/each}
                  </div>
                {:else}
                  <p class="small text-body-secondary mt-2 mb-0">No current FX rates are available for this dashboard.</p>
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

              {#if !dashboard.settled.complete}
                <div class="alert alert-warning mb-0" role="alert">
                  <strong>Income and expense totals are incomplete.</strong>
                   {dashboard.missingFx.length} value{dashboard.missingFx.length === 1 ? '' : 's'} were excluded because current FX rates are missing. Display totals are partial; native totals remain separate below.
                  <a class="alert-link" href="/admin/finance/fx" use:link>Open FX diagnostics</a>.
                </div>
              {/if}

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
                <p class="text-body-secondary mb-0">Booked and pending movement for the reporting window, valued with current FX.</p>
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
                  <p class="text-body-secondary mb-0">Largest income and expense categories for the selected period, valued with current FX.</p>
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
                  <p class="text-body-secondary mb-0">Largest booked and pending balances across connected accounts, valued with current FX.</p>
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
                            <td>
                              {account.displayBookedMinor === null ? 'Unavailable' : formatFinanceMoney(account.displayBookedMinor, dashboard.settled.displayCurrency)}
                              <span class="d-block small text-body-secondary">Native {formatFinanceMoney(account.nativeBookedMinor, account.currency)}</span>
                            </td>
                            <td>
                              {account.displayPendingMinor === null ? 'Unavailable' : formatFinanceMoney(account.displayPendingMinor, dashboard.settled.displayCurrency)}
                              <span class="d-block small text-body-secondary">Native {formatFinanceMoney(account.nativePendingMinor, account.currency)}</span>
                            </td>
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

        <div class="col-12">
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
                <FinanceTransactionList
                  tenantId={financeShell.selectedTenantId}
                  transactions={visibleRecentTransactions}
                  accountNameById={accountNameById}
                  ariaLabel="Recent transactions"
                  onTransactionUpdated={applyTransactionUpdate}
                />
              {/if}
            </div>
          </div>
        </div>

        <div class="col-12">
          <div class="card shadow-sm">
            <div class="card-body p-3 d-grid gap-3">
              <div class="d-flex flex-column flex-md-row justify-content-between gap-1 align-items-md-baseline">
                <h2 class="h6 mb-0">Needs attention</h2>
                <p class="small text-body-secondary mb-0">Follow-up signals for pending activity, FX, syncs, and imports.</p>
              </div>

              {#if attentionItems.length === 0}
                <div class="alert alert-success mb-0" role="status">No active attention signals right now.</div>
              {:else}
                <div class="row g-2">
                  {#each attentionItems as item (item.key)}
                    <div class="col-12 col-md-6 col-xl-4">
                    <div class="border rounded p-2 h-100 d-grid gap-2">
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
