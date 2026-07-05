import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceImports from './FinanceImports.svelte'

const mocks = vi.hoisted(() => ({
  listTenants: vi.fn(),
  previewCSVImport: vi.fn(),
  confirmCSVImport: vi.fn(),
  getCSVImportAudit: vi.fn(),
}))

vi.mock('../lib/finance/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/finance/api')>()
  return {
    ...actual,
    createSignalFinanceApiForAuth: vi.fn(() => ({ ...mocks })),
  }
})

vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

describe('Finance imports page', () => {
  beforeEach(() => {
    window.localStorage.clear()
    const now = new Date('2026-06-20T12:00:00Z')
    Object.values(mocks).forEach((mock) => mock.mockReset())
    mocks.listTenants.mockResolvedValue([{ id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now }])
    mocks.previewCSVImport.mockResolvedValue({ importId: 'import-1', importType: 'transactions', headers: ['account', 'amount'], mapping: { account: 'accountName', amount: 'amountMinor' }, duplicateRows: [], rejectedRows: [], wouldCreateAccounts: ['Checking'], wouldCreateCategories: [], wouldCreateTags: [] })
    mocks.confirmCSVImport.mockResolvedValue({ importId: 'import-1', jobId: 'job-1', jobType: 'finance.csv_import' })
    mocks.getCSVImportAudit.mockResolvedValue({ importId: 'import-1', tenantId: 'tenant-1', importType: 'transactions', status: 'completed', jobId: 'job-1', confirmedByUserId: 'user-1', importedCount: 2, createdAt: now, confirmedAt: now, completedAt: now })
  })

  it('previews and confirms a finance csv import', async () => {
    const user = userEvent.setup()
    render(FinanceImports)

    await user.click(await screen.findByRole('button', { name: 'Preview import' }))
    const previewCard = await screen.findByRole('heading', { name: 'Preview result' })
    expect(previewCard).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Would create accounts' })).toBeInTheDocument()
    expect(screen.getByText('Checking')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Confirm import' }))
    await waitFor(() => expect(mocks.confirmCSVImport).toHaveBeenCalled())
    expect(await screen.findByRole('link', { name: 'Open finance job detail' })).toHaveAttribute('href', '#/finance/jobs/job-1')
  })

  it('renders a loading state while the tenant list is pending', async () => {
    let resolveList: (value: unknown) => void = () => undefined
    mocks.listTenants.mockReturnValueOnce(new Promise((resolve) => { resolveList = resolve }))

    render(FinanceImports)
    expect(screen.getByText('Loading imports…')).toBeInTheDocument()

    resolveList([{ id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: new Date('2026-06-20T12:00:00Z'), createdAt: new Date('2026-06-20T12:00:00Z'), updatedAt: new Date('2026-06-20T12:00:00Z') }])
    expect(await screen.findByRole('button', { name: 'Preview import' })).toBeInTheDocument()
  })

  it('renders an error state when imports bootstrap fails', async () => {
    mocks.listTenants.mockRejectedValueOnce(new Error('imports exploded'))

    render(FinanceImports)

    expect(await screen.findByRole('alert')).toHaveTextContent('imports exploded')
  })

  it('falls back to a generic imports error when bootstrap rejects without an Error', async () => {
    mocks.listTenants.mockRejectedValueOnce('boom')

    render(FinanceImports)

    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to load imports')
  })

  it('renders a no-tenant state with preview disabled', async () => {
    mocks.listTenants.mockResolvedValueOnce([])

    render(FinanceImports)

    expect(await screen.findByRole('button', { name: 'Preview import' })).toBeDisabled()
  })

  it('renders preview fallback text for empty mapping results', async () => {
    mocks.previewCSVImport.mockResolvedValueOnce({ importId: 'import-2', importType: 'transactions', headers: [], mapping: {}, duplicateRows: [], rejectedRows: [], wouldCreateAccounts: [], wouldCreateCategories: [], wouldCreateTags: [] })
    const user = userEvent.setup()
    render(FinanceImports)

    await user.click(await screen.findByRole('button', { name: 'Preview import' }))
    expect(await screen.findByText(/headers —/)).toBeInTheDocument()

    const accountsCard = screen.getByRole('heading', { name: 'Would create accounts' }).closest('div')
    const categoriesCard = screen.getByRole('heading', { name: 'Would create categories' }).closest('div')
    const tagsCard = screen.getByRole('heading', { name: 'Would create tags' }).closest('div')

    expect(accountsCard).not.toBeNull()
    expect(categoriesCard).not.toBeNull()
    expect(tagsCard).not.toBeNull()

    expect(within(accountsCard as HTMLElement).getByText('—')).toBeInTheDocument()
    expect(within(categoriesCard as HTMLElement).getByText('—')).toBeInTheDocument()
    expect(within(tagsCard as HTMLElement).getByText('—')).toBeInTheDocument()
  })
})
