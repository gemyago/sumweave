import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import { faker } from '@faker-js/faker'
import FinanceRuleCreationForm from './FinanceRuleCreationForm.svelte'

const mocks = vi.hoisted(() => ({ createClassificationRule: vi.fn() }))
vi.mock('../lib/finance/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/finance/api')>()),
  createSignalFinanceApiForAuth: vi.fn(() => ({ ...mocks })),
}))
vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

describe('FinanceRuleCreationForm', () => {
  const now = new Date('2026-06-20T12:00:00Z')
  const props = {
    tenantId: 'tenant-1', offerId: 1, description: 'Visible coffee memo', categoryId: 'category-1',
    categories: [{ id: 'category-1', tenantId: 'tenant-1', name: 'Groceries', kind: 'expense', seededDefault: false, createdAt: now, updatedAt: now }],
    onCancel: vi.fn(), onSaved: vi.fn(),
  }

  beforeEach(() => { vi.clearAllMocks(); mocks.createClassificationRule.mockResolvedValue({ id: 'rule-1' }) })

  it('prefills current visible description and category, then explicitly saves the selected match type', async () => {
    const user = userEvent.setup()
    render(FinanceRuleCreationForm, { ...props })
    expect(screen.getByLabelText('Rule condition')).toHaveValue('Visible coffee memo')
    expect(screen.getByLabelText('Rule category')).toHaveValue('category-1')
    await user.selectOptions(screen.getByLabelText('Match type'), 'exact')
    await user.click(screen.getByRole('button', { name: 'Save rule' }))
    await waitFor(() => expect(mocks.createClassificationRule).toHaveBeenCalledWith({ tenantId: 'tenant-1', matchType: 'exact', condition: 'Visible coffee memo', categoryId: 'category-1' }))
    expect(props.onSaved).toHaveBeenCalledTimes(1)
  })

  it('keeps the optional form open on rule-save failure and cancels without another request', async () => {
    const user = userEvent.setup()
    mocks.createClassificationRule.mockRejectedValueOnce(new Error('rule failed'))
    render(FinanceRuleCreationForm, { ...props })
    await user.type(screen.getByLabelText('Rule condition'), ' updated')
    await user.click(screen.getByRole('button', { name: 'Save rule' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('rule failed')
    expect(screen.getByLabelText('Rule condition')).toHaveValue('Visible coffee memo updated')
    await user.click(screen.getByRole('button', { name: 'Cancel rule' }))
    expect(props.onCancel).toHaveBeenCalledTimes(1)
    expect(mocks.createClassificationRule).toHaveBeenCalledTimes(1)
  })

  it('resets only for a replacement offer and submits that offer’s defaults', async () => {
    const user = userEvent.setup()
    const firstCategoryId = faker.string.uuid()
    const editedCategoryId = faker.string.uuid()
    const latestCategoryId = faker.string.uuid()
    const firstOfferId = faker.number.int({ min: 1, max: 1_000_000 })
    const firstDescription = faker.lorem.sentence()
    const editedCondition = faker.lorem.sentence()
    const latestDescription = faker.lorem.sentence()
    const categories = [
      { ...props.categories[0], id: firstCategoryId, name: faker.commerce.department() },
      { ...props.categories[0], id: editedCategoryId, name: faker.commerce.department() },
      { ...props.categories[0], id: latestCategoryId, name: faker.commerce.department() },
    ]
    const view = render(FinanceRuleCreationForm, {
      ...props,
      offerId: firstOfferId,
      description: firstDescription,
      categoryId: firstCategoryId,
      categories,
    })

    await user.selectOptions(screen.getByLabelText('Match type'), 'exact')
    await user.clear(screen.getByLabelText('Rule condition'))
    await user.type(screen.getByLabelText('Rule condition'), editedCondition)
    await user.selectOptions(screen.getByLabelText('Rule category'), editedCategoryId)

    await view.rerender({
      ...props,
      offerId: firstOfferId,
      description: firstDescription,
      categoryId: firstCategoryId,
      categories,
    })
    expect(screen.getByLabelText('Match type')).toHaveValue('exact')
    expect(screen.getByLabelText('Rule condition')).toHaveValue(editedCondition)
    expect(screen.getByLabelText('Rule category')).toHaveValue(editedCategoryId)

    await view.rerender({
      ...props,
      offerId: firstOfferId + 1,
      description: firstDescription,
      categoryId: firstCategoryId,
      categories,
    })
    expect(screen.getByLabelText('Match type')).toHaveValue('contains')
    expect(screen.getByLabelText('Rule condition')).toHaveValue(firstDescription)
    expect(screen.getByLabelText('Rule category')).toHaveValue(firstCategoryId)

    await view.rerender({
      ...props,
      offerId: firstOfferId + 2,
      description: latestDescription,
      categoryId: latestCategoryId,
      categories,
    })
    await user.click(screen.getByRole('button', { name: 'Save rule' }))

    await waitFor(() => expect(mocks.createClassificationRule).toHaveBeenCalledWith({
      tenantId: 'tenant-1',
      matchType: 'contains',
      condition: latestDescription,
      categoryId: latestCategoryId,
    }))
  })
})
