import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'

const mocks = vi.hoisted(() => ({
  shellState: {
    embedded: true,
    loading: false,
    error: null,
    tenants: [
      { id: 'tenant-1', name: 'Household', displayCurrency: 'USD' },
      { id: 'tenant-2', name: 'Travel', displayCurrency: 'EUR' },
    ],
    selectedTenantId: '',
    selectedTenant: null,
    needsTenantSelection: true,
    hasTenants: true,
    initialize: vi.fn().mockResolvedValue(undefined),
    selectTenant: vi.fn(),
  },
}))

vi.mock('../lib/finance/shell-state.svelte', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/finance/shell-state.svelte')>()),
  useFinanceShellState: vi.fn(() => mocks.shellState),
}))

import FinanceJobDetail from './FinanceJobDetail.svelte'

describe('Finance job detail inside the embedded shell', () => {
  it('waits for the shared shell tenant choice without showing a local selector', async () => {
    render(FinanceJobDetail, { params: { jobId: 'job-1' } })

    expect(
      await screen.findByText('Select an active tenant to continue on this finance route.'),
    ).toBeInTheDocument()
    expect(screen.queryByRole('combobox', { name: 'Tenant' })).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Summary' })).not.toBeInTheDocument()
  })
})
