import { describe, it, expect, beforeEach, vi } from 'vitest'
import { faker } from '@faker-js/faker'
import { render, screen, waitFor } from '@testing-library/svelte'
import userEvent from '@testing-library/user-event'
import Login from './Login.svelte'
import LoginSource from './Login.svelte?raw'
import {
  POST_LOGIN_DESTINATION_KEY,
  rememberPostLoginDestination,
} from '../lib/routing/post-login-destination'

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  loginApi: vi.fn(),
  setAuth: vi.fn(),
}))

vi.mock('svelte-spa-router', () => ({
  push: mocks.push,
}))

vi.mock('../lib/auth/auth-api', () => ({
  loginApi: mocks.loginApi,
}))

vi.mock('../lib/auth/auth-store.svelte', () => ({
  authStore: { setAuth: mocks.setAuth },
}))

function makeLoginResponse() {
  return {
    accessToken: faker.string.alphanumeric(32),
    refreshToken: faker.string.alphanumeric(48),
    user: { id: faker.string.uuid(), username: faker.internet.username() },
  }
}

describe('Login', () => {
  beforeEach(() => {
    window.sessionStorage.clear()
    mocks.push.mockReset()
    mocks.loginApi.mockReset()
    mocks.setAuth.mockReset()
  })

  it('renders the canonical bootstrap login form without pilot naming', () => {
    render(Login)

    expect(screen.getByLabelText('Username')).toBeInTheDocument()
    expect(screen.getByLabelText('Password')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Sign in' })).toBeInTheDocument()
    expect(screen.getByLabelText('Username')).toHaveClass('form-control')
    expect(screen.getByLabelText('Password')).toHaveClass('form-control')
    expect(screen.getByRole('button', { name: 'Sign in' })).toHaveClass('btn', 'btn-primary')
    expect(screen.queryByText('Bootstrap pilot')).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Use canonical login' })).not.toBeInTheDocument()
  })

  it('submit is disabled when fields are empty', () => {
    render(Login)
    expect(screen.getByRole('button', { name: 'Sign in' })).toBeDisabled()
  })

  it('on successful login, calls setAuth and redirects to /finance by default', async () => {
    const user = userEvent.setup()
    const response = makeLoginResponse()
    mocks.loginApi.mockResolvedValue(response)

    render(Login)

    await user.type(screen.getByLabelText('Username'), faker.internet.username())
    await user.type(screen.getByLabelText('Password'), faker.internet.password())
    await user.click(screen.getByRole('button', { name: 'Sign in' }))

    await waitFor(() => {
      expect(mocks.setAuth).toHaveBeenCalledWith(
        response.accessToken,
        response.refreshToken,
        response.user,
      )
      expect(mocks.push).toHaveBeenCalledWith('/finance')
    })
  })

  it('returns to an explicitly requested protected route after login', async () => {
    const user = userEvent.setup()
    const response = makeLoginResponse()
    const requestedRoute = `/chat/${faker.string.uuid()}`
    mocks.loginApi.mockResolvedValue(response)
    rememberPostLoginDestination({ route: requestedRoute })

    render(Login)

    await user.type(screen.getByLabelText('Username'), faker.internet.username())
    await user.type(screen.getByLabelText('Password'), faker.internet.password())
    await user.click(screen.getByRole('button', { name: 'Sign in' }))

    await waitFor(() => {
      expect(mocks.push).toHaveBeenCalledWith(requestedRoute)
    })
    expect(window.sessionStorage.getItem(POST_LOGIN_DESTINATION_KEY)).toBeNull()
  })

  it('on failed login, shows error alert without redirecting', async () => {
    const user = userEvent.setup()
    mocks.loginApi.mockRejectedValue(new Error('Login failed: 401'))

    render(Login)

    await user.type(screen.getByLabelText('Username'), faker.internet.username())
    await user.type(screen.getByLabelText('Password'), faker.internet.password())
    await user.click(screen.getByRole('button', { name: 'Sign in' }))

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Invalid username or password.')
    })
    expect(mocks.push).not.toHaveBeenCalled()
  })

  it('shows loading state and keeps fields disabled while submit is in flight', async () => {
    const user = userEvent.setup()
    let resolveLogin: ((value: ReturnType<typeof makeLoginResponse>) => void) | undefined
    mocks.loginApi.mockReturnValue(
      new Promise((resolve) => {
        resolveLogin = resolve
      }),
    )

    render(Login)

    await user.type(screen.getByLabelText('Username'), faker.internet.username())
    await user.type(screen.getByLabelText('Password'), faker.internet.password())
    await user.click(screen.getByRole('button', { name: 'Sign in' }))

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Signing in…' })).toBeDisabled()
    })
    expect(screen.getByLabelText('Username')).toBeDisabled()
    expect(screen.getByLabelText('Password')).toBeDisabled()

    resolveLogin?.(makeLoginResponse())

    await waitFor(() => {
      expect(mocks.push).toHaveBeenCalledWith('/finance')
    })
  })

  it('does not define a route-local style block', () => {
    expect(LoginSource).not.toMatch(/<style[\s>]/)
  })
})
