import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import AdminFinanceFX from './AdminFinanceFX.svelte'

const mocks = vi.hoisted(() => ({
  getFXDiagnostics: vi.fn(),
  triggerFXSync: vi.fn(),
  getJob: vi.fn(),
}))

vi.mock('../lib/finance/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/finance/api')>()
  return {
    ...actual,
    createSignalFinanceApiForAuth: vi.fn(() => ({ ...mocks })),
  }
})

vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))
vi.mock('../lib/jobs/api', () => ({
  createSignalJobsApiForAuth: vi.fn(() => ({ getJob: mocks.getJob })),
}))

describe('Admin finance FX page', () => {
  beforeEach(() => {
    mocks.getFXDiagnostics.mockReset()
    mocks.triggerFXSync.mockReset()
    mocks.getJob.mockReset()
    mocks.getFXDiagnostics.mockResolvedValue({ defaultProvider: 'frankfurter', storedRatesCount: 14, providers: [{ name: 'frankfurter', default: true, ready: true }] })
    mocks.triggerFXSync.mockResolvedValue({ jobId: 'job-22', jobType: 'finance.fx_rates_refresh', provider: 'frankfurter' })
    mocks.getJob.mockResolvedValue({ id: 'job-22', status: 'queued', jobType: 'finance.fx_rates_refresh' })
  })

  it('shows diagnostics and can trigger a latest-rate refresh job', async () => {
    const user = userEvent.setup()
    render(AdminFinanceFX)

    expect(await screen.findByText(/current rates 14/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Refresh required rates' }))
    await waitFor(() => expect(mocks.triggerFXSync).toHaveBeenCalled())
    const request = mocks.triggerFXSync.mock.calls[0][0]
    expect(request).toEqual({})
    expect(await screen.findByRole('link', { name: 'Open job' })).toHaveAttribute('href', '#/admin/jobs/job-22')
    expect(screen.queryByLabelText('Base currencies')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Quote currency')).not.toBeInTheDocument()
  })

  it('explains dynamic required-rate discovery and has no pair or historical range controls', async () => {
    render(AdminFinanceFX)

    expect(await screen.findByText(/Discovers active-tenant account and transaction currency pairs/)).toBeInTheDocument()
    expect(screen.queryByLabelText('Base currencies')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Quote currency')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('FX start date')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('FX end date')).not.toBeInTheDocument()
  })

  it('renders an error state when diagnostics fail', async () => {
    mocks.getFXDiagnostics.mockRejectedValueOnce(new Error('fx exploded'))
    render(AdminFinanceFX)
    expect(await screen.findByRole('alert')).toHaveTextContent('fx exploded')
  })

  it('renders an empty provider list and fallback default provider label', async () => {
    mocks.getFXDiagnostics.mockResolvedValueOnce({ defaultProvider: '', storedRatesCount: 0, providers: [] })
    render(AdminFinanceFX)
    expect(await screen.findByText(/Default provider — · current rates 0/)).toBeInTheDocument()
  })
})
