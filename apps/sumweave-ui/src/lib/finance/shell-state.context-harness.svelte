<script lang="ts">
  import {
    FinanceShellState,
    provideFinanceShellState,
    useFinanceShellState,
  } from './shell-state.svelte'

  export let providedState: FinanceShellState | null = null

  const state = providedState
    ? provideFinanceShellState(providedState)
    : useFinanceShellState()
</script>

<output data-testid="embedded">{String(state.embedded)}</output>
<output data-testid="selected-tenant">{state.selectedTenantId}</output>
