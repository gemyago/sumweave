<script lang="ts">
  import JobDetail from './JobDetail.svelte'
  import { documentTitle } from '../lib/document-title'
  let { params = {} } = $props<{ params?: { jobId?: string } }>()
</script>

<JobDetail {params} heading="Admin job detail" description="Inspect one durable job from the admin diagnostics surface." primaryBackHref="/admin/jobs" primaryBackLabel="Back to admin jobs" secondaryBackHref="/admin" secondaryBackLabel="Back to admin overview" pageDocumentTitle={documentTitle('Job detail', 'Admin')} />
