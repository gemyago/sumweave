<script lang="ts">
  import { DatePicker, useDatePicker, type DatePickerValueChangeDetails } from '@ark-ui/svelte'
  import { CalendarDate } from '@internationalized/date'
  import {
    UTC_RANGE_PRESETS,
    buildUtcIsoFromParts,
    calendarDateFromUtcIso,
    resolveUtcRangePreset,
    timeTextFromUtcIso,
    validateUtcRange,
    type UtcRangePresetKey,
  } from '../lib/utc-date-range'

  interface ConstraintProps {
    minIso?: string | null
    maxIso?: string | null
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
    startValue?: string
    endValue?: string
    disabled?: boolean
    presetAnchorIso?: string | null
    showPresets?: boolean
    showValidation?: boolean
  }

  let {
    startValue = $bindable(''),
    endValue = $bindable(''),
    disabled = false,
    presetAnchorIso = null,
    minIso = null,
    maxIso = null,
    timeframeDurationMs = null,
    maxIntervals = null,
    showPresets = true,
    showValidation = false,
    requiredStartMessage = 'UTC start is required.',
    requiredEndMessage = 'UTC end is required.',
    invalidStartMessage = 'UTC start must be a valid ISO-8601 timestamp.',
    invalidEndMessage = 'UTC end must be a valid ISO-8601 timestamp.',
    notEarlierMessage = 'UTC start must be earlier than UTC end.',
    outOfBoundsMessage = undefined,
    maxIntervalsMessage = undefined,
  }: Props = $props()

  let startTime = $state('00:00:00')
  let endTime = $state('00:00:00')
  let startDateText = $state('')
  let endDateText = $state('')
  let lastPresetMessage = $state<string | null>(null)

  $effect(() => {
    startTime = timeTextFromUtcIso(startValue)
    startDateText = calendarDateFromUtcIso(startValue)?.toString() ?? ''
  })

  $effect(() => {
    endTime = timeTextFromUtcIso(endValue)
    endDateText = calendarDateFromUtcIso(endValue)?.toString() ?? ''
  })

  const calendarValue = $derived.by(() => {
    const values = [calendarDateFromUtcIso(startValue), calendarDateFromUtcIso(endValue)].filter(
      (value): value is CalendarDate => value !== null,
    )

    return values
  })

  const validationMessages = $derived(
    validateUtcRange({
      startIso: startValue,
      endIso: endValue,
      requiredStartMessage,
      requiredEndMessage,
      invalidStartMessage,
      invalidEndMessage,
      notEarlierMessage,
      minIso,
      maxIso,
      outOfBoundsMessage,
      timeframeDurationMs,
      maxIntervals,
      maxIntervalsMessage,
    }),
  )

  const picker = useDatePicker(() => ({
    selectionMode: 'range',
    inline: true,
    closeOnSelect: false,
    fixedWeeks: true,
    locale: 'en-CA',
    timeZone: 'UTC',
    value: calendarValue,
    min: calendarDateFromUtcIso(minIso ?? '') ?? undefined,
    max: calendarDateFromUtcIso(maxIso ?? '') ?? undefined,
    format: (value) => value.toString(),
    parse: (value) => parseCalendarInput(value),
    onValueChange: (details) => applyCalendarChange(details),
  }))

  function applyCalendarChange(details: DatePickerValueChangeDetails) {
    const nextStart = details.value[0] ?? null
    const nextEnd = details.value[1] ?? nextStart

    startValue = buildUtcIsoFromParts(nextStart, startTime, startValue)
    endValue = buildUtcIsoFromParts(nextEnd, endTime, endValue)
    lastPresetMessage = null
  }

  function handleStartTimeInput(value: string) {
    startTime = normalizeBrowserTime(value)
    startValue = buildUtcIsoFromParts(calendarDateFromUtcIso(startValue), startTime, startValue)
    lastPresetMessage = null
  }

  function handleEndTimeInput(value: string) {
    endTime = normalizeBrowserTime(value)
    endValue = buildUtcIsoFromParts(calendarDateFromUtcIso(endValue), endTime, endValue)
    lastPresetMessage = null
  }

  function handleStartDateInput(value: string) {
    startDateText = value
    startValue = buildUtcIsoFromParts(parseCalendarInput(value) ?? null, startTime, startValue)
    lastPresetMessage = null
  }

  function handleEndDateInput(value: string) {
    endDateText = value
    endValue = buildUtcIsoFromParts(parseCalendarInput(value) ?? null, endTime, endValue)
    lastPresetMessage = null
  }

  function applyPreset(presetKey: UtcRangePresetKey) {
    const nextRange = resolveUtcRangePreset({ presetKey, anchorIso: presetAnchorIso, minIso, maxIso })
    startValue = nextRange.startIso
    endValue = nextRange.endIso
    startTime = timeTextFromUtcIso(startValue)
    endTime = timeTextFromUtcIso(endValue)
    startDateText = calendarDateFromUtcIso(startValue)?.toString() ?? ''
    endDateText = calendarDateFromUtcIso(endValue)?.toString() ?? ''
    lastPresetMessage = nextRange.clamped
      ? 'Preset was clamped to the allowed UTC range.'
      : `Preset resolved once from ${endValue}.`
  }

  function parseCalendarInput(value: string) {
    const trimmed = value.trim()
    const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(trimmed)
    if (!match) {
      return undefined
    }

    const year = Number(match[1])
    const month = Number(match[2])
    const day = Number(match[3])
    if (month < 1 || month > 12 || day < 1 || day > 31) {
      return undefined
    }

    return new CalendarDate(year, month, day)
  }

  function normalizeBrowserTime(value: string): string {
    return value.length === 5 ? `${value}:00` : value
  }
