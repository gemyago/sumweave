import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import AdminFinanceFX from './AdminFinanceFX.svelte'

const mocks = vi.hoisted(() => ({
  getFXDiagnostics: vi.fn(),
  triggerFXSync: vi.fn(),
}))

vi.mock('../lib/finance/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/finance/api')>()
  return {
    ...actual,
    createSignalFinanceApiForAuth: vi.fn(() => ({ ...mocks })),
  }
})

vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

describe('Admin finance FX page', () => {
  beforeEach(() => {
    mocks.getFXDiagnostics.mockReset()
    mocks.triggerFXSync.mockReset()
    mocks.getFXDiagnostics.mockResolvedValue({ defaultProvider: 'frankfurter', storedRatesCount: 14, providers: [{ name: 'frankfurter', default: true, ready: true }] })
    mocks.triggerFXSync.mockResolvedValue({ jobId: 'job-22', jobType: 'finance.fx_rates_sync', provider: 'frankfurter' })
  })

  it('shows diagnostics and can trigger a sync job', async () => {
    const user = userEvent.setup()
    render(AdminFinanceFX)

    expect(await screen.findByText(/stored rates 14/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Trigger FX sync' }))
    await waitFor(() => expect(mocks.triggerFXSync).toHaveBeenCalled())
    expect(await screen.findByRole('link', { name: 'open admin job detail' })).toHaveAttribute('href', '#/admin/jobs/job-22')
  })

  it('renders an error state when diagnostics fail', async () => {
    mocks.getFXDiagnostics.mockRejectedValueOnce(new Error('fx exploded'))
    render(AdminFinanceFX)
    expect(await screen.findByRole('alert')).toHaveTextContent('fx exploded')
  })

  it('renders an empty provider list and fallback default provider label', async () => {
    mocks.getFXDiagnostics.mockResolvedValueOnce({ defaultProvider: '', storedRatesCount: 0, providers: [] })
    render(AdminFinanceFX)
    expect(await screen.findByText(/Default provider — · stored rates 0/)).toBeInTheDocument()
  })
})
