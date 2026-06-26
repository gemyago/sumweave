import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceCategories from './FinanceCategories.svelte'

const mocks = vi.hoisted(() => ({ listTenants: vi.fn(), listCategories: vi.fn(), listTags: vi.fn(), createCategory: vi.fn(), createTag: vi.fn() }))
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

  it('renders category and tag stacks', async () => {
    render(FinanceCategories)
    expect(await screen.findByText('Groceries')).toBeInTheDocument()
    expect(screen.getByText('Budget')).toBeInTheDocument()
  })

  it('submits category and tag create forms', async () => {
    const user = userEvent.setup()
    render(FinanceCategories)

    await user.type(await screen.findByLabelText('Category name'), 'Travel')
    await user.click(screen.getByRole('button', { name: 'Create category' }))
    await waitFor(() => expect(mocks.createCategory).toHaveBeenCalled())

    await user.type(screen.getByLabelText('Tag name'), 'Holiday')
    await user.click(screen.getByRole('button', { name: 'Create tag' }))
    await waitFor(() => expect(mocks.createTag).toHaveBeenCalled())
  })

  it('renders empty states for missing categories and tags', async () => {
    mocks.listCategories.mockResolvedValueOnce([])
    mocks.listTags.mockResolvedValueOnce([])
    render(FinanceCategories)
    expect(await screen.findByText('No categories yet.')).toBeInTheDocument()
    expect(screen.getByText('No tags yet.')).toBeInTheDocument()
  })

  it('renders a no-tenant state with disabled create buttons', async () => {
    mocks.listTenants.mockResolvedValueOnce([])
    render(FinanceCategories)
    expect(await screen.findByRole('button', { name: 'Create category' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Create tag' })).toBeDisabled()
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
