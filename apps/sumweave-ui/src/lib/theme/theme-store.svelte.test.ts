import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ThemeStore } from './theme-store.svelte'

describe('ThemeStore', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.removeAttribute('data-theme')
  })

  it('hydrates from storage and updates the dom theme', () => {
    const addEventListener = vi.fn()
    const removeEventListener = vi.fn()
    vi.stubGlobal('matchMedia', vi.fn(() => ({
      matches: false,
      addEventListener,
      removeEventListener,
    })))

    localStorage.setItem('sumweave-ui-theme', 'dark')
    const store = new ThemeStore()
    expect(store.preference).toBe('dark')
    expect(document.documentElement.dataset.theme).toBe('dark')

    localStorage.setItem('sumweave-ui-theme', 'auto')
    store.hydrateFromStorage()
    expect(store.preference).toBe('auto')
    expect(addEventListener).toHaveBeenCalled()

    store.setPreference('light')
    expect(document.documentElement.dataset.theme).toBe('light')
    expect(removeEventListener).toHaveBeenCalled()

    vi.unstubAllGlobals()
  })
})
