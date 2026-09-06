import { describe, expect, it, vi } from 'vitest'
import { requestFinanceLedgerRefresh, subscribeToFinanceLedgerRefresh } from './ledger-refresh'

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
})
