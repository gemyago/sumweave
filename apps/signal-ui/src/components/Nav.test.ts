import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import Nav from './Nav.svelte'
import { themeStore } from '../lib/theme/theme-store.svelte'

const mocks = vi.hoisted(() => ({
  clearAuth: vi.fn(),
  replace: vi.fn(),
}))

vi.mock('../lib/auth/auth-store.svelte', () => ({
  authStore: { clearAuth: mocks.clearAuth },
}))

vi.mock('svelte-spa-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('svelte-spa-router')>()
  return {
    ...actual,
    replace: mocks.replace,
  }
})

describe('Nav', () => {
  beforeEach(() => {
    mocks.clearAuth.mockClear()
    mocks.replace.mockClear()
    localStorage.clear()
    themeStore.setPreference('auto')
  })

  it('sign out clears auth and replaces route with /login', async () => {
    const user = userEvent.setup()
    render(Nav)

    await user.click(screen.getByRole('button', { name: 'Sign out' }))

    expect(mocks.clearAuth).toHaveBeenCalledTimes(1)
    expect(mocks.replace).toHaveBeenCalledWith('/login')
  })

  it('renders protected workspace links for authenticated navigation', () => {
    render(Nav)

    expect(screen.getByRole('link', { name: 'Chat' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Data' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Providers' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Strategies' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Evaluations' })).toBeInTheDocument()
  })

  it('lets the user switch the theme selection by click', async () => {
    const user = userEvent.setup()
    render(Nav)

    await user.click(screen.getByRole('radio', { name: 'Light' }))

    expect(screen.getByRole('radio', { name: 'Light' })).toHaveAttribute('aria-checked', 'true')
  })

  it('supports arrow-key theme navigation and ignores unrelated keys', async () => {
    const user = userEvent.setup()
    render(Nav)

    const auto = screen.getByRole('radio', { name: 'Auto' })
    auto.focus()
    await user.keyboard('{ArrowRight}')
    expect(screen.getByRole('radio', { name: 'Light' })).toHaveAttribute('aria-checked', 'true')

    await user.keyboard('{Escape}')
    expect(screen.getByRole('radio', { name: 'Light' })).toHaveAttribute('aria-checked', 'true')
  })
})
