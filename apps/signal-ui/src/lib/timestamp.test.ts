import { describe, expect, it } from 'vitest'
import { ResponseTimestampError, parseRequiredResponseTimestamp, serializeRequestTimestamp } from './timestamp'

describe('response timestamp helpers', () => {
  it.each([
    '0001-01-01T00:00:00Z',
    '2026-02-30T12:00:00Z',
    'not-a-timestamp',
  ])('rejects an unsupported or malformed timestamp: %s', (value) => {
    expect(() => parseRequiredResponseTimestamp(value, { api: 'Test', field: 'test.timestamp' }))
      .toThrow(ResponseTimestampError)
  })

  it('accepts an ordinary fixed-offset timestamp', () => {
    expect(parseRequiredResponseTimestamp('2026-06-20T12:00:00.123456789+05:30', { api: 'Test', field: 'test.offset' }))
      .toEqual(new Date('2026-06-20T12:00:00.123456789+05:30'))
  })

  it('serializes valid request instants and rejects invalid or year-one dates', () => {
    expect(serializeRequestTimestamp(new Date('2026-11-01T06:30:00+00:00'))).toBe('2026-11-01T06:30:00.000Z')
    expect(() => serializeRequestTimestamp(new Date('not-a-date'))).toThrow('Cannot serialize an invalid request timestamp')
    expect(() => serializeRequestTimestamp(new Date('0001-01-01T00:00:00Z'))).toThrow('Cannot serialize a year-one request timestamp')
  })

})
