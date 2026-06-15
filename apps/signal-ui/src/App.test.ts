import { describe, it, expect, beforeEach, vi } from 'vitest'
import { faker } from '@faker-js/faker'
import { render, screen } from '@testing-library/svelte'
import { waitFor } from '@testing-library/dom'
import userEvent from '@testing-library/user-event'
import App from './App.svelte'
import { POST_LOGIN_DESTINATION_KEY } from './lib/routing/post-login-destination'

// Mock the auth store so tests can control authentication state
const mocks = vi.hoisted(() => ({
  isAuthenticated: true,
  restoring: false,
  tryRestoreSession: vi.fn().mockResolvedValue(undefined),
  clearAuth: vi.fn(),
  accessToken: null as string | null,
  user: null as { id: string; username: string } | null,
}))

vi.mock('./lib/auth/auth-store.svelte', () => ({
  authStore: mocks,
}))

function navigateHash(hash: string) {
  window.location.hash = hash
  window.dispatchEvent(new HashChangeEvent('hashchange'))
}

describe('App shell', () => {
  beforeEach(() => {
    window.sessionStorage.clear()
    window.location.hash = ''
    mocks.isAuthenticated = true
    mocks.restoring = false
    mocks.tryRestoreSession.mockResolvedValue(undefined)
    mocks.accessToken = faker.string.alphanumeric(32)
  })

  it('shows Chat heading on initial load when authenticated', async () => {
    render(App)
    navigateHash('#/chat')
    expect(
      await screen.findByRole('heading', { name: 'Chat' }),
    ).toBeInTheDocument()
  })

  it('redirects the authenticated root route to data', async () => {
    window.location.hash = ''

    render(App)

    expect(
      await screen.findByRole('heading', { name: 'Historical data' }),
    ).toBeInTheDocument()
  })

  it('navigates to Providers when Providers link is clicked', async () => {
    const user = userEvent.setup()
    render(App)
    await user.click(screen.getByRole('link', { name: 'Providers' }))
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Providers' })).toBeInTheDocument()
    })
  })

  it('shows Chat, Data, and Providers navigation links when authenticated', async () => {
    render(App)

    expect(screen.getByRole('link', { name: 'Chat' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Data' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Providers' })).toBeInTheDocument()
  })

  it('renders the data browser route shell when authenticated', async () => {
    render(App)
    navigateHash('#/data')

    expect(
      await screen.findByRole('heading', { name: 'Historical data' }),
    ).toBeInTheDocument()
  })

  it('shows the message composer when the hash includes a session id', async () => {
    const sessionSlug = faker.string.uuid()
    render(App)
    navigateHash(`#/chat/${sessionSlug}`)
    expect(
      await screen.findByRole('textbox', { name: 'Message' }),
    ).toBeInTheDocument()
  })

  it('redirects to /login when navigating to protected route unauthenticated', async () => {
    mocks.isAuthenticated = false
    render(App)
    navigateHash('#/data')
    await waitFor(() => {
      expect(window.location.hash).toBe('#/login')
    })
    expect(window.sessionStorage.getItem(POST_LOGIN_DESTINATION_KEY)).toBe('/data')
  })

  it('preserves explicit protected deep links before redirecting to login', async () => {
    const sessionSlug = faker.string.uuid()
    mocks.isAuthenticated = false

    render(App)
    navigateHash(`#/chat/${sessionSlug}`)

    await waitFor(() => {
      expect(window.location.hash).toBe('#/login')
    })
    expect(window.sessionStorage.getItem(POST_LOGIN_DESTINATION_KEY)).toBe(
      `/chat/${sessionSlug}`,
    )
  })

  it('shows loading indicator while session is restoring', async () => {
    mocks.restoring = true
    render(App)
    expect(screen.getByLabelText('Loading')).toBeInTheDocument()
  })
})
