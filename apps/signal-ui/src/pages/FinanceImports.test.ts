import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import FinanceImports from './FinanceImports.svelte'
import { FinanceApiError } from '../lib/finance/api'

const mocks = vi.hoisted(() => ({ listTenants: vi.fn(), previewCSVImport: vi.fn(), confirmCSVImport: vi.fn(), getCSVImportAudit: vi.fn(), listRecentCSVImportAudits: vi.fn() }))
const scrollIntoView = vi.fn()

vi.mock('../lib/finance/api', async (importOriginal) => ({ ...(await importOriginal<typeof import('../lib/finance/api')>()), createSignalFinanceApiForAuth: vi.fn(() => ({ ...mocks })) }))
vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

const now = new Date('2026-06-20T12:00:00Z')
const audit = (status = 'completed') => ({ importId: 'import-1', tenantId: 'tenant-1', status, jobId: 'job-1', confirmedByUserId: 'user-1', importedCount: 2, rejectedRows: [], rowOutcomes: [{ rowNumber: 2, status: 'imported', reason: 'Imported', createdAt: now, updatedAt: now }], createdAt: now, confirmedAt: now, completedAt: status === 'completed' ? now : null })

describe('Finance imports page', () => {
  beforeEach(() => {
    window.localStorage.clear()
    scrollIntoView.mockReset()
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', { configurable: true, value: scrollIntoView })
    Object.values(mocks).forEach((mock) => mock.mockReset())
    mocks.listTenants.mockResolvedValue([{ id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now }])
    mocks.previewCSVImport.mockResolvedValue({ importId: 'import-1', importableCount: 1, headers: ['Date', 'Account', 'Category', 'Tags', 'Expense amount', 'Income amount', 'Currency', 'Description'], duplicateRows: [{ rowNumber: 4, field: 'Description', reason: 'Duplicate transaction' }], rejectedRows: [{ rowNumber: 3, field: 'Date', reason: 'Date must use dd.MM.yy' }], wouldCreateAccounts: ['Daily account'], wouldCreateCategories: ['Groceries'], wouldCreateTags: ['home', 'food'], accountOptions: [{ name: 'Daily account', sourceRowCount: 2, selected: true }] })
    mocks.confirmCSVImport.mockResolvedValue({ importId: 'import-1', jobId: 'job-1', jobType: 'finance.csv_import' })
    mocks.getCSVImportAudit.mockResolvedValue(audit())
    mocks.listRecentCSVImportAudits.mockResolvedValue([audit()])
  })

  it('uses the fixed transaction contract and confirms without a mapping', async () => {
    const user = userEvent.setup()
    render(FinanceImports)
    expect(await screen.findByText(/Required headers:/)).toBeInTheDocument()
    expect(screen.getByText('00')).toBeInTheDocument()
    expect(screen.getByText(/means 2000–2099/)).toBeInTheDocument()
    expect(screen.getByText(/matched by name in any order/)).toBeInTheDocument()
    expect(screen.getByText(/Optional header:/)).toBeInTheDocument()
    expect(screen.getByText(/Missing or blank descriptions import as/)).toBeInTheDocument()
    expect(screen.getByText(/Unsupported extra columns are ignored wherever they occur/)).toBeInTheDocument()
    expect(Array.from(document.querySelectorAll('code')).some((element) => element.textContent === '"8 300,00"')).toBe(true)
    expect(screen.queryByLabelText('Import type')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Preview transactions' }))
    expect(await screen.findByText('Row 3 · Date: Date must use dd.MM.yy')).toBeInTheDocument()
    expect(screen.getByText('Row 4 · Description: Duplicate transaction')).toBeInTheDocument()
    expect(screen.getByText('Transactions to import: 1')).toBeInTheDocument()
    expect(screen.getAllByText('Daily account').length).toBeGreaterThan(0)
    expect(mocks.previewCSVImport).toHaveBeenCalledWith(expect.objectContaining({ tenantId: 'tenant-1', fileName: 'finance-transactions.csv' }))
    expect(mocks.previewCSVImport.mock.calls[0][0]).not.toHaveProperty('importType')

    await user.click(screen.getByRole('button', { name: 'Confirm valid rows' }))
    await waitFor(() => expect(mocks.confirmCSVImport).toHaveBeenCalledWith({ tenantId: 'tenant-1', importId: 'import-1' }))
    expect(await screen.findByText('Imported')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open finance job detail' })).toHaveAttribute('href', '#/finance/jobs/job-1')
    expect(screen.queryByRole('button', { name: 'Confirm valid rows' })).not.toBeInTheDocument()
  })

  it('places the always-present active workspace before recent import history', async () => {
    render(FinanceImports)

    const source = await screen.findByRole('heading', { name: 'Choose a transaction CSV' })
    const workspace = screen.getByRole('heading', { name: 'Active import workspace' })
    const history = screen.getByRole('heading', { name: 'Reopen a durable import audit' })
    expect(source.compareDocumentPosition(workspace) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(workspace.compareDocumentPosition(history) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(screen.getByText(/No active import yet/)).toBeInTheDocument()
  })

  it('shows pending preview feedback, prevents duplicate submission, and focuses the workspace', async () => {
    const user = userEvent.setup()
    let resolvePreview: (value: unknown) => void = () => undefined
    mocks.previewCSVImport.mockReturnValueOnce(new Promise((resolve) => { resolvePreview = resolve }))
    render(FinanceImports)

    await user.click(await screen.findByRole('button', { name: 'Preview transactions' }))
    const workspace = screen.getByRole('heading', { name: 'Active import workspace' }).closest('section')
    expect(await screen.findByRole('status')).toHaveTextContent('Previewing transaction CSV…')
    expect(screen.getByRole('button', { name: 'Previewing…' })).toBeDisabled()
    expect(document.activeElement).toBe(workspace)
    expect(scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'start' })

    resolvePreview({ importId: 'import-1', importableCount: 1, headers: [], duplicateRows: [], rejectedRows: [], wouldCreateAccounts: [], wouldCreateCategories: [], wouldCreateTags: [] })
    expect(await screen.findByRole('button', { name: 'Confirm valid rows' })).toBeInTheDocument()
  })

  it('shows running audit progress and permits manual refresh', async () => {
    const user = userEvent.setup()
    mocks.getCSVImportAudit.mockResolvedValue(audit('running'))
    render(FinanceImports)
    await user.click(await screen.findByRole('button', { name: 'Preview transactions' }))
    await user.click(screen.getByRole('button', { name: 'Confirm valid rows' }))
    expect(await screen.findByText(/Import is running/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Refresh audit' }))
    expect(mocks.getCSVImportAudit).toHaveBeenCalledTimes(2)
  })

  it('renders a loading state while the tenant list is pending', async () => {
    let resolveList: (value: unknown) => void = () => undefined
    mocks.listTenants.mockReturnValueOnce(new Promise((resolve) => { resolveList = resolve }))
    render(FinanceImports)
    expect(screen.getByText('Loading imports…')).toBeInTheDocument()
    resolveList([{ id: 'tenant-1', name: 'Household', displayCurrency: 'USD', joinedAt: now, createdAt: now, updatedAt: now }])
    expect(await screen.findByRole('button', { name: 'Preview transactions' })).toBeInTheDocument()
  })

  it('renders a no-tenant state with preview disabled', async () => {
    mocks.listTenants.mockResolvedValueOnce([])
    render(FinanceImports)
    expect(await screen.findByRole('button', { name: 'Preview transactions' })).toBeDisabled()
  })

  it('rejects an oversized selected CSV before reading or previewing it', async () => {
    const oversizedFile = new File(['x'], 'oversized.csv', { type: 'text/csv' })
    Object.defineProperty(oversizedFile, 'size', { value: 64 * 1024 * 1024 + 1 })
    render(FinanceImports)

    await fireEvent.change(await screen.findByLabelText('CSV file'), { target: { files: [oversizedFile] } })

    expect(await screen.findByRole('alert')).toHaveTextContent('CSV files must be 64 MiB or smaller.')
    expect(mocks.previewCSVImport).not.toHaveBeenCalled()
  })

  it('keeps the form recoverable when preview or confirmation fails', async () => {
    const user = userEvent.setup()
    mocks.previewCSVImport.mockRejectedValueOnce('preview unavailable')
    render(FinanceImports)
    await user.click(await screen.findByRole('button', { name: 'Preview transactions' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Failed to preview import')

    mocks.previewCSVImport.mockResolvedValueOnce({ importId: 'import-1', importableCount: 1, headers: [], duplicateRows: [], rejectedRows: [], wouldCreateAccounts: [], wouldCreateCategories: [], wouldCreateTags: [] })
    await user.click(screen.getByRole('button', { name: 'Preview transactions' }))
    mocks.confirmCSVImport.mockRejectedValueOnce(new Error('confirmation unavailable'))
    await user.click(await screen.findByRole('button', { name: 'Confirm valid rows' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('confirmation unavailable')
  })

  it('shows a stable validation message when preview input is rejected', async () => {
    const user = userEvent.setup()
    mocks.previewCSVImport.mockRejectedValueOnce(new FinanceApiError({
      status: 400,
      method: 'POST',
      path: '/finance/tenants/tenant-1/imports/preview',
      message: 'currency "gBp" must be one of USD, EUR, PLN, UAH',
    }))
    render(FinanceImports)

    await user.click(await screen.findByRole('button', { name: 'Preview transactions' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('We could not validate this CSV preview. Check the file and try again.')
    expect(screen.queryByText('currency "gBp" must be one of USD, EUR, PLN, UAH')).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Active import workspace' })).toBeInTheDocument()
  })

  it('separates matched headers from ignored source headers', async () => {
    const user = userEvent.setup()
    mocks.previewCSVImport.mockResolvedValueOnce({
      importId: 'import-1',
      importableCount: 1,
      headers: ['Source note', 'Date', 'Account', 'Ignored middle', 'Category', 'Tags', 'Expense amount', 'Income amount', 'Currency', 'Description', 'Tail note'],
      duplicateRows: [], rejectedRows: [], wouldCreateAccounts: [], wouldCreateCategories: [], wouldCreateTags: [],
    })
    render(FinanceImports)

    await user.click(await screen.findByRole('button', { name: 'Preview transactions' }))

    expect(await screen.findByText(/Matched supported headers: Date, Account/)).toBeInTheDocument()
    expect(screen.getByText('Ignored source headers: Source note, Ignored middle, Tail note.')).toBeInTheDocument()
    expect(screen.queryByText(/Resolved headers/)).not.toBeInTheDocument()
  })

  it('renders the server-provided importable count for all-valid and zero-importable previews', async () => {
    const user = userEvent.setup()
    mocks.previewCSVImport.mockResolvedValueOnce({ importId: 'import-valid', importableCount: 2, headers: [], duplicateRows: [], rejectedRows: [], wouldCreateAccounts: [], wouldCreateCategories: [], wouldCreateTags: [] }).mockResolvedValueOnce({ importId: 'import-empty', importableCount: 0, headers: [], duplicateRows: [{ rowNumber: 2, reason: 'Duplicate transaction' }], rejectedRows: [{ rowNumber: 3, reason: 'Invalid date' }], wouldCreateAccounts: [], wouldCreateCategories: [], wouldCreateTags: [] })
    render(FinanceImports)

    await user.click(await screen.findByRole('button', { name: 'Preview transactions' }))
    expect(await screen.findByText('Transactions to import: 2')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Preview transactions' }))
    expect(await screen.findByText('Transactions to import: 0')).toBeInTheDocument()
    expect(screen.getByText(/Rejected and duplicate rows below are excluded/)).toBeInTheDocument()
  })

  it('regenerates the preview from accessible account checkboxes and disables confirmation while updating', async () => {
    const user = userEvent.setup()
    mocks.previewCSVImport.mockResolvedValueOnce({
      importId: 'import-all', importableCount: 2, headers: [], duplicateRows: [], rejectedRows: [], wouldCreateAccounts: ['Checking', 'Savings'], wouldCreateCategories: [], wouldCreateTags: [],
      accountOptions: [{ name: 'Checking', sourceRowCount: 1, selected: true }, { name: 'Savings', sourceRowCount: 2, selected: true }],
    }).mockResolvedValueOnce({
      importId: 'import-checking', importableCount: 1, headers: [], duplicateRows: [], rejectedRows: [], wouldCreateAccounts: ['Checking'], wouldCreateCategories: [], wouldCreateTags: [],
      accountOptions: [{ name: 'Checking', sourceRowCount: 1, selected: true }, { name: 'Savings', sourceRowCount: 2, selected: false }],
    })
    render(FinanceImports)

    await user.click(await screen.findByRole('button', { name: 'Preview transactions' }))
    expect(await screen.findByRole('checkbox', { name: /Checking/ })).toBeChecked()
    expect(screen.getByRole('checkbox', { name: /Savings/ })).toBeChecked()

    await user.click(screen.getByRole('checkbox', { name: /Savings/ }))
    expect(await screen.findByText('Updating preview…')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Confirm valid rows' })).toBeDisabled()
    await waitFor(() => expect(mocks.previewCSVImport).toHaveBeenLastCalledWith(expect.objectContaining({ selectedAccountNames: ['Checking'] })))
    expect(await screen.findByText('Transactions to import: 1')).toBeInTheDocument()
    expect(screen.getByRole('checkbox', { name: /Savings/ })).not.toBeChecked()
  })

  it('does not let an older account-filter response replace a newer checkbox selection', async () => {
    const user = userEvent.setup()
    let resolveStale: (value: unknown) => void = () => undefined
    mocks.previewCSVImport.mockResolvedValueOnce({
      importId: 'import-all', importableCount: 2, headers: [], duplicateRows: [], rejectedRows: [], wouldCreateAccounts: [], wouldCreateCategories: [], wouldCreateTags: [],
      accountOptions: [{ name: 'Checking', sourceRowCount: 1, selected: true }, { name: 'Savings', sourceRowCount: 1, selected: true }],
    }).mockReturnValueOnce(new Promise((resolve) => { resolveStale = resolve }))
    render(FinanceImports)

    await user.click(await screen.findByRole('button', { name: 'Preview transactions' }))
    await user.click(screen.getByRole('checkbox', { name: /Checking/ }))
    await waitFor(() => expect(mocks.previewCSVImport).toHaveBeenCalledTimes(2))
    await user.click(screen.getByRole('checkbox', { name: /Savings/ }))
    resolveStale({
      importId: 'stale-preview', importableCount: 1, headers: [], duplicateRows: [], rejectedRows: [], wouldCreateAccounts: [], wouldCreateCategories: [], wouldCreateTags: [],
      accountOptions: [{ name: 'Checking', sourceRowCount: 1, selected: false }, { name: 'Savings', sourceRowCount: 1, selected: true }],
    })
    await Promise.resolve()

    expect(screen.getByRole('checkbox', { name: /Checking/ })).not.toBeChecked()
    expect(screen.getByRole('checkbox', { name: /Savings/ })).not.toBeChecked()
  })

  it('synchronizes a terminal audit into recent imports without moving focus', async () => {
    const user = userEvent.setup()
    const terminalAudit = audit()
    mocks.getCSVImportAudit.mockResolvedValueOnce(terminalAudit)
    mocks.listRecentCSVImportAudits.mockResolvedValueOnce([{
      ...terminalAudit,
      status: 'confirmed',
      importedCount: 0,
    }]).mockResolvedValueOnce([{
      ...terminalAudit,
      status: 'confirmed',
      importedCount: 0,
    }])
    render(FinanceImports)
    await screen.findByRole('button', { name: 'Open audit' })
    scrollIntoView.mockClear()

    await user.click(screen.getByRole('button', { name: 'Open audit' }))

    expect(await screen.findByText(/Imported 2 rows/)).toBeInTheDocument()
    expect(scrollIntoView).toHaveBeenCalledTimes(1)
  })

  it('reopens a tenant-scoped durable audit and recovers a stale confirmation conflict', async () => {
    const user = userEvent.setup()
    mocks.confirmCSVImport.mockRejectedValueOnce(new FinanceApiError({
      status: 409,
      method: 'POST',
      path: '/finance/tenants/tenant-1/imports/import-1/confirm',
      message: 'already confirmed',
    }))
    render(FinanceImports)

    await user.click(await screen.findByRole('button', { name: 'Open audit' }))
    expect(await screen.findByText(/Status completed/)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Preview transactions' }))
    await user.click(screen.getByRole('button', { name: 'Confirm valid rows' }))

    expect(await screen.findByText('This import was already confirmed. Its durable audit was reopened.')).toBeInTheDocument()
    expect(screen.getByText(/Status completed/)).toBeInTheDocument()
    expect(mocks.getCSVImportAudit).toHaveBeenCalledWith({ tenantId: 'tenant-1', importId: 'import-1' })
  })

  it('marks the selected audit as loading and focuses the active workspace', async () => {
    const user = userEvent.setup()
    let resolveAudit: (value: unknown) => void = () => undefined
    mocks.getCSVImportAudit.mockReturnValue(new Promise((resolve) => { resolveAudit = resolve }))
    render(FinanceImports)

    await user.click(await screen.findByRole('button', { name: 'Open audit' }))
    const workspace = screen.getByRole('heading', { name: 'Active import workspace' }).closest('section')
    expect(await screen.findByText('Loading selected import audit…')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Loading audit…' })).toBeDisabled()
    expect(document.activeElement).toBe(workspace)
    expect(scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'start' })

    resolveAudit(audit())
    expect(await screen.findByText(/Status completed/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Audit open' })).toHaveAttribute('aria-current', 'true')
  })
})
