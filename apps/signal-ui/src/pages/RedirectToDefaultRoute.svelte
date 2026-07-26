<script>
  import { onMount } from 'svelte'
  import { replace } from 'svelte-spa-router'
  import { DEFAULT_AUTHENTICATED_ROUTE } from '../lib/routing/post-login-destination'
  import { documentTitle } from '../lib/document-title'
  import DocumentTitle from '../components/DocumentTitle.svelte'

  onMount(() => {
    replace(DEFAULT_AUTHENTICATED_ROUTE)
  })
</script>

<DocumentTitle title={documentTitle()} />

<p class="muted" role="status">Opening finance…</p>

<style>
  .muted {
    margin: 0;
    color: var(--text);
    font-size: var(--font-size-body);
    line-height: 1.5;
  }
</style>
