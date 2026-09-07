import { describe, expect, it } from 'vitest'
import { currentDashboardMonth, shiftDashboardMonth } from './dashboard-period'

describe('dashboard month periods', () => {
  it('uses browser-local calendar month boundaries across a daylight-saving offset change', () => {
    const range = currentDashboardMonth(new Date(2026, 2, 15, 12))

    expect(range.startDate).toEqual(new Date(2026, 2, 1, 0, 0, 0, 0))
    expect(range.endDate).toEqual(new Date(2026, 2, 31, 23, 59, 59, 999))
    expect(range.startDate.getTimezoneOffset()).toBe(300)
    expect(range.endDate.getTimezoneOffset()).toBe(240)
  })

  it('moves a whole month at a time across a year boundary and leap February', () => {
    const december = currentDashboardMonth(new Date(2025, 11, 18, 12))
    const january = shiftDashboardMonth(december, 1)
    const february = shiftDashboardMonth(january, 1)

    expect(january).toEqual({ startDate: new Date(2026, 0, 1), endDate: new Date(2026, 0, 31, 23, 59, 59, 999) })
    expect(february).toEqual({ startDate: new Date(2026, 1, 1), endDate: new Date(2026, 1, 28, 23, 59, 59, 999) })
    expect(shiftDashboardMonth(currentDashboardMonth(new Date(2024, 0, 18, 12)), 1)).toEqual({
      startDate: new Date(2024, 1, 1),
      endDate: new Date(2024, 1, 29, 23, 59, 59, 999),
    })
  })
})
