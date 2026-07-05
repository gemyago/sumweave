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
    provideFinanceShellState,
  } from '../lib/finance/shell-state.svelte'
  import { themeStore, type ThemePreference } from '../lib/theme/theme-store.svelte'

  let { currentPath, children } = $props<{
    currentPath: string
    children?: Snippet
  }>()

  const financeShell = provideFinanceShellState(createFinanceShellState())

  const navLinks = [
    { href: '/v2/finance', label: 'Overview' },
    { href: '/finance/accounts', label: 'Accounts' },
    { href: '/finance/transactions', label: 'Transactions' },
    { href: '/finance/connections', label: 'Connections' },
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

  const showsTenantControl = $derived(financeShell.hasMultipleTenants)

  onMount(() => {
    void financeShell.initialize()
  })

  function onTenantChange(event: Event): void {
    financeShell.selectTenant((event.currentTarget as HTMLSelectElement).value)
  }

  function setThemePreference(preference: ThemePreference): void {
    themeStore.setPreference(preference)
  }

  function signOut(): void {
    authStore.clearAuth()
    replace('/v2/login')
  }
</script>

<div
  class="container-fluid px-0"
  data-v2-finance-shell="true"
  data-bs-theme={themeStore.effective}
>
  <div class="row g-0 min-vh-100">
    <aside class="col-12 col-lg-4 col-xl-3 col-xxl-2 border-end bg-body-tertiary" aria-label="Finance navigation">
      <div class="d-flex h-100 flex-column gap-3 p-3">
        <div>
          <a class="navbar-brand fw-semibold" href="/v2/finance" use:link>Signal Foundry</a>
          <p class="mb-0 small text-body-secondary">Finance</p>
        </div>

        <nav class="nav nav-pills flex-column gap-2" aria-label="Finance destinations">
          {#each navLinks as item (item.href)}
            <a
              class="nav-link text-nowrap"
              class:active={currentPath === item.href}
              href={item.href}
              use:link
              aria-current={currentPath === item.href ? 'page' : undefined}
            >
              {item.label}
            </a>
          {/each}
        </nav>
      </div>
    </aside>

    <section class="col-12 col-lg-8 col-xl-9 col-xxl-10">
      <header class="border-bottom bg-body" aria-label="Finance utilities">
        <div class="d-flex flex-column gap-3 p-3 p-lg-4">
          <div class="d-flex flex-column flex-xl-row justify-content-between gap-3 align-items-xl-center">
            <p class="mb-0 text-uppercase small text-body-secondary fw-semibold">Finance</p>

            <div class="d-flex flex-wrap justify-content-xl-end align-items-center gap-2 gap-md-3">
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
                      id={`v2-theme-${option.value}`}
                      class="btn-check"
                      type="radio"
                      name="v2-theme-preference"
                      checked={checked}
                      onchange={() => setThemePreference(option.value)}
                    />
                    <label
                      class="btn btn-outline-secondary d-inline-flex align-items-center justify-content-center"
                      class:active={checked}
                      for={`v2-theme-${option.value}`}
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
        </div>
      </header>

      <div class="p-3 p-lg-4">
        {@render children?.()}
      </div>
    </section>
  </div>
</div>
