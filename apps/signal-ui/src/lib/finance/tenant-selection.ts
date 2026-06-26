const STORAGE_KEY = 'signal-ui-finance-tenant-id'

export function getPreferredFinanceTenantId(): string {
  if (typeof window === 'undefined') {
    return ''
  }
  return window.localStorage.getItem(STORAGE_KEY) ?? ''
}

export function setPreferredFinanceTenantId(tenantId: string): void {
  if (typeof window === 'undefined') {
    return
  }
  if (tenantId.trim() === '') {
    window.localStorage.removeItem(STORAGE_KEY)
    return
  }
  window.localStorage.setItem(STORAGE_KEY, tenantId)
}

export function chooseFinanceTenantId<T extends { id: string }>(items: T[]): string {
  const preferred = getPreferredFinanceTenantId()
  if (preferred && items.some((item) => item.id === preferred)) {
    return preferred
  }
  if (items.length === 1) {
    return items[0].id
  }
  return ''
}
