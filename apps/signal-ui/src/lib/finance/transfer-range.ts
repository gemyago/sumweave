export interface TransferCandidateRange {
  effectiveFrom: Date
  effectiveBefore: Date
  effectiveFromDate: string
  effectiveBeforeDate: string
}

export function defaultTransferCandidateRange(effectiveAt: Date): TransferCandidateRange {
  const start = localStartOfDay(effectiveAt)
  const effectiveFrom = addLocalDays(start, -2)
  const effectiveBefore = addLocalDays(start, 3)
  return {
    effectiveFrom,
    effectiveBefore,
    effectiveFromDate: localDateValue(effectiveFrom),
    effectiveBeforeDate: localDateValue(effectiveBefore),
  }
}

export function transferCandidateRangeFromDateInputs(
  effectiveFromDate: string,
  effectiveBeforeDate: string,
): TransferCandidateRange {
  const effectiveFrom = localDateStart(effectiveFromDate)
  const effectiveBefore = localDateStart(effectiveBeforeDate)
  if (effectiveFrom >= effectiveBefore) {
    throw new TypeError('Effective before must be after effective from.')
  }
  return { effectiveFrom, effectiveBefore, effectiveFromDate, effectiveBeforeDate }
}

function localStartOfDay(value: Date): Date {
  return new Date(value.getFullYear(), value.getMonth(), value.getDate())
}

function addLocalDays(value: Date, days: number): Date {
  const result = new Date(value)
  result.setDate(result.getDate() + days)
  return result
}

function localDateValue(value: Date): string {
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${value.getFullYear()}-${pad(value.getMonth() + 1)}-${pad(value.getDate())}`
}

function localDateStart(value: string): Date {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  if (!match) throw new TypeError('Enter a valid local calendar date.')
  const [year, month, day] = match.slice(1).map(Number)
  const parsed = new Date(year, month - 1, day)
  if (parsed.getFullYear() !== year || parsed.getMonth() !== month - 1 || parsed.getDate() !== day) {
    throw new TypeError('Enter a valid local calendar date.')
  }
  return parsed
}
