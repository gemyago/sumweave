<script lang="ts">
  import { onMount } from 'svelte'
  import { link, replace } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createFinanceShellState,
    isFinanceTenantScopedRoute,
    provideFinanceShellState,
  } from '../lib/finance/shell-state.svelte'
  import ThemeSegmentedControl from './ThemeSegmentedControl.svelte'

  let { currentPath } = $props<{ currentPath: string }>()

  const financeShell = provideFinanceShellState(createFinanceShellState())

  const railLinks = [
    { href: '/finance', label: 'Dashboard' },
    { href: '/finance/transactions', label: 'Transactions' },
    { href: '/finance/accounts', label: 'Accounts' },
    { href: '/finance/categories', label: 'Categories' },
    { href: '/finance/connections', label: 'Connections & sync' },
    { href: '/finance/imports', label: 'Imports' },
    { href: '/finance/tenants', label: 'Tenants' },
  ]

  const showsTenantControl = $derived(
    isFinanceTenantScopedRoute(currentPath) && financeShell.hasMultipleTenants,
  )

  const activeRailHref = $derived.by(() => {
    let activeHref = ''

    for (const item of railLinks) {
      if (currentPath === item.href || currentPath.startsWith(`${item.href}/`)) {
        if (item.href.length > activeHref.length) {
          activeHref = item.href
        }
      }
    }

    return activeHref
  })

  const activeRailLabel = $derived.by(
    () => railLinks.find((item) => item.href === activeRailHref)?.label ?? 'Workspace',
  )

  let compactRail = $state(false)
  let compactRailMenuOpen = $state(false)

  const showsCompactRailMenu = $derived(compactRail)
  const showsExpandedRailSections = $derived(!compactRail || compactRailMenuOpen)

  onMount(() => {
    const mediaQuery =
      typeof window.matchMedia === 'function'
        ? window.matchMedia('(max-width: 960px)')
        : null

    const syncCompactRail = (matches: boolean) => {
      compactRail = matches
      compactRailMenuOpen = false
    }

    syncCompactRail(mediaQuery?.matches ?? false)

    const onMediaChange = (event: MediaQueryListEvent) => {
      syncCompactRail(event.matches)
    }

    mediaQuery?.addEventListener('change', onMediaChange)
    void financeShell.initialize()

    return () => {
      mediaQuery?.removeEventListener('change', onMediaChange)
    }
  })

  $effect(() => {
    void currentPath
    if (compactRail) {
      compactRailMenuOpen = false
    }
  })

  function signOut(): void {
    authStore.clearAuth()
    replace('/login')
  }

  function onTenantChange(event: Event): void {
    financeShell.selectTenant((event.currentTarget as HTMLSelectElement).value)
  }

  function toggleCompactRailMenu(): void {
    compactRailMenuOpen = !compactRailMenuOpen
  }
</script>

