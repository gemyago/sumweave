import { describe, expect, it } from 'vitest'
import { dateInputValue, timeInputValue, withDateInput, withTimeInput } from './date-range'

describe('client-local date range DOM adapters', () => {
  it('edits native Date values through local browser date and time fields', () => {
    const instant = new Date(2026, 5, 15, 9, 30, 45, 120)

    expect(dateInputValue(instant)).toBe('2026-06-15')
    expect(timeInputValue(instant)).toBe('09:30:45')
    expect(withTimeInput(instant, '10:45:00')).toEqual(new Date(2026, 5, 15, 10, 45, 0, 120))
    expect(withDateInput(instant, '2026-06-17')).toEqual(new Date(2026, 5, 17, 9, 30, 45, 120))
  })

  it('rejects malformed DOM values without creating calendar models', () => {
    expect(withDateInput(undefined, '2026-02-31')).toBeUndefined()
    expect(withTimeInput(new Date(), '25:00')).toBeUndefined()
  })
})
