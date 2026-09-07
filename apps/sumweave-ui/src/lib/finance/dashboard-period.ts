export interface DashboardPeriodRange {
  startDate: Date
  endDate: Date
}

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

function dashboardMonthContaining(value: Date): DashboardPeriodRange {
  if (!(value instanceof Date) || Number.isNaN(value.getTime())) {
    throw new TypeError('Dashboard month must be a valid local date.')
  }
  const startDate = new Date(value.getFullYear(), value.getMonth(), 1)
  const nextMonthStart = new Date(value.getFullYear(), value.getMonth() + 1, 1)
  return { startDate, endDate: new Date(nextMonthStart.getTime() - 1) }
}
