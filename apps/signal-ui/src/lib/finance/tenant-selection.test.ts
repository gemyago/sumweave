import { beforeEach, describe, expect, it } from 'vitest'
import { chooseFinanceTenantId, getPreferredFinanceTenantId, setPreferredFinanceTenantId } from './tenant-selection'

describe('finance tenant selection helpers', () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  it('stores, reads, and clears the preferred tenant id', () => {
    setPreferredFinanceTenantId('tenant-1')
    expect(getPreferredFinanceTenantId()).toBe('tenant-1')
    setPreferredFinanceTenantId('')
    expect(getPreferredFinanceTenantId()).toBe('')
  })

  it('chooses preferred or first available tenant ids', () => {
    setPreferredFinanceTenantId('tenant-2')
    expect(chooseFinanceTenantId([{ id: 'tenant-1' }, { id: 'tenant-2' }])).toBe('tenant-2')
    expect(chooseFinanceTenantId([{ id: 'tenant-1' }])).toBe('tenant-1')
    setPreferredFinanceTenantId('')
    expect(chooseFinanceTenantId([{ id: 'tenant-1' }, { id: 'tenant-2' }])).toBe('')
    expect(chooseFinanceTenantId([])).toBe('')
  })

  it('handles missing window in non-browser contexts', () => {
    const originalWindow = globalThis.window
    // @ts-expect-error test-only window removal
    delete globalThis.window

    expect(getPreferredFinanceTenantId()).toBe('')
    expect(() => setPreferredFinanceTenantId('tenant-1')).not.toThrow()

    globalThis.window = originalWindow
  })
})
