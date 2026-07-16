import { describe, expect, it } from 'vitest'
import { defaultTransferCandidateRange, transferCandidateRangeFromDateInputs } from './transfer-range'

describe('transfer candidate local-date ranges', () => {
  it('defaults to the source local day plus the required exclusive two-day halo', () => {
    const range = defaultTransferCandidateRange(new Date(2026, 5, 20, 23, 59))
    expect(range.effectiveFromDate).toBe('2026-06-18')
    expect(range.effectiveBeforeDate).toBe('2026-06-23')
    expect(range.effectiveFrom.getHours()).toBe(0)
    expect(range.effectiveBefore.getHours()).toBe(0)
  })

  it('preserves local calendar-day boundaries across a daylight-saving transition', () => {
    const environment = (globalThis as unknown as { process: { env: Record<string, string | undefined> } }).process.env
    const previousTimezone = environment.TZ
    environment.TZ = 'America/Los_Angeles'
    try {
      const range = defaultTransferCandidateRange(new Date(2026, 10, 1, 1, 30))
      expect(range.effectiveFromDate).toBe('2026-10-30')
      expect(range.effectiveBeforeDate).toBe('2026-11-04')
      expect(range.effectiveFrom.getHours()).toBe(0)
      expect(range.effectiveBefore.getHours()).toBe(0)
    } finally {
      if (previousTimezone === undefined) delete environment.TZ
      else environment.TZ = previousTimezone
    }
  })

  it('rejects an edited non-forward date range', () => {
    expect(() => transferCandidateRangeFromDateInputs('2026-06-20', '2026-06-20')).toThrow('Effective before must be after effective from.')
    expect(() => transferCandidateRangeFromDateInputs('invalid', '2026-06-21')).toThrow('Enter a valid local calendar date.')
    expect(() => transferCandidateRangeFromDateInputs('2026-02-30', '2026-03-01')).toThrow('Enter a valid local calendar date.')
    const range = transferCandidateRangeFromDateInputs('2026-06-20', '2026-06-21')
    expect(range.effectiveFrom.getDate()).toBe(20)
  })
})
