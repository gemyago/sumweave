<script>
  import { onMount } from 'svelte'
  import { replace } from 'svelte-spa-router'
  import { DEFAULT_AUTHENTICATED_ROUTE } from '../lib/routing/post-login-destination'

  onMount(() => {
    replace(DEFAULT_AUTHENTICATED_ROUTE)
  })
</script>

<p class="muted" role="status">Opening data…</p>

<style>
  .muted {
    margin: 0;
    color: var(--text);
    font-size: var(--font-size-body);
    line-height: 1.5;
  }
</style>
