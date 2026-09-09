import { beforeEach, describe, expect, it } from 'vitest'
import { dateQueryValue, financeRouteQuery, readDateQuery, replaceFinanceRouteQuery } from './url-filters'

describe('finance URL filters', () => {
  beforeEach(() => {
    window.location.hash = '#/finance/transactions'
  })

  it('reads date filters from a direct hash URL', () => {
    window.location.hash = '#/finance/transactions?startDate=2026-06-01&endDate=2026-06-30&type=expense'

    const query = financeRouteQuery()

    expect(dateQueryValue(readDateQuery(query, 'startDate'))).toBe('2026-06-01')
    expect(dateQueryValue(readDateQuery(query, 'endDate'))).toBe('2026-06-30')
    expect(query.get('type')).toBe('expense')
  })

  it('replaces only populated filters without leaving the current finance route', () => {
    replaceFinanceRouteQuery({ startDate: dateQueryValue(new Date(2026, 5, 1)), type: 'income', endDate: undefined })

    expect(window.location.hash).toBe('#/finance/transactions?startDate=2026-06-01&type=income')
  })
})
