<script lang="ts">
  import Monitor from '@lucide/svelte/icons/monitor'
  import Moon from '@lucide/svelte/icons/moon'
  import Sun from '@lucide/svelte/icons/sun'
  import UserRound from '@lucide/svelte/icons/user-round'
  import type { Snippet } from 'svelte'
  import { onMount, tick } from 'svelte'
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
  let menuOpen = $state(false)
  let navigationOpen = $state(false)
  let navigationButton: HTMLButtonElement
  let menuContainer: HTMLDivElement
  let menuButton: HTMLButtonElement
  const activeTenantName = $derived(
    financeShell.tenants.find((tenant) => tenant.id === financeShell.selectedTenantId)?.name,
  )

  $effect(() => {
    void currentPath
    menuOpen = false
    navigationOpen = false
  })

  async function toggleMenu(): Promise<void> {
    menuOpen = !menuOpen
    if (menuOpen) {
      await tick()
      menuContainer.querySelector<HTMLElement>('select, .dropdown-item')?.focus()
    }
  }

  function dismissMenu(event: PointerEvent): void {
    if (menuOpen && !menuContainer.contains(event.target as Node)) menuOpen = false
  }

  function onEscape(event: KeyboardEvent): void {
    if (menuOpen && event.key === 'Escape') {
      event.preventDefault()
      menuOpen = false
      menuButton.focus()
    } else if (navigationOpen && event.key === 'Escape') {
      event.preventDefault()
      navigationOpen = false
      navigationButton.focus()
    }
  }

  function closeAfterNavigation(): void {
    menuOpen = false
    menuButton.focus()
  }

  function onMenuFocusOut(event: FocusEvent): void {
    if (event.relatedTarget && !menuContainer.contains(event.relatedTarget as Node)) {
      menuOpen = false
    }
  }

  const navLinks = [
    { href: '/finance', label: 'Dashboard' },
    { href: '/finance/transactions', label: 'Transactions' },
    { href: '/finance/accounts', label: 'Accounts' },
    { href: '/finance/categories', label: 'Categories' },
    { href: '/finance/rules', label: 'Rules' },
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

    const detailLabel = currentPathname === '/finance/connections/synthetic'
      ? 'Synthetic setup'
      : currentPathname.endsWith('/new')
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

<svelte:window onpointerdown={dismissMenu} onkeydown={onEscape} />

<div
  class="container-fluid px-0"
  data-bootstrap-finance-shell="true"
  data-bs-theme={themeStore.effective}
>
  <div class="min-vh-100">
      <header class="navbar navbar-expand-sm border-bottom bg-body px-2 px-lg-4 py-2" aria-label="Finance utilities">
          <a class="navbar-brand fw-semibold fs-6 me-2" href="/finance" use:link>Sumweave</a>
          <button bind:this={navigationButton} type="button" class="navbar-toggler me-auto finance-primary-nav-target" aria-label="Toggle Finance navigation" aria-expanded={navigationOpen} aria-controls="finance-primary-navigation" onclick={() => { navigationOpen = !navigationOpen }}>
            <span class="navbar-toggler-icon"></span>
          </button>
          <nav id="finance-primary-navigation" class="collapse navbar-collapse order-3 order-sm-0" class:show={navigationOpen} aria-label="Finance navigation">
            <div class="navbar-nav flex-row small">
              {#each navLinks.slice(0, 3) as item (item.href)}
                <a class="nav-link px-2 py-2 text-nowrap finance-primary-nav-target" class:active={activeNavHref === item.href} href={item.href} use:link aria-current={activeNavHref === item.href ? 'page' : undefined} onclick={() => { navigationOpen = false }}>{item.label}</a>
              {/each}
            </div>
          </nav>
          <div class="dropdown ms-auto" bind:this={menuContainer} onfocusout={onMenuFocusOut}>
            <button
              bind:this={menuButton}
              type="button"
              class="btn btn-outline-secondary btn-sm d-flex align-items-center gap-2"
              aria-label="User menu"
              aria-expanded={menuOpen}
              aria-controls="finance-user-menu"
              onclick={toggleMenu}
            >
              <UserRound size={20} aria-hidden="true" />
            </button>
            {#if menuOpen}
              <div id="finance-user-menu" class="finance-shell-user-menu dropdown-menu dropdown-menu-end show" aria-label="Workspace and preferences">
                <div class="px-3 py-2">
                  {#if showsTenantControl}
                    <label for="finance-active-tenant" class="form-label small">Active tenant</label>
                    <select id="finance-active-tenant" class="form-select form-select-sm" value={financeShell.selectedTenantId} onchange={onTenantChange} disabled={financeShell.loading}>
                      <option value="">{financeShell.hasTenants ? 'Select tenant' : 'No tenants yet'}</option>
                      {#each financeShell.tenants as tenant (tenant.id)}
                        <option value={tenant.id}>{tenant.name} · {tenant.displayCurrency}</option>
                      {/each}
                    </select>
                  {:else}
                    <span class="small text-body-secondary">Active tenant</span>
                    <div class="fw-semibold text-break">{activeTenantName ?? (financeShell.loading ? 'Loading tenants…' : 'No tenant selected')}</div>
                  {/if}
                </div>
                <hr class="dropdown-divider" />
                <h2 class="dropdown-header text-body-secondary">Finance setup</h2>
                {#each navLinks.slice(3, 7) as item (item.href)}
                  <a class="dropdown-item" class:active={activeNavHref === item.href} href={item.href} use:link aria-current={activeNavHref === item.href ? 'page' : undefined} onclick={closeAfterNavigation}>{item.label}</a>
                {/each}
                <hr class="dropdown-divider" />
                <h2 class="dropdown-header text-body-secondary">Workspace</h2>
                <a class="dropdown-item" class:active={activeNavHref === '/finance/tenants'} href="/finance/tenants" use:link aria-current={activeNavHref === '/finance/tenants' ? 'page' : undefined} onclick={closeAfterNavigation}>Tenants</a>
                <hr class="dropdown-divider" />
                <h2 class="dropdown-header text-body-secondary">Preferences</h2>
                <div class="d-flex align-items-center justify-content-between gap-2 px-3 py-2">
                  <span class="small">Theme</span>
                  <div class="btn-group btn-group-sm" role="radiogroup" aria-label="Theme">
                    {#each themeOptions as option (option.value)}
                      {@const Icon = option.icon}
                      {@const checked = themeStore.preference === option.value}
                      <input id={`finance-theme-${option.value}`} class="btn-check" type="radio" name="finance-theme-preference" checked={checked} onchange={() => setThemePreference(option.value)} />
                      <label class="btn btn-outline-secondary d-inline-flex align-items-center justify-content-center" class:active={checked} for={`finance-theme-${option.value}`} title={option.label}>
                        <Icon size={14} strokeWidth={1.5} aria-hidden="true" />
                        <span class="visually-hidden">{option.label}</span>
                      </label>
                    {/each}
                  </div>
                </div>
                <button type="button" class="dropdown-item" onclick={signOut}>Sign out</button>
              </div>
            {/if}
          </div>
      </header>
    <section>
          <nav class="finance-shell-breadcrumb-nav px-3 px-lg-4 pt-2" class:d-none={currentPathname === activeNavHref} class:d-sm-block={currentPathname === activeNavHref} aria-label="Breadcrumb">
            <ol class="finance-shell-breadcrumb breadcrumb mb-0">
              {#each breadcrumbItems as item (item.href || item.label)}
                 <li class="breadcrumb-item" class:active={item.current} aria-current={item.current ? 'page' : undefined}>
                  {#if item.current}
                    {item.label}
                  {:else}
                    <a href={item.href} use:link>{item.label}</a>
                  {/if}
                </li>
              {/each}
            </ol>
          </nav>

      <div class="p-3 p-lg-4">
        {@render children?.()}
      </div>
    </section>
  </div>
</div>
