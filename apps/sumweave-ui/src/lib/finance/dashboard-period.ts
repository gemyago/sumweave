export interface DashboardPeriodRange {
  /** Inclusive range boundary. */
  startDate: Date
  /** Exclusive range boundary. */
  endDate: Date
}

export type DashboardPeriodMode =
  | 'current_month'
  | 'previous_month'
  | 'next_month'
  | 'last_6_months'
  | 'last_12_months'
  | 'custom'

export function currentDashboardMonth(now = new Date()): DashboardPeriodRange {
  return dashboardMonthContaining(now)
}

export function shiftDashboardMonth(range: DashboardPeriodRange, months: number): DashboardPeriodRange {
  if (!Number.isInteger(months)) {
    throw new TypeError('Dashboard month shift must be a whole number.')
  }
  return dashboardMonthContaining(new Date(
    range.startDate.getFullYear(),
    range.startDate.getMonth() + months,
    1,
  ))
}

export function lastDashboardMonths(now: Date, monthCount: number): DashboardPeriodRange {
  if (!Number.isInteger(monthCount) || monthCount < 1) {
    throw new TypeError('Dashboard month count must be a positive whole number.')
  }
  const currentMonth = currentDashboardMonth(now)
  return {
    startDate: shiftDashboardMonth(currentMonth, -(monthCount - 1)).startDate,
    endDate: currentMonth.endDate,
  }
}

export function cashFlowGroupByForDashboardPeriod(
  mode: DashboardPeriodMode,
  range: DashboardPeriodRange,
): 'day' | 'month' {
  if (mode === 'last_6_months' || mode === 'last_12_months') return 'month'
  if (mode !== 'custom') return 'day'

  const inclusiveEndDate = range.endDate.getHours() === 0 && range.endDate.getMinutes() === 0 &&
    range.endDate.getSeconds() === 0 && range.endDate.getMilliseconds() === 0
    ? new Date(range.endDate.getFullYear(), range.endDate.getMonth(), range.endDate.getDate() - 1)
    : range.endDate
  const calendarDays = (Date.UTC(
    inclusiveEndDate.getFullYear(),
    inclusiveEndDate.getMonth(),
    inclusiveEndDate.getDate(),
  ) - Date.UTC(range.startDate.getFullYear(), range.startDate.getMonth(), range.startDate.getDate())) / 86_400_000 + 1

  return calendarDays <= 31 ? 'day' : 'month'
}

function dashboardMonthContaining(value: Date): DashboardPeriodRange {
  if (!(value instanceof Date) || Number.isNaN(value.getTime())) {
    throw new TypeError('Dashboard month must be a valid local date.')
  }
  const startDate = new Date(value.getFullYear(), value.getMonth(), 1)
  const endDate = new Date(value.getFullYear(), value.getMonth() + 1, 1)
  return { startDate, endDate }
}
