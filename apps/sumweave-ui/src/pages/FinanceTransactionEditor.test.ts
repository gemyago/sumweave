import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceTransactionEditor from './FinanceTransactionEditor.svelte'
import { FinanceResponseError } from '../lib/finance/api'

const mocks = vi.hoisted(() => ({
  listTenants: vi.fn(),
  listAccounts: vi.fn(),
  listCategories: vi.fn(),
  listTags: vi.fn(),
  getTransaction: vi.fn(),
  listTransferCandidates: vi.fn(),
  getTransferPartner: vi.fn(),
  linkTransferPair: vi.fn(),
  unlinkTransferPair: vi.fn(),
  createTransaction: vi.fn(),
  updateTransaction: vi.fn(),
  listTransactionProviderEvidence: vi.fn(),
  getTransactionProviderEvidence: vi.fn(),
}))

vi.mock('../lib/finance/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/finance/api')>()),
  createSignalFinanceApiForAuth: vi.fn(() => ({ ...mocks })),
}))

vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

describe('Finance transaction editor page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.localStorage.clear()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants.mockResolvedValue([
      { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now },
    ])
    mocks.listAccounts.mockResolvedValue([
      { id: 'account-1', tenantId: 'tenant-1', name: 'Checking', currency: 'USD', kind: 'manual', provider: '', providerAccountId: '', hiddenAt: null, createdAt: now, updatedAt: now },
      { id: 'account-2', tenantId: 'tenant-1', name: 'Savings', currency: 'EUR', kind: 'manual', provider: '', providerAccountId: '', hiddenAt: null, createdAt: now, updatedAt: now },
    ])
    mocks.listCategories.mockResolvedValue([
      { id: 'cat-1', tenantId: 'tenant-1', name: 'Groceries', kind: 'expense', seededDefault: true, hiddenAt: null, createdAt: now, updatedAt: now },
      { id: 'cat-2', tenantId: 'tenant-1', name: 'Travel', kind: 'expense', seededDefault: false, hiddenAt: null, createdAt: now, updatedAt: now },
    ])
    mocks.listTags.mockResolvedValue([
      { id: 'tag-1', tenantId: 'tenant-1', name: 'Household', hiddenAt: null, createdAt: now, updatedAt: now },
      { id: 'tag-2', tenantId: 'tenant-1', name: 'Shared', hiddenAt: null, createdAt: now, updatedAt: now },
    ])
    mocks.getTransaction.mockResolvedValue({
      id: 'tx-1',
      tenantId: 'tenant-1',
      accountId: 'account-1',
      source: 'provider',
      status: 'pending',
      kind: 'refund',
      amountMinor: 900,
      currency: 'USD',
      description: 'Refund',
      effectiveAt: now,
      categoryId: 'cat-1',
      tagIds: ['tag-1', 'tag-2'],
      transferGroupId: 'transfer-1',
      transferMatchedAt: null,
      hiddenAt: null,
      providerOriginal: {
        amountMinor: 950,
        currency: 'USD',
        description: 'Provider refund',
        effectiveAt: new Date('2026-06-19T12:00:00Z'),
      },
      createdAt: now,
      updatedAt: now,
    })
    mocks.listTransferCandidates.mockResolvedValue({ items: [] })
    mocks.getTransferPartner.mockResolvedValue(null)
    mocks.linkTransferPair.mockResolvedValue(undefined)
    mocks.unlinkTransferPair.mockResolvedValue(undefined)
    mocks.createTransaction.mockResolvedValue({
      id: 'tx-new',
      tenantId: 'tenant-1',
      accountId: 'account-1',
      source: 'manual',
      status: 'booked',
      kind: 'expense',
      amountMinor: 1200,
      currency: 'USD',
      description: 'Coffee',
      effectiveAt: now,
      categoryId: 'cat-1',
      tagIds: ['tag-1'],
      transferGroupId: null,
      transferMatchedAt: null,
      hiddenAt: null,
      providerOriginal: null,
      createdAt: now,
      updatedAt: now,
    })
    mocks.updateTransaction.mockResolvedValue({
      id: 'tx-1',
      tenantId: 'tenant-1',
      accountId: 'account-1',
      source: 'provider',
      status: 'pending',
      kind: 'refund',
      amountMinor: 800,
      currency: 'USD',
      description: 'Refund updated',
      effectiveAt: new Date('2026-06-21T12:00:00Z'),
      categoryId: null,
      tagIds: [],
      transferGroupId: 'transfer-1',
      transferMatchedAt: null,
      hiddenAt: null,
      providerOriginal: {
        amountMinor: 950,
        currency: 'USD',
        description: 'Provider refund',
        effectiveAt: new Date('2026-06-19T12:00:00Z'),
      },
      createdAt: now,
      updatedAt: now,
    })
    mocks.listTransactionProviderEvidence.mockResolvedValue([])
    mocks.getTransactionProviderEvidence.mockResolvedValue({})
  })

  it('initializes create mode with a blank editable record and submits through the shared editor', async () => {
    const user = userEvent.setup()
    const focus = vi.spyOn(HTMLElement.prototype, 'focus')
    render(FinanceTransactionEditor, { params: {} })

    expect(await screen.findByRole('heading', { name: 'Details' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Record transaction' })).not.toBeInTheDocument()
    expect(screen.queryByText('Transaction editor')).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Back to transactions' })).not.toBeInTheDocument()
    expect(mocks.getTransaction).not.toHaveBeenCalled()
    await screen.findByLabelText('Amount')

    expect(screen.getByLabelText('Transaction kind')).toHaveValue('expense')
    expect(screen.getByLabelText('Transaction effective at')).toHaveValue(localTodayAtMidnight())
    expect(screen.getByRole('combobox', { name: 'Transaction currency' })).toHaveValue('USD')

    await user.clear(screen.getByLabelText('Amount'))
    await user.type(screen.getByLabelText('Amount'), '12.00')
    await user.clear(screen.getByLabelText('Transaction description'))
    await user.type(screen.getByLabelText('Transaction description'), 'Coffee')
    await user.clear(screen.getByLabelText('Transaction effective at'))
    await user.type(screen.getByLabelText('Transaction effective at'), '2026-06-20T12:00')
    await user.selectOptions(screen.getByLabelText('Transaction category'), 'cat-1')
    await user.click(screen.getByLabelText('Household'))
    await user.click(screen.getByRole('button', { name: 'Save transaction' }))

    await waitFor(() => expect(mocks.createTransaction).toHaveBeenCalled())
    expect(await screen.findByText('Transaction recorded.')).toBeInTheDocument()
    expect(focus).toHaveBeenCalledWith({ preventScroll: true })
    expect(mocks.createTransaction).toHaveBeenCalledWith(expect.objectContaining({ amountMinor: 1200, tagIds: ['tag-1'] }))
    expect(screen.getByRole('link', { name: 'Cancel' })).toHaveAttribute('href', '#/finance/transactions')
    focus.mockRestore()
  })

  it('keeps details first, clears stale save feedback after an edit, and omits context copy', async () => {
    const user = userEvent.setup()
    render(FinanceTransactionEditor, { params: {} })

    await screen.findByRole('heading', { name: 'Details' })
    expect(screen.queryByText('Transaction context')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Save transaction' }))
    expect(await screen.findByText('Transaction recorded.')).toBeInTheDocument()
    await user.type(screen.getByLabelText('Transaction description'), ' changed')
    expect(screen.queryByText('Transaction recorded.')).not.toBeInTheDocument()
  })

  it('updates create-mode values and currency from the selected account without free-text transfer grouping', async () => {
    const user = userEvent.setup()
    render(FinanceTransactionEditor, { params: {} })

    await screen.findByRole('heading', { name: 'Details' })
    await screen.findByLabelText('Transaction account')
    await user.selectOptions(screen.getByLabelText('Transaction account'), 'account-2')
    await user.selectOptions(screen.getByLabelText('Transaction status'), 'pending')
    await user.selectOptions(screen.getByLabelText('Transaction kind'), 'refund')

    expect(screen.getByLabelText('Transaction currency')).toHaveValue('EUR')
    expect(screen.getByLabelText('Transaction status')).toHaveValue('pending')
    expect(screen.getByLabelText('Transaction kind')).toHaveValue('refund')
    expect(screen.queryByLabelText('Transfer group')).not.toBeInTheDocument()
  })

  it('does not offer hidden accounts when recording a new transaction', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listAccounts.mockResolvedValueOnce([
      { id: 'account-active', tenantId: 'tenant-1', name: 'Checking', currency: 'USD', kind: 'manual', provider: '', providerAccountId: '', hiddenAt: null, createdAt: now, updatedAt: now },
      { id: 'account-hidden', tenantId: 'tenant-1', name: 'Old checking', currency: 'USD', kind: 'linked', provider: 'bank', providerAccountId: '', hiddenAt: now, createdAt: now, updatedAt: now },
    ])
    render(FinanceTransactionEditor, { params: {} })

    expect(await screen.findByRole('option', { name: 'Checking' })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: /Old checking/ })).not.toBeInTheDocument()
    expect(mocks.listAccounts).toHaveBeenCalledWith({ tenantId: 'tenant-1', includeHidden: true })
  })

  it('renders persisted regular as a valid transaction kind option', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getTransaction.mockResolvedValueOnce({
      id: 'tx-regular', tenantId: 'tenant-1', accountId: 'account-1', source: 'provider', status: 'booked', kind: 'regular', amountMinor: -100,
      currency: 'USD', description: 'Regular debit', effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now,
    })
    render(FinanceTransactionEditor, { params: { transactionId: 'tx-regular' } })

    expect(await screen.findByLabelText('Transaction kind')).toHaveValue('regular')
  })

  it('uses other-account candidates, defensively disables stale ineligible rows without eligibility copy, and resets paging when applying the range', async () => {
    const user = userEvent.setup()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getTransaction.mockResolvedValueOnce({
      id: 'tx-out', tenantId: 'tenant-1', accountId: 'account-1', source: 'provider', status: 'booked', kind: 'regular', amountMinor: -100,
      currency: 'USD', description: 'Transfer out', effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now,
    })
    mocks.listTransferCandidates.mockResolvedValue({
      items: Array.from({ length: 20 }, (_, index) => ({
        id: `candidate-${index}`, tenantId: 'tenant-1', accountId: index === 0 ? 'account-1' : 'account-2', source: 'provider', status: 'booked', kind: 'regular', amountMinor: 100,
        currency: 'USD', description: `Candidate ${index}`, effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now,
      })),
    })
    render(FinanceTransactionEditor, { params: { transactionId: 'tx-out' } })

    await user.click(await screen.findByRole('button', { name: 'Link transfer' }))
    await waitFor(() => expect(mocks.listTransferCandidates).toHaveBeenCalledWith(expect.objectContaining({ tenantId: 'tenant-1', transactionId: 'tx-out', limit: 20, offset: 0 })))
    expect(screen.getByText('Candidates are from other visible accounts. The effective-before boundary is exclusive.')).toBeInTheDocument()
    expect(screen.getByLabelText('Select Candidate 0')).toBeDisabled()
    expect(screen.queryByText(/Eligibility:/)).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Transfer candidate pages: next page' }))
    await waitFor(() => expect(mocks.listTransferCandidates).toHaveBeenLastCalledWith(expect.objectContaining({ offset: 20 })))
    await user.click(screen.getByRole('button', { name: 'Apply' }))
    await waitFor(() => expect(mocks.listTransferCandidates).toHaveBeenLastCalledWith(expect.objectContaining({ offset: 0 })))
  })

  it('keeps transfer candidates and their pager usable when a later candidate page fails', async () => {
    const user = userEvent.setup()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getTransaction.mockResolvedValueOnce({
      id: 'tx-out', tenantId: 'tenant-1', accountId: 'account-1', source: 'provider', status: 'booked', kind: 'regular', amountMinor: -100,
      currency: 'USD', description: 'Transfer out', effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now,
    })
    mocks.listTransferCandidates
      .mockResolvedValueOnce({ items: Array.from({ length: 20 }, (_, index) => ({
        id: `candidate-${index}`, tenantId: 'tenant-1', accountId: 'account-2', source: 'provider', status: 'booked', kind: 'regular', amountMinor: 100,
        currency: 'USD', description: `Candidate ${index}`, effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now,
      })) })
      .mockRejectedValueOnce(new Error('Candidates changed'))
    render(FinanceTransactionEditor, { params: { transactionId: 'tx-out' } })

    await user.click(await screen.findByRole('button', { name: 'Link transfer' }))
    expect(await screen.findByText('Candidate 0')).toBeInTheDocument()
    const next = screen.getByRole('button', { name: 'Transfer candidate pages: next page' })
    await user.click(next)

    expect(await screen.findByRole('alert')).toHaveTextContent('Candidates changed')
    expect(screen.getByText('Candidate 0')).toBeInTheDocument()
    expect(next).toBeEnabled()
  })

  it('confirms an eligible candidate, surfaces recoverable link failures, and reloads after success', async () => {
    const user = userEvent.setup()
    const now = new Date('2026-06-20T12:00:00Z')
    const source = {
      id: 'tx-out', tenantId: 'tenant-1', accountId: 'account-1', source: 'provider', status: 'booked', kind: 'regular', amountMinor: -100,
      currency: 'USD', description: 'Transfer out', effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now,
    }
    mocks.getTransaction.mockResolvedValue(source)
    mocks.listTransferCandidates.mockResolvedValue({ items: [{ ...source, id: 'tx-in', accountId: 'account-2', amountMinor: 100, description: 'Transfer in' }] })
    mocks.linkTransferPair.mockRejectedValueOnce(new Error('Conflict'))
    render(FinanceTransactionEditor, { params: { transactionId: 'tx-out' } })

    await user.click(await screen.findByRole('button', { name: 'Link transfer' }))
    await user.click(await screen.findByLabelText('Select Transfer in'))
    expect(screen.getByText('Confirm internal transfer')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Confirm link transfer' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('The record may have changed; refresh candidates and try again.')
    mocks.linkTransferPair.mockResolvedValueOnce(undefined)
    await user.click(screen.getByRole('button', { name: 'Confirm link transfer' }))
    await waitFor(() => expect(mocks.linkTransferPair).toHaveBeenLastCalledWith({ tenantId: 'tenant-1', firstTransactionId: 'tx-out', secondTransactionId: 'tx-in' }))
    expect(await screen.findByText(/Internal transfer linked/)).toBeInTheDocument()
    expect(screen.queryByLabelText('Transfer candidates')).not.toBeInTheDocument()
  })

  it('keeps candidate lookup errors recoverable without leaving the detail route', async () => {
    const user = userEvent.setup()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getTransaction.mockResolvedValueOnce({
      id: 'tx-out', tenantId: 'tenant-1', accountId: 'account-1', source: 'provider', status: 'booked', kind: 'regular', amountMinor: -100,
      currency: 'USD', description: 'Transfer out', effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now,
    })
    mocks.listTransferCandidates.mockRejectedValueOnce(new Error('Candidate list changed')).mockResolvedValueOnce({ items: [] })
    render(FinanceTransactionEditor, { params: { transactionId: 'tx-out' } })

    await user.click(await screen.findByRole('button', { name: 'Link transfer' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Candidate list changed')
    await user.click(screen.getByRole('button', { name: 'Refresh candidates' }))
    expect(await screen.findByText('No transfer candidates matched this date range.')).toBeInTheDocument()
  })

  it('lazily loads a matched partner and requires confirmation before unlinking', async () => {
    const user = userEvent.setup()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getTransaction.mockResolvedValueOnce({
      id: 'tx-out', tenantId: 'tenant-1', accountId: 'account-1', source: 'provider', status: 'booked', kind: 'transfer', amountMinor: -100,
      currency: 'USD', description: 'Transfer out', effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: 'group-1', transferMatchedAt: now, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now,
    })
    mocks.getTransferPartner.mockResolvedValueOnce({
      id: 'tx-in', tenantId: 'tenant-1', accountId: 'account-2', source: 'provider', status: 'booked', kind: 'transfer', amountMinor: 100,
      currency: 'USD', description: 'Transfer in', effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: 'group-1', transferMatchedAt: now, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now,
    })
    render(FinanceTransactionEditor, { params: { transactionId: 'tx-out' } })

    await waitFor(() => expect(mocks.getTransferPartner).toHaveBeenCalledWith({ tenantId: 'tenant-1', transactionId: 'tx-out' }))
    expect(await screen.findByRole('link', { name: 'Open linked transaction' })).toHaveAttribute('href', '#/finance/transactions/tx-in')
    await user.click(await screen.findByRole('button', { name: 'Unlink transfer' }))
    expect(screen.getByLabelText('Confirm unlink transfer')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Confirm unlink' }))
    await waitFor(() => expect(mocks.unlinkTransferPair).toHaveBeenCalledWith({ tenantId: 'tenant-1', firstTransactionId: 'tx-out', secondTransactionId: 'tx-in' }))
  })

  it('keeps a matched-partner lookup failure recoverable', async () => {
    const user = userEvent.setup()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.getTransaction.mockResolvedValueOnce({
      id: 'tx-out', tenantId: 'tenant-1', accountId: 'account-1', source: 'provider', status: 'booked', kind: 'transfer', amountMinor: -100,
      currency: 'USD', description: 'Transfer out', effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: 'group-1', transferMatchedAt: now, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now,
    })
    mocks.getTransferPartner.mockRejectedValueOnce(new Error('Partner changed')).mockResolvedValueOnce({
      id: 'tx-in', tenantId: 'tenant-1', accountId: 'account-2', source: 'provider', status: 'booked', kind: 'transfer', amountMinor: 100,
      currency: 'USD', description: 'Transfer in', effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: 'group-1', transferMatchedAt: now, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now,
    })
    render(FinanceTransactionEditor, { params: { transactionId: 'tx-out' } })

    expect(await screen.findByRole('alert')).toHaveTextContent('Partner changed')
    await user.click(screen.getByRole('button', { name: 'Retry linked transfer' }))
    expect(await screen.findByText('Linked with Savings')).toBeInTheDocument()
  })

  it('uses the supported currency selector and keeps the mobile field order task-first', async () => {
    render(FinanceTransactionEditor, { params: {} })

    await screen.findByLabelText('Transaction account')

    expect(screen.queryByRole('textbox', { name: 'Transaction currency' })).not.toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Transaction currency' })).toHaveTextContent('USDEURPLNUAH')

    const fields = Array.from(document.querySelectorAll('form label, form legend')).map((field) => field.textContent)
    expect(fields.slice(0, 8)).toEqual(['Account', 'Category', 'Tags', 'Household', 'Shared', 'Kind', 'Amount', 'Currency'])
  })

  it('loads edit mode through the detail endpoint, shows provider-original context, and saves nullable category updates', async () => {
    const user = userEvent.setup()
    render(FinanceTransactionEditor, { params: { transactionId: 'tx-1' } })

    expect(await screen.findByRole('heading', { name: 'Details' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Transaction' })).not.toBeInTheDocument()
    await waitFor(() =>
      expect(mocks.getTransaction).toHaveBeenCalledWith({
        tenantId: 'tenant-1',
        transactionId: 'tx-1',
      }),
    )
    await screen.findByText('Provider refund')
    expect(screen.getByLabelText('Household')).toBeChecked()
    expect(screen.getByLabelText('Shared')).toBeChecked()
    expect(screen.getByLabelText('Transaction status')).toHaveValue('pending')
    expect(screen.getByLabelText('Transaction kind')).toHaveValue('refund')

    expect(screen.getByLabelText('Amount')).toHaveValue('9.00')
    await user.clear(screen.getByLabelText('Amount'))
    await user.type(screen.getByLabelText('Amount'), '8.00')
    await user.clear(screen.getByLabelText('Transaction description'))
    await user.type(screen.getByLabelText('Transaction description'), 'Refund updated')
    await user.selectOptions(screen.getByLabelText('Transaction category'), '')
    await user.click(screen.getByLabelText('Household'))
    await user.click(screen.getByLabelText('Shared'))
    await user.clear(screen.getByLabelText('Transaction effective at'))
    await user.type(screen.getByLabelText('Transaction effective at'), '2026-06-21T12:00')
    await user.click(screen.getByRole('button', { name: 'Save transaction' }))

    await waitFor(() =>
      expect(mocks.updateTransaction).toHaveBeenCalledWith({
        tenantId: 'tenant-1',
        transactionId: 'tx-1',
        description: 'Refund updated',
        amountMinor: 800,
        effectiveAt: new Date('2026-06-21T12:00'),
        categoryId: null,
        tagIds: [],
      }),
    )
    expect(await screen.findByRole('status')).toHaveTextContent('Transaction updated.')
  })

  it('submits a negative major-unit decimal as exact minor units', async () => {
    const user = userEvent.setup()
    render(FinanceTransactionEditor, { params: {} })

    await user.clear(await screen.findByLabelText('Amount'))
    await user.type(screen.getByLabelText('Amount'), '-553.00')
    await user.click(screen.getByRole('button', { name: 'Save transaction' }))

    await waitFor(() => expect(mocks.createTransaction).toHaveBeenCalled())
    expect(mocks.createTransaction).toHaveBeenCalledWith(
      expect.objectContaining({
        accountId: 'account-1',
        amountMinor: -55300,
      }),
    )
  })

  it('rejects malformed and over-precision major-unit amounts before calling the API', async () => {
    const user = userEvent.setup()
    render(FinanceTransactionEditor, { params: {} })

    await user.clear(await screen.findByLabelText('Amount'))
    await user.type(screen.getByLabelText('Amount'), '12.345')
    await user.click(screen.getByRole('button', { name: 'Save transaction' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Amount must be a whole number or have at most two decimal places.')
    expect(mocks.createTransaction).not.toHaveBeenCalled()
  })

  it('loads current evidence for distinct provider objects only after the disclosure opens and content only after reveal', async () => {
    const user = userEvent.setup()
    mocks.listTransactionProviderEvidence.mockResolvedValueOnce([
      { id: 'evidence-1', scope: 'transaction', providerObjectId: 'provider-tx', capturedAt: new Date('2026-06-20T12:00:00Z') },
      { id: 'evidence-2', scope: 'transaction', providerObjectId: 'provider-fee', capturedAt: new Date('2026-06-20T12:01:00Z') },
    ])
    mocks.getTransactionProviderEvidence.mockResolvedValueOnce({
      id: 'evidence-1', scope: 'transaction', providerObjectId: 'provider-tx', capturedAt: new Date('2026-06-20T12:00:00Z'), payload: { amount: 'sanitized' },
    })
    render(FinanceTransactionEditor, { params: { transactionId: 'tx-1' } })

    expect(await screen.findByText('Current provider evidence')).toBeInTheDocument()
    expect(mocks.listTransactionProviderEvidence).not.toHaveBeenCalled()
    await user.click(screen.getByText('Current provider evidence'))
    await waitFor(() => expect(mocks.listTransactionProviderEvidence).toHaveBeenCalledWith({ tenantId: 'tenant-1', transactionId: 'tx-1' }))
    expect(mocks.getTransactionProviderEvidence).not.toHaveBeenCalled()
    expect(screen.getByText('Provider object provider-tx', { exact: false })).toBeInTheDocument()
    expect(screen.getByText('Provider object provider-fee', { exact: false })).toBeInTheDocument()
    await user.click(screen.getAllByRole('button', { name: 'Reveal current sanitized details' })[0])
    await waitFor(() => expect(mocks.getTransactionProviderEvidence).toHaveBeenCalledWith({ tenantId: 'tenant-1', transactionId: 'tx-1', evidenceId: 'evidence-1' }))
    expect(screen.getByText('Current sanitized provider evidence')).toBeInTheDocument()
    expect(screen.getByText(/not the raw provider payload/i)).toBeInTheDocument()
  })

  it('round-trips a local datetime value through the transaction editor', async () => {
    const environment = (globalThis as unknown as { process: { env: Record<string, string | undefined> } }).process.env
    const previousTimezone = environment.TZ
    environment.TZ = 'America/Los_Angeles'
    try {
      const user = userEvent.setup()
      const localTime = new Date(2026, 10, 1, 1, 30)
      mocks.getTransaction.mockResolvedValueOnce({
        id: 'tx-1', tenantId: 'tenant-1', accountId: 'account-1', source: 'provider', status: 'pending', kind: 'refund',
        amountMinor: 900, currency: 'USD', description: 'Refund', effectiveAt: localTime, categoryId: 'cat-1', tagIds: ['tag-1'],
        transferGroupId: 'transfer-1', transferMatchedAt: null, hiddenAt: null, providerOriginal: null,
        createdAt: new Date('2026-06-20T12:00:00Z'), updatedAt: new Date('2026-06-20T12:00:00Z'),
      })

      render(FinanceTransactionEditor, { params: { transactionId: 'tx-1' } })

      expect(await screen.findByLabelText('Transaction effective at')).toHaveValue('2026-11-01T01:30')
      await user.click(screen.getByRole('button', { name: 'Save transaction' }))

      await waitFor(() => expect(mocks.updateTransaction).toHaveBeenCalled())
      expect(mocks.updateTransaction.mock.calls[0][0]).toMatchObject({ effectiveAt: localTime })
    } finally {
      if (previousTimezone === undefined) delete environment.TZ
      else environment.TZ = previousTimezone
    }
  })

  it('requires an explicit tenant choice for deep-link edit routes when multiple tenants are joined', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    const user = userEvent.setup()
    mocks.listTenants.mockResolvedValueOnce([
      { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now },
      { id: 'tenant-2', name: 'Travel', displayCurrency: 'EUR', joinedAt: now, createdAt: now, updatedAt: now },
    ])

    render(FinanceTransactionEditor, { params: { transactionId: 'tx-1' } })

    expect(await screen.findByText('Select an active tenant to continue on this finance route.')).toBeInTheDocument()
    expect(mocks.getTransaction).not.toHaveBeenCalled()

    await user.selectOptions(screen.getByRole('combobox', { name: 'Tenant' }), 'tenant-2')
    await waitFor(() => expect(mocks.getTransaction).toHaveBeenCalledWith({ tenantId: 'tenant-2', transactionId: 'tx-1' }))
  })

  it('shows the finance-tenant empty state when no joined tenant is available', async () => {
    mocks.listTenants.mockResolvedValueOnce([])

    render(FinanceTransactionEditor, { params: { transactionId: 'tx-1' } })

    const tenantsLink = await screen.findByRole('link', { name: 'Finance tenants' })
    expect(tenantsLink.closest('[role="status"]')).toHaveTextContent(
      'Create or join a tenant from Finance tenants before editing transactions.',
    )
    expect(mocks.listAccounts).not.toHaveBeenCalled()
    expect(mocks.getTransaction).not.toHaveBeenCalled()
  })

  it('shows save errors without leaving the shared editor flow', async () => {
    const user = userEvent.setup()
    mocks.updateTransaction.mockRejectedValueOnce(new Error('save exploded'))

    render(FinanceTransactionEditor, { params: { transactionId: 'tx-1' } })

    await screen.findByRole('heading', { name: 'Details' })
    await screen.findByRole('button', { name: 'Save transaction' })
    await user.click(screen.getByRole('button', { name: 'Save transaction' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('save exploded')
  })

  it('shows a bounded response error when transaction hydration receives a malformed timestamp', async () => {
    mocks.getTransaction.mockRejectedValueOnce(new FinanceResponseError({
      field: 'finance.transaction.effectiveAt',
      issue: 'must be a valid RFC3339 timestamp',
    }))

    render(FinanceTransactionEditor, { params: { transactionId: 'tx-1' } })

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Finance API response contract violation: finance.transaction.effectiveAt',
    )
    expect(screen.getByLabelText('Transaction effective at')).toHaveValue(localTodayAtMidnight())
  })
})

function localTodayAtMidnight(): string {
  const today = new Date()
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${today.getFullYear()}-${pad(today.getMonth() + 1)}-${pad(today.getDate())}T00:00`
}
