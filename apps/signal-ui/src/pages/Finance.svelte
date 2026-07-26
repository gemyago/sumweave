<script lang="ts">
  import BootstrapFinanceDashboard from './BootstrapFinanceDashboard.svelte'
  import { documentTitle } from '../lib/document-title'
  import DocumentTitle from '../components/DocumentTitle.svelte'
</script>

<DocumentTitle title={documentTitle('Finance')} />

<BootstrapFinanceDashboard />
