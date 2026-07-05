import { push } from 'svelte-spa-router'
import { loginApi } from './auth-api'
import { authStore } from './auth-store.svelte'
import { resolvePostLoginDestination } from '../routing/post-login-destination'

export class LoginFormState {
  username = $state('')
  password = $state('')
  submitting = $state(false)
  error = $state<string | null>(null)

  get submitDisabled(): boolean {
    return this.submitting || this.username === '' || this.password === ''
  }

  async submit(event: Event): Promise<void> {
    event.preventDefault()
    this.error = null
    this.submitting = true

    try {
      const res = await loginApi({ username: this.username, password: this.password })
      authStore.setAuth(res.accessToken, res.refreshToken, res.user)
      push(resolvePostLoginDestination())
    } catch {
      this.error = 'Invalid username or password.'
    } finally {
      this.submitting = false
    }
  }
}

export function createLoginFormState(): LoginFormState {
  return new LoginFormState()
}
