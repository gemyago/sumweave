import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import FinanceSubnav from './FinanceSubnav.svelte'

describe('FinanceSubnav', () => {
  it('shows the fallback tenant guidance and default dashboard current state', () => {
    render(FinanceSubnav)

    expect(screen.getByText('Select or create a tenant to continue.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Dashboard', current: 'page' })).toBeInTheDocument()
  })

  it('shows the active tenant copy and highlights the requested finance destination', () => {
    render(FinanceSubnav, {
      tenantName: 'Household',
      current: '/finance/transactions',
    })

    expect(screen.getByText('Tenant: Household')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Transactions', current: 'page' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Dashboard', current: 'page' })).not.toBeInTheDocument()
  })
})
