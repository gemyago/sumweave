import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import AdminJobs from './AdminJobs.svelte'

const mocks = vi.hoisted(() => ({ listJobs: vi.fn() }))

vi.mock('../lib/jobs/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/jobs/api')>()),
  createSignalJobsApiForAuth: vi.fn(() => mocks),
}))
vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { accessToken: 'token' } }))

function jobFixture() {
  return { id: 'job-1', jobType: 'finance.csv_import', status: 'queued', requester: { userId: 'user-1', source: 'operator', agentSessionId: '', agentRunId: '' }, createdAt: new Date(), updatedAt: new Date(), attemptCount: 0 }
}

describe('Admin jobs page', () => {
  beforeEach(() => vi.clearAllMocks())

  it('lists all retained finance job types with admin detail links', async () => {
    mocks.listJobs.mockResolvedValue({ items: [jobFixture()], nextCursor: '' })
    render(AdminJobs)

    expect(await screen.findByRole('link', { name: 'finance.csv_import' })).toHaveAttribute('href', '#/admin/jobs/job-1')
    expect(mocks.listJobs).toHaveBeenCalledWith({ jobType: ['finance.csv_import', 'finance.account_import', 'finance.bank_connection_sync', 'finance.fx_rates_refresh'] })
    expect(document.title).toBe('Jobs · Admin · Signal Foundry')
  })

  it('shows an empty state for no finance jobs', async () => {
    mocks.listJobs.mockResolvedValue({ items: [], nextCursor: '' })
    render(AdminJobs)
    expect(await screen.findByText('No finance jobs found.')).toBeInTheDocument()
  })

  it('shows a bounded API error', async () => {
    mocks.listJobs.mockRejectedValue(new Error('Jobs unavailable'))
    render(AdminJobs)
    expect(await screen.findByRole('alert')).toHaveTextContent('Jobs unavailable')
  })

  it('uses the generic error copy for a non-Error rejection', async () => {
    mocks.listJobs.mockRejectedValue('unavailable')
    render(AdminJobs)
    expect(await screen.findByRole('alert')).toHaveTextContent('Unable to load jobs.')
  })

  it('appends the next finance jobs page', async () => {
    mocks.listJobs
      .mockResolvedValueOnce({ items: [jobFixture()], nextCursor: 'next-page' })
      .mockResolvedValueOnce({ items: [{ ...jobFixture(), id: 'job-2', jobType: 'finance.fx_rates_refresh' }], nextCursor: '' })
    const user = userEvent.setup()
    render(AdminJobs)

    await user.click(await screen.findByRole('button', { name: 'Load more' }))

    expect(mocks.listJobs).toHaveBeenLastCalledWith({ jobType: ['finance.csv_import', 'finance.account_import', 'finance.bank_connection_sync', 'finance.fx_rates_refresh'], cursor: 'next-page' })
    expect(await screen.findByRole('link', { name: 'finance.fx_rates_refresh' })).toHaveAttribute('href', '#/admin/jobs/job-2')
  })
})
