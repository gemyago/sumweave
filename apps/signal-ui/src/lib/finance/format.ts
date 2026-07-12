export function formatFinanceDate(value: Date | null | undefined): string {
  return formatFinanceWithOptions(value, { dateStyle: 'medium' })
}

export function formatFinanceDateTime(value: Date | null | undefined): string {
  return formatFinanceWithOptions(value, { dateStyle: 'medium', timeStyle: 'short' })
}

export function formatFinanceMoney(minor: number | null | undefined, currency: string): string {
  if (minor === null || minor === undefined) {
    return '—'
  }
  return `${(minor / 100).toFixed(2)} ${currency}`
}

function formatFinanceWithOptions(
  value: Date | null | undefined,
  options: Intl.DateTimeFormatOptions,
): string {
  if (!value || Number.isNaN(value.getTime())) {
    return '—'
  }

  return new Intl.DateTimeFormat(undefined, options).format(value)
}
