import { dateInputValue, withDateInput } from '../date-range'

export function financeRouteQuery(): URLSearchParams {
  const hash = window.location.hash
  const queryStart = hash.indexOf('?')
  return new URLSearchParams(queryStart === -1 ? '' : hash.slice(queryStart + 1))
}

export function readDateQuery(query: URLSearchParams, key: string): Date | undefined {
  const value = query.get(key)
  return value ? withDateInput(undefined, value) : undefined
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
