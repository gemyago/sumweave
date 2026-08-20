import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import AdminFinanceProviders from './AdminFinanceProviders.svelte'

const financeMocks = vi.hoisted(() => ({ getFXDiagnostics: vi.fn() }))
const jobsMocks = vi.hoisted(() => ({ listJobs: vi.fn() }))
vi.mock('../lib/finance/api', async (importOriginal) => ({ ...(await importOriginal<typeof import('../lib/finance/api')>()), createSignalFinanceApiForAuth: vi.fn(() => ({ ...financeMocks })) }))
vi.mock('../lib/jobs/api', async (importOriginal) => ({ ...(await importOriginal<typeof import('../lib/jobs/api')>()), createSignalJobsApiForAuth: vi.fn(() => ({ ...jobsMocks })) }))
vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

describe('Admin finance providers page', () => {
  beforeEach(() => {
    financeMocks.getFXDiagnostics.mockResolvedValue({ defaultProvider: 'frankfurter', storedRatesCount: 2, providers: [{ name: 'frankfurter', default: true, ready: true }] })
    jobsMocks.listJobs.mockResolvedValue({ items: [{ id: 'job-1', jobType: 'finance.bank_connection_sync', status: 'failed', requester: { userId: 'user-1', source: 'operator' }, createdAt: new Date('2026-06-20T12:00:00Z'), updatedAt: new Date('2026-06-20T12:00:00Z'), startedAt: null, completedAt: null, attemptCount: 1 }], nextCursor: '' })
  })

  it('renders provider readiness and recent finance jobs', async () => {
    render(AdminFinanceProviders)
    expect(await screen.findByText('frankfurter')).toBeInTheDocument()
    expect(screen.getByText('finance.bank_connection_sync')).toBeInTheDocument()
  })

  it('renders provider and job empty states', async () => {
    financeMocks.getFXDiagnostics.mockResolvedValueOnce({ defaultProvider: '', storedRatesCount: 0, providers: [] })
    jobsMocks.listJobs.mockResolvedValueOnce({ items: [], nextCursor: '' })
    render(AdminFinanceProviders)
    expect(await screen.findByText('No provider diagnostics available.')).toBeInTheDocument()
    expect(screen.getByText('No recent finance jobs were returned.')).toBeInTheDocument()
  })

  it('renders an error state when provider diagnostics fail', async () => {
    financeMocks.getFXDiagnostics.mockRejectedValueOnce(new Error('providers exploded'))
    render(AdminFinanceProviders)
    expect(await screen.findByRole('alert')).toHaveTextContent('providers exploded')
  })
})
