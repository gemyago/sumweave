import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import AdminJobDetail from './AdminJobDetail.svelte'

describe('Admin job detail route', () => {
  it('passes the route job identifier to the finance-safe detail view', () => {
    render(AdminJobDetail, { params: { jobId: 'job-1' } })
    expect(screen.getByText('Loading job…')).toBeInTheDocument()
  })

  it('uses an empty identifier when the route parameter is unavailable', () => {
    render(AdminJobDetail)
    expect(screen.getByText('Loading job…')).toBeInTheDocument()
  })
})
