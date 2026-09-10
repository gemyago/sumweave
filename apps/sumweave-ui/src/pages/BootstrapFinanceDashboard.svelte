<script lang="ts">
  import { onMount, untrack } from 'svelte'
  import { link } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalFinanceApiForAuth,
    type FinanceAccount,
    type FinanceBankConnection,
    type FinanceDashboard,
    type FinanceCashFlowSeries,
    type FinanceCashFlowGroupBy,
    type FinanceTransaction,
  } from '../lib/finance/api'
  import {
    formatFinanceDate,
    formatFinanceDateTime,
    formatFinanceMoney,
  } from '../lib/finance/format'
  import { dateInputValue, withDateInput } from '../lib/date-range'
  import {
    currentDashboardMonth,
    cashFlowGroupByForDashboardPeriod,
    lastDashboardMonths,
    shiftDashboardMonth,
    type DashboardPeriodMode,
    type DashboardPeriodRange,
  } from '../lib/finance/dashboard-period'
  import { useFinanceShellState } from '../lib/finance/shell-state.svelte'
  import FinanceTransactionList from '../components/FinanceTransactionList.svelte'
  import FinancePager from '../components/FinancePager.svelte'
  import EChartsSvgChart from '../components/EChartsSvgChart.svelte'
  import { dateQueryValue, financeRouteQuery, readDateQuery, replaceFinanceRouteQuery } from '../lib/finance/url-filters'
  import type { EChartsCoreOption } from 'echarts/core'

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
  let historyAccounts = $state<FinanceAccount[]>([])
  let recentTransactions = $state<FinanceTransaction[]>([])
  let transactionOffset = $state(0)
  let loadingTransactions = $state(false)
  let recentConnections = $state<FinanceBankConnection[]>([])
  let dashboardPeriodMode = $state<DashboardPeriodMode>('current_month')
  let activeDashboardRange = $state<DashboardPeriodRange | undefined>(undefined)
  let customStartDate = $state<Date | undefined>(undefined)
  let customEndDate = $state<Date | undefined>(undefined)
  let reactiveReady = $state(false)
  let skipNextReactiveLoad = false
  let dashboardLoadRevision = 0
  let cashFlowSeries = $state<FinanceCashFlowSeries | null>(null)
  let loadingCashFlowSeries = $state(false)
  let cashFlowSeriesError = $state<string | null>(null)
  let cashFlowSeriesRequest = $state<CashFlowSeriesRequest | undefined>(undefined)
  let cashFlowSeriesLoadRevision = 0

  const financeShell = useFinanceShellState()

  interface CashFlowSeriesRequest {
    tenantId: string
    range: DashboardPeriodRange
    groupBy: FinanceCashFlowGroupBy
  }

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

  function cashFlowBucketRange(startDate: Date, endDate: Date): string {
    return `${formatFinanceDate(startDate)} → ${formatFinanceDate(inclusiveDashboardEndDate(endDate)!)}`
  }

  function cashFlowBucketLabel(series: FinanceCashFlowSeries, bucketIndex: number): string {
    if (series.groupBy !== 'month') return formatFinanceDate(series.buckets[bucketIndex].startDate)

    const firstIncludedMonth = series.period.startDate
    return new Intl.DateTimeFormat(undefined, { month: 'short', year: 'numeric' }).format(
      new Date(firstIncludedMonth.getFullYear(), firstIncludedMonth.getMonth() + bucketIndex, 1),
    )
  }

  const cashFlowHasActivity = $derived.by(() =>
    cashFlowSeries?.buckets.some((bucket) => bucket.incomeMinor !== 0 || bucket.expenseMinor !== 0) ?? false,
  )

  const cashFlowChartOption = $derived.by<EChartsCoreOption | undefined>(() => {
    if (!cashFlowSeries || !cashFlowHasActivity) return undefined

    const series = cashFlowSeries
    const labelInterval = series.buckets.length > 12 ? Math.ceil(series.buckets.length / 6) - 1 : 0

    return {
      grid: { left: 12, right: 12, top: 44, bottom: 56, containLabel: true },
      legend: { top: 8, textStyle: { color: 'var(--bs-body-color)' } },
      tooltip: {
        trigger: 'axis',
        confine: true,
        formatter: (params: unknown) => {
          const values = Array.isArray(params) ? params : [params]
          const dataIndex = (values[0] as { dataIndex?: number } | undefined)?.dataIndex
          const bucket = dataIndex === undefined ? undefined : series.buckets[dataIndex]
          if (!bucket) return ''
          return `${cashFlowBucketRange(bucket.startDate, bucket.endDate)}<br/>Income: ${formatFinanceMoney(bucket.incomeMinor, series.displayCurrency)}<br/>Expense: ${formatFinanceMoney(bucket.expenseMinor, series.displayCurrency)}`
        },
      },
      xAxis: {
        type: 'category',
        data: series.buckets.map((_, index) => cashFlowBucketLabel(series, index)),
        axisLabel: { color: 'var(--bs-secondary-color)', hideOverlap: true, interval: labelInterval },
        axisLine: { lineStyle: { color: 'var(--bs-border-color)' } },
      },
      yAxis: {
        type: 'value',
        axisLabel: {
          color: 'var(--bs-secondary-color)',
          formatter: (value: number) => formatFinanceMoney(value, series.displayCurrency),
        },
        splitLine: { lineStyle: { color: 'var(--bs-border-color)' } },
      },
      series: [
        {
          name: 'Income',
          type: 'bar',
          data: series.buckets.map((bucket) => bucket.incomeMinor),
          itemStyle: { color: 'var(--color-success)' },
          emphasis: { focus: 'none', itemStyle: { color: 'var(--color-success)', opacity: 1 } },
          blur: { itemStyle: { color: 'var(--color-success)', opacity: 1 } },
        },
        {
          name: 'Expense',
          type: 'bar',
          data: series.buckets.map((bucket) => bucket.expenseMinor),
          itemStyle: { color: 'var(--color-danger)' },
          emphasis: { focus: 'none', itemStyle: { color: 'var(--color-danger)', opacity: 1 } },
          blur: { itemStyle: { color: 'var(--color-danger)', opacity: 1 } },
        },
      ],
    }
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
  const dashboardTransactionPage = $derived(Math.floor(transactionOffset / TRANSACTION_SECTION_LIMIT) + 1)
  const hasOlderDashboardTransactions = $derived(recentTransactions.length > TRANSACTION_SECTION_LIMIT)
  const hasNewerDashboardTransactions = $derived(transactionOffset > 0)
  const accountNameById = $derived(new Map(historyAccounts.map((account) => [account.id, account.name])))
  const hiddenAccountIds = $derived(new Set(historyAccounts.filter((account) => account.hiddenAt).map((account) => account.id)))

  const failedSyncConnections = $derived.by(() =>
    recentConnections.filter((connection) => (connection.lastSyncError?.trim().length ?? 0) > 0),
  )

  const currentFxRates = $derived.by(() => dashboard?.currentFxRates ?? [])
  const fxCoverage = $derived.by(() => dashboard?.fxCoverage ?? [])
  const staleFxRates = $derived.by(() => currentFxRates.filter((rate) => rate.stale))
  const missingFxAffectedValueCount = $derived.by(() =>
    fxCoverage.reduce((total, coverage) => total + coverage.affectedTransactionCount + coverage.affectedAccountCount, 0),
  )
  const hasFxCoverageDetails = $derived(currentFxRates.length > 0 || fxCoverage.length > 0)
  const isHistoricalPeriod = $derived.by(() => dashboardPeriodMode !== 'current_month')

  function fxCoveragePairList() {
    return fxCoverage.map((coverage) => `${coverage.baseCurrency} → ${coverage.quoteCurrency}`).join(', ')
  }

  function fxCoverageSummary() {
    const pairCount = fxCoverage.length
    const affectedValues = missingFxAffectedValueCount
    return `FX coverage missing for ${pairCount} pair${pairCount === 1 ? '' : 's'} (${fxCoveragePairList()}), affecting ${affectedValues} value${affectedValues === 1 ? '' : 's'}.`
  }

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

    if (fxCoverage.length > 0) {
      items.push({
        key: 'missing-fx',
        title: 'Missing FX coverage',
        detail: fxCoverageSummary(),
        value: `${fxCoverage.length} pair${fxCoverage.length === 1 ? '' : 's'}`,
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

      if (fxCoverage.length > 0 && normalizedCode.includes('fx')) return
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
    restoreRangeFromUrl()
    void loadPage()
  })

  function restoreRangeFromUrl() {
    const query = financeRouteQuery()
    const startDate = readDateQuery(query, 'startDate')
    const inclusiveEndDate = readDateQuery(query, 'endDate')
    if (!startDate || !inclusiveEndDate) return
    const endDate = customRangeEndDate(dateInputValue(inclusiveEndDate))
    if (!endDate) return
    if (startDate >= endDate) return
    customStartDate = startDate
    customEndDate = endDate
    activeDashboardRange = { startDate, endDate }
    dashboardPeriodMode = 'custom'
  }

  function persistRange(range: DashboardPeriodRange) {
    replaceFinanceRouteQuery({
      startDate: dateQueryValue(range.startDate),
      endDate: dateQueryValue(inclusiveDashboardEndDate(range.endDate)),
    })
  }

  async function loadPage() {
    loading = true
    reactiveReady = false
    error = null

    try {
      await financeShell.initialize()
      if (financeShell.selectedTenantId) {
        await loadDashboard(rangeForDashboardMode())
      } else {
        dashboard = null
        recentTransactions = []
        recentConnections = []
        clearCashFlowSeries()
      }
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load finance workspace'
    } finally {
      skipNextReactiveLoad = true
      reactiveReady = true
      loading = false
    }
  }

  async function loadDashboard(range: DashboardPeriodRange | undefined): Promise<boolean> {
    const tenantId = financeShell.selectedTenantId
    if (!tenantId) {
      dashboard = null
      historyAccounts = []
      recentTransactions = []
      recentConnections = []
      clearCashFlowSeries()
      return false
    }
    if (!range) {
      error = 'Choose valid start and end dates.'
      return false
    }
    const requestRevision = ++dashboardLoadRevision

    loadingDashboard = true
    error = null
    void loadCashFlowSeries({
      tenantId,
      range,
      groupBy: cashFlowGroupByForDashboardPeriod(dashboardPeriodMode, range),
    })

    try {
      const [loadedDashboard, loadedAccounts, loadedTransactions, loadedConnections] = await Promise.all([
        financeApi.getDashboard({
          tenantId,
          startDate: range.startDate,
          endDate: range.endDate,
        }),
        financeApi.listAccounts({ tenantId, includeHidden: true }),
        financeApi.listTransactions({
          tenantId,
          includeHidden: true,
          startDate: range.startDate,
          endDate: range.endDate,
          limit: TRANSACTION_SECTION_LIMIT + 1,
          offset: 0,
        }),
        financeApi.listConnections({ tenantId }),
      ])

      if (financeShell.selectedTenantId !== tenantId || dashboardLoadRevision !== requestRevision) return false
      dashboard = loadedDashboard
      historyAccounts = loadedAccounts
      recentTransactions = [...loadedTransactions]
        .sort((left, right) => right.effectiveAt.getTime() - left.effectiveAt.getTime())
      transactionOffset = 0
      recentConnections = [...loadedConnections].sort((left, right) => {
        const leftTime = left.lastSyncStartedAt?.getTime() ?? left.updatedAt.getTime()
        const rightTime = right.lastSyncStartedAt?.getTime() ?? right.updatedAt.getTime()
        return rightTime - leftTime
      })
      activeDashboardRange = {
        startDate: loadedDashboard.period.startDate,
        endDate: loadedDashboard.period.endDate,
      }
      customStartDate = loadedDashboard.period.startDate
      customEndDate = loadedDashboard.period.endDate
      persistRange(activeDashboardRange)
      return true
    } catch (loadError) {
      if (financeShell.selectedTenantId !== tenantId || dashboardLoadRevision !== requestRevision) return false
      recentTransactions = []
      historyAccounts = []
      recentConnections = []
      error = loadError instanceof Error ? loadError.message : 'Failed to load dashboard'
      return false
    } finally {
      if (dashboardLoadRevision === requestRevision) {
        loadingDashboard = false
      }
    }
  }

  function clearCashFlowSeries() {
    cashFlowSeriesLoadRevision += 1
    cashFlowSeries = null
    cashFlowSeriesError = null
    cashFlowSeriesRequest = undefined
    loadingCashFlowSeries = false
  }

  async function loadCashFlowSeries(request: CashFlowSeriesRequest) {
    const requestRevision = ++cashFlowSeriesLoadRevision
    cashFlowSeriesRequest = request
    cashFlowSeries = null
    cashFlowSeriesError = null
    loadingCashFlowSeries = true

    try {
      const loadedSeries = await financeApi.getCashFlowSeries({
        tenantId: request.tenantId,
        startDate: request.range.startDate,
        endDate: request.range.endDate,
        groupBy: request.groupBy,
      })
      if (
        financeShell.selectedTenantId !== request.tenantId ||
        cashFlowSeriesLoadRevision !== requestRevision ||
        !isCurrentCashFlowSeriesRequest(request)
      ) return
      cashFlowSeries = loadedSeries
    } catch (loadError) {
      if (
        financeShell.selectedTenantId !== request.tenantId ||
        cashFlowSeriesLoadRevision !== requestRevision ||
        !isCurrentCashFlowSeriesRequest(request)
      ) return
      cashFlowSeriesError = loadError instanceof Error ? loadError.message : 'Failed to load cash-flow chart'
    } finally {
      if (cashFlowSeriesLoadRevision === requestRevision) loadingCashFlowSeries = false
    }
  }

  function isCurrentCashFlowSeriesRequest(request: CashFlowSeriesRequest): boolean {
    const currentRequest = cashFlowSeriesRequest
    return currentRequest?.tenantId === request.tenantId &&
      currentRequest.groupBy === request.groupBy &&
      currentRequest.range.startDate.getTime() === request.range.startDate.getTime() &&
      currentRequest.range.endDate.getTime() === request.range.endDate.getTime()
  }

  function retryCashFlowSeries() {
    if (!cashFlowSeriesRequest || loadingCashFlowSeries) return
    void loadCashFlowSeries(cashFlowSeriesRequest)
  }

  function rangeForDashboardMode(): DashboardPeriodRange | undefined {
    if (activeDashboardRange) return activeDashboardRange

    if (dashboardPeriodMode === 'custom') {
      if (!customStartDate || !customEndDate ||
        Number.isNaN(customStartDate.getTime()) || Number.isNaN(customEndDate.getTime())) {
        return undefined
      }
      return { startDate: customStartDate, endDate: customEndDate }
    }

    const currentMonth = currentDashboardMonth()
    if (dashboardPeriodMode === 'previous_month') return shiftDashboardMonth(currentMonth, -1)
    if (dashboardPeriodMode === 'next_month') return shiftDashboardMonth(currentMonth, 1)
    if (dashboardPeriodMode === 'last_6_months') return lastDashboardMonths(new Date(), 6)
    if (dashboardPeriodMode === 'last_12_months') return lastDashboardMonths(new Date(), 12)
    return currentMonth
  }

  function rangeForMonthAction(months: number): DashboardPeriodRange {
    const base = dashboardPeriodMode === 'custom' ? currentDashboardMonth() : activeDashboardRange ?? currentDashboardMonth()
    return shiftDashboardMonth(base, months)
  }

  function dashboardPeriodModeLabel(): string {
    switch (dashboardPeriodMode) {
      case 'previous_month': return 'Previous month'
      case 'next_month': return 'Next month'
      case 'last_6_months': return 'Last 6 months'
      case 'last_12_months': return 'Last 12 months'
      case 'custom': return 'Custom range'
      default: return 'Current month'
    }
  }

  $effect(() => {
    if (financeShell.loading || !reactiveReady) return
    void financeShell.selectedTenantId
    if (skipNextReactiveLoad) {
      skipNextReactiveLoad = false
      return
    }
    void untrack(async () => {
      await loadDashboard(rangeForDashboardMode())
    })
  })

  async function openPreviousPeriod() {
    if (loadingDashboard) return
    const range = rangeForMonthAction(-1)
    dashboardPeriodMode = 'previous_month'
    await loadDashboard(range)
  }

  async function openCurrentMonth() {
    if (loadingDashboard) return
    dashboardPeriodMode = 'current_month'
    await loadDashboard(currentDashboardMonth())
  }

  async function openNextPeriod() {
    if (loadingDashboard) return
    const range = rangeForMonthAction(1)
    dashboardPeriodMode = 'next_month'
    await loadDashboard(range)
  }

  async function openLastMonths(monthCount: 6 | 12) {
    if (loadingDashboard) return
    dashboardPeriodMode = monthCount === 6 ? 'last_6_months' : 'last_12_months'
    await loadDashboard(lastDashboardMonths(new Date(), monthCount))
  }

  async function applyCustomRange(event: SubmitEvent) {
    event.preventDefault()
    if (loadingDashboard) return
    if (!customStartDate || !customEndDate ||
      Number.isNaN(customStartDate.getTime()) || Number.isNaN(customEndDate.getTime())) {
      error = 'Choose valid start and end dates.'
      return
    }
    if (customStartDate.getTime() >= customEndDate.getTime()) {
      error = 'Choose an end date after the start date.'
      return
    }
    dashboardPeriodMode = 'custom'
    await loadDashboard({ startDate: customStartDate, endDate: customEndDate })
  }

  function applyTransactionUpdate(updated: FinanceTransaction) {
    recentTransactions = recentTransactions.map((item) => item.id === updated.id ? updated : item)
  }

  async function loadDashboardTransactionPage(offset: number): Promise<boolean> {
    const tenantId = financeShell.selectedTenantId!
    const range = activeDashboardRange!

    loadingTransactions = true
    try {
      const loadedTransactions = await financeApi.listTransactions({
        tenantId,
        includeHidden: true,
        startDate: range.startDate,
        endDate: range.endDate,
        limit: TRANSACTION_SECTION_LIMIT + 1,
        offset,
      })
      if (financeShell.selectedTenantId !== tenantId || activeDashboardRange !== range) return false
      recentTransactions = [...loadedTransactions]
        .sort((left, right) => right.effectiveAt.getTime() - left.effectiveAt.getTime())
      transactionOffset = offset
      return true
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load dashboard transactions'
      return false
    } finally {
      loadingTransactions = false
    }
  }

  function loadOlderDashboardTransactions(): Promise<boolean> {
    return loadDashboardTransactionPage(transactionOffset + TRANSACTION_SECTION_LIMIT)
  }

  function loadNewerDashboardTransactions(): Promise<boolean> {
    return loadDashboardTransactionPage(Math.max(0, transactionOffset - TRANSACTION_SECTION_LIMIT))
  }

  function dashboardTransactionsHref(): string {
    const range = activeDashboardRange!
    const query = new URLSearchParams({
      startDate: dateQueryValue(range.startDate)!,
      endDate: dateQueryValue(inclusiveDashboardEndDate(range.endDate))!,
    })
    return `/finance/transactions?${query.toString()}`
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
    date.setDate(date.getDate() + 1)
    date.setHours(0, 0, 0, 0)
    return date
  }

  function inclusiveDashboardEndDate(value: Date | undefined): Date | undefined {
    if (!value) return undefined
    if (value.getHours() !== 0 || value.getMinutes() !== 0 ||
      value.getSeconds() !== 0 || value.getMilliseconds() !== 0) {
      return value
    }
    return new Date(
      value.getFullYear(),
      value.getMonth(),
      value.getDate() - 1,
    )
  }
</script>

<section
  class="container-fluid px-0"
  aria-labelledby="finance-dashboard-heading"
>
  <div class="d-grid gap-2 gap-sm-4">
    <header class="card border-0 shadow-sm">
      <div class="card-body p-2 p-sm-3 p-xl-5">
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

        <hr class="d-none d-sm-block my-3 my-xl-4" />

        <div class="row g-2 g-sm-4 align-items-start">
          <div class="col-12 col-xl-5">
            <p class="d-none d-sm-block text-uppercase text-body-secondary fw-semibold small mb-2">Reporting period</p>
            {#if dashboard}
              <h2 class="h5 mb-1">
                {formatFinanceDate(dashboard.period.startDate)} → {formatFinanceDate(inclusiveDashboardEndDate(dashboard.period.endDate)!)}
              </h2>
              <p class="text-body-secondary mb-1 mb-sm-2">Period: {dashboardPeriodModeLabel()}</p>
              {#if isHistoricalPeriod}
                <p class="text-body-secondary small mb-0">Past activity is valued using today’s latest FX rates, not an end-of-period rate.</p>
              {/if}
            {:else}
              <h2 class="h5 mb-1">Choose a tenant and period</h2>
              <p class="text-body-secondary mb-2">Use the header tenant selector to load this dashboard.</p>
            {/if}
          </div>

          <div class="col-12 col-xl-7">
            <div class="d-flex flex-wrap gap-2 mb-2 mb-sm-3">
              <button type="button" class="btn btn-outline-secondary btn-sm" onclick={() => void openPreviousPeriod()} disabled={!dashboard || loadingDashboard}>
                Previous month
              </button>
              <button type="button" class="btn btn-outline-secondary btn-sm" onclick={() => void openCurrentMonth()} disabled={!financeShell.selectedTenantId || loadingDashboard}>
                Current month
              </button>
              <button type="button" class="btn btn-outline-secondary btn-sm" onclick={() => void openNextPeriod()} disabled={!dashboard || loadingDashboard}>
                Next month
              </button>
              <button type="button" class="btn btn-outline-secondary btn-sm" onclick={() => void openLastMonths(6)} disabled={!financeShell.selectedTenantId || loadingDashboard}>
                Last 6 months
              </button>
              <button type="button" class="btn btn-outline-secondary btn-sm" onclick={() => void openLastMonths(12)} disabled={!financeShell.selectedTenantId || loadingDashboard}>
                Last 12 months
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
                    value={dateInputValue(inclusiveDashboardEndDate(customEndDate))}
                    oninput={(event) => customEndDate = customRangeEndDate(event.currentTarget.value)}
                    aria-label="Custom end date"
                  />
                </div>
                <div class="col-12 col-md-2 d-grid align-content-end">
                  <button class="btn btn-primary" type="submit" disabled={!financeShell.selectedTenantId || loadingDashboard}>
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
        <div class="col-12">
          <div class="card shadow-sm h-100">
            <div class="card-body p-4 d-grid gap-4">
              <div class="d-flex flex-column flex-md-row justify-content-between gap-3 align-items-md-start">
                <div>
                  <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Period performance</p>
                  <h2 class="h5 mb-2">Period net</h2>
                  <p class="display-6 mb-1">
                    {formatFinanceMoney(dashboard.settled.netMinor, dashboard.settled.displayCurrency)}
                  </p>
                  <p class="text-body-secondary mb-0">
                    Income minus expenses for {formatFinanceDate(dashboard.period.startDate)} → {formatFinanceDate(inclusiveDashboardEndDate(dashboard.period.endDate)!)}.
                  </p>
                </div>

                <span class={`badge ${badgeClass(toneFromMoney(dashboard.settled.netMinor))}`}>
                  {dashboard.settled.netMinor < 0 ? 'Net outflow' : dashboard.settled.netMinor > 0 ? 'Net inflow' : 'Even period'}
                </span>
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
                    <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Pending net</p>
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
                  {fxCoverageSummary()} Display totals are partial; native totals remain separate below.
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

              {#if hasFxCoverageDetails}
                <details class="border rounded p-3">
                  <summary class="fw-semibold">FX coverage</summary>
                  <div class="small text-body-secondary mt-3 d-grid gap-2">
                    {#if fxCoverage.length > 0}
                      <div class="d-grid gap-1">
                        <strong class="text-body">Missing pairs</strong>
                        {#each fxCoverage as coverage (`${coverage.provider}-${coverage.baseCurrency}-${coverage.quoteCurrency}`)}
                          <div>
                            {coverage.baseCurrency} → {coverage.quoteCurrency} · {coverage.provider} · {coverage.affectedTransactionCount} transaction value{coverage.affectedTransactionCount === 1 ? '' : 's'} · {coverage.affectedAccountCount} account value{coverage.affectedAccountCount === 1 ? '' : 's'}
                          </div>
                        {/each}
                      </div>
                    {/if}
                    {#if currentFxRates.length > 0}
                      <div class="d-grid gap-1">
                        <strong class="text-body">Current rates</strong>
                        {#each currentFxRates as rate (`${rate.provider}-${rate.baseCurrency}-${rate.quoteCurrency}`)}
                          <div>{rate.baseCurrency} → {rate.quoteCurrency} · {rate.provider} · effective {formatFinanceDateTime(rate.effectiveAt)} · refreshed {formatFinanceDateTime(rate.lastSuccessfulRefreshAt)}{rate.stale ? ' · stale' : ''}</div>
                        {/each}
                      </div>
                    {/if}
                    <a href="/admin/finance/fx" use:link>Refresh required rates</a>
                  </div>
                </details>
              {/if}
            </div>
          </div>
        </div>

        <div class="col-12">
          <div class="card shadow-sm h-100">
            <div class="card-body p-4 d-grid gap-4">
              <div>
                <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Cash-flow visual</p>
                <h2 class="h5 mb-1">Cash flow over time</h2>
                <p class="text-body-secondary mb-0">Settled income and expense by reporting bucket, valued with current FX.</p>
              </div>

              {#if loadingCashFlowSeries}
                <div class="alert alert-secondary mb-0" role="status">Loading cash-flow chart…</div>
              {:else if cashFlowSeriesError}
                <div class="alert alert-danger mb-0" role="alert">
                  <p class="mb-2">{cashFlowSeriesError}</p>
                  <button type="button" class="btn btn-outline-danger btn-sm" onclick={retryCashFlowSeries}>Retry cash-flow chart</button>
                </div>
              {:else if !cashFlowHasActivity}
                <div class="alert alert-light border mb-0" role="status">
                  No settled cash flow to chart for this period.
                </div>
              {:else if cashFlowSeries}
                {#if !cashFlowSeries.complete}
                  <div class="alert alert-warning mb-0" role="alert">
                    <strong>Cash-flow chart data is incomplete.</strong>
                    {#each cashFlowSeries.missingFx as diagnostic (`${diagnostic.provider}-${diagnostic.baseCurrency}-${diagnostic.quoteCurrency}`)}
                      {diagnostic.baseCurrency} → {diagnostic.quoteCurrency} ({diagnostic.provider}, {diagnostic.affectedTransactionCount} transaction value{diagnostic.affectedTransactionCount === 1 ? '' : 's'})
                    {/each}
                    <a class="alert-link" href="/admin/finance/fx" use:link>Open FX diagnostics</a>.
                  </div>
                {/if}
                {#if cashFlowChartOption}
                  <EChartsSvgChart ariaLabel="Cash flow chart" option={cashFlowChartOption} />
                {/if}
                <details class="border rounded p-3">
                  <summary class="fw-semibold">Cash-flow values</summary>
                  <ul class="small text-body-secondary mb-0 mt-3 ps-3">
                    {#each cashFlowSeries.buckets as bucket (bucket.startDate.getTime())}
                      <li>{cashFlowBucketRange(bucket.startDate, bucket.endDate)}: Income {formatFinanceMoney(bucket.incomeMinor, cashFlowSeries.displayCurrency)} · Expense {formatFinanceMoney(bucket.expenseMinor, cashFlowSeries.displayCurrency)}</li>
                    {/each}
                  </ul>
                </details>
              {/if}

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

              {#if balanceSummary && balanceSummary.accountCount > 0}
                <div class="border rounded-3 p-3 bg-body-tertiary">
                  <p class="text-uppercase text-body-secondary fw-semibold small mb-2">Booked balance total</p>
                  {#if balanceSummary.bookedMinor === null}
                    <p class="fs-5 fw-semibold mb-1">Unavailable</p>
                  {:else}
                    <p class="fs-5 fw-semibold mb-1">{formatFinanceMoney(balanceSummary.bookedMinor, balanceSummary.currency)}</p>
                  {/if}
                  <p class="small text-body-secondary mb-0">
                    {balanceSummary.accountCount} accounts · pending movement {balanceSummary.pendingMinor === null ? 'unavailable' : formatFinanceMoney(balanceSummary.pendingMinor, balanceSummary.currency)}
                  </p>
                </div>
              {:else}
                <div class="alert alert-light border mb-0" role="status">
                  No booked account balances yet. Connect or create accounts to start tracking balances here.
                </div>
              {/if}

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
                  <h2 class="h5 mb-1">Transactions</h2>
                  <p class="text-body-secondary mb-0">Booked and pending activity in the reporting period.</p>
                </div>
                <a class="btn btn-outline-secondary btn-sm" href={dashboardTransactionsHref()} use:link>View all transactions</a>
              </div>

              {#if visibleRecentTransactions.length === 0}
                <div class="alert alert-light border mb-0" role="status">No transactions in this reporting period.</div>
              {:else}
                <div id="finance-dashboard-transactions">
                  <FinanceTransactionList
                    tenantId={financeShell.selectedTenantId}
                    transactions={visibleRecentTransactions}
                    accountNameById={accountNameById}
                    {hiddenAccountIds}
                    ariaLabel="Dashboard transactions"
                    onTransactionUpdated={applyTransactionUpdate}
                  />
                </div>
                <FinancePager
                  label="Dashboard transaction pages"
                  status={loadingTransactions ? 'Loading transaction page…' : `Page ${dashboardTransactionPage}`}
                  controls="finance-dashboard-transactions"
                  busy={loadingTransactions}
                  hasPrevious={hasNewerDashboardTransactions}
                  hasNext={hasOlderDashboardTransactions}
                  onPrevious={loadNewerDashboardTransactions}
                  onNext={loadOlderDashboardTransactions}
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
