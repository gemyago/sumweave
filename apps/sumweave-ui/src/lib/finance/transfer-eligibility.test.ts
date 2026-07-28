import { describe, expect, it } from 'vitest'
import type { FinanceTransaction } from './api'
import { isMatchedTransfer, transferLinkEligibilityIssue } from './transfer-eligibility'

function transaction(overrides: Partial<FinanceTransaction> = {}): FinanceTransaction {
  const now = new Date('2026-06-20T12:00:00Z')
  return {
    id: 'source', tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'regular', amountMinor: -100,
    currency: 'USD', description: 'Source', effectiveAt: now, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, createdAt: now, updatedAt: now,
    ...overrides,
  }
}

describe('transfer link eligibility', () => {
  it('matches the backend link rules with precise candidate reasons', () => {
    const source = transaction()
    expect(transferLinkEligibilityIssue(source, transaction({ id: 'candidate', accountId: 'account-2', amountMinor: 100 }))).toBeNull()
    expect(transferLinkEligibilityIssue(source, transaction({ id: 'source' }))).toBe('Choose a different transaction.')
    expect(transferLinkEligibilityIssue(source, transaction({ id: 'candidate', amountMinor: 100 }))).toBe('Choose a transaction from a different account.')
    expect(transferLinkEligibilityIssue(source, transaction({ id: 'candidate', accountId: 'account-2', status: 'pending', amountMinor: 100 }))).toBe('Both transactions must be booked.')
    expect(transferLinkEligibilityIssue(source, transaction({ id: 'candidate', accountId: 'account-2', amountMinor: 0 }))).toBe('Both transactions must have a non-zero amount.')
    expect(transferLinkEligibilityIssue(source, transaction({ id: 'candidate', accountId: 'account-2', amountMinor: -100 }))).toBe('The amounts must have opposite directions.')
    expect(transferLinkEligibilityIssue(source, transaction({ id: 'candidate', accountId: 'account-2', amountMinor: 100, transferGroupId: 'group-1' }))).toBe('Unlink an existing transfer before linking either transaction again.')
    expect(transferLinkEligibilityIssue(transaction({ transferMatchedAt: new Date() }), transaction({ id: 'candidate', accountId: 'account-2', amountMinor: 100 }))).toBe('Unlink an existing transfer before linking either transaction again.')
  })

  it('recognizes only persisted matched transfers', () => {
    expect(isMatchedTransfer(transaction({ kind: 'transfer', transferGroupId: 'group-1', transferMatchedAt: new Date() }))).toBe(true)
    expect(isMatchedTransfer(transaction({ kind: 'transfer', transferGroupId: 'group-1' }))).toBe(false)
    expect(isMatchedTransfer(transaction())).toBe(false)
  })
})
