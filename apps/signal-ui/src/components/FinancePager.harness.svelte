<script lang="ts">
  import FinancePager from './FinancePager.svelte'

  let { succeed = true }: { succeed?: boolean } = $props()
  let revision = $state(0)

  function changePage(): boolean {
    if (!succeed) return false
    revision += 1
    return true
  }
</script>

<FinancePager
  label="Ledger pages"
  status={`Page ${revision + 1}`}
  controls="ledger-results"
  hasPrevious={revision > 0}
  hasNext={revision === 0}
  onPrevious={changePage}
  onNext={changePage}
/>
