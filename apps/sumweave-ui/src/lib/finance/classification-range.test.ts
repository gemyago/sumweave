import { describe, expect, it } from 'vitest'
import {
  classificationRangeFromDateInputs,
  defaultClassificationDateRange,
} from './classification-range'

describe('classification local-date ranges', () => {
  it('defaults to today and the preceding twenty-nine local calendar dates', () => {
    expect(defaultClassificationDateRange(new Date(2026, 5, 20, 12))).toEqual({
      startDate: '2026-05-22',
      endDate: '2026-06-20',
    })
  })

  it('keeps visible inclusive boundaries and accepts wider history', () => {
    const range = classificationRangeFromDateInputs('2024-01-01', '2026-06-20')

    expect(range.startDate).toBe('2024-01-01')
    expect(range.endDate).toBe('2026-06-20')
    expect(range.rangeEndExclusive).toContain('2026-06-21T00:00:00')
  })

  it('rejects reversed or invalid local dates before submission', () => {
    expect(() => classificationRangeFromDateInputs('2026-06-21', '2026-06-20')).toThrow('Start date must be on or before end date.')
    expect(() => classificationRangeFromDateInputs('2026-02-30', '2026-03-01')).toThrow('Enter a valid local calendar date.')
  })

  it('converts the inclusive end date to a local half-open RFC3339 boundary', () => {
    const range = classificationRangeFromDateInputs('2026-06-20', '2026-06-20')

    expect(range.rangeStart).toMatch(/^2026-06-20T00:00:00(?:\.000)?[+-]\d{2}:\d{2}$/)
    expect(range.rangeEndExclusive).toMatch(/^2026-06-21T00:00:00(?:\.000)?[+-]\d{2}:\d{2}$/)
  })

  it('keeps each daylight-saving boundary offset when calendar dates cross the transition', () => {
    const range = classificationRangeFromDateInputs('2026-03-07', '2026-03-08')

    expect(range.rangeStart).toBe('2026-03-07T00:00:00-05:00')
    expect(range.rangeEndExclusive).toBe('2026-03-09T00:00:00-04:00')
  })
})
