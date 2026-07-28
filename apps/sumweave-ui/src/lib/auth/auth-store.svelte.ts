import { refreshApi, meApi, type AuthUser } from './auth-api'

const REFRESH_TOKEN_KEY = 'auth_refresh_token'

export class AuthStore {
  accessToken = $state<string | null>(null)
  user = $state<AuthUser | null>(null)
  /** True until the initial refresh-token restoration attempt has completed. */
  restoring = $state(true)

  get isAuthenticated(): boolean {
    return this.accessToken !== null
  }

  setAuth(accessToken: string, refreshToken: string, user: AuthUser): void {
    this.accessToken = accessToken
    this.user = user
    localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken)
  }

  clearAuth(): void {
    this.accessToken = null
    this.user = null
    localStorage.removeItem(REFRESH_TOKEN_KEY)
  }

  async tryRestoreSession(): Promise<void> {
    const refreshToken = localStorage.getItem(REFRESH_TOKEN_KEY)
    if (!refreshToken) {
      this.restoring = false
      return
    }
    try {
      const res = await refreshApi({ refreshToken })
      this.setAuth(res.accessToken, res.refreshToken, res.user)
    } catch {
      this.clearAuth()
    } finally {
      this.restoring = false
    }
  }
}

export const authStore = new AuthStore()

export { meApi }
