import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceRules from './FinanceRules.svelte'

const mocks = vi.hoisted(() => ({
  listTenants: vi.fn(), listCategories: vi.fn(), listClassificationRules: vi.fn(),
  createClassificationRule: vi.fn(), updateClassificationRule: vi.fn(),
  deleteClassificationRule: vi.fn(), moveClassificationRule: vi.fn(),
}))

vi.mock('../lib/finance/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/finance/api')>()),
  createSignalFinanceApiForAuth: vi.fn(() => ({ ...mocks })),
}))
vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

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
})
