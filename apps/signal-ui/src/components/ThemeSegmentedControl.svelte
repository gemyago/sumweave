<script lang="ts">
  import Monitor from '@lucide/svelte/icons/monitor'
  import Moon from '@lucide/svelte/icons/moon'
  import Sun from '@lucide/svelte/icons/sun'
  import { themeStore, type ThemePreference } from '../lib/theme/theme-store.svelte'

  const themeOptions: {
    value: ThemePreference
    label: string
    icon: typeof Monitor
  }[] = [
    { value: 'auto', label: 'Auto', icon: Monitor },
    { value: 'light', label: 'Light', icon: Sun },
    { value: 'dark', label: 'Dark', icon: Moon },
  ]

  function setTheme(pref: ThemePreference): void {
    themeStore.setPreference(pref)
  }

  function onThemeKeydown(e: KeyboardEvent, index: number): void {
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return
    e.preventDefault()
    const delta = e.key === 'ArrowRight' ? 1 : -1
    const next = (index + delta + themeOptions.length) % themeOptions.length
    const pref = themeOptions[next].value
    setTheme(pref)
    queueMicrotask(() => {
      document.getElementById(`theme-opt-${pref}`)?.focus()
    })
  }
</script>

<div class="theme-seg" role="radiogroup" aria-label="Theme">
  {#each themeOptions as o, i (o.value)}
    {@const Icon = o.icon}
    {@const checked = themeStore.preference === o.value}
    <button
      id="theme-opt-{o.value}"
      type="button"
      class="theme-seg__btn"
      role="radio"
      aria-checked={checked}
      aria-label={o.label}
      tabindex={checked ? 0 : -1}
      title={o.label}
      onclick={() => setTheme(o.value)}
      onkeydown={(e) => onThemeKeydown(e, i)}
    >
      <Icon size={14} strokeWidth={1.5} aria-hidden="true" />
    </button>
  {/each}
</div>

<style>
  .theme-seg {
    display: flex;
    align-items: stretch;
    padding: 1px;
    border-radius: 999px;
    border: 1px solid var(--border);
    background: transparent;
    box-sizing: border-box;
    opacity: 0.92;
  }

  .theme-seg__btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 0;
    width: 1.75rem;
    height: 1.625rem;
    padding: 0;
    border: none;
    border-radius: 999px;
    color: var(--text-muted);
    background: transparent;
    cursor: pointer;
    transition:
      background 0.1s ease,
      color 0.1s ease,
      opacity 0.1s ease;
  }

  .theme-seg__btn + .theme-seg__btn {
    box-shadow: -1px 0 0 var(--border);
  }

  .theme-seg__btn[aria-checked='true'] {
    color: var(--text-h);
    background: var(--secondary-hover-bg);
  }

  .theme-seg__btn:hover:not([aria-checked='true']) {
    color: var(--text);
  }

  .theme-seg__btn:focus-visible {
    outline: 2px solid var(--color-accent-blue);
    outline-offset: 1px;
  }
</style>
