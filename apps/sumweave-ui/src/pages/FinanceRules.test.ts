import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceRules from './FinanceRules.svelte'

const mocks = vi.hoisted(() => ({
  listTenants: vi.fn(), listCategories: vi.fn(), listClassificationRules: vi.fn(),
  createClassificationRule: vi.fn(), updateClassificationRule: vi.fn(),
  deleteClassificationRule: vi.fn(), moveClassificationRule: vi.fn(),
  submitTransactionClassification: vi.fn(),
  getJob: vi.fn(), requestFinanceLedgerRefresh: vi.fn(),
}))

vi.mock('../lib/finance/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/finance/api')>()),
  createSignalFinanceApiForAuth: vi.fn(() => ({ ...mocks })),
}))
vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))
vi.mock('../lib/jobs/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/jobs/api')>()),
  createSignalJobsApiForAuth: vi.fn(() => ({ getJob: mocks.getJob })),
}))
vi.mock('../lib/finance/ledger-refresh', () => ({ requestFinanceLedgerRefresh: mocks.requestFinanceLedgerRefresh }))

describe('Finance rules page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.localStorage.clear()
    window.location.hash = '#/finance/rules'
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants.mockResolvedValue([{ id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now }])
    mocks.listCategories.mockResolvedValue([
      { id: 'category-1', tenantId: 'tenant-1', name: 'Groceries', kind: 'expense', seededDefault: true, createdAt: now, updatedAt: now },
      { id: 'category-2', tenantId: 'tenant-1', name: 'Dining', kind: 'expense', seededDefault: false, createdAt: now, updatedAt: now },
    ])
    mocks.listClassificationRules.mockResolvedValue([
      { id: 'rule-1', matchType: 'contains', condition: 'SHOP', categoryId: 'category-1', position: 1, createdAt: now, updatedAt: now },
      { id: 'rule-2', matchType: 'exact', condition: 'CAFE', categoryId: 'category-2', position: 2, createdAt: now, updatedAt: now },
    ])
    mocks.createClassificationRule.mockResolvedValue({ id: 'rule-3' })
    mocks.updateClassificationRule.mockResolvedValue(undefined)
    mocks.deleteClassificationRule.mockResolvedValue(undefined)
    mocks.moveClassificationRule.mockResolvedValue(undefined)
    mocks.submitTransactionClassification.mockResolvedValue({ jobId: 'classification-job-1' })
    mocks.getJob.mockResolvedValue({ id: 'classification-job-1', status: 'queued', jobType: 'finance.classification' })
  })

  it('shows ordered rules with category names and edge-aware move controls', async () => {
    render(FinanceRules)
    expect(await screen.findByText('contains “SHOP”')).toBeInTheDocument()
    expect(screen.getByText('Target: Groceries · Position 1')).toBeInTheDocument()
    expect(screen.getByText('Target: Dining · Position 2')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Move SHOP up' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Move CAFE down' })).toBeDisabled()
    expect(document.title).toBe('Rules · Finance · Sumweave')
  })

  it('appends, edits, deletes, and moves rules by refetching the ordered list', async () => {
    const user = userEvent.setup()
    render(FinanceRules)
    await screen.findByRole('button', { name: 'Move SHOP up' })

    await user.click(screen.getByRole('button', { name: 'Add rule' }))
    await user.type(screen.getByLabelText('Condition'), 'NETFLIX')
    await user.selectOptions(screen.getByLabelText('Target category'), 'category-2')
    await user.click(screen.getByRole('button', { name: 'Save rule' }))
    await waitFor(() => expect(mocks.createClassificationRule).toHaveBeenCalledWith({ tenantId: 'tenant-1', matchType: 'contains', condition: 'NETFLIX', categoryId: 'category-2' }))

    await user.click(screen.getAllByRole('button', { name: 'Edit' })[0])
    await user.clear(screen.getByLabelText('Condition'))
    await user.type(screen.getByLabelText('Condition'), 'SHOP UPDATED')
    await user.click(screen.getByRole('button', { name: 'Save rule' }))
    await waitFor(() => expect(mocks.updateClassificationRule).toHaveBeenCalledWith(expect.objectContaining({ ruleId: 'rule-1', condition: 'SHOP UPDATED' })))

    await user.click(screen.getByRole('button', { name: 'Move CAFE up' }))
    await waitFor(() => expect(mocks.moveClassificationRule).toHaveBeenCalledWith({ tenantId: 'tenant-1', ruleId: 'rule-2', direction: 'up' }))
    await user.click(screen.getAllByRole('button', { name: 'Delete' })[0])
    await waitFor(() => expect(mocks.deleteClassificationRule).toHaveBeenCalledWith({ tenantId: 'tenant-1', ruleId: 'rule-1' }))
    expect(mocks.listClassificationRules).toHaveBeenLastCalledWith({ tenantId: 'tenant-1' })
  })

  it('keeps a rule form and its draft available after a mutation failure', async () => {
    const user = userEvent.setup()
    mocks.createClassificationRule.mockRejectedValueOnce(new Error('rule save failed'))
    render(FinanceRules)
    await screen.findByRole('button', { name: 'Move SHOP up' })
    await user.click(screen.getByRole('button', { name: 'Add rule' }))
    await user.type(screen.getByLabelText('Condition'), 'NETFLIX')
    await user.click(screen.getByRole('button', { name: 'Save rule' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('rule save failed')
    expect(screen.getByLabelText('Condition')).toHaveValue('NETFLIX')
  })

  it('shows the default delete failure message when the API does not provide one', async () => {
    const user = userEvent.setup()
    mocks.deleteClassificationRule.mockRejectedValueOnce('network unavailable')
    render(FinanceRules)
    await screen.findByRole('button', { name: 'Move SHOP up' })
    await user.click(screen.getAllByRole('button', { name: 'Delete' })[0])
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not delete the classification rule.')
  })

  it('closes the edited rule form when that rule is deleted', async () => {
    const user = userEvent.setup()
    render(FinanceRules)
    await screen.findByRole('button', { name: 'Move SHOP up' })
    await user.click(screen.getAllByRole('button', { name: 'Edit' })[0])
    expect(screen.getByRole('heading', { name: 'Edit rule' })).toBeInTheDocument()
    await user.click(screen.getAllByRole('button', { name: 'Delete' })[0])
    await waitFor(() => expect(mocks.deleteClassificationRule).toHaveBeenCalledWith({ tenantId: 'tenant-1', ruleId: 'rule-1' }))
    expect(screen.queryByRole('heading', { name: 'Edit rule' })).not.toBeInTheDocument()
  })

  it('shows the default move failure message when the API does not provide one', async () => {
    const user = userEvent.setup()
    mocks.moveClassificationRule.mockRejectedValueOnce('network unavailable')
    render(FinanceRules)
    await screen.findByRole('button', { name: 'Move SHOP up' })
    await user.click(screen.getByRole('button', { name: 'Move CAFE up' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not move the classification rule.')
  })

  it('shows the default update failure message when the API does not provide one', async () => {
    const user = userEvent.setup()
    mocks.updateClassificationRule.mockRejectedValueOnce('network unavailable')
    render(FinanceRules)
    await screen.findByRole('button', { name: 'Move SHOP up' })
    await user.click(screen.getAllByRole('button', { name: 'Edit' })[0])
    await user.click(screen.getByRole('button', { name: 'Save rule' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not save the classification rule.')
  })

  it('uses a category filter from the Rules hash and exposes a clear-filter state', async () => {
    window.location.hash = '#/finance/rules?categoryId=category-1'
    render(FinanceRules)
    expect(await screen.findByText('Showing rules that reference this category.')).toBeInTheDocument()
    expect(mocks.listClassificationRules).toHaveBeenCalledWith({ tenantId: 'tenant-1', categoryId: 'category-1' })
    expect(screen.getByRole('link', { name: 'Clear filter' })).toHaveAttribute('href', '#/finance/rules')
  })

  it('shows the local thirty-date default and submits an inclusive range as explicit classification', async () => {
    vi.setSystemTime(new Date(2026, 5, 20, 12))
    const user = userEvent.setup()
    render(FinanceRules)

    expect(await screen.findByLabelText('Classification start date')).toHaveValue('2026-05-22')
    expect(screen.getByLabelText('Classification end date')).toHaveValue('2026-06-20')
    await user.click(screen.getByRole('button', { name: 'Run classification' }))

    await waitFor(() => expect(mocks.submitTransactionClassification).toHaveBeenCalledWith({
      tenantId: 'tenant-1',
      rangeStart: expect.stringMatching(/^2026-05-22T00:00:00[+-]\d{2}:\d{2}$/),
      rangeEndExclusive: expect.stringMatching(/^2026-06-21T00:00:00[+-]\d{2}:\d{2}$/),
    }))
    expect(screen.getByRole('link', { name: 'Open finance job' })).toHaveAttribute('href', '#/finance/jobs/classification-job-1')
  })

  it('keeps an invalid date range local and recoverable without publication', async () => {
    const user = userEvent.setup()
    render(FinanceRules)
    const start = await screen.findByLabelText('Classification start date')
    const end = screen.getByLabelText('Classification end date')

    await fireEvent.input(start, { target: { value: '2026-06-21' } })
    await fireEvent.input(end, { target: { value: '2026-06-20' } })
    await user.click(screen.getByRole('button', { name: 'Run classification' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Start date must be on or before end date.')
    expect(mocks.submitTransactionClassification).not.toHaveBeenCalled()
  })

  it('keeps failure feedback and retry submission available when partial classification may have committed', async () => {
    const user = userEvent.setup()
    mocks.getJob.mockResolvedValue({
      id: 'classification-job-1', status: 'failed', jobType: 'finance.classification', error: { summary: 'A rule category was removed.' },
    })
    mocks.submitTransactionClassification
      .mockResolvedValueOnce({ jobId: 'classification-job-1' })
      .mockResolvedValueOnce({ jobId: 'classification-job-2' })
    render(FinanceRules)
    await screen.findByRole('button', { name: 'Run classification' })

    await user.click(screen.getByRole('button', { name: 'Run classification' }))
    expect(await screen.findByText(/Some transactions may already have been classified/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Run classification again' }))
    await waitFor(() => expect(mocks.submitTransactionClassification).toHaveBeenCalledTimes(2))
  })

  it('signals a ledger refresh after the initiating job succeeds', async () => {
    const user = userEvent.setup()
    mocks.getJob.mockResolvedValue({ id: 'classification-job-1', status: 'succeeded', jobType: 'finance.classification' })
    render(FinanceRules)
    await screen.findByRole('button', { name: 'Run classification' })

    await user.click(screen.getByRole('button', { name: 'Run classification' }))

    expect(await screen.findByText('Classification completed. Ledger data has been refreshed.')).toBeInTheDocument()
    expect(mocks.requestFinanceLedgerRefresh).toHaveBeenCalledWith('tenant-1')
  })
})
