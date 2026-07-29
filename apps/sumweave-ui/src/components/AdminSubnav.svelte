<script lang="ts">
  import { link } from 'svelte-spa-router'

  export let current = '/admin'

  const links = [
    { href: '/admin', label: 'Overview' },
    { href: '/admin/jobs', label: 'Jobs' },
    { href: '/admin/finance/fx', label: 'Finance FX' },
    { href: '/admin/finance/providers', label: 'Finance providers' },
  ]
</script>

<section class="admin-subnav" aria-label="Admin sections">
  <p class="eyebrow">Admin diagnostics</p>
  <div class="links">
    {#each links as item (item.href)}
      <a href={item.href} use:link aria-current={current === item.href ? 'page' : undefined}>{item.label}</a>
    {/each}
  </div>
</section>

<style>
  .admin-subnav {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
    padding: var(--space-16);
    border: 1px solid var(--border);
    border-radius: 4px;
    background: var(--bg-elevated, var(--bg));
  }
  .eyebrow {
    margin: 0;
    font-weight: 700;
  }
  .links {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-12);
  }
  .links a {
    color: var(--link);
    font-weight: 500;
    text-decoration: underline;
  }
  .links a[aria-current='page'] {
    color: var(--text-h);
  }
</style>
