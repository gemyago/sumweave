import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceCategories from './FinanceCategories.svelte'

const mocks = vi.hoisted(() => ({ listTenants: vi.fn(), listCategories: vi.fn(), listTags: vi.fn(), createCategory: vi.fn(), createTag: vi.fn(), updateCategory: vi.fn(), renameTag: vi.fn() }))
vi.mock('../lib/finance/api', async (importOriginal) => ({ ...(await importOriginal<typeof import('../lib/finance/api')>()), createSignalFinanceApiForAuth: vi.fn(() => ({ ...mocks })) }))
vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

describe('Finance categories page', () => {
  beforeEach(() => {
    const now = new Date('2026-06-20T12:00:00Z')
    mocks.listTenants.mockResolvedValue([{ id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now }])
    mocks.listCategories.mockResolvedValue([{ id: 'cat-1', tenantId: 'tenant-1', name: 'Groceries', kind: 'expense', seededDefault: true, hiddenAt: null, createdAt: now, updatedAt: now }])
    mocks.listTags.mockResolvedValue([{ id: 'tag-1', tenantId: 'tenant-1', name: 'Budget', hiddenAt: null, createdAt: now, updatedAt: now }])
    mocks.createCategory.mockResolvedValue({})
    mocks.createTag.mockResolvedValue({})
  })

  it('renders independent category and tag stacks with category context', async () => {
    render(FinanceCategories)
    expect(await screen.findByText('Groceries')).toBeInTheDocument()
    expect(screen.getByText('Budget')).toBeInTheDocument()
    expect(screen.getByText('expense')).toBeInTheDocument()
    expect(screen.getByText('Starter default')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add category' })).toBeVisible()
    expect(screen.getByRole('button', { name: 'Add tag' })).toBeVisible()
    expect(screen.queryByLabelText('Category name')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Tag name')).not.toBeInTheDocument()
  })

  it('reveals and submits each local create form on demand', async () => {
    const user = userEvent.setup()
    render(FinanceCategories)

    await user.click(await screen.findByRole('button', { name: 'Add category' }))
    await user.type(screen.getByLabelText('Category name'), 'Travel')
    await user.click(screen.getByRole('button', { name: 'Save category' }))
    await waitFor(() => expect(mocks.createCategory).toHaveBeenCalled())

    await user.click(screen.getByRole('button', { name: 'Add tag' }))
    await user.type(screen.getByLabelText('Tag name'), 'Holiday')
    await user.click(screen.getByRole('button', { name: 'Save tag' }))
    await waitFor(() => expect(mocks.createTag).toHaveBeenCalled())
  })

  it('cancels an unopened-on-load category add form', async () => {
    const user = userEvent.setup()
    render(FinanceCategories)

    await user.click(await screen.findByRole('button', { name: 'Add category' }))
    await user.type(screen.getByLabelText('Category name'), 'Travel')
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(screen.queryByLabelText('Category name')).not.toBeInTheDocument()
  })

  it('does not mark a non-starter category as a starter default', async () => {
    mocks.listCategories.mockResolvedValueOnce([
      { id: 'cat-2', tenantId: 'tenant-1', name: 'Travel', kind: 'expense', seededDefault: false, hiddenAt: null, createdAt: new Date('2026-06-20T12:00:00Z'), updatedAt: new Date('2026-06-20T12:00:00Z') },
    ])
    render(FinanceCategories)

    expect(await screen.findByText('Travel')).toBeInTheDocument()
    expect(screen.queryByText('Starter default')).not.toBeInTheDocument()
  })

  it('edits category name and type while leaving the tag editor independent', async () => {
    const user = userEvent.setup()
    render(FinanceCategories)

    await user.click(await screen.findByRole('button', { name: 'Edit Groceries' }))
    await user.click(screen.getByRole('button', { name: 'Edit Budget' }))
    await user.clear(screen.getByLabelText('Category name'))
    await user.type(screen.getByLabelText('Category name'), 'Food')
    await user.selectOptions(screen.getByLabelText('Type'), 'income')
    await user.click(screen.getAllByRole('button', { name: 'Save' })[0])
    await waitFor(() => expect(mocks.updateCategory).toHaveBeenCalledWith({ tenantId: 'tenant-1', categoryId: 'cat-1', name: 'Food', kind: 'income' }))

    expect(screen.getByLabelText('Tag name')).toHaveValue('Budget')
    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.getByText('Budget')).toBeInTheDocument()
    expect(mocks.renameTag).not.toHaveBeenCalled()
  })

  it('uses icon-only edit buttons with accessible item-specific names', async () => {
    render(FinanceCategories)

    const categoryEdit = await screen.findByRole('button', { name: 'Edit Groceries' })
    const tagEdit = screen.getByRole('button', { name: 'Edit Budget' })

    expect(categoryEdit).toHaveAttribute('title', 'Edit Groceries')
    expect(tagEdit).toHaveAttribute('title', 'Edit Budget')
    expect(categoryEdit).toHaveTextContent('')
    expect(tagEdit).toHaveTextContent('')
  })

  it('saves an inline tag edit without opening the category form', async () => {
    const user = userEvent.setup()
    render(FinanceCategories)

    await user.click(await screen.findByRole('button', { name: 'Edit Budget' }))
    await user.clear(screen.getByLabelText('Tag name'))
    await user.type(screen.getByLabelText('Tag name'), 'Essential')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(mocks.renameTag).toHaveBeenCalledWith({ tenantId: 'tenant-1', tagId: 'tag-1', name: 'Essential' }))
    expect(screen.queryByLabelText('Category name')).not.toBeInTheDocument()
  })

  it('keeps the category add form open after a create error', async () => {
    const user = userEvent.setup()
    mocks.createCategory.mockRejectedValueOnce(new Error('category create failed'))
    render(FinanceCategories)

    await user.click(await screen.findByRole('button', { name: 'Add category' }))
    await user.type(screen.getByLabelText('Category name'), 'Travel')
    await user.click(screen.getByRole('button', { name: 'Save category' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('category create failed')
    expect(screen.getByLabelText('Category name')).toHaveValue('Travel')
  })

  it('keeps the tag add form open after a create error', async () => {
    const user = userEvent.setup()
    mocks.createTag.mockRejectedValueOnce(new Error('tag create failed'))
    render(FinanceCategories)

    await user.click(await screen.findByRole('button', { name: 'Add tag' }))
    await user.type(screen.getByLabelText('Tag name'), 'Holiday')
    await user.click(screen.getByRole('button', { name: 'Save tag' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('tag create failed')
    expect(screen.getByLabelText('Tag name')).toHaveValue('Holiday')
  })

  it('keeps the category inline editor and type draft open after an update error', async () => {
    const user = userEvent.setup()
    mocks.updateCategory.mockRejectedValueOnce(new Error('category update failed'))
    render(FinanceCategories)

    await user.click(await screen.findByRole('button', { name: 'Edit Groceries' }))
    await user.clear(screen.getByLabelText('Category name'))
    await user.type(screen.getByLabelText('Category name'), 'Food')
    await user.selectOptions(screen.getByLabelText('Type'), 'income')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('category update failed')
    expect(screen.getByLabelText('Category name')).toHaveValue('Food')
    expect(screen.getByLabelText('Type')).toHaveValue('income')
  })

  it('keeps the tag inline editor open after an edit error', async () => {
    const user = userEvent.setup()
    mocks.renameTag.mockRejectedValueOnce(new Error('tag rename failed'))
    render(FinanceCategories)

    await user.click(await screen.findByRole('button', { name: 'Edit Budget' }))
    await user.clear(screen.getByLabelText('Tag name'))
    await user.type(screen.getByLabelText('Tag name'), 'Essential')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('tag rename failed')
    expect(screen.getByLabelText('Tag name')).toHaveValue('Essential')
  })

  it('renders empty states for missing categories and tags', async () => {
    mocks.listCategories.mockResolvedValueOnce([])
    mocks.listTags.mockResolvedValueOnce([])
    render(FinanceCategories)
    expect(await screen.findByText('No categories yet.')).toBeInTheDocument()
    expect(screen.getByText('No tags yet.')).toBeInTheDocument()
  })

  it('renders a no-tenant state with disabled add buttons', async () => {
    mocks.listTenants.mockResolvedValueOnce([])
    render(FinanceCategories)
    expect(await screen.findByRole('button', { name: 'Add category' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Add tag' })).toBeDisabled()
  })

  it('reloads catalogs when the tenant selection changes', async () => {
    const user = userEvent.setup()
    mocks.listTenants.mockResolvedValueOnce([
      { id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: new Date('2026-06-20T12:00:00Z'), createdAt: new Date('2026-06-20T12:00:00Z'), updatedAt: new Date('2026-06-20T12:00:00Z') },
      { id: 'tenant-2', name: 'Travel', displayCurrency: 'EUR', joinedAt: new Date('2026-06-20T12:00:00Z'), createdAt: new Date('2026-06-20T12:00:00Z'), updatedAt: new Date('2026-06-20T12:00:00Z') },
    ])
    render(FinanceCategories)
    await user.selectOptions(await screen.findByRole('combobox', { name: 'Tenant' }), 'tenant-2')
    await waitFor(() => expect(mocks.listCategories).toHaveBeenLastCalledWith({ tenantId: 'tenant-2' }))
    expect(mocks.listTags).toHaveBeenLastCalledWith({ tenantId: 'tenant-2' })
  })

  it('renders an error state when category loading fails', async () => {
    mocks.listTenants.mockRejectedValueOnce(new Error('categories exploded'))
    render(FinanceCategories)
    expect(await screen.findByRole('alert')).toHaveTextContent('categories exploded')
  })
})
