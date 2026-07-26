<script lang="ts">
  import { createLoginFormState } from '../lib/auth/login-form.svelte'
  import { documentTitle } from '../lib/document-title'
  import DocumentTitle from '../components/DocumentTitle.svelte'

  const form = createLoginFormState()
</script>

<DocumentTitle title={documentTitle('Sign in')} />

<section class="container py-5" aria-labelledby="login-heading">
  <div class="row justify-content-center">
    <div class="col-12 col-sm-10 col-md-8 col-lg-6 col-xl-5">
      <div class="card shadow-sm border-0">
        <div class="card-body p-4 p-lg-5">
          <h1 id="login-heading" class="h3 mb-3">Sign in</h1>

          <form class="d-grid gap-3" onsubmit={(event) => form.submit(event)}>
            <div>
              <label class="form-label" for="login-username">Username</label>
              <input
                id="login-username"
                class="form-control"
                type="text"
                autocomplete="username"
                bind:value={form.username}
                disabled={form.submitting}
                required
              />
            </div>

            <div>
              <label class="form-label" for="login-password">Password</label>
              <input
                id="login-password"
                class="form-control"
                type="password"
                autocomplete="current-password"
                bind:value={form.password}
                disabled={form.submitting}
                required
              />
            </div>

            {#if form.error}
              <div class="alert alert-danger mb-0" role="alert">{form.error}</div>
            {/if}

            <div class="d-grid">
              <button type="submit" class="btn btn-primary" disabled={form.submitDisabled}>
                {form.submitting ? 'Signing in…' : 'Sign in'}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </div>
</section>
