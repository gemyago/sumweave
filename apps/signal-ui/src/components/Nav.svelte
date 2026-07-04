<script lang="ts">
  import {link, replace} from 'svelte-spa-router'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import ThemeSegmentedControl from './ThemeSegmentedControl.svelte'

  function signOut(): void {
    authStore.clearAuth()
    replace('/login')
  }
</script>

<nav class="app-nav" aria-label="Main">
  <a class="brand" href="/chat" use:link>Signal Foundry</a>
  <ul class="links">
    <li><a href="/chat" use:link>Chat</a></li>
    <li><a href="/data" use:link>Data</a></li>
    <li><a href="/jobs" use:link>Jobs</a></li>
    <li><a href="/finance" use:link>Finance</a></li>
    <li><a href="/providers" use:link>Providers</a></li>
    <li><a href="/strategies" use:link>Strategies</a></li>
    <li><a href="/evaluations" use:link>Evaluations</a></li>
    <li><a href="/admin" use:link>Admin</a></li>
  </ul>
  <div class="nav-end">
    <button type="button" class="sign-out" onclick={signOut}>Sign out</button>
    <ThemeSegmentedControl />
  </div>
</nav>

<style>
  .app-nav {
    display: grid;
    grid-template-columns:
      auto
      min(var(--content-max-width), calc(100% - 2 * var(--main-padding-inline)))
      minmax(0, 1fr);
    align-items: center;
    column-gap: var(--space-16);
    padding: var(--space-16) var(--main-padding-inline);
    border-bottom: 1px solid var(--border);
    background: var(--bg);
    box-sizing: border-box;
    /* Col 1 = brand (auto so it never collapses to 0 and overlaps links). Center track = .main-inner width. */
  }

  .links {
    grid-column: 2;
    justify-self: start;
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-20);
    list-style: none;
    margin: 0;
    padding: 0;
    min-width: 0;
  }

  .links a {
    font-weight: 500;
    font-size: var(--font-size-body);
    line-height: 1;
    color: var(--link);
    text-decoration: underline;
    text-underline-offset: 2px;
  }

  .links a:hover,
  .links a:focus-visible {
    color: var(--color-accent-blue);
  }

  .brand {
    grid-column: 1;
    justify-self: start;
    font-weight: 700;
    font-size: var(--font-size-body);
    line-height: 1.5;
    color: var(--link);
    text-decoration: underline;
    text-underline-offset: 2px;
  }

  .brand:hover,
  .brand:focus-visible {
    color: var(--color-accent-blue);
  }

  /* Right cluster: sign out + theme — pinned to the nav’s right inset */
  .nav-end {
    grid-column: 3;
    justify-self: end;
    display: flex;
    align-items: center;
    gap: var(--space-16);
    min-width: 0;
  }

  .sign-out {
    margin: 0;
    padding: 0;
    border: none;
    font: inherit;
    font-weight: 500;
    font-size: var(--font-size-body);
    line-height: 1;
    color: var(--link);
    text-decoration: underline;
    text-underline-offset: 2px;
    background: transparent;
    cursor: pointer;
  }

  .sign-out:hover,
  .sign-out:focus-visible {
    color: var(--color-accent-blue);
  }

  .sign-out:focus-visible {
    outline: 2px solid var(--color-accent-blue);
    outline-offset: 1px;
  }
  @media (max-width: 700px) {
    .app-nav {
      grid-template-columns: minmax(0, 1fr) auto;
      row-gap: var(--space-12);
    }

    .brand {
      grid-column: 1;
      grid-row: 1;
    }

    .nav-end {
      grid-column: 2;
      grid-row: 1;
    }

    .links {
      grid-column: 1 / -1;
      grid-row: 2;
      gap: var(--space-16);
    }
  }
</style>
