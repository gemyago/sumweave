export function dateInputValue(value?: Date | null): string {
  if (!(value instanceof Date) || Number.isNaN(value.getTime())) return ''
  return [value.getFullYear(), value.getMonth() + 1, value.getDate()].map((part, index) => String(part).padStart(index === 0 ? 4 : 2, '0')).join('-')
}

export function withDateInput(existing: Date | undefined, value: string): Date | undefined {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  if (!match) return undefined
  const next = existing instanceof Date && !Number.isNaN(existing.getTime()) ? new Date(existing) : new Date(0)
  next.setFullYear(Number(match[1]), Number(match[2]) - 1, Number(match[3]))
  return next.getMonth() === Number(match[2]) - 1 && next.getDate() === Number(match[3]) ? next : undefined
}
