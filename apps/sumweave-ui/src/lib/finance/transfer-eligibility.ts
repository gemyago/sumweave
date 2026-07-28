import type { FinanceTransaction } from './api'

export function transferLinkEligibilityIssue(
  source: FinanceTransaction,
  candidate: FinanceTransaction,
): string | null {
  if (source.id === candidate.id) return 'Choose a different transaction.'
  if (source.accountId === candidate.accountId) return 'Choose a transaction from a different account.'
  if (source.status !== 'booked' || candidate.status !== 'booked') return 'Both transactions must be booked.'
  if (source.amountMinor === 0 || candidate.amountMinor === 0) return 'Both transactions must have a non-zero amount.'
  if (Math.sign(source.amountMinor) === Math.sign(candidate.amountMinor)) return 'The amounts must have opposite directions.'
  if (source.transferGroupId || source.transferMatchedAt || candidate.transferGroupId || candidate.transferMatchedAt) {
    return 'Unlink an existing transfer before linking either transaction again.'
  }
  return null
}

export function isMatchedTransfer(transaction: FinanceTransaction): boolean {
  return transaction.kind === 'transfer' && Boolean(transaction.transferGroupId) && Boolean(transaction.transferMatchedAt)
}
