<script lang="ts">
  import { link } from 'svelte-spa-router'

  export let current = '/finance'
  export let tenantName = ''

  const links = [
    { href: '/finance', label: 'Dashboard' },
    { href: '/finance/tenants', label: 'Tenants' },
    { href: '/finance/accounts', label: 'Accounts' },
    { href: '/finance/transactions', label: 'Transactions' },
    { href: '/finance/categories', label: 'Categories & tags' },
    { href: '/finance/connections', label: 'Connections' },
    { href: '/finance/imports', label: 'Imports' },
  ]
</script>

<section class="finance-subnav" aria-label="Finance sections">
  <div>
    <p class="eyebrow">Finance</p>
    <p class="muted">{tenantName ? `Tenant: ${tenantName}` : 'Select or create a tenant to continue.'}</p>
  </div>
  <div class="links">
    {#each links as item (item.href)}
      <a href={item.href} use:link aria-current={current === item.href ? 'page' : undefined}>{item.label}</a>
    {/each}
  </div>
</section>

<style>
  .finance-subnav {
    display: flex;
    flex-direction: column;
    gap: var(--space-12);
    padding: var(--space-16);
    border: 1px solid var(--border);
    border-radius: 4px;
    background: var(--bg-elevated, var(--bg));
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
  .eyebrow,
  .muted {
    margin: 0;
  }
  .eyebrow {
    font-weight: 700;
  }
  .muted {
    color: var(--text-muted);
  }
</style>
