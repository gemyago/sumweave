<script lang="ts">
  import { onMount } from 'svelte'
  import Router, { replace, router } from 'svelte-spa-router'
  import { wrap } from 'svelte-spa-router/wrap'
  import Nav from './components/Nav.svelte'
  import FinanceShell from './components/FinanceShell.svelte'
  import { themeStore } from './lib/theme/theme-store.svelte'
  import {
    LOGIN_ROUTE,
    rememberCurrentPostLoginDestination,
  } from './lib/routing/post-login-destination'
  import Chat from './pages/Chat.svelte'
  import Data from './pages/Data.svelte'
  import Finance from './pages/Finance.svelte'
  import FinanceTenants from './pages/FinanceTenants.svelte'
  import FinanceAccounts from './pages/FinanceAccounts.svelte'
  import FinanceAccountDetail from './pages/FinanceAccountDetail.svelte'
  import FinanceTransactions from './pages/FinanceTransactions.svelte'
  import FinanceTransactionEditor from './pages/FinanceTransactionEditor.svelte'
  import FinanceCategories from './pages/FinanceCategories.svelte'
  import FinanceConnections from './pages/FinanceConnections.svelte'
  import FinanceSyntheticConnectionSetup from './pages/FinanceSyntheticConnectionSetup.svelte'
  import FinanceImports from './pages/FinanceImports.svelte'
  import FinanceJobDetail from './pages/FinanceJobDetail.svelte'
  import EvaluationDetail from './pages/EvaluationDetail.svelte'
  import Evaluations from './pages/Evaluations.svelte'
  import Login from './pages/Login.svelte'
  import Admin from './pages/Admin.svelte'
  import AdminJobs from './pages/AdminJobs.svelte'
  import AdminJobDetail from './pages/AdminJobDetail.svelte'
  import AdminFinanceFX from './pages/AdminFinanceFX.svelte'
  import AdminFinanceProviders from './pages/AdminFinanceProviders.svelte'
  import JobDetail from './pages/JobDetail.svelte'
  import Jobs from './pages/Jobs.svelte'
  import Providers from './pages/Providers.svelte'
  import RedirectToDefaultRoute from './pages/RedirectToDefaultRoute.svelte'
  import Strategies from './pages/Strategies.svelte'
  import { authStore } from './lib/auth/auth-store.svelte'

  const routes = {
    [LOGIN_ROUTE]: Login,
    '/': RedirectToDefaultRoute,
    '/chat/:sessionId?': wrap({
      component: Chat,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/providers': wrap({
      component: Providers,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/jobs/:jobId': wrap({
      component: JobDetail,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/finance/jobs/:jobId': wrap({
      component: FinanceJobDetail,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/jobs': wrap({
      component: Jobs,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/finance/accounts/:accountId': wrap({
      component: FinanceAccountDetail,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/finance/tenants': wrap({
      component: FinanceTenants,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/finance/accounts': wrap({
      component: FinanceAccounts,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/finance/connections/synthetic': wrap({
      component: FinanceSyntheticConnectionSetup,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/finance/connections': wrap({
      component: FinanceConnections,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/finance/transactions': wrap({
      component: FinanceTransactions,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/finance/transactions/new': wrap({
      component: FinanceTransactionEditor,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/finance/transactions/:transactionId': wrap({
      component: FinanceTransactionEditor,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/finance/categories': wrap({
      component: FinanceCategories,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/finance/imports': wrap({
      component: FinanceImports,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/finance': wrap({
      component: Finance,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/admin/jobs/:jobId': wrap({
      component: AdminJobDetail,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/admin/jobs': wrap({
      component: AdminJobs,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/admin/finance/fx': wrap({
      component: AdminFinanceFX,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/admin/finance/providers': wrap({
      component: AdminFinanceProviders,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/admin': wrap({
      component: Admin,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/strategies/:strategyId/:version': wrap({
      component: Strategies,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/strategies': wrap({
      component: Strategies,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/evaluations/run/:strategyId/:version': wrap({
      component: Evaluations,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/evaluations/:runId': wrap({
      component: EvaluationDetail,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/evaluations': wrap({
      component: Evaluations,
      conditions: [() => authStore.isAuthenticated],
    }),
    '/data': wrap({
      component: Data,
      conditions: [() => authStore.isAuthenticated],
    }),
  }

  function handleConditionsFailed() {
    rememberCurrentPostLoginDestination()
    replace(LOGIN_ROUTE)
  }

  onMount(async () => {
    await authStore.tryRestoreSession()
  })

  /** Full-viewport chat layout: sidebar flush left, no document scroll; see `.main--chat` / `body` class. */
  const isChatRoute = $derived(
    typeof router.location === 'string' && router.location.startsWith('/chat'),
  )

  const isFinanceRoute = $derived(
    typeof router.location === 'string' && router.location.startsWith('/finance'),
  )

  const showsFinanceShell = $derived(authStore.isAuthenticated && isFinanceRoute)

  const usesFinanceShell = $derived(showsFinanceShell)

  const isWideWorkspaceRoute = $derived(
    typeof router.location === 'string' &&
      (router.location.startsWith('/strategies') ||
        router.location.startsWith('/evaluations') ||
        router.location.startsWith('/finance') ||
        router.location.startsWith('/admin')),
  )

  $effect(() => {
    if (typeof document === 'undefined') return
    document.body.classList.toggle('chat-route-fullheight', isChatRoute)
  })
