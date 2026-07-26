<script lang="ts">
  import Monitor from '@lucide/svelte/icons/monitor'
  import Moon from '@lucide/svelte/icons/moon'
  import Sun from '@lucide/svelte/icons/sun'
  import type { Snippet } from 'svelte'
  import { onMount } from 'svelte'
  import { link, replace } from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createFinanceShellState,
    isFinanceTenantScopedRoute,
    provideFinanceShellState,
  } from '../lib/finance/shell-state.svelte'
  import { themeStore, type ThemePreference } from '../lib/theme/theme-store.svelte'

  let { currentPath, children } = $props<{
    currentPath: string
    children?: Snippet
  }>()

  const financeShell = provideFinanceShellState(createFinanceShellState())

  const navLinks = [
    { href: '/finance', label: 'Dashboard' },
    { href: '/finance/transactions', label: 'Transactions' },
    { href: '/finance/accounts', label: 'Accounts' },
    { href: '/finance/categories', label: 'Categories' },
    { href: '/finance/connections', label: 'Connections & sync' },
    { href: '/finance/imports', label: 'Imports' },
    { href: '/finance/tenants', label: 'Tenants' },
  ]

  const themeOptions: {
    value: ThemePreference
    label: string
    icon: typeof Monitor
  }[] = [
    { value: 'auto', label: 'Auto', icon: Monitor },
    { value: 'light', label: 'Light', icon: Sun },
    { value: 'dark', label: 'Dark', icon: Moon },
  ]

  function normalizePath(path: string): string {
    const pathname = path.split('?')[0].replace(/\/+$/, '')
    return pathname || '/'
  }

  const currentPathname = $derived(normalizePath(currentPath))

  const showsTenantControl = $derived(
    isFinanceTenantScopedRoute(currentPathname) && financeShell.hasMultipleTenants,
  )

  const activeNavHref = $derived.by(() => {
    let activeHref = ''

    for (const item of navLinks) {
      if (
        currentPathname === item.href ||
        (item.href !== '/finance' && currentPathname.startsWith(`${item.href}/`))
      ) {
        if (item.href.length > activeHref.length) {
          activeHref = item.href
        }
      }
    }

    return activeHref
  })

  const currentSectionLabel = $derived.by(() => {
    if (currentPathname.startsWith('/finance/jobs/')) {
      return 'Jobs'
    }

    return navLinks.find((item) => item.href === activeNavHref)?.label ?? 'Workspace'
  })

  const breadcrumbItems = $derived.by(() => {
    const items = [{ label: 'Finance', href: '/finance' }]

    if (currentPathname === '/finance') {
      return [...items, { label: 'Dashboard', href: '' }].map((item, index, allItems) => ({
        ...item,
        current: index === allItems.length - 1,
      }))
    }

    if (!activeNavHref) {
      const fallbackLabel = currentPathname.startsWith('/finance/jobs/') ? 'Jobs' : 'Workspace'
      return [...items, { label: fallbackLabel, href: '' }].map((item, index, allItems) => ({
        ...item,
        current: index === allItems.length - 1,
      }))
    }

    items.push({ label: currentSectionLabel, href: activeNavHref })
    const isSectionPage = currentPathname === activeNavHref

    if (isSectionPage) {
      return items.map((item, index) => ({ ...item, current: index === items.length - 1 }))
    }

    const detailLabel = currentPathname.endsWith('/new')
      ? `Record ${currentSectionLabel.slice(0, -1).toLowerCase()}`
      : currentSectionLabel === 'Accounts'
        ? 'Account detail'
        : currentSectionLabel === 'Transactions'
          ? 'Transaction'
          : currentSectionLabel

    return [...items, { label: detailLabel, href: '' }].map((item, index, allItems) => ({
      ...item,
      current: index === allItems.length - 1,
    }))
  })

  onMount(() => {
    void financeShell.initialize()
  })

  function signOut(): void {
    authStore.clearAuth()
    replace('/login')
  }

  function onTenantChange(event: Event): void {
    financeShell.selectTenant((event.currentTarget as HTMLSelectElement).value)
  }

  function setThemePreference(preference: ThemePreference): void {
    themeStore.setPreference(preference)
  }
</script>

<div
  class="container-fluid px-0"
  data-bootstrap-finance-shell="true"
  data-bs-theme={themeStore.effective}
>
  <div class="row g-0 min-vh-100">
    <aside class="col-12 col-lg-4 col-xl-3 col-xxl-2 border-end bg-body-tertiary">
      <div class="d-flex h-100 flex-column gap-2 p-2 p-lg-3">
        <div>
          <a class="navbar-brand fw-semibold" href="/finance" use:link>Signal Foundry</a>
          <p class="d-none d-lg-block mb-0 small text-body-secondary">Finance</p>
        </div>

        <nav class="nav nav-pills flex-row flex-lg-column gap-2" aria-label="Finance navigation">
          {#each navLinks as item (item.href)}
            <a
              class="nav-link flex-grow-1 flex-lg-grow-0 px-2 px-lg-3 py-2 text-nowrap"
              class:active={activeNavHref === item.href}
              href={item.href}
              use:link
              aria-current={activeNavHref === item.href ? 'page' : undefined}
            >
              {item.label}
            </a>
          {/each}
        </nav>
      </div>
    </aside>

    <section class="col-12 col-lg-8 col-xl-9 col-xxl-10">
      <header class="border-bottom bg-body" aria-label="Finance utilities">
        <div class="d-flex flex-wrap justify-content-between align-items-center gap-2 p-2 p-lg-4">
          <nav class="finance-shell-breadcrumb-nav d-none d-sm-block" aria-label="Breadcrumb">
            <ol class="finance-shell-breadcrumb breadcrumb mb-0">
              {#each breadcrumbItems as item (item.href || item.label)}
                <li class="breadcrumb-item d-none d-sm-block" class:active={item.current} aria-current={item.current ? 'page' : undefined}>
                  {#if item.current}
                    {item.label}
                  {:else}
                    <a href={item.href} use:link>{item.label}</a>
                  {/if}
                </li>
              {/each}
            </ol>
          </nav>

          <div class="d-flex flex-wrap align-items-center gap-2 gap-md-3">
            {#if showsTenantControl}
              <label class="input-group input-group-sm w-auto">
                <span class="input-group-text">Tenant</span>
                <select
                  class="form-select form-select-sm"
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

            <div class="d-flex align-items-center gap-2">
              <span class="small text-body-secondary">Theme</span>
              <div class="btn-group btn-group-sm" role="radiogroup" aria-label="Theme">
                {#each themeOptions as option (option.value)}
                  {@const Icon = option.icon}
                  {@const checked = themeStore.preference === option.value}
                  <input
                    id={`finance-theme-${option.value}`}
                    class="btn-check"
                    type="radio"
                    name="finance-theme-preference"
                    checked={checked}
                    onchange={() => setThemePreference(option.value)}
                  />
                  <label
                    class="btn btn-outline-secondary d-inline-flex align-items-center justify-content-center"
                    class:active={checked}
                    for={`finance-theme-${option.value}`}
                    aria-label={option.label}
                    title={option.label}
                  >
                    <Icon size={14} strokeWidth={1.5} aria-hidden="true" />
                    <span class="visually-hidden">{option.label}</span>
                  </label>
                {/each}
              </div>
            </div>

            <button type="button" class="btn btn-outline-secondary btn-sm" onclick={signOut}>
              Sign out
            </button>
          </div>
        </div>
      </header>

      <div class="p-3 p-lg-4">
        {@render children?.()}
      </div>
    </section>
  </div>
</div>
