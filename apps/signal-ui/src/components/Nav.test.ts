import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import Nav from './Nav.svelte'

const mocks = vi.hoisted(() => ({ clearAuth: vi.fn(), replace: vi.fn() }))
vi.mock('../lib/auth/auth-store.svelte', () => ({ authStore: { clearAuth: mocks.clearAuth } }))
vi.mock('svelte-spa-router', async (importOriginal) => ({ ...(await importOriginal<typeof import('svelte-spa-router')>()), replace: mocks.replace }))

describe('Nav', () => {
  beforeEach(() => vi.clearAllMocks())

  it('keeps only finance, generic agent, provider, and admin exits', () => {
    render(Nav)
    expect(screen.getByRole('link', { name: 'Signal Foundry' })).toHaveAttribute('href', '#/finance')
    for (const [label, route] of [['Finance', '#/finance'], ['Chat', '#/chat'], ['Providers', '#/providers'], ['Admin', '#/admin']]) {
      expect(screen.getByRole('link', { name: label })).toHaveAttribute('href', route)
    }
    expect(screen.queryByRole('link', { name: /strategy|evaluation|data/i })).not.toBeInTheDocument()
  })

  it('clears authentication before replacing the current route with login', async () => {
    const user = userEvent.setup()
    render(Nav)
    await user.click(screen.getByRole('button', { name: 'Sign out' }))
    expect(mocks.clearAuth).toHaveBeenCalledOnce()
    expect(mocks.replace).toHaveBeenCalledWith('/login')
  })
})
