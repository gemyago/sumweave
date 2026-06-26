export function formatFinanceDate(value: Date | null): string {
  return formatFinanceWithOptions(value, { dateStyle: 'medium', timeZone: 'UTC' })
}

export function formatFinanceDateTime(value: Date | null): string {
  return formatFinanceWithOptions(value, { dateStyle: 'medium', timeStyle: 'short' })
}

export function formatFinanceMoney(minor: number | null | undefined, currency: string): string {
  if (minor === null || minor === undefined) {
    return '—'
  }
  return `${(minor / 100).toFixed(2)} ${currency}`
}

function formatFinanceWithOptions(
  value: Date | null,
  options: Intl.DateTimeFormatOptions,
): string {
  if (!value || Number.isNaN(value.getTime()) || value.getUTCFullYear() <= 1) {
    return '—'
  }

  return new Intl.DateTimeFormat(undefined, options).format(value)
}
