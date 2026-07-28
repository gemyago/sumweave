<script lang="ts">
  import FinanceJobDetail from './FinanceJobDetail.svelte'
  let { params = {} } = $props<{ params?: { jobId?: string } }>()
</script>
<FinanceJobDetail jobId={params.jobId ?? ''} />
