export interface LocalCalendarDateRange {
  startDate: string
  endDate: string
}

export interface LocalCalendarRequestRange extends LocalCalendarDateRange {
  rangeStart: string
  rangeEndExclusive: string
}

export function defaultLocalCalendarDateRange(now = new Date()): LocalCalendarDateRange {
  const end = localStartOfDay(now)
  return {
    startDate: localDateValue(addLocalDays(end, -29)),
    endDate: localDateValue(end),
  }
}

export function localCalendarRangeFromDateInputs(startDate: string, endDate: string): LocalCalendarRequestRange {
  const rangeStart = localDateStart(startDate)
  const inclusiveEnd = localDateStart(endDate)
  if (rangeStart > inclusiveEnd) {
    throw new TypeError('Start date must be on or before end date.')
  }

  return {
    startDate,
    endDate,
    rangeStart: serializeLocalRFC3339(rangeStart),
    rangeEndExclusive: serializeLocalRFC3339(addLocalDays(inclusiveEnd, 1)),
  }
}

function localStartOfDay(value: Date): Date {
  return new Date(value.getFullYear(), value.getMonth(), value.getDate())
}

function addLocalDays(value: Date, days: number): Date {
  const result = new Date(value)
  result.setDate(result.getDate() + days)
  return result
}

function localDateValue(value: Date): string {
  return `${value.getFullYear()}-${pad(value.getMonth() + 1)}-${pad(value.getDate())}`
}

function localDateStart(value: string): Date {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  if (!match) throw new TypeError('Enter a valid local calendar date.')
  const [year, month, day] = match.slice(1).map(Number)
  const parsed = new Date(year, month - 1, day)
  if (parsed.getFullYear() !== year || parsed.getMonth() !== month - 1 || parsed.getDate() !== day) {
    throw new TypeError('Enter a valid local calendar date.')
  }
  return parsed
}

function serializeLocalRFC3339(value: Date): string {
  const offsetMinutes = -value.getTimezoneOffset()
  const sign = offsetMinutes >= 0 ? '+' : '-'
  const absoluteOffset = Math.abs(offsetMinutes)
  return `${value.getFullYear()}-${pad(value.getMonth() + 1)}-${pad(value.getDate())}T${pad(value.getHours())}:${pad(value.getMinutes())}:${pad(value.getSeconds())}${sign}${pad(Math.floor(absoluteOffset / 60))}:${pad(absoluteOffset % 60)}`
}

function pad(value: number): string {
  return String(value).padStart(2, '0')
}
