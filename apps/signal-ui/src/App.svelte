<script lang="ts">
  import { onMount } from 'svelte'
  import Router, { replace, router } from 'svelte-spa-router'
  import { wrap } from 'svelte-spa-router/wrap'
  import Nav from './components/Nav.svelte'
  import { themeStore } from './lib/theme/theme-store.svelte'
  import {
    LOGIN_ROUTE,
    rememberCurrentPostLoginDestination,
  } from './lib/routing/post-login-destination'
  import Chat from './pages/Chat.svelte'
  import Data from './pages/Data.svelte'
  import EvaluationDetail from './pages/EvaluationDetail.svelte'
  import Evaluations from './pages/Evaluations.svelte'
  import Login from './pages/Login.svelte'
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
    '/jobs': wrap({
      component: Jobs,
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

  const isWideWorkspaceRoute = $derived(
    typeof router.location === 'string' &&
      (router.location.startsWith('/strategies') || router.location.startsWith('/evaluations')),
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
    {#if authStore.isAuthenticated}
      <Nav />
    {/if}
    <main class="main" class:main--chat={isChatRoute}>
      <!-- Inner column: DESIGN.md ~800–900px reading width; `/chat` uses full width (see `.main-inner--chat`). -->
      <div
        class="main-inner"
        class:main-inner--chat={isChatRoute}
        class:main-inner--wide={isWideWorkspaceRoute}
      >
        <Router {routes} onConditionsFailed={handleConditionsFailed} />
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
