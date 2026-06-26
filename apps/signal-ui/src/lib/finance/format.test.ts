import { describe, expect, it, vi } from 'vitest'
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
    expect(formatFinanceDate(zeroValue)).toBe('—')
    expect(formatFinanceDate(value)).toBe(expectedDate)
    expect(formatFinanceDateTime(zeroValue)).toBe('—')
    expect(formatFinanceDateTime(value)).toBe(expectedDateTime)
  })

  it('formats absent and minor-unit money values', () => {
    expect(formatFinanceMoney(null, 'USD')).toBe('—')
    expect(formatFinanceMoney(undefined, 'USD')).toBe('—')
    expect(formatFinanceMoney(12345, 'USD')).toBe('123.45 USD')
  })

  it('keeps date-only values on their intended calendar day outside UTC', () => {
    const dateOnlyValue = new Date('2026-06-30T00:00:00Z')
    const actualDateTimeFormat = Intl.DateTimeFormat
    const expectedDate = new actualDateTimeFormat('en-US', {
      dateStyle: 'medium',
      timeZone: 'UTC',
    }).format(dateOnlyValue)
    const expectedDateTime = new actualDateTimeFormat('en-US', {
      dateStyle: 'medium',
      timeStyle: 'short',
      timeZone: 'America/Los_Angeles',
    }).format(dateOnlyValue)

    function MockDateTimeFormat(
      locales?: Intl.LocalesArgument,
      options?: Intl.DateTimeFormatOptions,
    ) {
      return new actualDateTimeFormat(locales, {
        ...(options ?? {}),
        timeZone: options?.timeZone ?? 'America/Los_Angeles',
      })
    }

    vi.spyOn(Intl, 'DateTimeFormat').mockImplementation(
      MockDateTimeFormat as typeof Intl.DateTimeFormat,
    )

    expect(formatFinanceDate(dateOnlyValue)).toBe(expectedDate)
    expect(formatFinanceDateTime(dateOnlyValue)).toBe(expectedDateTime)
  })
})
