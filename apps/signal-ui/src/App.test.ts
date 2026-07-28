import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import App from './App.svelte'
import { authStore } from './lib/auth/auth-store.svelte'

function setRoute(route: string): void {
  window.history.replaceState({}, '', `/#${route}`)
  window.dispatchEvent(new HashChangeEvent('hashchange'))
}

describe('application route shell', () => {
  beforeEach(() => {
    window.localStorage.clear()
    authStore.clearAuth()
    authStore.restoring = false
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({ items: [], models: [], profiles: [], sessions: [] }), { status: 200 })))
  })

  afterEach(() => vi.unstubAllGlobals())

  it('renders the public login route without authenticated navigation', async () => {
    setRoute('/login')
    render(App)

    expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeInTheDocument()
    expect(screen.queryByLabelText('Main')).not.toBeInTheDocument()
  })

  it('shows the restoration state before route content is available', () => {
    const restore = vi.spyOn(authStore, 'tryRestoreSession').mockImplementation(() => new Promise(() => {}))
    authStore.restoring = true
    setRoute('/login')
    render(App)

    expect(screen.getByLabelText('Loading')).toBeInTheDocument()
    restore.mockRestore()
  })

  it('redirects an unauthenticated protected route to login', async () => {
    setRoute('/admin')
    render(App)

    expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeInTheDocument()
  })

  it('uses the retained generic navigation for an authenticated admin route', async () => {
    authStore.accessToken = 'token'
    authStore.user = { id: 'user-1', username: 'casey' }
    setRoute('/admin')
    render(App)

    expect(await screen.findByRole('heading', { name: 'Admin' })).toBeInTheDocument()
    expect(screen.getByLabelText('Main')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Finance' })).toHaveAttribute('href', '#/finance')
  })

  it('uses the dedicated finance shell rather than generic navigation for finance routes', async () => {
    authStore.accessToken = 'token'
    authStore.user = { id: 'user-1', username: 'casey' }
    setRoute('/finance')
    render(App)

    expect(await screen.findByLabelText('Finance navigation')).toBeInTheDocument()
    expect(screen.queryByLabelText('Main')).not.toBeInTheDocument()
  })

  it('uses the full-height generic shell for the retained chat route', async () => {
    authStore.accessToken = 'token'
    authStore.user = { id: 'user-1', username: 'casey' }
    setRoute('/chat')
    const { container } = render(App)

    expect(await screen.findByRole('heading', { name: 'Chat' })).toBeInTheDocument()
    expect(container.querySelector('.shell--chat')).toBeInTheDocument()
    expect(document.body).toHaveClass('chat-route-fullheight')
  })
})
