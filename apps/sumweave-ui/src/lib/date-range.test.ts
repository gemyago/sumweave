import { describe, expect, it } from 'vitest'
import { dateInputValue, withDateInput } from './date-range'

describe('date input helpers', () => {
  it('formats valid local dates and rejects absent or invalid values', () => {
    expect(dateInputValue(new Date(2026, 6, 3))).toBe('2026-07-03')
    expect(dateInputValue(null)).toBe('')
    expect(dateInputValue(new Date('invalid'))).toBe('')
  })

  it('preserves an existing time, parses new dates at local midnight, and rejects invalid input', () => {
    const existing = new Date(2026, 0, 1, 14, 30)
    expect(withDateInput(existing, '2026-07-03')).toEqual(new Date(2026, 6, 3, 14, 30))
    const nodeProcess = (globalThis as typeof globalThis & {
      process: { env: Record<string, string | undefined> }
    }).process
    const originalTimeZone = nodeProcess.env.TZ
    nodeProcess.env.TZ = 'America/New_York'
    try {
      expect(withDateInput(undefined, '2026-07-03')).toEqual(new Date(2026, 6, 3))
    } finally {
      if (originalTimeZone === undefined) delete nodeProcess.env.TZ
      else nodeProcess.env.TZ = originalTimeZone
    }
    expect(withDateInput(existing, 'bad')).toBeUndefined()
    expect(withDateInput(existing, '2026-02-31')).toBeUndefined()
  })
})
