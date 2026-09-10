import { describe, expect, it } from 'vitest'
import {
  cashFlowGroupByForDashboardPeriod,
  currentDashboardMonth,
  lastDashboardMonths,
  shiftDashboardMonth,
} from './dashboard-period'

describe('dashboard month periods', () => {
  it('uses browser-local calendar month boundaries across a daylight-saving offset change', () => {
    const range = currentDashboardMonth(new Date(2026, 2, 15, 12))

    expect(range.startDate).toEqual(new Date(2026, 2, 1, 0, 0, 0, 0))
    expect(range.endDate).toEqual(new Date(2026, 3, 1, 0, 0, 0, 0))
    expect(range.startDate.getTimezoneOffset()).toBe(300)
    expect(range.endDate.getTimezoneOffset()).toBe(240)
  })

  it('moves a whole month at a time across a year boundary and leap February', () => {
    const december = currentDashboardMonth(new Date(2025, 11, 18, 12))
    const january = shiftDashboardMonth(december, 1)
    const february = shiftDashboardMonth(january, 1)

    expect(january).toEqual({ startDate: new Date(2026, 0, 1), endDate: new Date(2026, 1, 1) })
    expect(february).toEqual({ startDate: new Date(2026, 1, 1), endDate: new Date(2026, 2, 1) })
    expect(shiftDashboardMonth(currentDashboardMonth(new Date(2024, 0, 18, 12)), 1)).toEqual({
      startDate: new Date(2024, 1, 1),
      endDate: new Date(2024, 2, 1),
    })
  })

  it('rejects a fractional month shift', () => {
    expect(() => shiftDashboardMonth(currentDashboardMonth(new Date(2026, 0, 1)), 0.5)).toThrow(
      'Dashboard month shift must be a whole number.',
    )
  })

  it('rejects a non-positive longer-period month count', () => {
    expect(() => lastDashboardMonths(new Date(2026, 0, 1), 0)).toThrow(
      'Dashboard month count must be a positive whole number.',
    )
  })

  it('rejects an invalid current-month date', () => {
    expect(() => currentDashboardMonth(new Date('not a date'))).toThrow(
      'Dashboard month must be a valid local date.',
    )
  })

  it('includes the current month in aligned six- and twelve-month ranges', () => {
    const now = new Date(2026, 5, 20, 12)

    expect(lastDashboardMonths(now, 6)).toEqual({
      startDate: new Date(2026, 0, 1),
      endDate: new Date(2026, 6, 1),
    })
    expect(lastDashboardMonths(now, 12)).toEqual({
      startDate: new Date(2025, 6, 1),
      endDate: new Date(2026, 6, 1),
    })
  })

  it('chooses grouping by local calendar dates rather than elapsed duration', () => {
    expect(cashFlowGroupByForDashboardPeriod('current_month', {
      startDate: new Date(2026, 2, 1),
      endDate: new Date(2026, 3, 1),
    })).toBe('day')
    expect(cashFlowGroupByForDashboardPeriod('last_6_months', {
      startDate: new Date(2026, 0, 1),
      endDate: new Date(2026, 6, 1),
    })).toBe('month')
    expect(cashFlowGroupByForDashboardPeriod('custom', {
      startDate: new Date(2026, 2, 1),
      endDate: new Date(2026, 3, 1),
    })).toBe('day')
    expect(cashFlowGroupByForDashboardPeriod('custom', {
      startDate: new Date(2026, 2, 1),
      endDate: new Date(2026, 3, 2),
    })).toBe('month')
  })
})
