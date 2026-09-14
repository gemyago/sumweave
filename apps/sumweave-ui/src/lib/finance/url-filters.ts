import { dateInputValue, withDateInput } from '../date-range'

const RFC3339_TIMESTAMP = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/

export function financeRouteQuery(): URLSearchParams {
  const hash = window.location.hash
  const queryStart = hash.indexOf('?')
  return new URLSearchParams(queryStart === -1 ? '' : hash.slice(queryStart + 1))
}

export function readDateQuery(query: URLSearchParams, key: string): Date | undefined {
  const value = query.get(key)
  return value ? withDateInput(undefined, value) : undefined
}

export function readTimestampQuery(query: URLSearchParams, key: string): Date | undefined {
  const value = query.get(key)
  if (!value || !RFC3339_TIMESTAMP.test(value)) return undefined

  const timestamp = new Date(value)
  return Number.isNaN(timestamp.getTime()) ? undefined : timestamp
}

export function replaceFinanceRouteQuery(values: Record<string, string | undefined>) {
  const hash = window.location.hash
  const route = (hash.startsWith('#') ? hash.slice(1) : hash).split('?')[0]
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(values)) {
    if (value) query.set(key, value)
  }
  const suffix = query.size ? `?${query}` : ''
  window.history.replaceState(window.history.state, '', `${window.location.pathname}${window.location.search}#${route}${suffix}`)
}

export function dateQueryValue(value: Date | undefined): string | undefined {
  return value ? dateInputValue(value) : undefined
}

export function timestampQueryValue(value: Date | undefined): string | undefined {
  if (!value || Number.isNaN(value.getTime())) return undefined

  const offsetMinutes = -value.getTimezoneOffset()
  const sign = offsetMinutes >= 0 ? '+' : '-'
  const absoluteOffset = Math.abs(offsetMinutes)
  const pad = (part: number, length = 2) => String(part).padStart(length, '0')
  const milliseconds = value.getMilliseconds() === 0 ? '' : `.${pad(value.getMilliseconds(), 3)}`
  return `${pad(value.getFullYear(), 4)}-${pad(value.getMonth() + 1)}-${pad(value.getDate())}T${pad(value.getHours())}:${pad(value.getMinutes())}:${pad(value.getSeconds())}${milliseconds}${sign}${pad(Math.floor(absoluteOffset / 60))}:${pad(absoluteOffset % 60)}`
}
