<script lang="ts">
  import { authStore } from '../lib/auth/auth-store.svelte'
  import {
    createSignalFinanceApiForAuth,
    type FinanceProviderSnapshot,
    type FinanceProviderSnapshotMetadata,
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
  let metadata = $state<FinanceProviderSnapshotMetadata[] | null>(null)
  let loadingMetadata = $state(false)
  let error = $state<string | null>(null)
  let revealedSnapshot = $state<FinanceProviderSnapshot | null>(null)
  let revealingSnapshotId = $state<string | null>(null)
  let detailError = $state<string | null>(null)
  let failedSnapshotId = $state<string | null>(null)
  let sourceDataExpanded = $state(false)

  async function loadMetadata() {
    if (metadata || loadingMetadata) return

    loadingMetadata = true
    error = null
    try {
      metadata = scope === 'account'
        ? await financeApi.listAccountProviderSnapshots({ tenantId, accountId: entityId })
        : await financeApi.listTransactionProviderSnapshots({ tenantId, transactionId: entityId })
    } catch (loadError) {
      error = loadError instanceof Error ? loadError.message : 'Failed to load provider source data'
    } finally {
      loadingMetadata = false
    }
  }

  async function revealSnapshot(snapshotId: string) {
    revealingSnapshotId = snapshotId
    detailError = null
    failedSnapshotId = null
    try {
      revealedSnapshot = scope === 'account'
        ? await financeApi.getAccountProviderSnapshot({ tenantId, accountId: entityId, snapshotId })
        : await financeApi.getTransactionProviderSnapshot({ tenantId, transactionId: entityId, snapshotId })
    } catch (loadError) {
      detailError = loadError instanceof Error ? loadError.message : 'Failed to reveal provider source data'
      failedSnapshotId = snapshotId
    } finally {
      revealingSnapshotId = null
    }
  }

  function kindLabel(kind: string) {
    return kind.charAt(0).toUpperCase() + kind.slice(1).replaceAll('_', ' ')
  }
</script>

<details
  class={`border rounded p-1 ${sourceDataExpanded ? 'd-block w-100' : 'd-inline-block align-self-start'}`}
  ontoggle={(event) => {
    sourceDataExpanded = event.currentTarget.open
    if (event.currentTarget.open) void loadMetadata()
  }}
>
  <summary class="d-inline-flex align-items-center gap-2 fw-semibold p-3 px-md-2 py-md-2">
    <Database size={16} aria-hidden="true" />
    <span>Provider source data</span>
  </summary>
  <div class="d-grid gap-3 mt-3">
    <p class="small text-body-secondary mb-0">
      Each document is the latest schema-derived provider snapshot for its kind and provider object, not a raw HTTP response. Only current snapshots are shown.
    </p>

    {#if error}
      <div class="alert alert-danger d-flex flex-column flex-sm-row align-items-sm-center justify-content-between gap-2 mb-0" role="alert">
        <span>{error}</span>
        <button class="btn btn-outline-danger btn-sm align-self-start text-nowrap" type="button" onclick={() => void loadMetadata()}>Retry loading source data</button>
      </div>
    {/if}

    {#if loadingMetadata}
      <div class="text-body-secondary" role="status">Loading provider source data…</div>
    {:else if metadata?.length === 0}
      <div class="text-body-secondary" role="status">No provider source data is available for this {entityLabel}.</div>
    {:else if metadata}
      <div class="list-group">
        {#each metadata as item (item.id)}
          <div class="list-group-item d-grid gap-2">
            <div class="d-flex flex-column flex-md-row justify-content-between gap-2">
              <div>
                <strong>{kindLabel(item.kind)}</strong>
                <div class="small text-body-secondary text-break">Provider object {item.providerObjectId} · captured {formatFinanceDateTime(item.capturedAt)}</div>
              </div>
              <button class="btn btn-outline-secondary btn-sm align-self-start text-nowrap" type="button" onclick={() => void revealSnapshot(item.id)} disabled={revealingSnapshotId !== null}>
                {revealingSnapshotId === item.id ? 'Revealing…' : 'Reveal source data'}
              </button>
            </div>

            {#if detailError && failedSnapshotId === item.id}
              <div class="alert alert-danger d-flex flex-column flex-sm-row align-items-sm-center justify-content-between gap-2 mb-0" role="alert">
                <span>{detailError}</span>
                <button class="btn btn-outline-danger btn-sm align-self-start text-nowrap" type="button" onclick={() => void revealSnapshot(item.id)}>Retry reveal</button>
              </div>
            {/if}

            {#if revealedSnapshot?.id === item.id}
              <div class="alert alert-secondary mb-0">
                <strong>Provider snapshot data</strong>
                <div class="small mt-1">Sensitive credential-like fields are removed. This schema-derived document is the latest snapshot, not a raw HTTP response.</div>
                <pre class="mb-0 mt-2 text-wrap text-break">{JSON.stringify(revealedSnapshot.data ?? null, null, 2)}</pre>
              </div>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>
</details>
