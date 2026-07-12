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
    const request = mocks.triggerFXSync.mock.calls[0][0]
    expect(request.startDate).toBeInstanceOf(Date)
    expect(request.endDate).toBeInstanceOf(Date)
    expect(await screen.findByRole('link', { name: 'open admin job detail' })).toHaveAttribute('href', '#/admin/jobs/job-22')
  })

  it('keeps its initial native date defaults in a negative-offset timezone', async () => {
    const environment = (globalThis as unknown as { process: { env: Record<string, string | undefined> } }).process.env
    const previousTimezone = environment.TZ
    environment.TZ = 'America/Los_Angeles'
    try {
      render(AdminFinanceFX)

      expect((await screen.findByLabelText('FX start date') as HTMLInputElement).value).toBe('2026-01-01')
      expect((screen.getByLabelText('FX end date') as HTMLInputElement).value).toBe('2026-01-31')
    } finally {
      if (previousTimezone === undefined) delete environment.TZ
      else environment.TZ = previousTimezone
    }
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
