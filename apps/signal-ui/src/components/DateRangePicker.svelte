<script lang="ts">
  import {
    RANGE_PRESETS,
    dateInputValue,
    resolveRangePreset,
    timeInputValue,
    validateRange,
    withDateInput,
    withTimeInput,
    type RangePresetKey,
  } from '../lib/date-range'

  interface ConstraintProps {
    min?: Date | null
    max?: Date | null
    timeframeDurationMs?: number | null
    maxIntervals?: number | null
    requiredStartMessage?: string
    requiredEndMessage?: string
    invalidStartMessage?: string
    invalidEndMessage?: string
    notEarlierMessage?: string
    outOfBoundsMessage?: string
    maxIntervalsMessage?: string
  }

  interface Props extends ConstraintProps {
    startValue?: Date
    endValue?: Date
    disabled?: boolean
    presetAnchor?: Date | null
    showPresets?: boolean
    showValidation?: boolean
  }

  let {
    startValue = $bindable(),
    endValue = $bindable(),
    disabled = false,
    presetAnchor = null,
    min = null,
    max = null,
    timeframeDurationMs = null,
    maxIntervals = null,
    showPresets = true,
    showValidation = false,
    requiredStartMessage = 'Start is required.',
    requiredEndMessage = 'End is required.',
    invalidStartMessage = 'Start must be a valid timestamp.',
    invalidEndMessage = 'End must be a valid timestamp.',
    notEarlierMessage = 'Start must be earlier than end.',
    outOfBoundsMessage = undefined,
    maxIntervalsMessage = undefined,
  }: Props = $props()

  let lastPresetMessage = $state<string | null>(null)
  const validationMessages = $derived(
    validateRange({
      start: startValue,
      end: endValue,
      requiredStartMessage,
      requiredEndMessage,
      invalidStartMessage,
      invalidEndMessage,
      notEarlierMessage,
      min,
      max,
      outOfBoundsMessage,
      timeframeDurationMs,
      maxIntervals,
      maxIntervalsMessage,
    }),
  )

  function handleStartDateInput(value: string) {
    startValue = withDateInput(startValue, value)
    lastPresetMessage = null
  }

  function handleEndDateInput(value: string) {
    endValue = withDateInput(endValue, value)
    lastPresetMessage = null
  }

  function handleStartTimeInput(value: string) {
    startValue = withTimeInput(startValue, value)
    lastPresetMessage = null
  }

  function handleEndTimeInput(value: string) {
    endValue = withTimeInput(endValue, value)
    lastPresetMessage = null
  }

  function applyPreset(presetKey: RangePresetKey) {
    const nextRange = resolveRangePreset({ presetKey, anchor: presetAnchor, min, max })
    startValue = nextRange.start
    endValue = nextRange.end
    lastPresetMessage = nextRange.clamped
      ? 'Preset was clamped to the allowed range.'
      : `Preset resolved once from ${nextRange.end.toISOString()}.`
  }
</script>

<div class="date-range-picker">
  {#if showPresets}
    <div class="date-range-picker__presets" aria-label="Range presets">
      {#each RANGE_PRESETS as preset (preset.key)}
        <button class="secondary" type="button" disabled={disabled} onclick={() => applyPreset(preset.key)}>
          {preset.label}
        </button>
      {/each}
    </div>
  {/if}

  <div class="date-range-picker__inputs">
    <label>
      <span>Start date</span>
      <input type="date" value={dateInputValue(startValue)} disabled={disabled} oninput={(event) => handleStartDateInput(event.currentTarget.value)} />
    </label>
    <label>
      <span>Start time</span>
      <input type="time" step="1" value={timeInputValue(startValue)} disabled={disabled} oninput={(event) => handleStartTimeInput(event.currentTarget.value)} />
    </label>
    <label>
      <span>End date</span>
      <input type="date" value={dateInputValue(endValue)} disabled={disabled} oninput={(event) => handleEndDateInput(event.currentTarget.value)} />
    </label>
    <label>
      <span>End time</span>
      <input type="time" step="1" value={timeInputValue(endValue)} disabled={disabled} oninput={(event) => handleEndTimeInput(event.currentTarget.value)} />
    </label>
  </div>

  <div class="date-range-picker__resolved" aria-label="Resolved range">
    <p>Client-local range controls. End remains exclusive.</p>
    <dl>
      <div><dt>Start</dt><dd><code>{startValue?.toISOString() ?? '—'}</code></dd></div>
      <div><dt>End</dt><dd><code>{endValue?.toISOString() ?? '—'}</code></dd></div>
    </dl>
    {#if lastPresetMessage}<p>{lastPresetMessage}</p>{/if}
  </div>

  {#if showValidation && validationMessages.length > 0}
    <div class="alert" role="alert"><ul>{#each validationMessages as message (message)}<li>{message}</li>{/each}</ul></div>
  {/if}
</div>

<style>
  .date-range-picker, .date-range-picker__inputs, .date-range-picker__resolved { display: flex; flex-direction: column; gap: var(--space-16); }
  .date-range-picker__presets, .date-range-picker__inputs, .date-range-picker__resolved dl { display: grid; gap: var(--space-12); }
  .date-range-picker__presets { grid-template-columns: repeat(auto-fit, minmax(7rem, 1fr)); }
  .date-range-picker__inputs { grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr)); }
  .date-range-picker__resolved { padding: var(--space-16); border: 1px solid var(--border); border-radius: var(--radius-default); background: var(--surface-raised); color: var(--text-on-raised); }
  .date-range-picker__resolved dl { grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr)); margin: 0; }
  .date-range-picker__resolved dt { font-weight: 700; margin-bottom: var(--space-8); }
  .date-range-picker__resolved dd { margin: 0; word-break: break-word; }
  label { display: flex; flex-direction: column; gap: var(--space-8); font-weight: 500; }
  input { width: 100%; box-sizing: border-box; padding: var(--space-12); font: inherit; border-radius: var(--radius-input); border: 1px solid var(--border); background: var(--color-input-bg); color: var(--color-input-text); }
  ul { margin: 0; padding-left: var(--space-20); }
</style>
