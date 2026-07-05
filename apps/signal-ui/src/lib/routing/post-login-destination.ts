export const DEFAULT_AUTHENTICATED_ROUTE = '/finance'
export const LOGIN_ROUTE = '/login'
export const POST_LOGIN_DESTINATION_KEY = 'signal-ui-post-login-destination'

const PROTECTED_ROUTE_PREFIXES = [
  '/chat',
  '/data',
  '/providers',
  '/jobs',
  '/finance',
  '/strategies',
  '/evaluations',
  '/admin',
]

export function routeFromHash(hash: string): string {
  if (hash === '' || hash === '#') {
    return '/'
  }

  const withoutHash = hash.startsWith('#') ? hash.slice(1) : hash
  if (withoutHash === '') {
    return '/'
  }

  return withoutHash.startsWith('/') ? withoutHash : `/${withoutHash}`
}

export function isProtectedRoute(route: string): boolean {
  return PROTECTED_ROUTE_PREFIXES.some(
    (prefix) => route === prefix || route.startsWith(`${prefix}/`),
  )
}

export function rememberPostLoginDestination(params: {
  route: string
  storage?: Storage
}): void {
  if (!isProtectedRoute(params.route)) {
    return
  }

  ;(params.storage ?? sessionStorage).setItem(POST_LOGIN_DESTINATION_KEY, params.route)
}

export function rememberCurrentPostLoginDestination(params?: {
  hash?: string
  storage?: Storage
}): void {
  rememberPostLoginDestination({
    route: routeFromHash(params?.hash ?? window.location.hash),
    storage: params?.storage,
  })
}

export function consumePostLoginDestination(storage: Storage = sessionStorage): string | null {
  const storedRoute = storage.getItem(POST_LOGIN_DESTINATION_KEY)
  storage.removeItem(POST_LOGIN_DESTINATION_KEY)

  if (!storedRoute || !isProtectedRoute(storedRoute)) {
    return null
  }

  return storedRoute
}

export function resolvePostLoginDestination(storage: Storage = sessionStorage): string {
  return consumePostLoginDestination(storage) ?? DEFAULT_AUTHENTICATED_ROUTE
}
