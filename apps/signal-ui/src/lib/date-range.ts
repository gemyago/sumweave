export type RangePresetKey =
  | 'last-24h'
  | 'last-7d'
  | 'last-30d'
  | 'last-90d'
  | 'last-180d'

export interface RangePresetDefinition {
  key: RangePresetKey
  label: string
  durationMs: number
}

export const RANGE_PRESETS: RangePresetDefinition[] = [
  { key: 'last-24h', label: 'Last 24h', durationMs: 24 * 60 * 60 * 1000 },
  { key: 'last-7d', label: 'Last 7d', durationMs: 7 * 24 * 60 * 60 * 1000 },
  { key: 'last-30d', label: 'Last 30d', durationMs: 30 * 24 * 60 * 60 * 1000 },
  { key: 'last-90d', label: 'Last 90d', durationMs: 90 * 24 * 60 * 60 * 1000 },
  { key: 'last-180d', label: 'Last 180d', durationMs: 180 * 24 * 60 * 60 * 1000 },
]

export interface ValidateRangeParams {
  start?: Date | null
  end?: Date | null
  requiredStartMessage: string
  requiredEndMessage: string
  invalidStartMessage: string
  invalidEndMessage: string
  notEarlierMessage: string
  min?: Date | null
  max?: Date | null
  outOfBoundsMessage?: string
  timeframeDurationMs?: number | null
  maxIntervals?: number | null
  maxIntervalsMessage?: string
}

export function parseTimestamp(value: string): Date | null {
  const trimmed = value.trim()
  if (!trimmed || !/(Z|[+-]\d{2}:\d{2})$/.test(trimmed)) return null
  const date = new Date(trimmed)
  return validDate(date) ? date : null
}

export function dateInputValue(value?: Date | null): string {
  if (!validDate(value)) return ''
  return [value.getFullYear(), value.getMonth() + 1, value.getDate()]
    .map((part, index) => String(part).padStart(index === 0 ? 4 : 2, '0'))
    .join('-')
}

export function timeInputValue(value?: Date | null): string {
  if (!validDate(value)) return ''
  return [value.getHours(), value.getMinutes(), value.getSeconds()]
    .map((part) => String(part).padStart(2, '0'))
    .join(':')
}

export function withDateInput(existing: Date | undefined, value: string): Date | undefined {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  if (!match) return undefined
  const year = Number(match[1])
  const month = Number(match[2]) - 1
  const day = Number(match[3])
  const next = validDate(existing) ? new Date(existing) : new Date(0)
  if (!validDate(existing)) next.setHours(0, 0, 0, 0)
  next.setFullYear(year, month, day)
  return next.getFullYear() === year && next.getMonth() === month && next.getDate() === day
    ? next
    : undefined
}

export function withTimeInput(existing: Date | undefined, value: string): Date | undefined {
  if (!validDate(existing)) return undefined
  const match = /^(\d{2}):(\d{2})(?::(\d{2}))?$/.exec(value)
  if (!match) return undefined
  const hour = Number(match[1])
  const minute = Number(match[2])
  const second = Number(match[3] ?? '0')
  if (hour > 23 || minute > 59 || second > 59) return undefined
  const next = new Date(existing)
  next.setHours(hour, minute, second)
  return next
}

export function resolveRangePreset(params: {
  presetKey: RangePresetKey
  anchor?: Date | null
  min?: Date | null
  max?: Date | null
}): { start: Date; end: Date; clamped: boolean } {
  const preset = RANGE_PRESETS.find((item) => item.key === params.presetKey)
  if (!preset) throw new Error(`Unknown range preset: ${params.presetKey}`)

  const anchor = validDate(params.anchor) ? new Date(params.anchor) : new Date()
  let start = new Date(anchor.getTime() - preset.durationMs)
  let end = new Date(anchor)
  let clamped = false

  if (validDate(params.min) && start < params.min) {
    start = new Date(params.min)
    clamped = true
  }
  if (validDate(params.min) && end < params.min) {
    end = new Date(params.min)
    clamped = true
  }
  if (validDate(params.max) && end > params.max) {
    end = new Date(params.max)
    clamped = true
  }
  if (validDate(params.max) && start > params.max) {
    start = new Date(params.max)
    clamped = true
  }
  return { start, end, clamped }
}

export function validateRange(params: ValidateRangeParams): string[] {
  const errors: string[] = []
  const start = validDate(params.start) ? params.start : null
  const end = validDate(params.end) ? params.end : null
  const startValid = start !== null
  const endValid = end !== null
  if (params.start == null) errors.push(params.requiredStartMessage)
  else if (!startValid) errors.push(params.invalidStartMessage)
  if (params.end == null) errors.push(params.requiredEndMessage)
  else if (!endValid) errors.push(params.invalidEndMessage)
  if (start && end && start >= end) errors.push(params.notEarlierMessage)
  if (
    start &&
    end &&
    params.outOfBoundsMessage &&
    ((validDate(params.min) && (start < params.min || end < params.min)) ||
      (validDate(params.max) && (start > params.max || end > params.max)))
  ) {
    errors.push(params.outOfBoundsMessage)
  }
  if (
    start &&
    end &&
    params.timeframeDurationMs &&
    params.maxIntervals &&
    params.maxIntervalsMessage &&
    end.getTime() - start.getTime() >
      params.timeframeDurationMs * params.maxIntervals
  ) {
    errors.push(params.maxIntervalsMessage)
  }
  return errors
}

function validDate(value?: Date | null): value is Date {
  return value instanceof Date && !Number.isNaN(value.getTime())
}
