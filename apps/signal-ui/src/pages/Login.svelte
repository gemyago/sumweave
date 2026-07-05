<script lang="ts">
  import { createLoginFormState } from '../lib/auth/login-form.svelte'

  const form = createLoginFormState()
</script>

<section class="page" aria-labelledby="login-heading">
  <h1 id="login-heading">Sign in</h1>

  <form class="form" onsubmit={(event) => form.submit(event)}>
    <div class="field">
      <label for="login-username">Username</label>
      <input
        id="login-username"
        type="text"
        autocomplete="username"
        bind:value={form.username}
        disabled={form.submitting}
        required
      />
    </div>

    <div class="field">
      <label for="login-password">Password</label>
      <input
        id="login-password"
        type="password"
        autocomplete="current-password"
        bind:value={form.password}
        disabled={form.submitting}
        required
      />
    </div>

    {#if form.error}
      <p class="error" role="alert">{form.error}</p>
    {/if}

    <button type="submit" class="primary" disabled={form.submitDisabled}>
      {form.submitting ? 'Signing in…' : 'Sign in'}
    </button>
  </form>
</section>

<style>
  .page {
    max-width: 24rem;
    margin-inline: auto;
  }

  h1 {
    font-size: var(--font-size-h1);
    font-weight: 700;
    color: var(--text-h);
    margin: 0 0 var(--space-24);
  }

  .form {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
  }

  label {
    font-size: var(--font-size-caption);
    line-height: 2;
    font-weight: 500;
    color: var(--text);
  }

  input {
    font: inherit;
    font-weight: 400;
    padding: var(--space-20);
    border-radius: var(--radius-input);
    border: 1px solid var(--border);
    background: var(--color-input-bg);
    color: var(--color-input-text);
  }

  input:focus-visible {
    outline: 1px solid var(--color-accent-blue);
    outline-offset: 0;
  }

  input:disabled {
    opacity: 0.55;
  }

  .error {
    margin: 0;
    padding: var(--space-8) var(--space-12);
    border-radius: var(--radius-default);
    background: var(--danger-bg);
    border: 1px solid var(--color-danger);
    color: var(--color-danger);
    font-size: var(--font-size-caption);
  }

  .primary {
    align-self: flex-start;
  }
</style>
