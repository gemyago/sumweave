import { describe, expect, it, vi } from 'vitest'
import { faker } from '@faker-js/faker'
import {
  acknowledgeFinanceLedgerRefresh,
  financeLedgerRefreshRevision,
  isFinanceLedgerRefreshPending,
  requestFinanceLedgerRefresh,
  subscribeToFinanceLedgerRefresh,
} from './ledger-refresh'

describe('finance ledger refresh signal', () => {
  it('notifies mounted ledger views only until they unsubscribe', () => {
    const listener = vi.fn()
    const unsubscribe = subscribeToFinanceLedgerRefresh(listener)

    requestFinanceLedgerRefresh('tenant-1')
    unsubscribe()
    requestFinanceLedgerRefresh('tenant-2')

    expect(listener).toHaveBeenCalledTimes(1)
    expect(listener).toHaveBeenCalledWith('tenant-1')
  })

  it('keeps a newer tenant refresh pending until its matching ledger load completes', () => {
    const tenantId = faker.string.uuid()

    requestFinanceLedgerRefresh(tenantId)
    const firstRevision = financeLedgerRefreshRevision(tenantId)
    requestFinanceLedgerRefresh(tenantId)
    const secondRevision = financeLedgerRefreshRevision(tenantId)

    acknowledgeFinanceLedgerRefresh(tenantId, firstRevision)
    expect(isFinanceLedgerRefreshPending(tenantId)).toBe(true)
    acknowledgeFinanceLedgerRefresh(tenantId, secondRevision)
    expect(isFinanceLedgerRefreshPending(tenantId)).toBe(false)
  })
})
