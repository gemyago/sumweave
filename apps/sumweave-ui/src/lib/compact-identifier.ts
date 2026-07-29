export function formatCompactIdentifier(value: string, options?: { start?: number; end?: number }): string {
  const trimmed = value.trim()
  if (!trimmed) {
    return trimmed
  }

  const start = options?.start ?? 10
  const end = options?.end ?? 8

  if (trimmed.length <= start + end + 3) {
    return trimmed
  }

  return `${trimmed.slice(0, start)}...${trimmed.slice(-end)}`
}