</script>

<div class="utc-range-picker">
  {#if showPresets}
    <div class="utc-range-picker__presets" aria-label="UTC range presets">
      {#each UTC_RANGE_PRESETS as preset (preset.key)}
        <button
          class="secondary utc-range-picker__preset"
          type="button"
          disabled={disabled}
          onclick={() => applyPreset(preset.key)}
        >
          {preset.label}
        </button>
      {/each}
    </div>
  {/if}

  <DatePicker.RootProvider value={picker}>
    <div class="utc-range-picker__inputs">
      <label>
        <span>UTC start date</span>
        <input
          class="utc-range-picker__date-input"
          type="date"
          value={startDateText}
          disabled={disabled}
          oninput={(event) => handleStartDateInput((event.currentTarget as HTMLInputElement).value)}
        />
      </label>
      <label>
        <span>UTC start time</span>
        <input
          class="utc-range-picker__time-input"
          type="time"
          step="1"
          value={startTime}
          disabled={disabled}
          oninput={(event) => handleStartTimeInput((event.currentTarget as HTMLInputElement).value)}
        />
      </label>
      <label>
        <span>UTC end date</span>
        <input
          class="utc-range-picker__date-input"
          type="date"
          value={endDateText}
          disabled={disabled}
          oninput={(event) => handleEndDateInput((event.currentTarget as HTMLInputElement).value)}
        />
      </label>
      <label>
        <span>UTC end time</span>
        <input
          class="utc-range-picker__time-input"
          type="time"
          step="1"
          value={endTime}
          disabled={disabled}
          oninput={(event) => handleEndTimeInput((event.currentTarget as HTMLInputElement).value)}
        />
      </label>
    </div>

    <div class="utc-range-picker__calendar-wrap">
      <DatePicker.View view="day">
        <div class="utc-range-picker__calendar-header">
          <DatePicker.PrevTrigger class="secondary utc-range-picker__nav-button">←</DatePicker.PrevTrigger>
          <DatePicker.ViewTrigger class="secondary utc-range-picker__nav-button">
            <DatePicker.RangeText />
          </DatePicker.ViewTrigger>
          <DatePicker.NextTrigger class="secondary utc-range-picker__nav-button">→</DatePicker.NextTrigger>
        </div>

        <DatePicker.Table class="utc-range-picker__calendar-table">
          <DatePicker.TableHead>
            <DatePicker.TableRow>
              {#each picker().weekDays as weekDay, weekDayIndex (`${weekDay.long}-${weekDayIndex}`)}
                <DatePicker.TableHeader>{weekDay.narrow}</DatePicker.TableHeader>
              {/each}
            </DatePicker.TableRow>
          </DatePicker.TableHead>
          <DatePicker.TableBody>
            {#each picker().weeks as week, weekIndex (`week-${weekIndex}`)}
              <DatePicker.TableRow>
                {#each week as day (`${day.year}-${day.month}-${day.day}`)}
                  <DatePicker.TableCell value={day}>
                    <DatePicker.TableCellTrigger class="utc-range-picker__day-button">
                      {day.day}
                    </DatePicker.TableCellTrigger>
                  </DatePicker.TableCell>
                {/each}
              </DatePicker.TableRow>
            {/each}
          </DatePicker.TableBody>
        </DatePicker.Table>
      </DatePicker.View>
    </div>
  </DatePicker.RootProvider>

  <div class="utc-range-picker__resolved" aria-label="Resolved UTC range">
    <p>Half-open UTC range. End remains exclusive.</p>
    <dl>
      <div>
        <dt>UTC start</dt>
        <dd><code>{startValue || '—'}</code></dd>
      </div>
      <div>
        <dt>UTC end</dt>
        <dd><code>{endValue || '—'}</code></dd>
      </div>
    </dl>
    {#if lastPresetMessage}
      <p class="utc-range-picker__preset-note">{lastPresetMessage}</p>
    {/if}
  </div>

  {#if showValidation && validationMessages.length > 0}
    <div class="alert utc-range-picker__validation" role="alert">
      <ul>
        {#each validationMessages as message (message)}
          <li>{message}</li>
        {/each}
      </ul>
    </div>
  {/if}
</div>

<style>
  .utc-range-picker,
  .utc-range-picker__inputs,
  .utc-range-picker__resolved,
  .utc-range-picker__calendar-wrap {
    display: flex;
    flex-direction: column;
    gap: var(--space-16);
  }

  .utc-range-picker__presets,
  .utc-range-picker__inputs,
  .utc-range-picker__calendar-header,
  .utc-range-picker__resolved dl {
    display: grid;
    gap: var(--space-12);
  }

  .utc-range-picker__presets {
    grid-template-columns: repeat(auto-fit, minmax(7rem, 1fr));
  }

  .utc-range-picker__inputs {
    grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  }

  .utc-range-picker__calendar-wrap,
  .utc-range-picker__resolved {
    padding: var(--space-16);
    border: 1px solid var(--border);
    border-radius: var(--radius-default);
    background: var(--surface-raised);
    color: var(--text-on-raised);
  }

  .utc-range-picker__calendar-header {
    grid-template-columns: auto 1fr auto;
    align-items: center;
  }

  :global(.utc-range-picker__calendar-table) {
    width: 100%;
    max-width: 100%;
    border-collapse: collapse;
    table-layout: fixed;
  }

  :global(.utc-range-picker__calendar-table th),
  :global(.utc-range-picker__calendar-table td) {
    padding: var(--space-4);
    text-align: center;
  }

  :global(.utc-range-picker__day-button),
  :global(.utc-range-picker__date-input),
  .utc-range-picker__time-input {
    width: 100%;
    box-sizing: border-box;
    font: inherit;
    border-radius: var(--radius-input);
    border: 1px solid var(--border);
    background: var(--color-input-bg);
    color: var(--color-input-text);
  }

  :global(.utc-range-picker__date-input),
  .utc-range-picker__time-input {
    padding: var(--space-12);
  }

  :global(.utc-range-picker__day-button) {
    padding: var(--space-8);
    cursor: pointer;
  }

  :global(.utc-range-picker__day-button[data-selected]),
  :global(.utc-range-picker__day-button[data-in-range]) {
    background: var(--accent-bg);
    border-color: var(--accent-border);
    color: var(--text-h);
  }

  :global(.utc-range-picker__day-button[data-today]) {
    outline: 1px solid var(--accent);
    outline-offset: -1px;
  }

  :global(.utc-range-picker__day-button[data-outside-range]),
  :global(.utc-range-picker__day-button[data-disabled]) {
    opacity: 0.45;
    cursor: not-allowed;
  }

  .utc-range-picker__resolved dl {
    grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
    margin: 0;
  }

  .utc-range-picker__resolved dt {
    font-weight: 700;
    margin-bottom: var(--space-8);
  }

  .utc-range-picker__resolved dd {
    margin: 0;
    word-break: break-word;
  }

  .utc-range-picker__preset-note {
    color: var(--text-on-raised);
  }

  .utc-range-picker__validation ul {
    margin: 0;
    padding-left: var(--space-20);
  }

  label {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    font-weight: 500;
  }
 </style>