<div class="finance-shell" data-finance-shell="true">
  <aside class="rail" aria-label="Finance navigation">
    <div class="rail-top-wrap">
      <div class="rail-top">
        <a class="brand" href="/data" use:link>Signal Foundry</a>
        <p class="rail-copy">Finance workspace</p>
      </div>

      {#if showsCompactRailMenu}
        <div class="rail-mobile-summary">
          <p class="rail-mobile-label">Finance / {activeRailLabel}</p>
          <button
            type="button"
            class="secondary rail-menu-toggle"
            aria-expanded={compactRailMenuOpen}
            aria-controls="finance-rail-sections"
            onclick={toggleCompactRailMenu}
          >
            {#if compactRailMenuOpen}
              Close menu
            {:else}
              Open menu
            {/if}
          </button>
        </div>
      {/if}
    </div>

    {#if showsExpandedRailSections}
      <div id="finance-rail-sections" class="rail-sections">
        <nav class="rail-nav" aria-label="Finance destinations">
          {#each railLinks as item (item.href)}
            <a
              class="rail-link"
              href={item.href}
              use:link
              aria-current={activeRailHref === item.href ? 'page' : undefined}
            >
              <span>{item.label}</span>
            </a>
          {/each}
        </nav>

        <div class="rail-footer">
          <p class="rail-footer-label">Other workspaces</p>
          <a href="/chat" use:link>Return to chat</a>
          <a href="/data" use:link>Open data workspace</a>
        </div>
      </div>
    {/if}
  </aside>

  <section class="workspace">
    <header class="utility-row" aria-label="Finance utilities">
      <div class="utility-copy">
        <p class="utility-label">Finance / {activeRailLabel}</p>
        <p class="utility-text">Shared finance workspace.</p>
      </div>

      <div class="utility-side">
        {#if showsTenantControl}
          <label class="tenant-control">
            <span>Active tenant</span>
            <select
              value={financeShell.selectedTenantId}
              onchange={onTenantChange}
              aria-label="Active tenant"
              disabled={financeShell.loading}
            >
              <option value="">{financeShell.hasTenants ? 'Select tenant' : 'No tenants yet'}</option>
              {#each financeShell.tenants as tenant (tenant.id)}
                <option value={tenant.id}>{tenant.name} · {tenant.displayCurrency}</option>
              {/each}
            </select>
          </label>
        {/if}

        <div class="utility-actions">
          <button type="button" class="sign-out" onclick={signOut}>Sign out</button>
          <ThemeSegmentedControl />
        </div>
      </div>
    </header>

    <div class="workspace-main">
      <slot />
    </div>
  </section>
</div>

<style>
  .finance-shell {
    display: grid;
    grid-template-columns: 236px minmax(0, 1fr);
    min-height: 0;
    flex: 1;
    width: 100%;
  }

  .rail {
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    gap: var(--space-20);
    padding: var(--space-20) var(--space-16);
    position: sticky;
    top: 0;
    align-self: start;
    min-height: 100dvh;
    box-sizing: border-box;
    border-right: 1px solid var(--border);
  }

  .rail-top-wrap,
  .rail-top,
  .rail-mobile-summary,
  .rail-sections,
  .rail-nav,
  .rail-footer,
  .workspace,
  .workspace-main,
  .utility-copy,
  .utility-side,
  .utility-actions,
  .tenant-control {
    display: flex;
    flex-direction: column;
  }

  .rail-sections,
  .rail-nav,
  .rail-footer,
  .workspace,
  .workspace-main {
    gap: var(--space-16);
  }

  .rail-top,
  .rail-mobile-summary,
  .utility-copy {
    gap: var(--space-8);
  }

  .rail-mobile-summary {
    display: none;
  }

  .rail-mobile-label {
    margin: 0;
    color: var(--text-h);
    font-weight: 700;
  }

  .rail-menu-toggle {
    align-self: start;
  }

  .brand,
  .rail-footer a,
  .sign-out {
    color: var(--link);
    text-decoration: underline;
    text-underline-offset: 2px;
    font-weight: 500;
  }

  .brand {
    font-weight: 700;
  }

  .rail-link {
    display: flex;
    align-items: center;
    min-height: 2.5rem;
    padding: 0.625rem var(--space-12);
    border: 1px solid transparent;
    border-radius: 4px;
    color: var(--text);
    text-decoration: none;
    font-weight: 500;
    transition:
      background-color 120ms ease,
      border-color 120ms ease,
      color 120ms ease;
  }

  .rail-link:hover,
  .rail-link:focus-visible {
    color: var(--text-h);
    background: var(--secondary-hover-bg);
  }

  .rail-link[aria-current='page'] {
    color: var(--text-h);
    background: var(--surface-raised);
    border-color: var(--border);
  }

  .rail-copy,
  .rail-footer-label,
  .utility-label,
  .utility-text {
    margin: 0;
  }

  .rail-copy,
  .rail-footer-label,
  .utility-text {
    color: var(--text-muted);
  }

  .rail-footer-label {
    font-size: var(--font-size-caption);
  }

  .workspace {
    min-width: 0;
    padding: var(--space-20) clamp(var(--space-18), 2.75vw, var(--space-28));
  }

  .utility-row {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    gap: var(--space-12) var(--space-20);
    align-items: flex-start;
    padding-bottom: var(--space-12);
    border-bottom: 1px solid var(--border);
  }

  .utility-label {
    font-weight: 700;
  }

  .utility-side {
    flex: 0 1 auto;
    align-items: flex-end;
    gap: var(--space-8);
  }

  .tenant-control {
    gap: var(--space-4);
    min-width: min(320px, 100%);
  }

  .tenant-control select {
    min-width: 0;
  }

  .utility-actions {
    flex-direction: row;
    align-items: center;
    justify-content: flex-end;
    gap: var(--space-16);
  }

  .sign-out {
    margin: 0;
    padding: 0;
    border: none;
    background: transparent;
    font: inherit;
    cursor: pointer;
  }

  .workspace-main {
    min-width: 0;
  }

  @media (max-width: 960px) {
    .finance-shell {
      grid-template-columns: 1fr;
    }

    .rail {
      position: static;
      min-height: auto;
      border-right: none;
      border-bottom: 1px solid var(--border);
      gap: var(--space-12);
      padding: var(--space-16);
    }

    .rail-top-wrap {
      gap: var(--space-12);
    }

    .rail-top {
      gap: var(--space-4);
    }

    .rail-copy {
      display: none;
    }

    .rail-mobile-summary {
      display: flex;
      flex-direction: row;
      justify-content: space-between;
      align-items: center;
      gap: var(--space-12);
    }

    .rail-sections {
      gap: var(--space-12);
    }

    .rail-nav {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: var(--space-10) var(--space-12);
    }

    .rail-link {
      min-height: 2.25rem;
      padding: 0.5rem var(--space-12);
    }

    .rail-footer {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: var(--space-8) var(--space-12);
    }

    .rail-footer-label {
      grid-column: 1 / -1;
    }

    .workspace {
      padding: var(--space-16) var(--space-16) var(--space-24);
    }

    .utility-row {
      align-items: stretch;
      gap: var(--space-10);
    }

    .utility-side {
      align-items: stretch;
    }

    .tenant-control {
      min-width: 0;
    }

    .utility-actions {
      justify-content: space-between;
      flex-wrap: wrap;
    }
  }

  @media (max-width: 640px) {
    .rail-mobile-summary {
      flex-wrap: wrap;
    }

    .workspace {
      padding: var(--space-14) var(--space-14) var(--space-20);
    }

    .utility-text {
      display: none;
    }
  }
</style>
