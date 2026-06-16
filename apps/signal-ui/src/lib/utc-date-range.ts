import { CalendarDate, type DateValue } from '@internationalized/date'

export type UtcRangePresetKey =
  | 'last-24h'
  | 'last-7d'
  | 'last-30d'
  | 'last-90d'
  | 'last-180d'

export interface UtcRangePresetDefinition {
  key: UtcRangePresetKey
  label: string
  durationMs: number
}

export const UTC_RANGE_PRESETS: UtcRangePresetDefinition[] = [
  { key: 'last-24h', label: 'Last 24h', durationMs: 24 * 60 * 60 * 1000 },
  { key: 'last-7d', label: 'Last 7d', durationMs: 7 * 24 * 60 * 60 * 1000 },
  { key: 'last-30d', label: 'Last 30d', durationMs: 30 * 24 * 60 * 60 * 1000 },
  { key: 'last-90d', label: 'Last 90d', durationMs: 90 * 24 * 60 * 60 * 1000 },
  { key: 'last-180d', label: 'Last 180d', durationMs: 180 * 24 * 60 * 60 * 1000 },
]

export interface ValidateUtcRangeParams {
  startIso: string
  endIso: string
  requiredStartMessage: string
  requiredEndMessage: string
  invalidStartMessage: string
  invalidEndMessage: string
  notEarlierMessage: string
  minIso?: string | null
  maxIso?: string | null
  outOfBoundsMessage?: string
  timeframeDurationMs?: number | null
  maxIntervals?: number | null
  maxIntervalsMessage?: string
}

export function parseUtcTimestamp(value: string): Date | null {
  const trimmed = value.trim()
  if (!trimmed) {
    return null
  }

  if (!/(Z|[+-]\d{2}:\d{2})$/.test(trimmed)) {
    return null
  }

  const date = new Date(trimmed)
  return Number.isNaN(date.getTime()) ? null : date
}

export function calendarDateFromUtcIso(value: string): CalendarDate | null {
  const parsed = parseUtcTimestamp(value)
  if (!parsed) {
    return null
  }

  return new CalendarDate(parsed.getUTCFullYear(), parsed.getUTCMonth() + 1, parsed.getUTCDate())
}

export function timeTextFromUtcIso(value: string, fallback = '00:00:00'): string {
  const parsed = parseUtcTimestamp(value)
  if (!parsed) {
    return fallback
  }

  return [parsed.getUTCHours(), parsed.getUTCMinutes(), parsed.getUTCSeconds()]
    .map((part) => String(part).padStart(2, '0'))
    .join(':')
}

export function buildUtcIsoFromParts(
  dateValue: DateValue | null | undefined,
  timeText: string,
  existingIso = '',
): string {
  if (!dateValue) {
    return ''
  }

  const normalizedTime = normalizeTimeText(timeText)
  if (!normalizedTime) {
    return ''
  }

  const [hour, minute, second] = normalizedTime.split(':').map(Number)
  const milliseconds = parseUtcTimestamp(existingIso)?.getUTCMilliseconds() ?? 0

  return new Date(
    Date.UTC(dateValue.year, dateValue.month - 1, dateValue.day, hour ?? 0, minute ?? 0, second ?? 0, milliseconds),
  ).toISOString()
}

export function resolveUtcRangePreset(params: {
  presetKey: UtcRangePresetKey
  anchorIso?: string | null
  minIso?: string | null
  maxIso?: string | null
}): { startIso: string; endIso: string; clamped: boolean } {
  const preset = UTC_RANGE_PRESETS.find((item) => item.key === params.presetKey)
  if (!preset) {
    throw new Error(`Unknown UTC range preset: ${params.presetKey}`)
  }

  const anchor = parseUtcTimestamp(params.anchorIso ?? '') ?? new Date()
  let start = new Date(anchor.getTime() - preset.durationMs)
  let end = new Date(anchor)
  let clamped = false

  const min = parseUtcTimestamp(params.minIso ?? '')
  const max = parseUtcTimestamp(params.maxIso ?? '')

  if (min && start < min) {
    start = new Date(min)
    clamped = true
  }

  if (min && end < min) {
    end = new Date(min)
    clamped = true
  }

  if (max && end > max) {
    end = new Date(max)
    clamped = true
  }

  if (max && start > max) {
    start = new Date(max)
    clamped = true
  }

  return {
    startIso: start.toISOString(),
    endIso: end.toISOString(),
    clamped,
  }
}

export function validateUtcRange(params: ValidateUtcRangeParams): string[] {
  const errors: string[] = []
  const start = parseUtcTimestamp(params.startIso)
  const end = parseUtcTimestamp(params.endIso)

  if (!params.startIso.trim()) errors.push(params.requiredStartMessage)
  if (!params.endIso.trim()) errors.push(params.requiredEndMessage)
  if (params.startIso.trim() && !start) errors.push(params.invalidStartMessage)
  if (params.endIso.trim() && !end) errors.push(params.invalidEndMessage)

  if (start && end && start >= end) {
    errors.push(params.notEarlierMessage)
  }

  const min = parseUtcTimestamp(params.minIso ?? '')
  const max = parseUtcTimestamp(params.maxIso ?? '')
  if (
    start &&
    end &&
    params.outOfBoundsMessage &&
    ((min && (start < min || end < min)) || (max && (start > max || end > max)))
  ) {
    errors.push(params.outOfBoundsMessage)
  }

  if (
    start &&
    end &&
    params.timeframeDurationMs &&
    params.maxIntervals &&
    params.maxIntervalsMessage &&
    end.getTime() - start.getTime() > params.timeframeDurationMs * params.maxIntervals
  ) {
    errors.push(params.maxIntervalsMessage)
  }

  return errors
}

function normalizeTimeText(value: string): string | null {
  const trimmed = value.trim()
  const match = /^(\d{2}):(\d{2})(?::(\d{2}))?$/.exec(trimmed)
  if (!match) {
    return null
  }

  const hour = Number(match[1])
  const minute = Number(match[2])
  const second = Number(match[3] ?? '00')
  if (hour > 23 || minute > 59 || second > 59) {
    return null
  }

  return [hour, minute, second].map((part) => String(part).padStart(2, '0')).join(':')
}