</script>

{#if authStore.restoring}
  <div class="shell loading" aria-busy="true" aria-label="Loading">
    <span class="loading-indicator">Loading…</span>
  </div>
{:else}
  <div class="shell" class:shell--chat={isChatRoute}>
    <span class="sr-only" aria-hidden="true">{themeStore.preference}</span>
    {#if authStore.isAuthenticated && !usesFinanceShell}
      <Nav />
    {/if}
    <main class="main" class:main--chat={isChatRoute} class:main--finance={usesFinanceShell}>
      <!-- Inner column: DESIGN.md ~800–900px reading width; `/chat` uses full width (see `.main-inner--chat`). -->
      <div
        class="main-inner"
        class:main-inner--chat={isChatRoute}
        class:main-inner--wide={isWideWorkspaceRoute}
        class:main-inner--finance={usesFinanceShell}
      >
        {#if showsFinanceShell}
          <FinanceShell currentPath={router.location}>
            <Router {routes} onConditionsFailed={handleConditionsFailed} />
          </FinanceShell>
        {:else}
          <Router {routes} onConditionsFailed={handleConditionsFailed} />
        {/if}
      </div>
    </main>
  </div>
{/if}

<style>
  .shell {
    min-height: 100vh;
    min-height: 100dvh;
    display: flex;
    flex-direction: column;
    background: var(--bg);
    color: var(--text);
  }

  /**
   * Chat: one screen below Nav — no page-level scroll; transcript scrolls inside Chat.
   */
  .shell--chat {
    height: 100dvh;
    max-height: 100dvh;
    min-height: 100dvh;
  }

  .main {
    flex: 1;
    display: flex;
    flex-direction: column;
    padding: var(--main-padding-block) var(--main-padding-inline);
    box-sizing: border-box;
    min-height: 0;
  }

  .main--chat {
    padding: 0;
    min-height: 0;
  }

  .main--finance {
    padding: 0;
    min-height: 0;
  }

  .main-inner {
    width: 100%;
    max-width: var(--content-max-width);
    margin-inline: auto;
    box-sizing: border-box;
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }

  .main-inner--chat {
    max-width: none;
    margin-inline: 0;
    flex: 1;
    min-height: 0;
  }

  .main-inner--wide {
    max-width: 1100px;
  }

  .main-inner--finance {
    max-width: none;
    margin-inline: 0;
  }

  .loading {
    align-items: center;
    justify-content: center;
    background: var(--bg);
    color: var(--text-h);
  }

  .loading-indicator {
    font-size: var(--font-size-body);
    font-weight: 500;
  }

  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  :global(body.chat-route-fullheight) {
    overflow: hidden;
  }
</style>
