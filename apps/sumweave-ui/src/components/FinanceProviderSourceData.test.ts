import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceProviderSourceData from './FinanceProviderSourceData.svelte'

const mocks = vi.hoisted(() => ({
  listAccountProviderSnapshots: vi.fn(),
  listTransactionProviderSnapshots: vi.fn(),
  getAccountProviderSnapshot: vi.fn(),
  getTransactionProviderSnapshot: vi.fn(),
}))

vi.mock('../lib/finance/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/finance/api')>()),
  createSignalFinanceApiForAuth: vi.fn(() => mocks),
}))
vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

function renderSourceData(scope: 'account' | 'transaction' = 'account') {
  return render(FinanceProviderSourceData, { tenantId: 'tenant-1', entityId: 'entity-1', entityLabel: scope, scope })
}

describe('Finance provider source data', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.listAccountProviderSnapshots.mockResolvedValue([])
    mocks.listTransactionProviderSnapshots.mockResolvedValue([])
  })

  it('loads account metadata only when opened and reveals sanitized details', async () => {
    const user = userEvent.setup()
    mocks.listAccountProviderSnapshots.mockResolvedValue([
      { id: 'snapshot-account', kind: 'account', providerObjectId: 'provider-account', capturedAt: new Date('2026-07-27T12:00:00Z') },
      { id: 'snapshot-balance', kind: 'account_balance', providerObjectId: 'provider-account', capturedAt: new Date('2026-07-27T12:00:00Z') },
    ])
    mocks.getAccountProviderSnapshot.mockResolvedValue({ id: 'snapshot-account', kind: 'account', providerObjectId: 'provider-account', capturedAt: new Date('2026-07-27T12:00:00Z'), data: { balance: 'sanitized' } })
    renderSourceData()

    expect(mocks.listAccountProviderSnapshots).not.toHaveBeenCalled()
    await user.click(screen.getByText('Provider source data'))
    expect(await screen.findByText('Account')).toBeInTheDocument()
    expect(screen.getByText('Account balance')).toBeInTheDocument()
    expect(await screen.findAllByText('Provider object provider-account', { exact: false })).toHaveLength(2)
    await user.click(screen.getAllByRole('button', { name: 'Reveal source data' })[0])
    expect(await screen.findByText('Provider snapshot data')).toBeInTheDocument()
    expect(mocks.getAccountProviderSnapshot).toHaveBeenCalledWith({ tenantId: 'tenant-1', accountId: 'entity-1', snapshotId: 'snapshot-account' })
    await user.click(screen.getByText('Provider source data'))
    await user.click(screen.getByText('Provider source data'))
    expect(mocks.listAccountProviderSnapshots).toHaveBeenCalledOnce()
  })

  it('shows a scoped empty state for transaction source data', async () => {
    const user = userEvent.setup()
    renderSourceData('transaction')
    await user.click(screen.getByText('Provider source data'))
    expect(await screen.findByText('No provider source data is available for this transaction.')).toBeInTheDocument()
    expect(mocks.listTransactionProviderSnapshots).toHaveBeenCalledWith({ tenantId: 'tenant-1', transactionId: 'entity-1' })
  })

  it('keeps loading feedback inside the expanded disclosure', async () => {
    const user = userEvent.setup()
    mocks.listAccountProviderSnapshots.mockReturnValue(new Promise(() => {}))
    renderSourceData()

    await user.click(screen.getByText('Provider source data'))

    expect(await screen.findByRole('status')).toHaveTextContent('Loading provider source data…')
  })

  it('identifies populated transaction snapshots without loading account snapshots', async () => {
    const user = userEvent.setup()
    mocks.listTransactionProviderSnapshots.mockResolvedValue([{ id: 'snapshot-transaction', kind: 'transaction', providerObjectId: 'provider-transaction', capturedAt: new Date('2026-07-27T12:00:00Z') }])
    renderSourceData('transaction')
    await user.click(screen.getByText('Provider source data'))
    expect(await screen.findByText('Transaction')).toBeInTheDocument()
    expect(mocks.listAccountProviderSnapshots).not.toHaveBeenCalled()
  })

  it('keeps metadata failures bounded', async () => {
    const user = userEvent.setup()
    mocks.listAccountProviderSnapshots.mockRejectedValueOnce('unavailable').mockResolvedValueOnce([])
    renderSourceData()
    await user.click(screen.getByText('Provider source data'))
    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to load provider source data')
    await user.click(screen.getByRole('button', { name: 'Retry loading source data' }))
    expect(await screen.findByText('No provider source data is available for this account.')).toBeInTheDocument()
  })

  it('keeps sanitized-detail failures bounded on the transaction scope', async () => {
    const user = userEvent.setup()
    mocks.listTransactionProviderSnapshots.mockResolvedValue([{ id: 'snapshot-1', kind: 'transaction', providerObjectId: 'provider-transaction', capturedAt: new Date('2026-07-27T12:00:00Z') }])
    mocks.getTransactionProviderSnapshot
      .mockRejectedValueOnce('unavailable')
      .mockResolvedValueOnce({ id: 'snapshot-1', kind: 'transaction', providerObjectId: 'provider-transaction', capturedAt: new Date('2026-07-27T12:00:00Z'), data: { amount: 10 } })
    renderSourceData('transaction')
    await user.click(screen.getByText('Provider source data'))
    await user.click(await screen.findByRole('button', { name: 'Reveal source data' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to reveal provider source data')
    await user.click(screen.getByRole('button', { name: 'Retry reveal' }))
    expect(await screen.findByText('Provider snapshot data')).toBeInTheDocument()
  })

  it('retains Error messages from account metadata and detail requests', async () => {
    const user = userEvent.setup()
    mocks.listAccountProviderSnapshots.mockRejectedValueOnce(new Error('metadata failed'))
    const first = renderSourceData()
    await user.click(screen.getByText('Provider source data'))
    expect(await screen.findByRole('alert')).toHaveTextContent('metadata failed')
    first.unmount()

    mocks.listAccountProviderSnapshots.mockResolvedValue([{ id: 'snapshot-1', kind: 'account', providerObjectId: 'provider-account', capturedAt: new Date('2026-07-27T12:00:00Z') }])
    mocks.getAccountProviderSnapshot.mockRejectedValueOnce(new Error('detail failed'))
    renderSourceData()
    await user.click(screen.getByText('Provider source data'))
    await user.click(await screen.findByRole('button', { name: 'Reveal source data' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('detail failed')
  })

  it('renders a safe null document when a current snapshot has no data', async () => {
    const user = userEvent.setup()
    mocks.listAccountProviderSnapshots.mockResolvedValue([{ id: 'snapshot-1', kind: 'account', providerObjectId: 'provider-account', capturedAt: new Date('2026-07-27T12:00:00Z') }])
    mocks.getAccountProviderSnapshot.mockResolvedValue({ id: 'snapshot-1', kind: 'account', providerObjectId: 'provider-account', capturedAt: new Date('2026-07-27T12:00:00Z') })
    renderSourceData()
    await user.click(screen.getByText('Provider source data'))
    await user.click(await screen.findByRole('button', { name: 'Reveal source data' }))
    expect(await screen.findByText('null')).toBeInTheDocument()
  })
})
