import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceTransactions from './FinanceTransactions.svelte'

const mocks = vi.hoisted(() => ({
  listTenants: vi.fn(),
  listAccounts: vi.fn(),
  listCategories: vi.fn(),
  listTags: vi.fn(),
  listTransactions: vi.fn(),
  updateTransaction: vi.fn(),
}))

vi.mock('../lib/finance/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/finance/api')>()),
  createSignalFinanceApiForAuth: vi.fn(() => ({ ...mocks })),
}))

vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

describe('Finance transactions page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.localStorage.clear()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants.mockResolvedValue([
      { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now },
    ])
    mocks.listAccounts.mockResolvedValue([
      { id: 'account-1', tenantId: 'tenant-1', name: 'Checking', currency: 'USD', kind: 'manual', provider: '', providerAccountId: '', hiddenAt: null, createdAt: now, updatedAt: now },
    ])
    mocks.listCategories.mockResolvedValue([
      { id: 'cat-1', tenantId: 'tenant-1', name: 'Groceries', kind: 'expense', seededDefault: true, hiddenAt: null, createdAt: now, updatedAt: now },
    ])
    mocks.listTags.mockResolvedValue([
      { id: 'tag-1', tenantId: 'tenant-1', name: 'Household', hiddenAt: null, createdAt: now, updatedAt: now },
      { id: 'tag-2', tenantId: 'tenant-1', name: 'Shared', hiddenAt: null, createdAt: now, updatedAt: now },
    ])
    mocks.listTransactions.mockResolvedValue([
      {
        id: 'tx-1',
        tenantId: 'tenant-1',
        accountId: 'account-1',
        source: 'manual',
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
        hiddenAt: now,
        providerOriginal: null,
        createdAt: now,
        updatedAt: now,
      },
    ])
    mocks.updateTransaction.mockImplementation(async (params) => ({
      ...((await mocks.listTransactions.mock.results[0]?.value)?.[0] ?? {}),
      ...params,
      id: params.transactionId,
      categoryId: params.categoryId ?? null,
    }))
  })

  it('renders the shared transaction list and full-editor navigation link', async () => {
    render(FinanceTransactions)
    expect(await screen.findByText('Refund')).toBeInTheDocument()
    expect(screen.getByLabelText('Transactions ledger')).toHaveClass('finance-transaction-list')
    expect(screen.getByRole('link', { name: 'Create transaction' })).toHaveAttribute('href', '#/finance/transactions/new')
    expect(screen.getByRole('link', { name: 'Open full transaction details' })).toHaveAttribute('href', '#/finance/transactions/tx-1')
    expect(screen.queryByLabelText('Selected transaction details')).not.toBeInTheDocument()
    expect(screen.getAllByText('pending').length).toBeGreaterThan(0)
    expect(screen.getByText('hidden')).toBeInTheDocument()
    expect(screen.getAllByText('refund').length).toBeGreaterThan(0)
  })

  it('keeps hidden-account names and badges in transaction history and filters', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listAccounts.mockResolvedValueOnce([
      { id: 'account-hidden', tenantId: 'tenant-1', name: 'Old checking', currency: 'USD', kind: 'linked', provider: 'bank', providerAccountId: '', hiddenAt: now, createdAt: now, updatedAt: now },
    ])
    mocks.listTransactions.mockResolvedValueOnce([
      { id: 'tx-hidden-account', tenantId: 'tenant-1', accountId: 'account-hidden', source: 'provider', status: 'booked', kind: 'expense', amountMinor: -100, currency: 'USD', description: 'Historic charge', effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, createdAt: now, updatedAt: now },
    ])
    render(FinanceTransactions)

    expect(await screen.findByText('Old checking')).toBeInTheDocument()
    expect(screen.getByText('Hidden account')).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Old checking (Hidden)' })).toBeInTheDocument()
    expect(mocks.listAccounts).toHaveBeenCalledWith({ tenantId: 'tenant-1', includeHidden: true })
  })

  it('marks repeated inline icon actions for the shared mobile touch-target treatment', async () => {
    const user = userEvent.setup()
    render(FinanceTransactions)
    await screen.findByText('Refund')

    for (const name of ['Edit description', 'Edit category', 'Edit tags', 'Open full transaction details']) {
      expect(screen.getByRole(name === 'Open full transaction details' ? 'link' : 'button', { name })).toHaveClass('finance-transaction-list-action')
    }

    await user.click(screen.getByRole('button', { name: 'Edit description' }))
    expect(screen.getByRole('button', { name: 'Save description' })).toHaveClass('finance-transaction-list-action')
    expect(screen.getByRole('button', { name: 'Cancel description edit' })).toHaveClass('finance-transaction-list-action')
  })

  it('uses equal responsive grid cells while keeping active category and tag editors beside their actions', async () => {
    const user = userEvent.setup()
    render(FinanceTransactions)
    await screen.findByText('Refund')

    await user.click(screen.getByRole('button', { name: 'Edit category' }))
    const category = screen.getByLabelText('Category')
    const categoryRow = category.closest('.finance-transaction-list-editor-row')
    const categoryCell = category.closest('.col-12.col-md-6')
    expect(categoryCell?.parentElement).toHaveClass('row', 'mx-0')
    expect(categoryRow).toHaveClass('flex-nowrap')
    expect(categoryRow).toContainElement(screen.getByRole('button', { name: 'Save category' }))
    expect(categoryRow).toContainElement(screen.getByRole('button', { name: 'Cancel category edit' }))

    await user.click(screen.getByRole('button', { name: 'Cancel category edit' }))
    await user.click(screen.getByRole('button', { name: 'Edit tags' }))
    const tags = screen.getByRole('group', { name: 'Tags' })
    const tagsRow = tags.closest('.finance-transaction-list-editor-row')
    const tagChoices = tags.querySelector('.finance-transaction-list-tag-choices')
    const tagsCell = tags.closest('.col-12.col-md-6')
    expect(tagsCell?.parentElement).toHaveClass('row', 'mx-0')
    expect(tagsCell).toHaveClass('col-12', 'col-md-6')
    expect(tagsRow).toHaveClass('flex-nowrap')
    expect(tagsRow).toContainElement(screen.getByRole('button', { name: 'Save tags' }))
    expect(tagsRow).toContainElement(screen.getByRole('button', { name: 'Cancel tags edit' }))
    expect(tagChoices).toHaveClass('flex-wrap')
  })

  it('keeps a matched transfer to one transfer badge while retaining inline category and tag actions', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTransactions.mockResolvedValueOnce([
      {
        id: 'tx-transfer',
        tenantId: 'tenant-1',
        accountId: 'account-1',
        source: 'manual',
        status: 'booked',
        kind: 'transfer',
        amountMinor: 900,
        currency: 'USD',
        description: 'Transfer out',
        effectiveAt: now,
        categoryId: 'cat-1',
        tagIds: ['tag-1'],
        transferGroupId: 'transfer-1',
        transferMatchedAt: now,
        hiddenAt: null,
        providerOriginal: null,
        createdAt: now,
        updatedAt: now,
      },
    ])

    render(FinanceTransactions)

    expect(await screen.findByText('Transfer out')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Edit category' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Edit tags' })).toBeInTheDocument()
    expect(screen.getAllByText('internal transfer')).toHaveLength(1)
    expect(screen.queryByText('transfer', { exact: true })).not.toBeInTheDocument()
  })

  it('withholds unavailable category and tag edits, then retries each catalog without blocking description editing', async () => {
    const user = userEvent.setup()
    mocks.listCategories.mockRejectedValueOnce(new Error('Categories unavailable'))
    mocks.listTags.mockRejectedValueOnce(new Error('Tags unavailable'))
    render(FinanceTransactions)

    expect(await screen.findByText('Categories could not load.')).toBeInTheDocument()
    expect(screen.getByText('Tags could not load.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Edit category' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Edit tags' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Save category' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Save tags' })).not.toBeInTheDocument()
    expect(mocks.updateTransaction).not.toHaveBeenCalled()

    await user.click(screen.getByRole('button', { name: 'Edit description' }))
    expect(screen.getByLabelText('Description')).toBeEnabled()
    await user.click(screen.getByRole('button', { name: 'Cancel description edit' }))

    await user.click(screen.getByRole('button', { name: 'Retry category catalog' }))
    expect(await screen.findByRole('button', { name: 'Edit category' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Edit tags' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Retry tag catalog' }))
    expect(await screen.findByRole('button', { name: 'Edit tags' })).toBeInTheDocument()
    expect(mocks.updateTransaction).not.toHaveBeenCalled()
  })

  it('saves a focused inline description update without navigating away', async () => {
    const user = userEvent.setup()
    mocks.updateTransaction.mockResolvedValueOnce({
      ...(await mocks.listTransactions.mock.results[0]?.value)?.[0],
      id: 'tx-1', description: 'Refund corrected', tagIds: ['tag-1', 'tag-2'], categoryId: 'cat-1',
    })
    render(FinanceTransactions)
    await screen.findByText('Refund')

    await user.click(screen.getByRole('button', { name: 'Edit description' }))
    const description = screen.getByLabelText('Description')
    await user.clear(description)
    await user.type(description, 'Refund corrected')
    await user.click(screen.getByRole('button', { name: 'Save description' }))

    await waitFor(() => expect(mocks.updateTransaction).toHaveBeenCalledWith(expect.objectContaining({
      tenantId: 'tenant-1', transactionId: 'tx-1', description: 'Refund corrected', categoryId: 'cat-1', tagIds: ['tag-1', 'tag-2'],
    })))
    expect(await screen.findByText('Refund corrected')).toBeInTheDocument()
  })

  it('autofocuses the description editor, submits with Enter, and cancels with Escape', async () => {
    const user = userEvent.setup()
    render(FinanceTransactions)
    await screen.findByText('Refund')

    await user.click(screen.getByRole('button', { name: 'Edit description' }))
    const description = screen.getByLabelText('Description')
    expect(description).toHaveFocus()
    await user.clear(description)
    await user.type(description, 'Enter saves')
    await user.keyboard('{Enter}')
    await waitFor(() => expect(mocks.updateTransaction).toHaveBeenCalledWith(expect.objectContaining({ description: 'Enter saves' })))

    await user.click(screen.getByRole('button', { name: 'Edit description' }))
    await user.type(screen.getByLabelText('Description'), ' discarded')
    await user.keyboard('{Escape}')
    expect(screen.queryByLabelText('Description')).not.toBeInTheDocument()
    expect(mocks.updateTransaction).toHaveBeenCalledTimes(1)
  })

  it('can clear a category inline while preserving description and tags', async () => {
    const user = userEvent.setup()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.updateTransaction.mockResolvedValueOnce({
      id: 'tx-1', tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'pending', kind: 'refund', amountMinor: 900, currency: 'USD', description: 'Refund', effectiveAt: now, categoryId: null, tagIds: ['tag-1', 'tag-2'], createdAt: now, updatedAt: now,
    })
    render(FinanceTransactions)
    await screen.findByText('Refund')

    await user.click(screen.getByRole('button', { name: 'Edit category' }))
    await user.selectOptions(screen.getByLabelText('Category'), '')
    await user.click(screen.getByRole('button', { name: 'Save category' }))

    await waitFor(() => expect(mocks.updateTransaction).toHaveBeenCalledWith(expect.objectContaining({
      categoryId: null, description: 'Refund', tagIds: ['tag-1', 'tag-2'],
    })))
    expect(await screen.findByText('No category')).toBeInTheDocument()
  })

  it('cancels an inline tag edit without calling the API', async () => {
    const user = userEvent.setup()
    render(FinanceTransactions)
    await screen.findByText('Refund')

    await user.click(screen.getByRole('button', { name: 'Edit tags' }))
    await user.click(screen.getByRole('checkbox', { name: 'Household' }))
    await user.click(screen.getByRole('button', { name: 'Cancel tags edit' }))

    expect(mocks.updateTransaction).not.toHaveBeenCalled()
    expect(screen.getByText('Shared')).toBeInTheDocument()
  })

  it('keeps all transfer selection and pairing controls off the browse route', async () => {
    render(FinanceTransactions)
    await screen.findByText('Refund')
    expect(screen.queryByLabelText(/Select Refund/)).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Internal transfer pairing')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /link.*transfer/i })).not.toBeInTheDocument()
  })

  it('resolves category labels from the tenant category catalog', async () => {
    render(FinanceTransactions)

    expect(await screen.findByText('Groceries')).toBeInTheDocument()
    expect(screen.queryByText('cat-1')).not.toBeInTheDocument()
    expect(mocks.listCategories).toHaveBeenCalledWith({ tenantId: 'tenant-1' })
  })

  it('resolves tag labels from the tenant tag catalog and hides raw IDs', async () => {
    render(FinanceTransactions)

    expect((await screen.findAllByText('Household')).length).toBeGreaterThan(0)
    expect(screen.getByText('Shared')).toBeInTheDocument()
    expect(screen.queryByText('tag-1')).not.toBeInTheDocument()
    expect(mocks.listTags).toHaveBeenCalledWith({ tenantId: 'tenant-1' })
  })

  it('shows unknown tag when an assigned ID is absent from the tag catalog', async () => {
    mocks.listTransactions.mockResolvedValue([{
      id: 'tx-unknown-tag', tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'expense', amountMinor: 100, currency: 'USD', description: 'Unknown tag transaction', effectiveAt: new Date('2026-06-20T12:00:00Z'), categoryId: null, tagIds: ['missing-tag'], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: new Date('2026-06-20T12:00:00Z'), updatedAt: new Date('2026-06-20T12:00:00Z'),
    }])

    render(FinanceTransactions)

    expect(await screen.findByText('Unknown tag')).toBeInTheDocument()
    expect(screen.queryByText('missing-tag')).not.toBeInTheDocument()
  })

  it('renders the empty state when filters return no transactions', async () => {
    mocks.listTransactions.mockResolvedValueOnce([])
    render(FinanceTransactions)
    expect(await screen.findByText('No transactions matched the current filters.')).toBeInTheDocument()
  })

  it('renders reconciliation state badges and oldest-first sorting', async () => {
    const earlier = new Date('2026-06-19T12:00:00Z')
    const later = new Date('2026-06-20T12:00:00Z')
    mocks.listTransactions.mockResolvedValueOnce([
      { id: 'tx-1', tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'expense', amountMinor: 300, currency: 'USD', description: 'Later', effectiveAt: later, categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: later, updatedAt: later },
      { id: 'tx-2', tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'reconciliation', amountMinor: 100, currency: 'USD', description: 'Earlier', effectiveAt: earlier, categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: earlier, updatedAt: earlier },
    ])
    const user = userEvent.setup()
    const { container } = render(FinanceTransactions)
    await user.selectOptions(await screen.findByRole('combobox', { name: 'Sort order' }), 'asc')
    await waitFor(() => {
      const rows = Array.from(container.querySelectorAll('article'))
      expect(rows[0]?.textContent).toContain('Earlier')
    })
    expect(screen.getAllByText('booked').length).toBeGreaterThan(0)
    expect(screen.getAllByText('reconciliation').length).toBeGreaterThan(0)
  })

  it('keeps provider-original details off the browse route', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTransactions.mockResolvedValueOnce([
      {
        id: 'tx-1',
        tenantId: 'tenant-1',
        accountId: 'account-1',
        source: 'provider',
        status: 'booked',
        kind: 'expense',
        amountMinor: 300,
        currency: 'USD',
        description: 'Card charge',
        effectiveAt: now,
        categoryId: null,
        tagIds: [],
        transferGroupId: null,
        transferMatchedAt: null,
        hiddenAt: null,
        providerOriginal: {
          amountMinor: 350,
          currency: 'USD',
          description: 'Original provider memo',
          effectiveAt: now,
        },
        createdAt: now,
        updatedAt: now,
      },
    ])

    render(FinanceTransactions)

    expect(await screen.findByText('Card charge')).toBeInTheDocument()
    expect(screen.queryByText('Provider original')).not.toBeInTheDocument()
    expect(screen.queryByText('Original provider memo')).not.toBeInTheDocument()
  })

  it('renders only the empty table state when a later filter result is empty', async () => {
    const user = userEvent.setup()
    mocks.listTransactions
      .mockResolvedValueOnce([
        {
          id: 'tx-1',
          tenantId: 'tenant-1',
          accountId: 'account-1',
          source: 'manual',
          status: 'pending',
          kind: 'expense',
          amountMinor: 900,
          currency: 'USD',
          description: 'Refund',
          effectiveAt: new Date('2026-06-20T12:00:00Z'),
          categoryId: 'cat-1',
          tagIds: [],
          transferGroupId: null,
          transferMatchedAt: null,
          hiddenAt: null,
          providerOriginal: null,
          createdAt: new Date('2026-06-20T12:00:00Z'),
          updatedAt: new Date('2026-06-20T12:00:00Z'),
        },
      ])
      .mockResolvedValueOnce([])

    render(FinanceTransactions)

    await screen.findByText('Refund')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Transaction status filter' }), 'booked')

    expect(await screen.findByText('No transactions matched the current filters.')).toBeInTheDocument()
    expect(screen.queryByLabelText('Selected transaction details')).not.toBeInTheDocument()
  })

  it('reloads transaction filters when selectors change', async () => {
    const user = userEvent.setup()
    render(FinanceTransactions)
    await user.selectOptions(await screen.findByRole('combobox', { name: 'Account filter' }), 'account-1')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Transaction status filter' }), 'pending')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Transaction source filter' }), 'manual')
    await waitFor(() =>
      expect(mocks.listTransactions).toHaveBeenLastCalledWith({
        tenantId: 'tenant-1',
        accountId: 'account-1',
        status: 'pending',
        source: 'manual',
        limit: 20,
        offset: 0,
      }),
    )
  })

  it('loads the next fixed-size transaction page', async () => {
    const user = userEvent.setup()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTransactions
      .mockResolvedValueOnce(Array.from({ length: 20 }, (_, index) => ({
        id: `tx-${index + 1}`,
        tenantId: 'tenant-1',
        accountId: 'account-1',
        source: 'manual',
        status: 'booked',
        kind: 'expense',
        amountMinor: 1200,
        currency: 'USD',
        description: `Transaction ${index + 1}`,
        effectiveAt: now,
        categoryId: null,
        tagIds: [],
        transferGroupId: null,
        transferMatchedAt: null,
        hiddenAt: null,
        providerOriginal: null,
        createdAt: now,
        updatedAt: now,
      })))
      .mockResolvedValueOnce([
        { id: 'tx-21', tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'expense', amountMinor: 1200, currency: 'USD', description: 'Transaction 21', effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now },
      ])

    render(FinanceTransactions)

    expect(await screen.findByText('Transaction 1')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Transaction pages: next page' }))

    expect(await screen.findByText('Transaction 21')).toBeInTheDocument()
    expect(mocks.listTransactions).toHaveBeenLastCalledWith({
      tenantId: 'tenant-1',
      accountId: '',
      status: '',
      source: '',
      limit: 20,
      offset: 20,
    })
  })

  it('keeps the ledger and pager mounted while a page request is pending', async () => {
    const user = userEvent.setup()
    const now = new Date('2026-06-20T12:00:00Z')
    let resolveNextPage!: (transactions: Awaited<ReturnType<typeof mocks.listTransactions>>) => void
    mocks.listTransactions
      .mockResolvedValueOnce(Array.from({ length: 20 }, (_, index) => ({
        id: `tx-${index}`, tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'expense', amountMinor: 100,
        currency: 'USD', description: `Transaction ${index}`, effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now,
      })))
      .mockImplementationOnce(() => new Promise((resolve) => { resolveNextPage = resolve }))
    render(FinanceTransactions)

    expect(await screen.findByText('Transaction 0')).toBeInTheDocument()
    const next = screen.getByRole('button', { name: 'Transaction pages: next page' })
    await user.click(next)

    await waitFor(() => expect(next).toBeDisabled())
    expect(screen.getByText('Transaction 0')).toBeInTheDocument()
    expect(screen.getByText('Loading transaction page…')).toBeInTheDocument()
    resolveNextPage([{ id: 'tx-20', tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'expense', amountMinor: 100, currency: 'USD', description: 'Transaction 20', effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now }])

    expect(await screen.findByText('Transaction 20')).toBeInTheDocument()
    expect(next).toHaveFocus()
  })

  it('keeps the current ledger page usable after a pager failure', async () => {
    const user = userEvent.setup()
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTransactions
      .mockResolvedValueOnce(Array.from({ length: 20 }, (_, index) => ({
        id: `tx-${index}`, tenantId: 'tenant-1', accountId: 'account-1', source: 'manual', status: 'booked', kind: 'expense', amountMinor: 100,
        currency: 'USD', description: `Transaction ${index}`, effectiveAt: now, categoryId: null, tagIds: [], transferGroupId: null, transferMatchedAt: null, hiddenAt: null, providerOriginal: null, createdAt: now, updatedAt: now,
      })))
      .mockRejectedValueOnce(new Error('Page unavailable'))
    render(FinanceTransactions)

    expect(await screen.findByText('Transaction 0')).toBeInTheDocument()
    const next = screen.getByRole('button', { name: 'Transaction pages: next page' })
    await user.click(next)

    expect(await screen.findByRole('alert')).toHaveTextContent('Page unavailable')
    expect(screen.getByText('Transaction 0')).toBeInTheDocument()
    expect(next).toBeEnabled()
    expect(screen.getByText('Page 1')).toBeInTheDocument()
  })

  it('renders a no-tenant state and keeps the create link available', async () => {
    mocks.listTenants.mockResolvedValueOnce([])
    render(FinanceTransactions)
    expect(await screen.findByText('Select a finance tenant to load transaction history and editor links.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Create transaction' })).toHaveAttribute('href', '#/finance/transactions/new')
  })

  it('requires an explicit tenant choice before loading standalone multi-tenant transaction routes', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    const user = userEvent.setup()
    mocks.listTenants.mockResolvedValueOnce([
      { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now },
      { id: 'tenant-2', name: 'Travel', displayCurrency: 'EUR', joinedAt: now, createdAt: now, updatedAt: now },
    ])
    mocks.listAccounts.mockResolvedValueOnce([
      { id: 'account-2', tenantId: 'tenant-2', name: 'Travel card', currency: 'EUR', kind: 'manual', provider: '', providerAccountId: '', hiddenAt: null, createdAt: now, updatedAt: now },
    ])
    mocks.listTransactions.mockResolvedValueOnce([
      {
        id: 'tx-2',
        tenantId: 'tenant-2',
        accountId: 'account-2',
        source: 'manual',
        status: 'booked',
        kind: 'expense',
        amountMinor: 1200,
        currency: 'EUR',
        description: 'Hotel',
        effectiveAt: now,
        categoryId: null,
        tagIds: [],
        transferGroupId: null,
        transferMatchedAt: null,
        hiddenAt: null,
        providerOriginal: null,
        createdAt: now,
        updatedAt: now,
      },
    ])

    render(FinanceTransactions)

    expect(await screen.findByText('Select an active tenant to continue on this finance route.')).toBeInTheDocument()
    await user.selectOptions(screen.getByRole('combobox', { name: 'Tenant' }), 'tenant-2')

    await waitFor(() => expect(mocks.listTransactions).toHaveBeenLastCalledWith({ tenantId: 'tenant-2', accountId: '', status: '', source: '', limit: 20, offset: 0 }))
    expect(await screen.findByText('Hotel')).toBeInTheDocument()
  })

  it('returns to the no-tenant route state when the standalone tenant selection is cleared', async () => {
    const now = new Date('2026-06-20T12:00:00Z')
    const user = userEvent.setup()
    mocks.listTenants.mockResolvedValueOnce([
      { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now },
      { id: 'tenant-2', name: 'Travel', displayCurrency: 'EUR', joinedAt: now, createdAt: now, updatedAt: now },
    ])
    mocks.listAccounts.mockResolvedValueOnce([
      { id: 'account-2', tenantId: 'tenant-2', name: 'Travel card', currency: 'EUR', kind: 'manual', provider: '', providerAccountId: '', hiddenAt: null, createdAt: now, updatedAt: now },
    ])
    mocks.listTransactions.mockResolvedValueOnce([
      {
        id: 'tx-2',
        tenantId: 'tenant-2',
        accountId: 'account-2',
        source: 'manual',
        status: 'booked',
        kind: 'expense',
        amountMinor: 1200,
        currency: 'EUR',
        description: 'Hotel',
        effectiveAt: now,
        categoryId: null,
        tagIds: [],
        transferGroupId: null,
        transferMatchedAt: null,
        hiddenAt: null,
        providerOriginal: null,
        createdAt: now,
        updatedAt: now,
      },
    ])

    render(FinanceTransactions)

    await user.selectOptions(await screen.findByRole('combobox', { name: 'Tenant' }), 'tenant-2')
    expect(await screen.findByText('Hotel')).toBeInTheDocument()

    await user.selectOptions(screen.getByRole('combobox', { name: 'Tenant' }), '')

    expect(
      await screen.findByText('Select an active tenant to continue on this finance route.'),
    ).toBeInTheDocument()
    expect(screen.queryByText('Hotel')).not.toBeInTheDocument()
  })

  it('renders an error state when transaction loading fails', async () => {
    mocks.listTenants.mockRejectedValueOnce(new Error('transactions exploded'))
    render(FinanceTransactions)
    expect(await screen.findByRole('alert')).toHaveTextContent('transactions exploded')
  })

  it('falls back to a generic transactions error when workspace loading rejects without an Error', async () => {
    mocks.listTenants.mockRejectedValueOnce('boom')

    render(FinanceTransactions)

    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to load transactions')
  })
})
