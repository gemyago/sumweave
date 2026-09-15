<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import AdminSubnav from '../components/AdminSubnav.svelte'
  import DocumentTitle from '../components/DocumentTitle.svelte'
  import { createAuthFetch } from '../lib/auth/auth-fetch'
  import { authStore } from '../lib/auth/auth-store.svelte'
  import { documentTitle } from '../lib/document-title'
  import {
    createAccessTokensApi,
    type AccessTokenIssuedResponse,
    type AccessTokenMetadata,
    type AccessTokenPermission,
  } from '../lib/auth/access-tokens-api'

  const accessTokensApi = createAccessTokensApi(createAuthFetch(authStore))
  let items = $state<AccessTokenMetadata[]>([])
  let loading = $state(true)
  let error = $state('')
  let name = $state('')
  let permission = $state<AccessTokenPermission>('read-only')
  let expiresAt = $state('')
  let submitting = $state(false)
  let issued = $state<AccessTokenIssuedResponse | null>(null)
  let confirming = $state<{ action: 'rotate' | 'revoke'; token: AccessTokenMetadata } | null>(null)
  let mutationError = $state('')
  let copied = $state(false)

  onMount(() => { void load() })
  onDestroy(() => { issued = null })

  async function load() {
    loading = true
    error = ''
    try {
      items = await accessTokensApi.list()
    } catch (cause) {
      error = messageFor(cause, 'Unable to load access tokens.')
    } finally {
      loading = false
    }
  }

  async function create() {
    mutationError = ''
    if (!name.trim()) { mutationError = 'Enter a token name.'; return }
    submitting = true
    try {
      issued = await accessTokensApi.create({ name, permission, expiresAt: expiresAt ? localDateTimeWithOffset(expiresAt) : null })
      items = [issued.token, ...items]
      name = ''
      expiresAt = ''
    } catch (cause) {
      mutationError = messageFor(cause, 'Unable to create the access token.')
    } finally {
      submitting = false
    }
  }

  async function confirm() {
    if (!confirming) return
    mutationError = ''
    submitting = true
    const { action, token } = confirming
    try {
      if (action === 'revoke') {
        await accessTokensApi.revoke(token.id)
        await load()
      } else {
        issued = await accessTokensApi.rotate(token.id, { expiresAt: token.expiresAt })
        items = [issued.token, ...items.filter((item) => item.id !== token.id)]
      }
      confirming = null
    } catch (cause) {
      mutationError = messageFor(cause, `Unable to ${action} the access token.`)
    } finally {
      submitting = false
    }
  }

  async function copyIssued() {
    if (!issued) return
    await navigator.clipboard.writeText(issued.apiToken)
    copied = true
  }

  function dismissIssued() { issued = null; copied = false }
  function messageFor(cause: unknown, fallback: string) { return cause instanceof Error ? cause.message : fallback }

  function localDateTimeWithOffset(value: string) {
    const date = new Date(value)
    const offset = -date.getTimezoneOffset()
    const sign = offset >= 0 ? '+' : '-'
    const hours = String(Math.floor(Math.abs(offset) / 60)).padStart(2, '0')
    const minutes = String(Math.abs(offset) % 60).padStart(2, '0')
    return `${value}:00${sign}${hours}:${minutes}`
  }
</script>

