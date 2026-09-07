type LedgerRefreshListener = (tenantId: string) => void

const listeners = new Set<LedgerRefreshListener>()
const pendingRefreshRevisions = new Map<string, number>()

/** Signals mounted ledger views that completed finance work changed a tenant ledger. */
export function requestFinanceLedgerRefresh(tenantId: string): void {
  pendingRefreshRevisions.set(tenantId, (pendingRefreshRevisions.get(tenantId) ?? 0) + 1)
  listeners.forEach((listener) => listener(tenantId))
}

export function subscribeToFinanceLedgerRefresh(listener: LedgerRefreshListener): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function financeLedgerRefreshRevision(tenantId: string): number {
  return pendingRefreshRevisions.get(tenantId) ?? 0
}

export function acknowledgeFinanceLedgerRefresh(tenantId: string, revision: number): void {
  if (pendingRefreshRevisions.get(tenantId) === revision) {
    pendingRefreshRevisions.delete(tenantId)
  }
}

export function isFinanceLedgerRefreshPending(tenantId: string): boolean {
  return pendingRefreshRevisions.has(tenantId)
}
