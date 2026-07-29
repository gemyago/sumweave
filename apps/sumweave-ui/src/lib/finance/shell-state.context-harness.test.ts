import { render, screen } from '@testing-library/svelte'
import { describe, expect, it } from 'vitest'

import ShellStateContextHarness from './shell-state.context-harness.svelte'
import { FinanceShellState } from './shell-state.svelte'

describe('shell-state.context-harness', () => {
  it('falls back to useFinanceShellState when no state is provided', () => {
    render(ShellStateContextHarness)

    expect(screen.getByTestId('embedded')).toHaveTextContent('false')
    expect(screen.getByTestId('selected-tenant').textContent).toBe('')
  })

  it('provides the supplied shell state through context when present', () => {
    const state = new FinanceShellState()
    state.selectedTenantId = 'tenant-2'

    render(ShellStateContextHarness, {
      providedState: state,
    })

    expect(screen.getByTestId('embedded')).toHaveTextContent('true')
    expect(screen.getByTestId('selected-tenant')).toHaveTextContent('tenant-2')
    expect(state.embedded).toBe(true)
  })
})
