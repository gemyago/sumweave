import { describe, expect, it } from 'vitest'
import { dateInputValue, withDateInput } from './date-range'

describe('date input helpers', () => {
  it('formats valid local dates and rejects absent or invalid values', () => {
    expect(dateInputValue(new Date(2026, 6, 3))).toBe('2026-07-03')
    expect(dateInputValue(null)).toBe('')
    expect(dateInputValue(new Date('invalid'))).toBe('')
  })

  it('preserves an existing time, provides a baseline, and rejects invalid input', () => {
    const existing = new Date(2026, 0, 1, 14, 30)
    expect(withDateInput(existing, '2026-07-03')).toEqual(new Date(2026, 6, 3, 14, 30))
    expect(dateInputValue(withDateInput(undefined, '2026-07-03'))).toBe('2026-07-03')
    expect(withDateInput(existing, 'bad')).toBeUndefined()
    expect(withDateInput(existing, '2026-02-31')).toBeUndefined()
  })
})
