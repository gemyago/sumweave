<script lang="ts">
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalFinanceApiForAuth,
    type FinanceProviderEvidence,
    type FinanceProviderEvidenceMetadata,
  } from '../lib/finance/api'
  import { formatFinanceDateTime } from '../lib/finance/format'
  import Database from '@lucide/svelte/icons/database'

  let {
    tenantId,
    entityId,
    entityLabel,
    scope,
  } = $props<{
    tenantId: string
    entityId: string
    entityLabel: string
    scope: 'account' | 'transaction'
  }>()

  const appBaseUrl = import.meta.env.VITE_APP_API_BASE_URL ?? '/api/v1'
  const financeApi = $derived.by(() => createSignalFinanceApiForAuth({ baseUrl: appBaseUrl, authStore }))
  let metadata = $state<FinanceProviderEvidenceMetadata[] | null>(null)
  let loadingMetadata = $state(false)
  let error = $state<string | null>(null)
  let revealedEvidence = $state<FinanceProviderEvidence | null>(null)
  let revealingEvidenceId = $state<string | null>(null)
  let evidenceExpanded = $state(false)

  async function loadMetadata() {
    if (metadata || loadingMetadata) return

    loadingMetadata = true
    error = null
    try {
      metadata = scope === 'account'
        ? await financeApi.listAccountProviderEvidence({ tenantId, accountId: entityId })
        : await financeApi.listTransactionProviderEvidence({ tenantId, transactionId: entityId })
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load provider evidence metadata'
    } finally {
      loadingMetadata = false
    }
  }

  async function revealEvidence(evidenceId: string) {
    revealingEvidenceId = evidenceId
    error = null
    try {
      revealedEvidence = scope === 'account'
        ? await financeApi.getAccountProviderEvidence({ tenantId, accountId: entityId, evidenceId })
        : await financeApi.getTransactionProviderEvidence({ tenantId, transactionId: entityId, evidenceId })
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to reveal sanitized provider evidence'
    } finally {
      revealingEvidenceId = null
    }
  }
</script>

<details
  class={`border rounded p-1 ${evidenceExpanded ? 'd-block w-100' : 'd-inline-block align-self-start'}`}
  ontoggle={(event) => {
    evidenceExpanded = event.currentTarget.open
    if (event.currentTarget.open) void loadMetadata()
  }}
>
  <summary class="d-inline-flex align-items-center gap-2 fw-semibold p-3 px-md-2 py-md-2" aria-label="Current provider evidence" title="Current provider evidence">
    <Database size={16} aria-hidden="true" />
    <span class="visually-hidden">Current provider evidence</span>
  </summary>
  <div class="d-grid gap-3 mt-3">
    <p class="small text-body-secondary mb-0">
      Metadata loads only when opened. Each row is the latest sanitized observation for a current provider object, not raw provider payload data.
    </p>

    {#if error}
      <div class="alert alert-danger mb-0" role="alert">{error}</div>
    {/if}

    {#if loadingMetadata}
      <div class="text-body-secondary" role="status">Loading current provider evidence…</div>
    {:else if metadata?.length === 0}
      <div class="text-body-secondary" role="status">No current provider evidence is available for this {entityLabel}.</div>
    {:else if metadata}
      <div class="list-group">
        {#each metadata as item (item.id)}
          <div class="list-group-item d-grid gap-2">
            <div class="d-flex flex-column flex-md-row justify-content-between gap-2">
              <div>
                <strong>Current {item.scope} evidence</strong>
                <div class="small text-body-secondary">Provider object {item.providerObjectId} · latest captured {formatFinanceDateTime(item.capturedAt)}</div>
              </div>
              <button class="btn btn-outline-secondary btn-sm align-self-start" type="button" onclick={() => void revealEvidence(item.id)} disabled={revealingEvidenceId !== null}>
                {revealingEvidenceId === item.id ? 'Revealing…' : 'Reveal current sanitized details'}
              </button>
            </div>

            {#if revealedEvidence?.id === item.id}
              <div class="alert alert-secondary mb-0">
                <strong>Current sanitized provider evidence</strong>
                <div class="small mt-1">Sensitive credential-like fields are removed. This is the latest observation for this provider object, not the raw provider payload.</div>
                <pre class="mb-0 mt-2 text-wrap">{JSON.stringify(revealedEvidence.payload ?? null, null, 2)}</pre>
              </div>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>
</details>
