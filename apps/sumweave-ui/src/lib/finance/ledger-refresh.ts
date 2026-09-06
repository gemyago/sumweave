type LedgerRefreshListener = (tenantId: string) => void

const listeners = new Set<LedgerRefreshListener>()

/** Signals mounted ledger views that a completed classification changed categories. */
export function requestFinanceLedgerRefresh(tenantId: string): void {
  listeners.forEach((listener) => listener(tenantId))
}

export function subscribeToFinanceLedgerRefresh(listener: LedgerRefreshListener): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}
