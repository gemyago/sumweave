import { describe, expect, it } from 'vitest'
import {
  UTC_RANGE_PRESETS,
  buildUtcIsoFromParts,
  calendarDateFromUtcIso,
  parseUtcTimestamp,
  resolveUtcRangePreset,
  validateUtcRange,
} from './utc-date-range'

describe('utc date range utilities', () => {
  it('parses explicit utc timestamps and rejects non-utc text', () => {
    expect(parseUtcTimestamp('2026-06-15T12:00:00Z')?.toISOString()).toBe('2026-06-15T12:00:00.000Z')
    expect(parseUtcTimestamp('2026-06-15T12:00:00')).toBeNull()
  })

  it('builds utc iso values from a calendar date and time while preserving milliseconds', () => {
    const date = calendarDateFromUtcIso('2026-06-15T12:34:56.789Z')

    expect(buildUtcIsoFromParts(date, '08:09:10', '2026-06-15T12:34:56.789Z')).toBe(
      '2026-06-15T08:09:10.789Z',
    )
  })

  it('resolves the required quick presets against an explicit utc anchor', () => {
    const anchorIso = '2026-06-16T12:00:00.000Z'

    expect(
      UTC_RANGE_PRESETS.map((preset) => resolveUtcRangePreset({ presetKey: preset.key, anchorIso })),
    ).toEqual([
      { startIso: '2026-06-15T12:00:00.000Z', endIso: anchorIso, clamped: false },
      { startIso: '2026-06-09T12:00:00.000Z', endIso: anchorIso, clamped: false },
      { startIso: '2026-05-17T12:00:00.000Z', endIso: anchorIso, clamped: false },
      { startIso: '2026-03-18T12:00:00.000Z', endIso: anchorIso, clamped: false },
      { startIso: '2025-12-18T12:00:00.000Z', endIso: anchorIso, clamped: false },
    ])
  })

  it('clamps preset ranges to provided utc bounds', () => {
    expect(
      resolveUtcRangePreset({
        presetKey: 'last-7d',
        anchorIso: '2026-06-16T12:00:00.000Z',
        minIso: '2026-06-12T00:00:00.000Z',
        maxIso: '2026-06-15T12:00:00.000Z',
      }),
    ).toEqual({
      startIso: '2026-06-12T00:00:00.000Z',
      endIso: '2026-06-15T12:00:00.000Z',
      clamped: true,
    })
  })

  it('reports invalid, reversed, bounded, and capped ranges', () => {
    expect(
      validateUtcRange({
        startIso: '2026-06-15T13:00:00Z',
        endIso: '2026-06-15T13:00:00Z',
        requiredStartMessage: 'UTC start is required.',
        requiredEndMessage: 'UTC end is required.',
        invalidStartMessage: 'UTC start must be a valid ISO-8601 timestamp.',
        invalidEndMessage: 'UTC end must be a valid ISO-8601 timestamp.',
        notEarlierMessage: 'UTC start must be earlier than UTC end.',
      }),
    ).toEqual(['UTC start must be earlier than UTC end.'])

    expect(
      validateUtcRange({
        startIso: '2026-06-15T11:00:00Z',
        endIso: '2026-06-15T12:00:00Z',
        requiredStartMessage: 'UTC start is required.',
        requiredEndMessage: 'UTC end is required.',
        invalidStartMessage: 'UTC start must be a valid ISO-8601 timestamp.',
        invalidEndMessage: 'UTC end must be a valid ISO-8601 timestamp.',
        notEarlierMessage: 'UTC start must be earlier than UTC end.',
        minIso: '2026-06-15T11:30:00Z',
        maxIso: '2026-06-15T12:30:00Z',
        outOfBoundsMessage: 'UTC range must stay within the selected availability window.',
        timeframeDurationMs: 60_000,
        maxIntervals: 10,
        maxIntervalsMessage: 'Selected range exceeds the server limit of 10,000 1m intervals.',
      }),
    ).toEqual([
      'UTC range must stay within the selected availability window.',
      'Selected range exceeds the server limit of 10,000 1m intervals.',
    ])

    expect(
      validateUtcRange({
        startIso: '2026-06-15T11:00:00Z',
        endIso: '2026-06-23T00:00:01Z',
        requiredStartMessage: 'UTC start is required.',
        requiredEndMessage: 'UTC end is required.',
        invalidStartMessage: 'UTC start must be a valid ISO-8601 timestamp.',
        invalidEndMessage: 'UTC end must be a valid ISO-8601 timestamp.',
        notEarlierMessage: 'UTC start must be earlier than UTC end.',
        timeframeDurationMs: 60_000,
        maxIntervals: 10_000,
        maxIntervalsMessage: 'Selected range exceeds the server limit of 10,000 1m intervals.',
      }),
    ).toEqual(['Selected range exceeds the server limit of 10,000 1m intervals.'])
  })
})