<DocumentTitle title={documentTitle('Access tokens', 'Admin')} />
<section class="page" aria-labelledby="access-tokens-heading">
  <header>
    <h1 id="access-tokens-heading">Access tokens</h1>
    <p class="muted">Create credentials for your own integrations. This is not an administrator role.</p>
  </header>
  <AdminSubnav current="/admin/access-tokens" />

  <section class="panel" aria-labelledby="create-access-token-heading">
    <h2 id="create-access-token-heading">Create token</h2>
    <form onsubmit={(event) => { event.preventDefault(); void create() }}>
      <label>Name <input bind:value={name} maxlength="100" disabled={submitting} /></label>
      <label>Permission <select bind:value={permission} disabled={submitting}><option value="read-only">Read only</option><option value="read-write">Read and write</option></select></label>
      <label>Expiry (optional) <input type="datetime-local" bind:value={expiresAt} disabled={submitting} /></label>
      <button class="primary" disabled={submitting}>{submitting ? 'Creating…' : 'Create token'}</button>
    </form>
    {#if mutationError}<p class="error" role="alert">{mutationError}</p>{/if}
  </section>

  {#if issued}
    <section class="panel issued" aria-labelledby="issued-access-token-heading">
      <h2 id="issued-access-token-heading">Save this token now</h2>
      <p class="warning">This value will not be shown again. Copy it before dismissing this message.</p>
      <code aria-label="Issued access token">{issued.apiToken}</code>
      <div class="actions"><button type="button" class="primary" onclick={() => void copyIssued()}>{copied ? 'Copied' : 'Copy token'}</button><button type="button" class="secondary" onclick={dismissIssued}>Dismiss</button></div>
    </section>
  {/if}

  {#if confirming}
    <section class="panel" aria-labelledby="confirm-access-token-action">
      <h2 id="confirm-access-token-action">Confirm {confirming.action}</h2>
      <p>{confirming.action === 'rotate' ? 'The current token stops working immediately and a replacement is issued once.' : 'This token stops working immediately.'}</p>
      <div class="actions"><button class:danger={confirming.action === 'revoke'} class="primary" disabled={submitting} onclick={() => void confirm()}>Confirm {confirming.action}</button><button type="button" class="secondary" disabled={submitting} onclick={() => { confirming = null }}>Cancel</button></div>
    </section>
  {/if}

  <section class="panel" aria-labelledby="your-access-tokens-heading" aria-busy={loading}>
    <h2 id="your-access-tokens-heading">Your access tokens</h2>
    {#if error}<p class="error" role="alert">{error} <button type="button" class="secondary" onclick={() => void load()}>Try again</button></p>
    {:else if loading}<p role="status">Loading access tokens…</p>
    {:else if items.length === 0}<p class="muted">You have not created an access token yet.</p>
    {:else}<div class="tokens">{#each items as token (token.id)}<article class="token"><div><strong>{token.name}</strong><span>{token.hint} · {token.permission} · {token.status}</span><span>Created {new Date(token.createdAt).toLocaleString()}</span>{#if token.expiresAt}<span>Expires {new Date(token.expiresAt).toLocaleString()}</span>{/if}</div>{#if token.status === 'active'}<div class="actions"><button type="button" class="secondary" onclick={() => { confirming = { action: 'rotate', token } }}>Rotate</button><button type="button" class="danger" onclick={() => { confirming = { action: 'revoke', token } }}>Revoke</button></div>{/if}</article>{/each}</div>{/if}
  </section>
</section>

<style>
  .page,.panel,form,.tokens,.token,.actions { display:flex; gap:var(--space-12) }
  .page,.panel,form,.tokens { flex-direction:column }
  .panel { padding:var(--space-16); border:1px solid var(--border); border-radius:4px; background:var(--bg-elevated,var(--bg)) }
  .page h1,.page h2,.page p { margin:0 }
  form label { display:grid; gap:var(--space-8); font-weight:500 }
  input,select { max-width:28rem; padding:var(--space-8); color:var(--text); background:var(--bg); border:1px solid var(--border); border-radius:4px; font:inherit }
  code { overflow-wrap:anywhere; padding:var(--space-12); border:1px solid var(--border); background:var(--bg) }
  .token { justify-content:space-between; align-items:flex-start; padding-block:var(--space-12); border-top:1px solid var(--border) }
  .token > div:first-child { display:grid; gap:var(--space-4) }
  .muted { color:var(--text-muted) }.error { color:var(--danger) }.warning { color:var(--warning) }
  @media (max-width: 640px) { .token { flex-direction:column }.actions { flex-wrap:wrap } }
</style>
