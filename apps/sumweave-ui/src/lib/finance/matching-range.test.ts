import { faker } from '@faker-js/faker'
import { describe, expect, it, vi } from 'vitest'
import {
  defaultMatchingDateRange,
  matchingRangeFromDateInputs,
} from './matching-range'

describe('transfer matching local-date ranges', () => {
  it('defaults to today and the preceding twenty-nine local calendar dates', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 5, 20, 12))

    expect(defaultMatchingDateRange()).toEqual({
      startDate: '2026-05-22',
      endDate: '2026-06-20',
    })

    vi.useRealTimers()
  })

  it('keeps displayed inclusive boundaries while allowing wider history', () => {
    const startDate = faker.date.past({ years: 2, refDate: new Date(2024, 0, 1) })
    const start = `${startDate.getFullYear()}-${String(startDate.getMonth() + 1).padStart(2, '0')}-${String(startDate.getDate()).padStart(2, '0')}`
    const range = matchingRangeFromDateInputs(start, '2026-06-20')

    expect(range.startDate).toBe(start)
    expect(range.endDate).toBe('2026-06-20')
    expect(range.rangeEndExclusive).toContain('2026-06-21T00:00:00')
  })

  it('rejects invalid or reversed local calendar dates', () => {
    expect(() => matchingRangeFromDateInputs('2026-06-21', '2026-06-20')).toThrow('Start date must be on or before end date.')
    expect(() => matchingRangeFromDateInputs('2026-02-30', '2026-03-01')).toThrow('Enter a valid local calendar date.')
  })

  it('converts displayed inclusive dates to offset-bearing half-open boundaries', () => {
    const range = matchingRangeFromDateInputs('2026-06-20', '2026-06-20')

    expect(range.rangeStart).toMatch(/^2026-06-20T00:00:00[+-]\d{2}:\d{2}$/)
    expect(range.rangeEndExclusive).toMatch(/^2026-06-21T00:00:00[+-]\d{2}:\d{2}$/)
  })

  it('uses each local calendar boundary offset across daylight saving time', () => {
    const range = matchingRangeFromDateInputs('2026-03-07', '2026-03-08')

    expect(range.rangeStart).toBe('2026-03-07T00:00:00-05:00')
    expect(range.rangeEndExclusive).toBe('2026-03-09T00:00:00-04:00')
  })
})
