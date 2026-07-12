const RFC3339_TIMESTAMP = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?(Z|([+-])(\d{2}):(\d{2}))$/

export class ResponseTimestampError extends Error {
  readonly field: string

  constructor(params: { api: string; field: string; issue: string }) {
    super(`${params.api} API response contract violation: ${params.field} ${params.issue}`)
    this.name = 'ResponseTimestampError'
    this.field = params.field
  }
}

/** Encodes a request instant without sending invalid or backend-zero timestamps. */
export function serializeRequestTimestamp(value: Date): string {
  if (!(value instanceof Date) || Number.isNaN(value.getTime())) {
    throw new TypeError('Cannot serialize an invalid request timestamp')
  }

  const serialized = value.toISOString()
  if (serialized.startsWith('0001-')) {
    throw new TypeError('Cannot serialize a year-one request timestamp')
  }
  return serialized
}

/** Decodes an RFC3339 instant without letting malformed values reach UI state. */
export function parseRequiredResponseTimestamp(value: unknown, params: { api: string; field: string }): Date {
  if (typeof value !== 'string' || value === '') {
    throw new ResponseTimestampError({ ...params, issue: 'must be a non-empty RFC3339 timestamp' })
  }
  const parts = RFC3339_TIMESTAMP.exec(value)
  if (!parts || !isValidTimestampParts(parts)) {
    throw new ResponseTimestampError({ ...params, issue: 'must be a valid RFC3339 timestamp' })
  }
  if (parts[1] === '0001') {
    throw new ResponseTimestampError({ ...params, issue: 'must not be a year-one timestamp' })
  }
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    throw new ResponseTimestampError({ ...params, issue: 'must be a valid RFC3339 timestamp' })
  }
  return parsed
}

/** Ordinary lifecycle instants are shown in the browser's local timezone. */
export function formatLocalDateTime(value: Date | null | undefined): string {
  if (!value || Number.isNaN(value.getTime())) {
    return '—'
  }
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(value)
}

function isValidTimestampParts(parts: RegExpExecArray): boolean {
  const year = Number(parts[1])
  const month = Number(parts[2])
  const day = Number(parts[3])
  const hour = Number(parts[4])
  const minute = Number(parts[5])
  const second = Number(parts[6])
  const fraction = parts[7] ?? ''
  const offsetHour = parts[9] === undefined ? 0 : Number(parts[10])
  const offsetMinute = parts[9] === undefined ? 0 : Number(parts[11])

  if (month < 1 || month > 12 || day < 1 || day > daysInMonth(year, month)) return false
  if (hour > 23 || minute > 59 || second > 59) return false
  if (fraction.length > 9 || offsetHour > 23 || offsetMinute > 59) return false
  return true
}

function daysInMonth(year: number, month: number): number {
  if (month === 2) return year % 400 === 0 || (year % 4 === 0 && year % 100 !== 0) ? 29 : 28
  return [4, 6, 9, 11].includes(month) ? 30 : 31
}
