import { describe, expect, it } from 'vitest'
import { formatFinanceDate, formatFinanceDateTime, formatFinanceMoney } from './format'

describe('finance format helpers', () => {
  it('formats null and date values', () => {
    const value = new Date('2026-06-20T12:00:00Z')
    const zeroValue = new Date('0001-01-01T00:00:00Z')
    const expectedDate = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' }).format(value)
    const expectedDateTime = new Intl.DateTimeFormat(undefined, {
      dateStyle: 'medium',
      timeStyle: 'short',
    }).format(value)

    expect(formatFinanceDate(null)).toBe('—')
    expect(formatFinanceDate(zeroValue)).toBe(
      new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' }).format(zeroValue),
    )
    expect(formatFinanceDate(value)).toBe(expectedDate)
    expect(formatFinanceDateTime(zeroValue)).toBe(
      new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(zeroValue),
    )
    expect(formatFinanceDateTime(value)).toBe(expectedDateTime)
  })

  it('formats absent and minor-unit money values', () => {
    expect(formatFinanceMoney(null, 'USD')).toBe('—')
    expect(formatFinanceMoney(undefined, 'USD')).toBe('—')
    expect(formatFinanceMoney(12345, 'USD')).toBe('123.45 USD')
  })

})
