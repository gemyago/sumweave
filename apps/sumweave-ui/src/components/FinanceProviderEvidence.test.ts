import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceProviderEvidence from './FinanceProviderEvidence.svelte'

const mocks = vi.hoisted(() => ({
  listAccountProviderEvidence: vi.fn(),
  listTransactionProviderEvidence: vi.fn(),
  getAccountProviderEvidence: vi.fn(),
  getTransactionProviderEvidence: vi.fn(),
}))

vi.mock('../lib/finance/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/finance/api')>()),
  createSignalFinanceApiForAuth: vi.fn(() => mocks),
}))
vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

function renderEvidence(scope: 'account' | 'transaction' = 'account') {
  return render(FinanceProviderEvidence, { tenantId: 'tenant-1', entityId: 'entity-1', entityLabel: scope, scope })
}

describe('Finance provider evidence', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.listAccountProviderEvidence.mockResolvedValue([])
    mocks.listTransactionProviderEvidence.mockResolvedValue([])
  })

  it('loads account metadata only when opened and reveals sanitized details', async () => {
    const user = userEvent.setup()
    mocks.listAccountProviderEvidence.mockResolvedValue([
      { id: 'evidence-1', scope: 'account', providerObjectId: 'provider-account', capturedAt: new Date('2026-07-27T12:00:00Z') },
      { id: 'evidence-2', scope: 'account', providerObjectId: 'provider-savings', capturedAt: new Date('2026-07-27T12:00:00Z') },
    ])
    mocks.getAccountProviderEvidence.mockResolvedValue({ id: 'evidence-1', scope: 'account', providerObjectId: 'provider-account', capturedAt: new Date('2026-07-27T12:00:00Z'), payload: { balance: 'sanitized' } })
    renderEvidence()

    expect(mocks.listAccountProviderEvidence).not.toHaveBeenCalled()
    await user.click(screen.getByLabelText('Current provider evidence'))
    expect(await screen.findByText('Provider object provider-account', { exact: false })).toBeInTheDocument()
    await user.click(screen.getAllByRole('button', { name: 'Reveal current sanitized details' })[0])
    expect(await screen.findByText('Current sanitized provider evidence')).toBeInTheDocument()
    expect(mocks.getAccountProviderEvidence).toHaveBeenCalledWith({ tenantId: 'tenant-1', accountId: 'entity-1', evidenceId: 'evidence-1' })
    await user.click(screen.getByLabelText('Current provider evidence'))
    await user.click(screen.getByLabelText('Current provider evidence'))
    expect(mocks.listAccountProviderEvidence).toHaveBeenCalledOnce()
  })

  it('shows a scoped empty state for transaction evidence', async () => {
    const user = userEvent.setup()
    renderEvidence('transaction')
    await user.click(screen.getByLabelText('Current provider evidence'))
    expect(await screen.findByText('No current provider evidence is available for this transaction.')).toBeInTheDocument()
    expect(mocks.listTransactionProviderEvidence).toHaveBeenCalledWith({ tenantId: 'tenant-1', transactionId: 'entity-1' })
  })

  it('identifies populated transaction observations without loading account evidence', async () => {
    const user = userEvent.setup()
    mocks.listTransactionProviderEvidence.mockResolvedValue([{ id: 'evidence-transaction', scope: 'transaction', providerObjectId: 'provider-transaction', capturedAt: new Date('2026-07-27T12:00:00Z') }])
    renderEvidence('transaction')
    await user.click(screen.getByLabelText('Current provider evidence'))
    expect(await screen.findByText('Current transaction evidence')).toBeInTheDocument()
    expect(mocks.listAccountProviderEvidence).not.toHaveBeenCalled()
  })

  it('keeps metadata failures bounded', async () => {
    const user = userEvent.setup()
    mocks.listAccountProviderEvidence.mockRejectedValue('unavailable')
    renderEvidence()
    await user.click(screen.getByLabelText('Current provider evidence'))
    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to load provider evidence metadata')
  })

  it('keeps sanitized-detail failures bounded on the transaction scope', async () => {
    const user = userEvent.setup()
    mocks.listTransactionProviderEvidence.mockResolvedValue([{ id: 'evidence-1', scope: 'transaction', providerObjectId: 'provider-transaction', capturedAt: new Date('2026-07-27T12:00:00Z') }])
    mocks.getTransactionProviderEvidence.mockRejectedValue('unavailable')
    renderEvidence('transaction')
    await user.click(screen.getByLabelText('Current provider evidence'))
    await user.click(await screen.findByRole('button', { name: 'Reveal current sanitized details' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to reveal sanitized provider evidence')
  })

  it('retains Error messages from account metadata and detail requests', async () => {
    const user = userEvent.setup()
    mocks.listAccountProviderEvidence.mockRejectedValueOnce(new Error('metadata failed'))
    const first = renderEvidence()
    await user.click(screen.getByLabelText('Current provider evidence'))
    expect(await screen.findByRole('alert')).toHaveTextContent('metadata failed')
    first.unmount()

    mocks.listAccountProviderEvidence.mockResolvedValue([{ id: 'evidence-1', scope: 'account', providerObjectId: 'provider-account', capturedAt: new Date('2026-07-27T12:00:00Z') }])
    mocks.getAccountProviderEvidence.mockRejectedValueOnce(new Error('detail failed'))
    renderEvidence()
    await user.click(screen.getByLabelText('Current provider evidence'))
    await user.click(await screen.findByRole('button', { name: 'Reveal current sanitized details' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('detail failed')
  })

  it('renders a safe null payload when a current observation has no detail body', async () => {
    const user = userEvent.setup()
    mocks.listAccountProviderEvidence.mockResolvedValue([{ id: 'evidence-1', scope: 'account', providerObjectId: 'provider-account', capturedAt: new Date('2026-07-27T12:00:00Z') }])
    mocks.getAccountProviderEvidence.mockResolvedValue({ id: 'evidence-1', scope: 'account', providerObjectId: 'provider-account', capturedAt: new Date('2026-07-27T12:00:00Z') })
    renderEvidence()
    await user.click(screen.getByLabelText('Current provider evidence'))
    await user.click(await screen.findByRole('button', { name: 'Reveal current sanitized details' }))
    expect(await screen.findByText('null')).toBeInTheDocument()
  })
})
