import { describe, expect, it } from 'vitest'
import { RANGE_PRESETS, parseTimestamp, resolveRangePreset, validateRange } from './date-range'

const messages = {
  requiredStartMessage: 'Start is required.',
  requiredEndMessage: 'End is required.',
  invalidStartMessage: 'Start must be a valid timestamp.',
  invalidEndMessage: 'End must be a valid timestamp.',
  notEarlierMessage: 'Start must be earlier than end.',
}

describe('date range utilities', () => {
  it('parses explicit boundary instants and rejects timezone-free text', () => {
    expect(parseTimestamp('2026-06-15T12:00:00Z')?.toISOString()).toBe('2026-06-15T12:00:00.000Z')
    expect(parseTimestamp('2026-06-15T12:00:00')).toBeNull()
  })

  it('resolves quick presets as native Dates against an explicit instant anchor', () => {
    const anchor = new Date('2026-06-16T12:00:00.000Z')
    expect(RANGE_PRESETS.map((preset) => resolveRangePreset({ presetKey: preset.key, anchor }))).toEqual([
      { start: new Date('2026-06-15T12:00:00.000Z'), end: anchor, clamped: false },
      { start: new Date('2026-06-09T12:00:00.000Z'), end: anchor, clamped: false },
      { start: new Date('2026-05-17T12:00:00.000Z'), end: anchor, clamped: false },
      { start: new Date('2026-03-18T12:00:00.000Z'), end: anchor, clamped: false },
      { start: new Date('2025-12-18T12:00:00.000Z'), end: anchor, clamped: false },
    ])
  })

  it('clamps and validates timestamp ranges', () => {
    expect(resolveRangePreset({
      presetKey: 'last-7d',
      anchor: new Date('2026-06-16T12:00:00.000Z'),
      min: new Date('2026-06-12T00:00:00.000Z'),
      max: new Date('2026-06-15T12:00:00.000Z'),
    })).toEqual({
      start: new Date('2026-06-12T00:00:00.000Z'),
      end: new Date('2026-06-15T12:00:00.000Z'),
      clamped: true,
    })

    expect(validateRange({
      start: new Date('2026-06-15T11:00:00Z'),
      end: new Date('2026-06-15T12:00:00Z'),
      min: new Date('2026-06-15T11:30:00Z'),
      outOfBoundsMessage: 'outside',
      timeframeDurationMs: 60_000,
      maxIntervals: 10,
      maxIntervalsMessage: 'too many',
      ...messages,
    })).toEqual(['outside', 'too many'])
  })

  it('preserves missing, malformed, and reversed validation', () => {
    expect(validateRange({ start: undefined, end: undefined, ...messages })).toEqual([
      'Start is required.', 'End is required.',
    ])
    expect(validateRange({ start: new Date(Number.NaN), end: new Date(Number.NaN), ...messages })).toEqual([
      'Start must be a valid timestamp.', 'End must be a valid timestamp.',
    ])
    const instant = new Date('2026-06-15T13:00:00Z')
    expect(validateRange({ start: instant, end: instant, ...messages })).toEqual([
      'Start must be earlier than end.',
    ])
  })
})
