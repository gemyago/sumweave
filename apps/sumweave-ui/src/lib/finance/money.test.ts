import { describe, expect, it } from 'vitest'
import { formatMinorAmountForInput, parseMajorAmountToMinor } from './money'

describe('finance major-unit money input', () => {
  it('converts a negative two-place decimal to exact minor units', () => {
    expect(parseMajorAmountToMinor('-553.00')).toBe(-55300)
  })

  it('converts a positive whole amount to exact minor units', () => {
    expect(parseMajorAmountToMinor('553')).toBe(55300)
  })

  it('formats API minor units for the major-unit editor field', () => {
    expect(formatMinorAmountForInput(-55300)).toBe('-553.00')
  })

  it('formats a positive API minor-unit value', () => {
    expect(formatMinorAmountForInput(55350)).toBe('553.50')
  })

  it('rejects malformed values', () => {
    expect(() => parseMajorAmountToMinor('5.')).toThrow('Amount must')
  })

  it('rejects fractions with more than two places', () => {
    expect(() => parseMajorAmountToMinor('5.001')).toThrow('Amount must')
  })

  it('rejects unsafe minor-unit values', () => {
    expect(() => parseMajorAmountToMinor('90071992547410')).toThrow('supported range')
  })

  it('rejects a non-integer API minor-unit value', () => {
    expect(() => formatMinorAmountForInput(553.5)).toThrow('supported range')
  })
})
