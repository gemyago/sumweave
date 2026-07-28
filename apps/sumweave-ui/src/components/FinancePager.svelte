<script lang="ts">
  import { tick } from 'svelte'

  let {
    label,
    status,
    controls,
    busy = false,
    hasPrevious = false,
    hasNext = false,
    onPrevious,
    onNext,
  }: {
    label: string
    status: string
    controls: string
    busy?: boolean
    hasPrevious?: boolean
    hasNext?: boolean
    onPrevious: () => boolean | Promise<boolean>
    onNext: () => boolean | Promise<boolean>
  } = $props()

  let pager = $state<HTMLElement | undefined>(undefined)

  async function changePage(action: () => boolean | Promise<boolean>) {
    const priorTop = pager?.getBoundingClientRect().top
    if (!await action()) return

    await tick()
    if (!pager) return

    if (priorTop !== undefined) {
      const delta = pager.getBoundingClientRect().top - priorTop
      if (delta !== 0) window.scrollBy(0, delta)
    }
    pager.focus({ preventScroll: true })
  }
</script>

<nav class="d-flex flex-column flex-md-row justify-content-between gap-2 align-items-md-center" aria-label={label} aria-busy={busy} tabindex="-1" bind:this={pager}>
  <span class="text-body-secondary small" role="status" aria-live="polite">{status}</span>
  <div class="btn-group" role="group" aria-label={`${label} controls`}>
    <button class="btn btn-outline-secondary btn-sm" type="button" aria-label={`${label}: previous page`} aria-controls={controls} aria-busy={busy} onclick={() => void changePage(onPrevious)} disabled={busy || !hasPrevious}>Previous</button>
    <button class="btn btn-outline-secondary btn-sm" type="button" aria-label={`${label}: next page`} aria-controls={controls} aria-busy={busy} onclick={() => void changePage(onNext)} disabled={busy || !hasNext}>Next</button>
  </div>
</nav>
