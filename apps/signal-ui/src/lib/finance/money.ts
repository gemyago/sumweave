const majorAmountPattern = /^([+-]?)(\d+)(?:\.(\d{1,2}))?$/

/** Converts a user-entered major-unit decimal to an exact, safe minor-unit integer. */
export function parseMajorAmountToMinor(value: string): number {
  const match = majorAmountPattern.exec(value)
  if (!match) {
    throw new TypeError('Amount must be a whole number or have at most two decimal places.')
  }

  const [, sign, wholePart, fractionalPart = ''] = match
  const magnitude = BigInt(wholePart) * 100n + BigInt(fractionalPart.padEnd(2, '0') || '0')
  const signed = sign === '-' ? -magnitude : magnitude

  if (signed > BigInt(Number.MAX_SAFE_INTEGER) || signed < BigInt(Number.MIN_SAFE_INTEGER)) {
    throw new TypeError('Amount is outside the supported range.')
  }

  return Number(signed)
}

/** Formats a minor-unit API value as a decimal string without float conversion. */
export function formatMinorAmountForInput(minor: number): string {
  if (!Number.isSafeInteger(minor)) {
    throw new TypeError('Amount is outside the supported range.')
  }

  const value = BigInt(minor)
  const magnitude = value < 0n ? -value : value
  const whole = magnitude / 100n
  const fractional = (magnitude % 100n).toString().padStart(2, '0')
  return `${value < 0n ? '-' : ''}${whole.toString()}.${fractional}`
}
