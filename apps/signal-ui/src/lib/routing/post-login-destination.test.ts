import { describe, expect, it } from 'vitest'
import {
  DEFAULT_AUTHENTICATED_ROUTE,
  POST_LOGIN_DESTINATION_KEY,
  consumePostLoginDestination,
  isProtectedRoute,
  rememberCurrentPostLoginDestination,
  rememberPostLoginDestination,
  resolvePostLoginDestination,
  routeFromHash,
} from './post-login-destination'

describe('post-login destination routing', () => {
  it('normalizes hash routes and detects protected paths', () => {
    expect(routeFromHash('')).toBe('/')
    expect(routeFromHash('#')).toBe('/')
    expect(routeFromHash('#/chat/example-session')).toBe('/chat/example-session')
    expect(isProtectedRoute('/chat/example-session')).toBe(true)
    expect(isProtectedRoute('/login')).toBe(false)
  })

  it('remembers only protected routes and falls back to data', () => {
    const storage = window.sessionStorage
    storage.clear()

    rememberPostLoginDestination({ route: '/login', storage })
    expect(storage.getItem(POST_LOGIN_DESTINATION_KEY)).toBeNull()

    rememberCurrentPostLoginDestination({ hash: '#/providers', storage })
    expect(storage.getItem(POST_LOGIN_DESTINATION_KEY)).toBe('/providers')
    expect(consumePostLoginDestination(storage)).toBe('/providers')
    expect(storage.getItem(POST_LOGIN_DESTINATION_KEY)).toBeNull()
    expect(resolvePostLoginDestination(storage)).toBe(DEFAULT_AUTHENTICATED_ROUTE)
  })
})
